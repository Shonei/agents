package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

type DeleteFileTool struct {
	requireConfirmation bool
	allowedRoot         string
}

func (d *DeleteFileTool) Name() string {
	return "delete_file"
}

func (d *DeleteFileTool) Description() string {
	return "Deletes a single file from the local filesystem. It refuses to delete directories and refuses paths outside the configured allowed_root (defaults to the current working directory). By default the user is prompted to confirm; set require_confirmation: \"false\" in the tool config to auto-approve deletes."
}

func (d *DeleteFileTool) Init(config map[string]string, _ *config.ConfigFactory) {
	d.requireConfirmation = true
	if val, ok := config["require_confirmation"]; ok && val == "false" {
		d.requireConfirmation = false
	}
	if val, ok := config["allowed_root"]; ok && val != "" {
		d.allowedRoot = val
	}
}

func (d *DeleteFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The file to delete, relative to the repository root or absolute.",
				"example":     "./systems/example/obsolete.md",
			},
			"ignore_missing": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, missing files are treated as a successful no-op. Defaults to false.",
				"example":     false,
			},
		},
		"required": []interface{}{"path"},
	}
}

type DeleteFileToolInput struct {
	FilePath      string `json:"path"`
	IgnoreMissing bool   `json:"ignore_missing"`
}

func (d *DeleteFileTool) Call(input map[string]interface{}) (interface{}, error) {
	var toolInput DeleteFileToolInput
	if err := mapstruct(input, &toolInput); err != nil {
		return "", err
	}
	if toolInput.FilePath == "" {
		return "", sdk.NewAIError("path is required when deleting a file")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	if !filepath.IsAbs(toolInput.FilePath) {
		toolInput.FilePath = filepath.Join(cwd, toolInput.FilePath)
	}
	toolInput.FilePath = filepath.Clean(toolInput.FilePath)

	allowedRoot := d.allowedRoot
	if allowedRoot == "" {
		allowedRoot = cwd
	}
	if !filepath.IsAbs(allowedRoot) {
		allowedRoot = filepath.Join(cwd, allowedRoot)
	}
	allowedRoot = filepath.Clean(allowedRoot)

	info, err := os.Lstat(toolInput.FilePath)
	if err != nil {
		if os.IsNotExist(err) && toolInput.IgnoreMissing {
			if err := ensureDeletePathWithinRoot(toolInput.FilePath, allowedRoot, true); err != nil {
				return "", err
			}

			return fmt.Sprintf("<exitcode>0</exitcode><output>File already absent: %s</output>", toolInput.FilePath), nil
		}

		return "", fmt.Errorf("failed to inspect file before delete: %w", err)
	}
	if info.IsDir() {
		return "", sdk.NewAIError("delete_file refuses to delete directories")
	}
	if err := ensureDeletePathWithinRoot(toolInput.FilePath, allowedRoot, true); err != nil {
		return "", err
	}

	if d.requireConfirmation {
		color.New(color.FgYellow, color.Bold).Println("\nYou are about to delete the following file:")
		color.Cyan("  %s", toolInput.FilePath)

		answer, _ := utils.AskUserConfirmation()
		switch answer {
		case utils.ToolExecutionYes:
			// continue
		case utils.ToolExecutionSkip:
			return "<exitcode>1</exitcode><output>Skipped by user</output>", nil
		case utils.ToolExecutionAbort:
			utils.NewExitError().WithMessage("tool execution aborted by user").Done()
		case utils.ToolExecutionUnknown:
			utils.NewExitError().WithMessage("unknown user choice").Done()
		}
	} else {
		color.New(color.FgYellow, color.Bold).Println("\nDeleting file (auto-confirmed):")
		color.Cyan("  %s", toolInput.FilePath)
	}

	if err := os.Remove(toolInput.FilePath); err != nil {
		return "", fmt.Errorf("failed to delete file: %w", err)
	}

	return fmt.Sprintf("<exitcode>0</exitcode><output>File deleted: %s</output>", toolInput.FilePath), nil
}

func ensureDeletePathWithinRoot(path, root string, requireExistingParent bool) error {
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("failed to resolve delete allowed_root: %w", err)
	}
	parent := filepath.Dir(path)
	parentReal := parent
	if requireExistingParent {
		parentReal, err = filepath.EvalSymlinks(parent)
		if err != nil {
			return fmt.Errorf("failed to resolve file parent before delete: %w", err)
		}
	}
	if !isPathWithinRoot(path, root) || !isPathWithinRoot(parentReal, rootReal) {
		return sdk.NewAIError(fmt.Sprintf("delete_file refuses to delete outside allowed_root %s", root))
	}

	return nil
}

func isPathWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}

	return rel == "." || (rel != ".." && !filepath.IsAbs(rel) && !startsWithParentTraversal(rel))
}

func startsWithParentTraversal(path string) bool {
	return path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator))
}
