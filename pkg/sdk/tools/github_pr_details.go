package tools

import (
	"context"
	"fmt"

	"github.com/google/go-github/v62/github"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

type GithubPRDetailsTool struct {
	client *github.Client
}

func (t *GithubPRDetailsTool) Name() string {
	return "github_pr_details"
}

func (t *GithubPRDetailsTool) Description() string {
	return "Fetches GitHub Pull Request metadata including title, description, author, base/head branches, and current state. Provides high-level context for a PR."
}

func (t *GithubPRDetailsTool) Init(_ map[string]string, c *config.ConfigFactory) {
	token := c.GetGitHubToken()
	if token != "" {
		t.client = github.NewClient(nil).WithAuthToken(token)
	} else {
		t.client = github.NewClient(nil)
	}
}

func (t *GithubPRDetailsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"owner": map[string]interface{}{
				"type":        "string",
				"description": "The repository owner (e.g., 'octocat').",
			},
			"repo": map[string]interface{}{
				"type":        "string",
				"description": "The repository name (e.g., 'Hello-World').",
			},
			"pr_number": map[string]interface{}{
				"type":        "integer",
				"description": "The pull request number.",
			},
		},
		"required": []interface{}{"owner", "repo", "pr_number"},
	}
}

type GithubPRDetailsInput struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	PRNumber int    `json:"pr_number"`
}

func (t *GithubPRDetailsTool) Call(input map[string]interface{}) (interface{}, error) {
	var in GithubPRDetailsInput
	if err := mapstruct(input, &in); err != nil {
		return nil, err
	}

	if in.Owner == "" || in.Repo == "" || in.PRNumber == 0 {
		return nil, sdk.NewAIError("owner, repo, and pr_number are required")
	}

	if t.client == nil {
		return nil, sdk.NewAIError("GitHub client is not initialized. Please set GITHUB_TOKEN.")
	}

	pr, _, err := t.client.PullRequests.Get(context.Background(), in.Owner, in.Repo, in.PRNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR details: %w", err)
	}

	result := map[string]interface{}{
		"title": pr.GetTitle(),
		"body":  pr.GetBody(),
		"state": pr.GetState(),
	}

	if pr.User != nil {
		result["author"] = pr.User.GetLogin()
	}
	if pr.Base != nil {
		result["base_branch"] = pr.Base.GetRef()
	}
	if pr.Head != nil {
		result["head_branch"] = pr.Head.GetRef()
	}
	if pr.CreatedAt != nil {
		result["created_at"] = pr.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	return result, nil
}
