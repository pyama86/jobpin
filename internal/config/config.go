package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

const (
	defaultNotifyTemplateSuccess = "<@{{.Requester}}> :white_check_mark: *{{.WorkflowName}}* (#{{.RunNumber}}) が成功しました\n{{.RunURL}}"
	defaultNotifyTemplateFailure = "<@{{.Requester}}> :x: *{{.WorkflowName}}* (#{{.RunNumber}}) が {{.Conclusion}} で終了しました\n{{.RunURL}}"
	defaultNotifyTemplateTimeout = "<@{{.Requester}}> :hourglass_flowing_sand: {{.RunURL}} は監視期限を超えたため打ち切りました"
)

type Config struct {
	SlackBotToken           string        `envconfig:"SLACK_BOT_TOKEN" required:"true"`
	SlackAppToken           string        `envconfig:"SLACK_APP_TOKEN" required:"true"`
	GitHubToken             string        `envconfig:"GITHUB_TOKEN"`
	GitHubAppID             int64         `envconfig:"GITHUB_APP_ID"`
	GitHubAppPrivateKeyPath string        `envconfig:"GITHUB_APP_PRIVATE_KEY_PATH"`
	DynamoDBTable           string        `envconfig:"DYNAMODB_TABLE" default:"jobpin"`
	DynamoDBEndpoint        string        `envconfig:"DYNAMODB_ENDPOINT"`
	PollInterval            time.Duration `envconfig:"POLL_INTERVAL" default:"30s"`
	WatchTTL                time.Duration `envconfig:"WATCH_TTL" default:"168h"`
	MaxWatchDuration        time.Duration `envconfig:"MAX_WATCH_DURATION" default:"1h"`
	AckReaction             string        `envconfig:"ACK_REACTION" default:"eyes"`
	NotifyTemplateSuccess   string        `envconfig:"NOTIFY_TEMPLATE_SUCCESS"`
	NotifyTemplateFailure   string        `envconfig:"NOTIFY_TEMPLATE_FAILURE"`
	NotifyTemplateTimeout   string        `envconfig:"NOTIFY_TEMPLATE_TIMEOUT"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process env config: %w", err)
	}

	if cfg.NotifyTemplateSuccess == "" {
		cfg.NotifyTemplateSuccess = defaultNotifyTemplateSuccess
	}
	if cfg.NotifyTemplateFailure == "" {
		cfg.NotifyTemplateFailure = defaultNotifyTemplateFailure
	}
	if cfg.NotifyTemplateTimeout == "" {
		cfg.NotifyTemplateTimeout = defaultNotifyTemplateTimeout
	}

	if cfg.GitHubToken == "" && (cfg.GitHubAppID == 0 || cfg.GitHubAppPrivateKeyPath == "") {
		return nil, errors.New("GITHUB_TOKEN か GITHUB_APP_ID+GITHUB_APP_PRIVATE_KEY_PATH のどちらかが必要")
	}

	return &cfg, nil
}
