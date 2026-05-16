package tools

import (
	"context"
	"fmt"

	"github.com/google/go-github/v62/github"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

type GithubPRDiffTool struct {
	client *github.Client
}

func (t *GithubPRDiffTool) Name() string {
	return "github_pr_diff"
}

func (t *GithubPRDiffTool) Description() string {
	return "Fetches the raw diff/patch for a GitHub Pull Request. Tells the agent exactly what code was modified."
}

func (t *GithubPRDiffTool) Init(_ map[string]string, c *config.ConfigFactory) {
	token := c.GetGitHubToken()
	if token != "" {
		t.client = github.NewClient(nil).WithAuthToken(token)
	} else {
		t.client = github.NewClient(nil)
	}
}

func (t *GithubPRDiffTool) InputSchema() map[string]interface{} {
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

type GithubPRDiffInput struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	PRNumber int    `json:"pr_number"`
}

func (t *GithubPRDiffTool) Call(input map[string]interface{}) (interface{}, error) {
	var in GithubPRDiffInput
	if err := mapstruct(input, &in); err != nil {
		return nil, err
	}

	if in.Owner == "" || in.Repo == "" || in.PRNumber == 0 {
		return nil, sdk.NewAIError("owner, repo, and pr_number are required")
	}

	if t.client == nil {
		return nil, sdk.NewAIError("GitHub client is not initialized. Please set GITHUB_TOKEN.")
	}

	diff, _, err := t.client.PullRequests.GetRaw(context.Background(), in.Owner, in.Repo, in.PRNumber, github.RawOptions{Type: github.Diff})
	if err != nil {
		return nil, sdk.NewAIError(fmt.Sprintf("failed to fetch PR diff: %v", err)).WithReason(err)
	}

	return diff, nil
}
