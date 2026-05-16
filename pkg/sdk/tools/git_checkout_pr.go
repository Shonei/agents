package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	return "Clones the given GitHub repository over SSH, fetches the requested pull request branch and checks it out locally. By default the repo is cloned into a fresh temporary directory; pass `clone_path` to clone into a specific folder instead (the directory will be created if missing and must be empty if it already exists). Returns the absolute path to the cloned working tree so the agent can use existing tools (e.g. list_dir, view_file, bash) to inspect the code, run tests, or execute linters."
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
			"clone_path": map[string]interface{}{
				"type":        "string",
				"description": "Optional destination folder for the clone. May be absolute, relative to the current working directory, or start with '~' for the user's home directory. The directory will be created if it does not exist, and must be empty if it does. If omitted, a fresh temporary directory is created.",
			},
		},
		"required": []interface{}{"owner", "repo", "pr_number"},
	}
}

type GitCheckoutPRInput struct {
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	PRNumber  int    `json:"pr_number"`
	ClonePath string `json:"clone_path"`
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

	cloneDir, createdDir, err := resolveClonePath(in.ClonePath, in.Owner, in.Repo, in.PRNumber)
	if err != nil {
		return nil, err
	}

	color.New(color.FgYellow, color.Bold).Println("\nYou are about to clone and checkout the following PR:")
	color.Cyan("  repo:   %s/%s", in.Owner, in.Repo)
	color.Cyan("  pr:     #%d", in.PRNumber)
	color.Cyan("  branch: %s", branchName)
	color.Cyan("  ssh:    %s", sshURL)
	if cloneDir != "" {
		color.Cyan("  path:   %s", cloneDir)
	} else {
		color.Cyan("  path:   <temp dir>")
	}
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

	if cloneDir == "" {
		cloneDir, err = os.MkdirTemp("", fmt.Sprintf("agents-%s-%s-pr-%d-*", in.Owner, in.Repo, in.PRNumber))
		if err != nil {
			return nil, sdk.NewAIError(fmt.Sprintf("failed to create temp directory: %v", err)).WithReason(err)
		}
		createdDir = true
	}

	// cleanup removes the destination on failure, but only if we created it.
	cleanup := func() {
		if createdDir {
			_ = os.RemoveAll(cloneDir)
		}
	}

	cloneCmd := exec.Command("git", "clone", sshURL, cloneDir)
	if output, err := cloneCmd.CombinedOutput(); err != nil {
		cleanup()

		return nil, sdk.NewAIError(fmt.Sprintf("failed to clone repo over SSH (%s): %s, error: %v", sshURL, string(output), err)).WithReason(err)
	}

	fetchCmd := exec.Command("git", "-C", cloneDir, "fetch", "origin", fmt.Sprintf("pull/%d/head:%s", in.PRNumber, branchName))
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		cleanup()

		return nil, sdk.NewAIError(fmt.Sprintf("failed to fetch PR branch: %s, error: %v", string(output), err)).WithReason(err)
	}

	checkoutCmd := exec.Command("git", "-C", cloneDir, "checkout", branchName)
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		cleanup()

		return nil, sdk.NewAIError(fmt.Sprintf("failed to checkout PR branch: %s, error: %v", string(output), err)).WithReason(err)
	}

	return map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Cloned %s/%s over SSH into %s and checked out branch %s. Use this path as the working directory for any follow-up file or command tools.", in.Owner, in.Repo, cloneDir, branchName),
		"branch":  branchName,
		"path":    cloneDir,
		"repo":    fmt.Sprintf("%s/%s", in.Owner, in.Repo),
	}, nil
}

// resolveClonePath validates a user-supplied clone destination and returns its
// absolute form. The returned createdDir flag indicates whether the path is
// safe to remove on failure (true when we own the directory because it did not
// previously exist). An empty returned path means the caller should fall back
// to creating a temporary directory.
func resolveClonePath(raw, owner, repo string, prNumber int) (string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, nil
	}

	expanded := raw
	if strings.HasPrefix(expanded, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, sdk.NewAIError(fmt.Sprintf("failed to resolve home directory for clone_path %q: %v", raw, err)).WithReason(err)
		}
		switch {
		case expanded == "~":
			expanded = home
		case strings.HasPrefix(expanded, "~/"):
			expanded = filepath.Join(home, expanded[2:])
		default:
			return "", false, sdk.NewAIError(fmt.Sprintf("clone_path %q: only '~' and '~/...' are supported", raw))
		}
	}

	if !filepath.IsAbs(expanded) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", false, sdk.NewAIError(fmt.Sprintf("failed to resolve relative clone_path %q: %v", raw, err)).WithReason(err)
		}
		expanded = filepath.Join(cwd, expanded)
	}
	expanded = filepath.Clean(expanded)

	info, err := os.Stat(expanded)
	switch {
	case err == nil:
		if !info.IsDir() {
			return "", false, sdk.NewAIError(fmt.Sprintf("clone_path %q exists and is not a directory", expanded))
		}
		entries, readErr := os.ReadDir(expanded)
		if readErr != nil {
			return "", false, sdk.NewAIError(fmt.Sprintf("failed to read clone_path %q: %v", expanded, readErr)).WithReason(readErr)
		}
		if len(entries) > 0 {
			return "", false, sdk.NewAIError(fmt.Sprintf("clone_path %q already exists and is not empty; pick a different folder", expanded))
		}

		return expanded, false, nil
	case os.IsNotExist(err):
		parent := filepath.Dir(expanded)
		if _, perr := os.Stat(parent); perr != nil {
			return "", false, sdk.NewAIError(fmt.Sprintf("parent directory %q for clone_path does not exist: %v", parent, perr)).WithReason(perr)
		}
		if mkErr := os.MkdirAll(expanded, 0o755); mkErr != nil {
			return "", false, sdk.NewAIError(fmt.Sprintf("failed to create clone_path %q: %v", expanded, mkErr)).WithReason(mkErr)
		}

		return expanded, true, nil
	default:
		return "", false, sdk.NewAIError(fmt.Sprintf("failed to stat clone_path %q: %v", expanded, err)).WithReason(err)
	}
}
