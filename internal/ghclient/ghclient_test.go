package ghclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
			"name":        "CI",
			"run_number":  7,
			"status":      "completed",
			"conclusion":  "success",
			"html_url":    "https://github.com/pyama86/jobpin/actions/runs/42",
			"head_branch": "main",
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
		WorkflowName: "CI",
		RunNumber:    7,
		Status:       "completed",
		Conclusion:   "success",
		HTMLURL:      "https://github.com/pyama86/jobpin/actions/runs/42",
		Branch:       "main",
	}
	if *run != want {
		t.Errorf("Run = %+v, want %+v", *run, want)
	}
}
