package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

const (
	maxLinesPerEdit = 150

	strReplaceCommand = "str_replace"
	insertCommand     = "insert"
)

type StrReplaceEditorTool struct{}

func (t *StrReplaceEditorTool) Name() string { return "str_replace_editor" }

func (t *StrReplaceEditorTool) Description() string {
	return "Safely edits existing files using precise string replacement or insertion by line number. Never creates files and enforces <=150-line edits. Use view to inspect files before editing."
}

func (t *StrReplaceEditorTool) Init(config map[string]string, _ *config.ConfigFactory) {
}

func (t *StrReplaceEditorTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Operation to perform: \"str_replace\" or \"insert\". If you are editing empty files or adding to the beginning of the file, use insert. \"insert\" can be used to add to the beginning of the file. \"str_replace\" should be used to replace string.",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File to edit, relative to the repository root or absolute.",
			},
			"new_str": map[string]interface{}{
				"type":        "string",
				"description": "The new string to insert or replace with. If the string spans multiple lines, include the newlines. The string will be inserted if the command is insert or replace the old string if the command is str_replace.",
			},
			"old_str": map[string]interface{}{
				"type":        "string",
				"description": "The old string to replace. It has to match exactly and be unique in the file. New lines and spaces matter. Only used for str_replace.",
			},
			"insert_line": map[string]interface{}{
				"type":        "integer",
				"description": "The 1-based line number to insert the new string before. Only used for insert. If command is insert, insert_line and new_str must be provided. Use the insert command when writing to an empty file over the str_replace command.",
			},
		},
		"required": []interface{}{"command", "path"},
		"example": map[string]interface{}{
			"command": "str_replace",
			"path":    "pkg/sdk/tools/str_replace_editor.go",
			"new_str": "new string",
			"old_str": "old string",
		},
	}
}

type StrReplaceEditorToolInput struct {
	Command string `json:"command"` // "str_replace" or "insert"
	Path    string `json:"path"`

	// new string to insert or replace with
	NewStr1 string `json:"new_str"`

	// string replacement params
	OldStr1 string `json:"old_str"`

	// insert params
	InsertLine1 int `json:"insert_line"`
}

func (t *StrReplaceEditorTool) Call(input map[string]interface{}) (interface{}, error) {
	var in StrReplaceEditorToolInput
	if err := mapstruct(input, &in); err != nil {
		return "", err
	}

	if in.Command != "str_replace" && in.Command != "insert" {
		return fmt.Sprintf("Unsupported command: %s", in.Command), nil
	}

	if in.Path == "" {
		return "Path is required.", nil
	}

	if strings.Count(in.NewStr1, "\n") > maxLinesPerEdit {
		return fmt.Sprintf("New string exceeds %d line limit.", maxLinesPerEdit), nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	path := in.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("File not found: %s", path), nil
	}

	if info.IsDir() {
		return fmt.Sprintf("Path is a directory: %s", path), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)

	lines := strings.SplitAfter(content, "\n")
	totalLines := len(lines)

	// pos is the index of the start of each line
	pos := make([]int, totalLines+1)
	for i, line := range lines {
		pos[i+1] = pos[i] + len(line)
	}

	switch in.Command {
	case strReplaceCommand:
		return t.handleStrReplace(path, content, in)
	case insertCommand:
		return t.handleInsert(path, content, pos, totalLines, in)
	default:
		return "", fmt.Errorf("unsupported command: %q", in.Command)
	}
}

func (t *StrReplaceEditorTool) handleStrReplace(path, content string, in StrReplaceEditorToolInput) (interface{}, error) {
	if in.OldStr1 == "" {
		return "", sdk.NewAIError("old_str is required")
	}

	count := strings.Count(content, in.OldStr1)
	if count == 0 {
		return "old_str not found in file. Please make sure it matches exactly, including spaces and newlines.", nil
	}
	if count > 1 {
		return "old_str found multiple times in file. Please provide more context in old_str to make it unique.", nil
	}

	newContent := strings.Replace(content, in.OldStr1, in.NewStr1, 1)

	if err := os.WriteFile(path, []byte(newContent), 0o600); err != nil {
		return "", fmt.Errorf("failed to write updated file: %w", err)
	}

	return fmt.Sprintf("Applied str_replace edit to %s.", path), nil
}

func (t *StrReplaceEditorTool) handleInsert(path, content string, pos []int, totalLines int, in StrReplaceEditorToolInput) (interface{}, error) {
	if in.InsertLine1 == 0 {
		return "", sdk.NewAIError("insert_line and new_str are required")
	}

	if in.InsertLine1 > totalLines {
		return fmt.Sprintf("insert_line=%d is out of range (0-%d)", in.InsertLine1, totalLines), nil
	}

	insertIdx := pos[in.InsertLine1-1]

	var b strings.Builder
	b.WriteString(content[:insertIdx])
	b.WriteString(in.NewStr1)
	b.WriteString(content[insertIdx:])

	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", fmt.Errorf("failed to write updated file: %w", err)
	}

	return fmt.Sprintf("Applied insert edit to %s.", path), nil
}
