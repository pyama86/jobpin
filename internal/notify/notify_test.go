package notify

import (
	"strings"
	"testing"

	"github.com/pyama86/jobpin/internal/ghclient"
	"github.com/pyama86/jobpin/internal/store"
)

func TestNewRendererInvalidTemplate(t *testing.T) {
	if _, err := NewRenderer("{{.Invalid", "ok", "ok"); err == nil {
		t.Fatal("expected error for invalid success template")
	}
	if _, err := NewRenderer("ok", "{{.Invalid", "ok"); err == nil {
		t.Fatal("expected error for invalid failure template")
	}
	if _, err := NewRenderer("ok", "ok", "{{.Invalid"); err == nil {
		t.Fatal("expected error for invalid timeout template")
	}
}

func TestRender(t *testing.T) {
	r, err := NewRenderer(
		"ok <@{{.Requester}}> {{.Owner}}/{{.Repo}} {{.WorkflowName}} #{{.RunNumber}} {{.Branch}} {{.RunURL}}",
		"ng <@{{.Requester}}> {{.Conclusion}} {{.Status}} run={{.RunID}} {{.RunURL}}",
		"timeout <@{{.Requester}}> {{.Owner}}/{{.Repo}} run={{.RunID}} {{.RunURL}}",
	)
	if err != nil {
		t.Fatal(err)
	}

	job := &store.Job{
		Owner:     "pyama86",
		Repo:      "jobpin",
		RunID:     123,
		Requester: "U123",
	}

	t.Run("success", func(t *testing.T) {
		run := &ghclient.Run{
			WorkflowName: "CI",
			RunNumber:    42,
			Status:       "completed",
			Conclusion:   "success",
			HTMLURL:      "https://github.com/pyama86/jobpin/actions/runs/123",
			Branch:       "main",
		}
		got, err := r.Render(job, run)
		if err != nil {
			t.Fatal(err)
		}
		want := "ok <@U123> pyama86/jobpin CI #42 main https://github.com/pyama86/jobpin/actions/runs/123"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("failure", func(t *testing.T) {
		run := &ghclient.Run{
			Status:     "completed",
			Conclusion: "failure",
		}
		got, err := r.Render(job, run)
		if err != nil {
			t.Fatal(err)
		}
		want := "ng <@U123> failure completed run=123 https://github.com/pyama86/jobpin/actions/runs/123"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("cancelled uses failure template", func(t *testing.T) {
		run := &ghclient.Run{Status: "completed", Conclusion: "cancelled"}
		got, err := r.Render(job, run)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(got, "ng ") {
			t.Errorf("expected failure template, got %q", got)
		}
	})
}

func TestRenderTimeout(t *testing.T) {
	r, err := NewRenderer(
		"ok",
		"ng",
		"timeout <@{{.Requester}}> {{.Owner}}/{{.Repo}} run={{.RunID}} {{.RunURL}}",
	)
	if err != nil {
		t.Fatal(err)
	}

	job := &store.Job{
		Owner:     "pyama86",
		Repo:      "jobpin",
		RunID:     123,
		Requester: "U123",
	}

	got, err := r.RenderTimeout(job)
	if err != nil {
		t.Fatal(err)
	}
	want := "timeout <@U123> pyama86/jobpin run=123 https://github.com/pyama86/jobpin/actions/runs/123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
