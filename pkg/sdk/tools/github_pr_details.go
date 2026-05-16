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
	return "Fetches GitHub Pull Request metadata including title, description, author, base/head branches, current state, merge conflict status, and CI check results. Provides high-level context for a PR."
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

	ctx := context.Background()

	pr, _, err := t.client.PullRequests.Get(ctx, in.Owner, in.Repo, in.PRNumber)
	if err != nil {
		return nil, sdk.NewAIError(fmt.Sprintf("failed to fetch PR details: %v", err)).WithReason(err)
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

	result["merge"] = buildMergeStatus(pr)

	if pr.Head != nil {
		if headSHA := pr.Head.GetSHA(); headSHA != "" {
			result["ci"] = t.buildCIStatus(ctx, in.Owner, in.Repo, headSHA)
		}
	}

	return result, nil
}

// buildMergeStatus summarises mergeability and conflict info from a PR.
// GitHub computes `mergeable` asynchronously; if it's nil, the result is
// reported as "unknown" so callers know to retry later.
func buildMergeStatus(pr *github.PullRequest) map[string]interface{} {
	merge := map[string]interface{}{
		"merged": pr.GetMerged(),
	}

	if pr.Mergeable != nil {
		merge["mergeable"] = pr.GetMergeable()
		merge["has_conflicts"] = !pr.GetMergeable()
	} else {
		merge["mergeable"] = "unknown"
	}

	if state := pr.GetMergeableState(); state != "" {
		merge["mergeable_state"] = state
	}
	if pr.Rebaseable != nil {
		merge["rebaseable"] = pr.GetRebaseable()
	}
	if sha := pr.GetMergeCommitSHA(); sha != "" {
		merge["merge_commit_sha"] = sha
	}

	return merge
}

// buildCIStatus aggregates check runs (Checks API) and legacy commit statuses
// for the PR's head SHA. Errors are surfaced inline so partial CI info is
// still returned even if one source is unavailable.
func (t *GithubPRDetailsTool) buildCIStatus(ctx context.Context, owner, repo, headSHA string) map[string]interface{} {
	ci := map[string]interface{}{
		"head_sha": headSHA,
	}

	checkRuns, _, err := t.client.Checks.ListCheckRunsForRef(ctx, owner, repo, headSHA, &github.ListCheckRunsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil {
		ci["check_runs_error"] = err.Error()
	} else if checkRuns != nil {
		runs := make([]map[string]interface{}, 0, len(checkRuns.CheckRuns))
		for _, cr := range checkRuns.CheckRuns {
			run := map[string]interface{}{
				"name":   cr.GetName(),
				"status": cr.GetStatus(),
			}
			if conclusion := cr.GetConclusion(); conclusion != "" {
				run["conclusion"] = conclusion
			}
			if url := cr.GetHTMLURL(); url != "" {
				run["url"] = url
			}
			runs = append(runs, run)
		}
		ci["check_runs"] = runs
		ci["check_runs_total"] = checkRuns.GetTotal()
	}

	combined, _, err := t.client.Repositories.GetCombinedStatus(ctx, owner, repo, headSHA, nil)
	if err != nil {
		ci["statuses_error"] = err.Error()
	} else if combined != nil {
		ci["combined_state"] = combined.GetState()
		statuses := make([]map[string]interface{}, 0, len(combined.Statuses))
		for _, s := range combined.Statuses {
			status := map[string]interface{}{
				"context": s.GetContext(),
				"state":   s.GetState(),
			}
			if desc := s.GetDescription(); desc != "" {
				status["description"] = desc
			}
			if url := s.GetTargetURL(); url != "" {
				status["url"] = url
			}
			statuses = append(statuses, status)
		}
		ci["statuses"] = statuses
	}

	return ci
}
