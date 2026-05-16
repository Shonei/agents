package tools

import (
	"context"
	"fmt"

	"github.com/google/go-github/v62/github"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

type GithubPRCommentsTool struct {
	client *github.Client
}

func (t *GithubPRCommentsTool) Name() string {
	return "github_pr_comments"
}

func (t *GithubPRCommentsTool) Description() string {
	return "Fetches existing review comments and general PR discussion threads. Ensures the agent understands ongoing discussions and avoids repeating feedback."
}

func (t *GithubPRCommentsTool) Init(_ map[string]string, c *config.ConfigFactory) {
	token := c.GetGitHubToken()
	if token != "" {
		t.client = github.NewClient(nil).WithAuthToken(token)
	} else {
		t.client = github.NewClient(nil)
	}
}

func (t *GithubPRCommentsTool) InputSchema() map[string]interface{} {
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

type GithubPRCommentsInput struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	PRNumber int    `json:"pr_number"`
}

func (t *GithubPRCommentsTool) Call(input map[string]interface{}) (interface{}, error) {
	var in GithubPRCommentsInput
	if err := mapstruct(input, &in); err != nil {
		return nil, err
	}

	if in.Owner == "" || in.Repo == "" || in.PRNumber == 0 {
		return nil, sdk.NewAIError("owner, repo, and pr_number are required")
	}

	if t.client == nil {
		return nil, sdk.NewAIError("GitHub client is not initialized. Please set GITHUB_TOKEN.")
	}

	ctx := context.Background()

	// Fetch issue comments (general PR comments)
	issueComments, _, err := t.client.Issues.ListComments(ctx, in.Owner, in.Repo, in.PRNumber, nil)
	if err != nil {
		return nil, sdk.NewAIError(fmt.Sprintf("failed to fetch issue comments: %v", err)).WithReason(err)
	}

	// Fetch review comments (inline code comments)
	reviewComments, _, err := t.client.PullRequests.ListComments(ctx, in.Owner, in.Repo, in.PRNumber, nil)
	if err != nil {
		return nil, sdk.NewAIError(fmt.Sprintf("failed to fetch review comments: %v", err)).WithReason(err)
	}

	var results []map[string]interface{}

	for _, c := range issueComments {
		comment := map[string]interface{}{
			"type": "general",
			"body": c.GetBody(),
		}
		if c.User != nil {
			comment["author"] = c.User.GetLogin()
		}
		if c.CreatedAt != nil {
			comment["created_at"] = c.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		}
		results = append(results, comment)
	}

	for _, c := range reviewComments {
		comment := map[string]interface{}{
			"type": "inline",
			"body": c.GetBody(),
			"path": c.GetPath(),
		}
		if c.Line != nil {
			comment["line"] = c.GetLine()
		}
		if c.User != nil {
			comment["author"] = c.User.GetLogin()
		}
		if c.CreatedAt != nil {
			comment["created_at"] = c.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		}
		results = append(results, comment)
	}

	return results, nil
}
