package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

// WriteToFileTool is a tool for creating files and write content to them
type WriteToFileTool struct {
	requireConfirmation bool
}

func (b *WriteToFileTool) Name() string {
	return "write_to_file"
}

func (b *WriteToFileTool) Description() string {
	return "Given a file path and content, creates the file and writes the content to it. It will create all relative directories if needed. This can also be used to overwrite existing files. By default the tool will not overwrite existing files. To overwrite the file set the force parameter to true. By default the user is prompted to confirm; set require_confirmation: \"false\" in the tool config to auto-approve writes."
}

func (b *WriteToFileTool) Init(config map[string]string, _ *config.ConfigFactory) {
	b.requireConfirmation = true
	if val, ok := config["require_confirmation"]; ok && val == "false" {
		b.requireConfirmation = false
	}
}

func (b *WriteToFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The location of where to create the file. This can be relative or absolute. The file type created depends of the file extension provided.",
				"example":     "./new-dir/file.txt or /tmp/file.txt",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to write to the file. If left empty it will create an empty file.",
				"example":     "Hello World",
			},
			"force": map[string]interface{}{
				"type":        "boolean",
				"description": "If set to true it will overwrite the content of the file provided. By default it will not overwrite the file but return a conflict error.",
				"example":     "false",
			},
		},
		"required": []interface{}{"path"},
		"example": map[string]interface{}{
			"path":    "./new-dir/file.txt",
			"content": "Hello World",
			"force":   false,
		},
	}
}

type WriteToFileToolInput struct {
	FilePath string `json:"path"`
	Content  string `json:"content"`
	Force    bool   `json:"force"`
}

func (b *WriteToFileTool) Call(input map[string]interface{}) (interface{}, error) {
	var toolInput WriteToFileToolInput
	if err := mapstruct(input, &toolInput); err != nil {
		return "", err
	}

	// check if file exists so we can validate input
	if toolInput.FilePath == "" {
		return "", sdk.NewAIError("path is required when writing to file")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	if !filepath.IsAbs(toolInput.FilePath) {
		toolInput.FilePath = filepath.Join(cwd, toolInput.FilePath)
	}

	if _, err := os.Stat(toolInput.FilePath); err == nil && !toolInput.Force {
		return "", sdk.NewAIError("file already exists, set force to true to overwrite the file content")
	}

	action := "create"
	if toolInput.Force {
		action = "create or overwrite"
	}

	if b.requireConfirmation {
		color.New(color.FgYellow, color.Bold).Printf("\nYou are about to %s the following file:\n", action)
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
		color.New(color.FgYellow, color.Bold).Printf("\nWriting file (auto-confirmed — %s):\n", action)
		color.Cyan("  %s", toolInput.FilePath)
	}

	dir := filepath.Dir(toolInput.FilePath)

	// Create all relative directories
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Write the file
	if err := os.WriteFile(toolInput.FilePath, []byte(toolInput.Content), 0o600); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	output := fmt.Sprintf("File written to %s", toolInput.FilePath)
	if toolInput.Content == "" {
		output = fmt.Sprintf("Empty file created at %s", toolInput.FilePath)
	}

	return fmt.Sprintf("<exitcode>0</exitcode><output>%s</output>", output), nil
}

func mapstruct(in map[string]interface{}, out interface{}) error {
	bytes, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	err = json.Unmarshal(bytes, out)
	if err != nil {
		return fmt.Errorf("failed to unmarshal input: %w", err)
	}

	return nil
}
