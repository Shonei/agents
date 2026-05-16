package tools

import (
	"context"
	"fmt"

	"github.com/google/go-github/v62/github"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

type GithubPRReviewTool struct {
	client *github.Client
}

func (t *GithubPRReviewTool) Name() string {
	return "github_pr_review"
}

func (t *GithubPRReviewTool) Description() string {
	return "Submits a formal PR review. Supports adding line-specific comments on the diff and providing a general summary."
}

func (t *GithubPRReviewTool) Init(_ map[string]string, c *config.ConfigFactory) {
	token := c.GetGitHubToken()
	if token != "" {
		t.client = github.NewClient(nil).WithAuthToken(token)
	} else {
		t.client = github.NewClient(nil)
	}
}

func (t *GithubPRReviewTool) InputSchema() map[string]interface{} {
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
			"event": map[string]interface{}{
				"type":        "string",
				"description": "The review action (APPROVE, REQUEST_CHANGES, or COMMENT).",
				"enum":        []string{"APPROVE", "REQUEST_CHANGES", "COMMENT"},
			},
			"body": map[string]interface{}{
				"type":        "string",
				"description": "The main summary body of the review.",
			},
			"comments": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The relative path to the file that necessitates a comment.",
						},
						"line": map[string]interface{}{
							"type":        "integer",
							"description": "The line of the blob in the pull request diff that the comment applies to.",
						},
						"body": map[string]interface{}{
							"type":        "string",
							"description": "Text of the review comment.",
						},
					},
					"required": []interface{}{"path", "line", "body"},
				},
				"description": "Optional inline comments.",
			},
		},
		"required": []interface{}{"owner", "repo", "pr_number", "event", "body"},
	}
}

type GithubPRReviewComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Body string `json:"body"`
}

type GithubPRReviewInput struct {
	Owner    string                  `json:"owner"`
	Repo     string                  `json:"repo"`
	PRNumber int                     `json:"pr_number"`
	Event    string                  `json:"event"`
	Body     string                  `json:"body"`
	Comments []GithubPRReviewComment `json:"comments"`
}

func (t *GithubPRReviewTool) Call(input map[string]interface{}) (interface{}, error) {
	var in GithubPRReviewInput
	if err := mapstruct(input, &in); err != nil {
		return nil, err
	}

	if in.Owner == "" || in.Repo == "" || in.PRNumber == 0 || in.Event == "" || in.Body == "" {
		return nil, sdk.NewAIError("owner, repo, pr_number, event, and body are required")
	}

	if t.client == nil {
		return nil, sdk.NewAIError("GitHub client is not initialized. Please set GITHUB_TOKEN.")
	}

	reviewRequest := &github.PullRequestReviewRequest{
		Body:  github.String(in.Body),
		Event: github.String(in.Event),
	}

	if len(in.Comments) > 0 {
		var comments []*github.DraftReviewComment
		for _, c := range in.Comments {
			comments = append(comments, &github.DraftReviewComment{
				Path: github.String(c.Path),
				Line: github.Int(c.Line),
				Body: github.String(c.Body),
			})
		}
		reviewRequest.Comments = comments
	}

	review, _, err := t.client.PullRequests.CreateReview(context.Background(), in.Owner, in.Repo, in.PRNumber, reviewRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create PR review: %w", err)
	}

	return map[string]interface{}{
		"status": "success",
		"url":    review.GetHTMLURL(),
	}, nil
}
