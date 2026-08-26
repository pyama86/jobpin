package notify

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/pyama86/jobpin/internal/ghclient"
	"github.com/pyama86/jobpin/internal/store"
)

type Renderer struct {
	success *template.Template
	failure *template.Template
	timeout *template.Template
}

func NewRenderer(successTmpl, failureTmpl, timeoutTmpl string) (*Renderer, error) {
	success, err := template.New("success").Parse(successTmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse success template: %w", err)
	}
	failure, err := template.New("failure").Parse(failureTmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse failure template: %w", err)
	}
	timeout, err := template.New("timeout").Parse(timeoutTmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timeout template: %w", err)
	}
	return &Renderer{success: success, failure: failure, timeout: timeout}, nil
}

type templateData struct {
	Requester    string
	Owner        string
	Repo         string
	RunID        int64
	RunURL       string
	WorkflowName string
	RunNumber    int
	Status       string
	Conclusion   string
	Branch       string
	Note         string
}

func (r *Renderer) Render(job *store.Job, run *ghclient.Run) (string, error) {
	runURL := run.HTMLURL
	if runURL == "" {
		runURL = fmt.Sprintf("https://github.com/%s/%s/actions/runs/%d", job.Owner, job.Repo, job.RunID)
	}

	data := templateData{
		Requester:    job.Requester,
		Owner:        job.Owner,
		Repo:         job.Repo,
		RunID:        job.RunID,
		RunURL:       runURL,
		WorkflowName: run.WorkflowName,
		RunNumber:    run.RunNumber,
		Status:       run.Status,
		Conclusion:   run.Conclusion,
		Branch:       run.Branch,
		Note:         job.Note,
	}

	tmpl := r.failure
	if run.Conclusion == "success" {
		tmpl = r.success
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("failed to render notify message: %w", err)
	}
	return sb.String(), nil
}

func (r *Renderer) RenderTimeout(job *store.Job) (string, error) {
	data := templateData{
		Requester: job.Requester,
		Owner:     job.Owner,
		Repo:      job.Repo,
		RunID:     job.RunID,
		RunURL:    fmt.Sprintf("https://github.com/%s/%s/actions/runs/%d", job.Owner, job.Repo, job.RunID),
		Note:      job.Note,
	}

	var sb strings.Builder
	if err := r.timeout.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("failed to render timeout message: %w", err)
	}
	return sb.String(), nil
}
