package store

import (
	"context"
	"fmt"
	"time"
)

const (
	StatusWatching = "watching"
	StatusNotified = "notified"
)

type Job struct {
	ID        string    `dynamodbav:"id"`
	Owner     string    `dynamodbav:"owner"`
	Repo      string    `dynamodbav:"repo"`
	RunID     int64     `dynamodbav:"run_id"`
	Channel   string    `dynamodbav:"channel"`
	ThreadTS  string    `dynamodbav:"thread_ts"`
	Requester string    `dynamodbav:"requester"`
	Note      string    `dynamodbav:"note"`
	Status    string    `dynamodbav:"status"`
	CreatedAt time.Time `dynamodbav:"created_at"`
	ExpiresAt int64     `dynamodbav:"expires_at"`
}

type Store interface {
	Put(ctx context.Context, job *Job) error
	ListWatching(ctx context.Context) ([]*Job, error)
	UpdateStatus(ctx context.Context, id, status string) error
}

func NewJobID(owner, repo string, runID int64, channel, threadTS string) string {
	return fmt.Sprintf("%s/%s#%d#%s#%s", owner, repo, runID, channel, threadTS)
}
