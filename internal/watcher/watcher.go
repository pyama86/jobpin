package watcher

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v90/github"
	"github.com/pyama86/jobpin/internal/config"
	"github.com/pyama86/jobpin/internal/ghclient"
	"github.com/pyama86/jobpin/internal/notify"
	"github.com/pyama86/jobpin/internal/store"
	"github.com/slack-go/slack"
)

type SlackPoster interface {
	PostMessageContext(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error)
}

type Watcher struct {
	interval         time.Duration
	maxWatchDuration time.Duration
	st               store.Store
	gh               ghclient.Client
	slack            SlackPoster
	renderer         *notify.Renderer
}

func New(cfg *config.Config, st store.Store, gh ghclient.Client, slackClient SlackPoster, renderer *notify.Renderer) *Watcher {
	return &Watcher{
		interval:         cfg.PollInterval,
		maxWatchDuration: cfg.MaxWatchDuration,
		st:               st,
		gh:               gh,
		slack:            slackClient,
		renderer:         renderer,
	}
}

func (w *Watcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.checkAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.checkAll(ctx)
		}
	}
}

func (w *Watcher) checkAll(ctx context.Context) {
	jobs, err := w.st.ListWatching(ctx)
	if err != nil {
		slog.Warn("failed to list watching jobs", "error", err)
		return
	}
	for _, job := range jobs {
		w.checkJob(ctx, job)
	}
}

func (w *Watcher) checkJob(ctx context.Context, job *store.Job) {
	if time.Since(job.CreatedAt) > w.maxWatchDuration {
		w.notifyTimeout(ctx, job)
		return
	}

	run, err := w.gh.GetWorkflowRun(ctx, job.Owner, job.Repo, job.RunID)
	if err != nil {
		if isNotFound(err) {
			slog.Warn("workflow run not found, stop watching", "job", job.ID)
			if err := w.st.UpdateStatus(ctx, job.ID, store.StatusNotified); err != nil {
				slog.Warn("failed to update job status", "job", job.ID, "error", err)
			}
			return
		}
		slog.Warn("failed to get workflow run", "job", job.ID, "error", err)
		return
	}

	if run.Status != "completed" {
		return
	}

	if run.Conclusion == "cancelled" {
		replacement, err := w.gh.FindReplacementRun(ctx, job.Owner, job.Repo, run)
		if err != nil {
			slog.Warn("failed to find replacement workflow run", "job", job.ID, "error", err)
			return
		}
		if replacement != nil {
			cancelledRunID := job.RunID
			job.RunID = replacement.ID
			if err := w.st.Put(ctx, job); err != nil {
				job.RunID = cancelledRunID
				slog.Warn("failed to switch watched workflow run", "job", job.ID, "run_id", replacement.ID, "error", err)
				return
			}
			slog.Info("switched watched workflow run", "job", job.ID, "cancelled_run_id", cancelledRunID, "run_id", replacement.ID)
			return
		}
	}

	text, err := w.renderer.Render(job, run)
	if err != nil {
		slog.Warn("failed to render message", "job", job.ID, "error", err)
		return
	}

	if _, _, err := w.slack.PostMessageContext(ctx, job.Channel,
		slack.MsgOptionText(text, false), slack.MsgOptionTS(job.ThreadTS)); err != nil {
		slog.Warn("failed to post message", "job", job.ID, "error", err)
		return
	}

	if err := w.st.UpdateStatus(ctx, job.ID, store.StatusNotified); err != nil {
		slog.Warn("failed to update job status", "job", job.ID, "error", err)
	}
}

func (w *Watcher) notifyTimeout(ctx context.Context, job *store.Job) {
	text, err := w.renderer.RenderTimeout(job)
	if err != nil {
		slog.Warn("failed to render timeout message", "job", job.ID, "error", err)
		return
	}

	if _, _, err := w.slack.PostMessageContext(ctx, job.Channel,
		slack.MsgOptionText(text, false), slack.MsgOptionTS(job.ThreadTS)); err != nil {
		slog.Warn("failed to post timeout message", "job", job.ID, "error", err)
		return
	}

	if err := w.st.UpdateStatus(ctx, job.ID, store.StatusNotified); err != nil {
		slog.Warn("failed to update job status", "job", job.ID, "error", err)
	}
}

func isNotFound(err error) bool {
	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) && ghErr.Response != nil {
		return ghErr.Response.StatusCode == http.StatusNotFound
	}
	return strings.Contains(err.Error(), "404")
}
