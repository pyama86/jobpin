package ghclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v90/github"
	"github.com/pyama86/jobpin/internal/config"
)

func TestNewWithPAT(t *testing.T) {
	c, err := New(&config.Config{GitHubToken: "ghp_test"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if _, ok := c.(*tokenClient); !ok {
		t.Fatalf("expected *tokenClient, got %T", c)
	}
}

func TestNewWithInvalidAppPrivateKey(t *testing.T) {
	_, err := New(&config.Config{
		GitHubAppID:         12345,
		GitHubAppPrivateKey: "invalid-pem-data",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTokenClientGetWorkflowRun(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/pyama86/jobpin/actions/runs/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"id":          42,
			"workflow_id": 10,
			"name":        "CI",
			"run_number":  7,
			"event":       "push",
			"status":      "completed",
			"conclusion":  "success",
			"html_url":    "https://github.com/pyama86/jobpin/actions/runs/42",
			"head_branch": "main",
			"created_at":  "2026-08-09T00:00:00Z",
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// v90ではBaseURLが構築後に変更できないため、WithURLsで組み立てたクライアントを注入する
	gh, err := github.NewClient(
		github.WithAuthToken("ghp_test"),
		github.WithURLs(&srv.URL, &srv.URL),
	)
	if err != nil {
		t.Fatalf("failed to create github client: %v", err)
	}
	c := &tokenClient{client: gh}

	run, err := c.GetWorkflowRun(context.Background(), "pyama86", "jobpin", 42)
	if err != nil {
		t.Fatalf("GetWorkflowRun failed: %v", err)
	}

	want := Run{
		ID:           42,
		WorkflowID:   10,
		WorkflowName: "CI",
		RunNumber:    7,
		Event:        "push",
		Status:       "completed",
		Conclusion:   "success",
		HTMLURL:      "https://github.com/pyama86/jobpin/actions/runs/42",
		Branch:       "main",
		CreatedAt:    time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC),
	}
	if *run != want {
		t.Errorf("Run = %+v, want %+v", *run, want)
	}
}

func TestTokenClientFindReplacementRun(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/pyama86/jobpin/actions/workflows/10/runs", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("branch"); got != "main" {
			t.Errorf("branch = %q, want main", got)
		}
		if got := r.URL.Query().Get("event"); got != "push" {
			t.Errorf("event = %q, want push", got)
		}
		if got := r.URL.Query().Get("per_page"); got != "100" {
			t.Errorf("per_page = %q, want 100", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"workflow_runs": []map[string]any{
				{"id": 44, "workflow_id": 10, "run_number": 9, "created_at": "2026-08-09T00:02:00Z"},
				{"id": 43, "workflow_id": 10, "run_number": 8, "created_at": "2026-08-09T00:01:00Z"},
				{"id": 42, "workflow_id": 10, "run_number": 7, "created_at": "2026-08-09T00:00:00Z"},
			},
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	gh, err := github.NewClient(
		github.WithAuthToken("ghp_test"),
		github.WithURLs(&srv.URL, &srv.URL),
	)
	if err != nil {
		t.Fatalf("failed to create github client: %v", err)
	}
	c := &tokenClient{client: gh}

	replacement, err := c.FindReplacementRun(context.Background(), "pyama86", "jobpin", &Run{
		ID:         42,
		WorkflowID: 10,
		RunNumber:  7,
		Event:      "push",
		Branch:     "main",
		CreatedAt:  time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("FindReplacementRun failed: %v", err)
	}
	if replacement == nil || replacement.ID != 43 {
		t.Fatalf("replacement = %+v, want run 43", replacement)
	}
}
