package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/pyama86/jobpin/internal/bot"
	"github.com/pyama86/jobpin/internal/config"
	"github.com/pyama86/jobpin/internal/ghclient"
	"github.com/pyama86/jobpin/internal/notify"
	"github.com/pyama86/jobpin/internal/store"
	"github.com/pyama86/jobpin/internal/watcher"
	"github.com/slack-go/slack"
	"golang.org/x/sync/errgroup"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("jobpin exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	st, err := store.NewDynamoDB(ctx, cfg.DynamoDBTable, cfg.DynamoDBEndpoint)
	if err != nil {
		return err
	}
	// DescribeTable/CreateTable権限がない環境(テーブルはTerraform等で管理)でも起動を継続する
	if err := st.EnsureTable(ctx); err != nil {
		slog.Warn("failed to ensure table, assuming it is managed externally", "error", err)
	}

	gh, err := ghclient.New(cfg)
	if err != nil {
		return err
	}

	renderer, err := notify.NewRenderer(cfg.NotifyTemplateSuccess, cfg.NotifyTemplateFailure, cfg.NotifyTemplateTimeout)
	if err != nil {
		return err
	}

	b := bot.New(cfg, st, gh, renderer)
	slackAPI := slack.New(cfg.SlackBotToken)
	w := watcher.New(cfg, st, gh, slackAPI, renderer)

	slog.Info("starting jobpin",
		"dynamodb_table", cfg.DynamoDBTable,
		"dynamodb_endpoint", cfg.DynamoDBEndpoint,
		"poll_interval", cfg.PollInterval.String(),
		"watch_ttl", cfg.WatchTTL.String(),
		"ack_reaction", cfg.AckReaction,
	)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return b.Run(ctx) })
	g.Go(func() error { return w.Run(ctx) })
	return g.Wait()
}
