package config

import (
	"os"
	"testing"
	"time"
)

// t.Setenvで復元を登録した上でunsetし、実行環境の値がテストに漏れないようにする
func unsetenv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		t.Setenv(key, os.Getenv(key))
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("failed to unset %s: %v", key, err)
		}
	}
}

func setupEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-test")
	t.Setenv("SLACK_APP_TOKEN", "xapp-test")
	unsetenv(t,
		"GITHUB_TOKEN",
		"GITHUB_APP_ID",
		"GITHUB_APP_PRIVATE_KEY_PATH",
		"DYNAMODB_TABLE",
		"DYNAMODB_ENDPOINT",
		"POLL_INTERVAL",
		"WATCH_TTL",
		"ACK_REACTION",
		"NOTIFY_TEMPLATE_SUCCESS",
		"NOTIFY_TEMPLATE_FAILURE",
	)
}

func TestLoadWithPAT(t *testing.T) {
	setupEnv(t)
	t.Setenv("GITHUB_TOKEN", "ghp_test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GitHubToken != "ghp_test" {
		t.Errorf("GitHubToken = %q, want %q", cfg.GitHubToken, "ghp_test")
	}
}

func TestLoadWithGitHubApp(t *testing.T) {
	setupEnv(t)
	t.Setenv("GITHUB_APP_ID", "12345")
	t.Setenv("GITHUB_APP_PRIVATE_KEY_PATH", "/path/to/key.pem")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GitHubAppID != 12345 {
		t.Errorf("GitHubAppID = %d, want 12345", cfg.GitHubAppID)
	}
}

func TestLoadWithoutGitHubAuth(t *testing.T) {
	setupEnv(t)

	if _, err := Load(); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadDefaults(t *testing.T) {
	setupEnv(t)
	t.Setenv("GITHUB_TOKEN", "ghp_test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.PollInterval != 30*time.Second {
		t.Errorf("PollInterval = %v, want 30s", cfg.PollInterval)
	}
	if cfg.WatchTTL != 168*time.Hour {
		t.Errorf("WatchTTL = %v, want 168h", cfg.WatchTTL)
	}
	if cfg.AckReaction != "eyes" {
		t.Errorf("AckReaction = %q, want %q", cfg.AckReaction, "eyes")
	}
	if cfg.DynamoDBTable != "jobpin" {
		t.Errorf("DynamoDBTable = %q, want %q", cfg.DynamoDBTable, "jobpin")
	}
	if cfg.NotifyTemplateSuccess != defaultNotifyTemplateSuccess {
		t.Errorf("NotifyTemplateSuccess = %q, want default", cfg.NotifyTemplateSuccess)
	}
	if cfg.NotifyTemplateFailure != defaultNotifyTemplateFailure {
		t.Errorf("NotifyTemplateFailure = %q, want default", cfg.NotifyTemplateFailure)
	}
}
