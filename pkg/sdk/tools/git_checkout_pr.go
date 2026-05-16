package tools

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/fatih/color"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

type GitCheckoutPRTool struct{}

func (t *GitCheckoutPRTool) Name() string {
	return "git_checkout_pr"
}

func (t *GitCheckoutPRTool) Description() string {
	return "Clones the given GitHub repository into a fresh temporary directory over SSH, fetches the requested pull request branch and checks it out locally. Returns the absolute path to the cloned working tree so the agent can use existing tools (e.g. list_dir, view_file, bash) to inspect the code, run tests, or execute linters."
}

func (t *GitCheckoutPRTool) Init(_ map[string]string, _ *config.ConfigFactory) {
}

func (t *GitCheckoutPRTool) InputSchema() map[string]interface{} {
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
				"description": "The pull request number to check out.",
			},
		},
		"required": []interface{}{"owner", "repo", "pr_number"},
	}
}

type GitCheckoutPRInput struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	PRNumber int    `json:"pr_number"`
}

func (t *GitCheckoutPRTool) Call(input map[string]interface{}) (interface{}, error) {
	var in GitCheckoutPRInput
	if err := mapstruct(input, &in); err != nil {
		return nil, err
	}

	if in.Owner == "" || in.Repo == "" || in.PRNumber == 0 {
		return nil, sdk.NewAIError("owner, repo, and pr_number are required")
	}

	branchName := fmt.Sprintf("pr-%d", in.PRNumber)
	sshURL := fmt.Sprintf("git@github.com:%s/%s.git", in.Owner, in.Repo)

	color.New(color.FgYellow, color.Bold).Println("\nYou are about to clone and checkout the following PR:")
	color.Cyan("  repo:   %s/%s", in.Owner, in.Repo)
	color.Cyan("  pr:     #%d", in.PRNumber)
	color.Cyan("  branch: %s", branchName)
	color.Cyan("  ssh:    %s", sshURL)
	answer, _ := utils.AskUserConfirmation()
	switch answer {
	case utils.ToolExecutionYes:
		// continue
	case utils.ToolExecutionSkip:
		return map[string]interface{}{
			"status":  "skipped",
			"message": "PR checkout was skipped by the user.",
		}, nil
	case utils.ToolExecutionAbort:
		utils.NewExitError().WithMessage("tool execution aborted by user").Done()
	case utils.ToolExecutionUnknown:
		utils.NewExitError().WithMessage("unknown user choice").Done()
	}

	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("agents-%s-%s-pr-%d-*", in.Owner, in.Repo, in.PRNumber))
	if err != nil {
		return nil, sdk.NewAIError(fmt.Sprintf("failed to create temp directory: %v", err)).WithReason(err)
	}

	cloneCmd := exec.Command("git", "clone", sshURL, tmpDir)
	if output, err := cloneCmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, sdk.NewAIError(fmt.Sprintf("failed to clone repo over SSH (%s): %s, error: %v", sshURL, string(output), err)).WithReason(err)
	}

	fetchCmd := exec.Command("git", "-C", tmpDir, "fetch", "origin", fmt.Sprintf("pull/%d/head:%s", in.PRNumber, branchName))
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, sdk.NewAIError(fmt.Sprintf("failed to fetch PR branch: %s, error: %v", string(output), err)).WithReason(err)
	}

	checkoutCmd := exec.Command("git", "-C", tmpDir, "checkout", branchName)
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, sdk.NewAIError(fmt.Sprintf("failed to checkout PR branch: %s, error: %v", string(output), err)).WithReason(err)
	}

	return map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Cloned %s/%s over SSH into %s and checked out branch %s. Use this path as the working directory for any follow-up file or command tools.", in.Owner, in.Repo, tmpDir, branchName),
		"branch":  branchName,
		"path":    tmpDir,
		"repo":    fmt.Sprintf("%s/%s", in.Owner, in.Repo),
	}, nil
}
