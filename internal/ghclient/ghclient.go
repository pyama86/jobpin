package ghclient

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v90/github"
	"github.com/pyama86/jobpin/internal/config"
)

type Run struct {
	ID           int64
	WorkflowName string
	RunNumber    int
	Status       string
	Conclusion   string
	HTMLURL      string
	Branch       string
}

type Client interface {
	GetWorkflowRun(ctx context.Context, owner, repo string, runID int64) (*Run, error)
}

func New(cfg *config.Config) (Client, error) {
	if cfg.GitHubToken != "" {
		gh, err := github.NewClient(github.WithAuthToken(cfg.GitHubToken))
		if err != nil {
			return nil, fmt.Errorf("failed to create github client: %w", err)
		}
		return &tokenClient{client: gh}, nil
	}

	var tr *ghinstallation.AppsTransport
	var err error
	if cfg.GitHubAppPrivateKey != "" {
		tr, err = ghinstallation.NewAppsTransport(
			http.DefaultTransport, cfg.GitHubAppID, []byte(cfg.GitHubAppPrivateKey),
		)
	} else {
		tr, err = ghinstallation.NewAppsTransportKeyFromFile(
			http.DefaultTransport, cfg.GitHubAppID, cfg.GitHubAppPrivateKeyPath,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create apps transport: %w", err)
	}
	return &appClient{appsTransport: tr}, nil
}

type tokenClient struct {
	client *github.Client
}

func (c *tokenClient) GetWorkflowRun(ctx context.Context, owner, repo string, runID int64) (*Run, error) {
	return getWorkflowRun(ctx, c.client, owner, repo, runID)
}

type appClient struct {
	appsTransport *ghinstallation.AppsTransport
	clients       sync.Map
}

func (c *appClient) GetWorkflowRun(ctx context.Context, owner, repo string, runID int64) (*Run, error) {
	client, err := c.clientFor(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	return getWorkflowRun(ctx, client, owner, repo, runID)
}

func (c *appClient) clientFor(ctx context.Context, owner, repo string) (*github.Client, error) {
	key := owner + "/" + repo
	if v, ok := c.clients.Load(key); ok {
		return v.(*github.Client), nil
	}

	appGH, err := github.NewClient(github.WithTransport(c.appsTransport))
	if err != nil {
		return nil, fmt.Errorf("failed to create app client: %w", err)
	}
	inst, _, err := appGH.Apps.GetRepositoryInstallation(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to find installation for %s/%s: %w", owner, repo, err)
	}

	itr := ghinstallation.NewFromAppsTransport(c.appsTransport, inst.GetID())
	client, err := github.NewClient(github.WithTransport(itr))
	if err != nil {
		return nil, fmt.Errorf("failed to create installation client: %w", err)
	}

	actual, _ := c.clients.LoadOrStore(key, client)
	return actual.(*github.Client), nil
}

func getWorkflowRun(ctx context.Context, client *github.Client, owner, repo string, runID int64) (*Run, error) {
	run, _, err := client.Actions.GetWorkflowRunByID(ctx, owner, repo, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow run %s/%s#%d: %w", owner, repo, runID, err)
	}

	return &Run{
		ID:           run.GetID(),
		WorkflowName: run.GetName(),
		RunNumber:    run.GetRunNumber(),
		Status:       run.GetStatus(),
		Conclusion:   run.GetConclusion(),
		HTMLURL:      run.GetHTMLURL(),
		Branch:       run.GetHeadBranch(),
	}, nil
}
