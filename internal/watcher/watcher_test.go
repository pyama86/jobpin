package watcher

import (
	"context"
	"errors"
	"testing"

	"github.com/pyama86/jobpin/internal/ghclient"
	"github.com/pyama86/jobpin/internal/notify"
	"github.com/pyama86/jobpin/internal/store"
	"github.com/slack-go/slack"
)

type stubStore struct {
	jobs          []*store.Job
	updatedID     string
	updatedStatus string
}

func (s *stubStore) Put(ctx context.Context, job *store.Job) error { return nil }

func (s *stubStore) ListWatching(ctx context.Context) ([]*store.Job, error) {
	return s.jobs, nil
}

func (s *stubStore) UpdateStatus(ctx context.Context, id, status string) error {
	s.updatedID = id
	s.updatedStatus = status
	return nil
}

type stubGH struct {
	run *ghclient.Run
	err error
}

func (g *stubGH) GetWorkflowRun(ctx context.Context, owner, repo string, runID int64) (*ghclient.Run, error) {
	return g.run, g.err
}

type stubPoster struct {
	posted  []string
	postErr error
}

func (p *stubPoster) PostMessageContext(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error) {
	if p.postErr != nil {
		return "", "", p.postErr
	}
	p.posted = append(p.posted, channelID)
	return channelID, "ts", nil
}

func newTestWatcher(t *testing.T, st *stubStore, gh *stubGH, poster *stubPoster) *Watcher {
	t.Helper()
	renderer, err := notify.NewRenderer("ok {{.Owner}}/{{.Repo}}", "ng {{.Conclusion}}")
	if err != nil {
		t.Fatal(err)
	}
	return &Watcher{st: st, gh: gh, slack: poster, renderer: renderer}
}

func testJob() *store.Job {
	return &store.Job{
		ID:       "pyama86/jobpin#1#C123#1234.5678",
		Owner:    "pyama86",
		Repo:     "jobpin",
		RunID:    1,
		Channel:  "C123",
		ThreadTS: "1234.5678",
		Status:   store.StatusWatching,
	}
}

func TestCheckAllCompleted(t *testing.T) {
	st := &stubStore{jobs: []*store.Job{testJob()}}
	gh := &stubGH{run: &ghclient.Run{Status: "completed", Conclusion: "success"}}
	poster := &stubPoster{}

	newTestWatcher(t, st, gh, poster).checkAll(context.Background())

	if len(poster.posted) != 1 || poster.posted[0] != "C123" {
		t.Errorf("expected 1 post to C123, got %v", poster.posted)
	}
	if st.updatedID != testJob().ID || st.updatedStatus != store.StatusNotified {
		t.Errorf("expected UpdateStatus(%q, notified), got (%q, %q)", testJob().ID, st.updatedID, st.updatedStatus)
	}
}

func TestCheckAllInProgress(t *testing.T) {
	st := &stubStore{jobs: []*store.Job{testJob()}}
	gh := &stubGH{run: &ghclient.Run{Status: "in_progress"}}
	poster := &stubPoster{}

	newTestWatcher(t, st, gh, poster).checkAll(context.Background())

	if len(poster.posted) != 0 {
		t.Errorf("expected no posts, got %v", poster.posted)
	}
	if st.updatedID != "" {
		t.Errorf("expected no UpdateStatus, got %q", st.updatedID)
	}
}

func TestCheckAllPostFailure(t *testing.T) {
	st := &stubStore{jobs: []*store.Job{testJob()}}
	gh := &stubGH{run: &ghclient.Run{Status: "completed", Conclusion: "failure"}}
	poster := &stubPoster{postErr: errors.New("slack down")}

	newTestWatcher(t, st, gh, poster).checkAll(context.Background())

	if st.updatedID != "" {
		t.Errorf("expected no UpdateStatus on post failure, got %q", st.updatedID)
	}
}

func TestCheckAllRunNotFound(t *testing.T) {
	st := &stubStore{jobs: []*store.Job{testJob()}}
	gh := &stubGH{err: errors.New("failed to get workflow run: 404 Not Found")}
	poster := &stubPoster{}

	newTestWatcher(t, st, gh, poster).checkAll(context.Background())

	if len(poster.posted) != 0 {
		t.Errorf("expected no posts, got %v", poster.posted)
	}
	if st.updatedStatus != store.StatusNotified {
		t.Errorf("expected UpdateStatus notified for 404, got %q", st.updatedStatus)
	}
}

func TestCheckAllGetRunError(t *testing.T) {
	st := &stubStore{jobs: []*store.Job{testJob()}}
	gh := &stubGH{err: errors.New("temporary network error")}
	poster := &stubPoster{}

	newTestWatcher(t, st, gh, poster).checkAll(context.Background())

	if st.updatedID != "" {
		t.Errorf("expected no UpdateStatus on transient error, got %q", st.updatedID)
	}
}
