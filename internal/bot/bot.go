package bot

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"time"

	"github.com/pyama86/jobpin/internal/config"
	"github.com/pyama86/jobpin/internal/ghclient"
	"github.com/pyama86/jobpin/internal/notify"
	"github.com/pyama86/jobpin/internal/store"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

const usageMessage = "使い方: `@jobpin <GitHub Actions run URL>` — run URLを渡すと完了時にこのスレッドへ通知します"

type RunRef struct {
	Owner string
	Repo  string
	RunID int64
}

// Slackは本文中のURLを `<url>` や `<url|label>` で囲むため `>` `|` をrepo名から除外する
var runURLRe = regexp.MustCompile(`https://github\.com/([^/\s]+)/([^/\s>|]+)/actions/runs/(\d+)`)

func parseRunURLs(text string) []RunRef {
	var refs []RunRef
	for _, m := range runURLRe.FindAllStringSubmatch(text, -1) {
		runID, err := strconv.ParseInt(m[3], 10, 64)
		if err != nil {
			continue
		}
		refs = append(refs, RunRef{Owner: m[1], Repo: m[2], RunID: runID})
	}
	return refs
}

type Bot struct {
	cfg      *config.Config
	st       store.Store
	gh       ghclient.Client
	renderer *notify.Renderer
	api      *slack.Client
	sm       *socketmode.Client
}

func New(cfg *config.Config, st store.Store, gh ghclient.Client, renderer *notify.Renderer) *Bot {
	api := slack.New(cfg.SlackBotToken, slack.OptionAppLevelToken(cfg.SlackAppToken))
	return &Bot{
		cfg:      cfg,
		st:       st,
		gh:       gh,
		renderer: renderer,
		api:      api,
		sm:       socketmode.New(api),
	}
}

func (b *Bot) Run(ctx context.Context) error {
	go func() {
		for evt := range b.sm.Events {
			b.handleEvent(ctx, &evt)
		}
	}()

	err := b.sm.RunContext(ctx)
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func (b *Bot) handleEvent(ctx context.Context, evt *socketmode.Event) {
	if evt.Type != socketmode.EventTypeEventsAPI {
		return
	}
	if evt.Request != nil {
		if err := b.sm.Ack(*evt.Request); err != nil {
			slog.Warn("failed to ack event", "error", err)
		}
	}

	apiEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}
	mention, ok := apiEvent.InnerEvent.Data.(*slackevents.AppMentionEvent)
	if !ok {
		return
	}
	b.handleMention(ctx, mention)
}

func (b *Bot) handleMention(ctx context.Context, mention *slackevents.AppMentionEvent) {
	threadTS := mention.ThreadTimeStamp
	if threadTS == "" {
		threadTS = mention.TimeStamp
	}

	refs := parseRunURLs(mention.Text)
	if len(refs) == 0 {
		b.reply(ctx, mention.Channel, threadTS, usageMessage)
		return
	}

	acked := false
	for _, ref := range refs {
		run, err := b.gh.GetWorkflowRun(ctx, ref.Owner, ref.Repo, ref.RunID)
		if err != nil {
			slog.Warn("failed to get workflow run", "owner", ref.Owner, "repo", ref.Repo, "run_id", ref.RunID, "error", err)
			b.reply(ctx, mention.Channel, threadTS,
				fmt.Sprintf("%s/%s の run %d を取得できませんでした。URLと権限を確認してください", ref.Owner, ref.Repo, ref.RunID))
			continue
		}

		job := &store.Job{
			ID:        store.NewJobID(ref.Owner, ref.Repo, ref.RunID, mention.Channel, threadTS),
			Owner:     ref.Owner,
			Repo:      ref.Repo,
			RunID:     ref.RunID,
			Channel:   mention.Channel,
			ThreadTS:  threadTS,
			Requester: mention.User,
			Status:    store.StatusWatching,
		}

		if run.Status == "completed" {
			text, err := b.renderer.Render(job, run)
			if err != nil {
				slog.Warn("failed to render message", "job", job.ID, "error", err)
				continue
			}
			b.reply(ctx, mention.Channel, threadTS, text)
			continue
		}

		now := time.Now()
		job.CreatedAt = now
		job.ExpiresAt = now.Add(b.cfg.WatchTTL).Unix()
		if err := b.st.Put(ctx, job); err != nil {
			slog.Warn("failed to put job", "job", job.ID, "error", err)
			b.reply(ctx, mention.Channel, threadTS,
				fmt.Sprintf("%s/%s の run %d の監視登録に失敗しました", ref.Owner, ref.Repo, ref.RunID))
			continue
		}

		if !acked {
			if err := b.api.AddReactionContext(ctx, b.cfg.AckReaction,
				slack.NewRefToMessage(mention.Channel, mention.TimeStamp)); err != nil {
				slog.Warn("failed to add reaction", "channel", mention.Channel, "error", err)
			}
			acked = true
		}
	}
}

func (b *Bot) reply(ctx context.Context, channel, threadTS, text string) {
	if _, _, err := b.api.PostMessageContext(ctx, channel,
		slack.MsgOptionText(text, false), slack.MsgOptionTS(threadTS)); err != nil {
		slog.Warn("failed to post message", "channel", channel, "error", err)
	}
}
