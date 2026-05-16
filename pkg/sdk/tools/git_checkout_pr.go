package tools

import (
	"fmt"
	"os/exec"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

type GitCheckoutPRTool struct{}

func (t *GitCheckoutPRTool) Name() string {
	return "git_checkout_pr"
}

func (t *GitCheckoutPRTool) Description() string {
	return "Fetches the PR branch and checks it out locally. Once checked out, the agent can use existing tools to inspect the code, run tests, or execute linters."
}

func (t *GitCheckoutPRTool) Init(_ map[string]string, _ *config.ConfigFactory) {
}

func (t *GitCheckoutPRTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pr_number": map[string]interface{}{
				"type":        "integer",
				"description": "The pull request number to check out.",
			},
		},
		"required": []interface{}{"pr_number"},
	}
}

type GitCheckoutPRInput struct {
	PRNumber int `json:"pr_number"`
}

func (t *GitCheckoutPRTool) Call(input map[string]interface{}) (interface{}, error) {
	var in GitCheckoutPRInput
	if err := mapstruct(input, &in); err != nil {
		return nil, err
	}

	if in.PRNumber == 0 {
		return nil, sdk.NewAIError("pr_number is required")
	}

	branchName := fmt.Sprintf("pr-%d", in.PRNumber)

	// Fetch the PR branch
	fetchCmd := exec.Command("git", "fetch", "origin", fmt.Sprintf("pull/%d/head:%s", in.PRNumber, branchName))
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to fetch PR branch: %s, error: %w", string(output), err)
	}

	// Checkout the PR branch
	checkoutCmd := exec.Command("git", "checkout", branchName)
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to checkout PR branch: %s, error: %w", string(output), err)
	}

	return map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Successfully fetched and checked out branch %s", branchName),
		"branch":  branchName,
	}, nil
}
