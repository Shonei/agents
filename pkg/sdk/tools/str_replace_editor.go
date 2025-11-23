package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	instructionReminderText = "ALWAYS BREAK DOWN EDITS INTO SMALLER CHUNKS OF AT MOST 150 LINES EACH."
	maxLinesPerEdit         = 150

	strReplaceCommand = "str_replace"
	insertCommand     = "insert"
)

type StrReplaceEditorTool struct{}

func (t *StrReplaceEditorTool) Name() string { return "str_replace_editor" }

func (t *StrReplaceEditorTool) Description() string {
	return "Safely edits existing files using precise string replacement or insertion by line number. Never creates files and enforces <=150-line edits. Use view to inspect files before editing."
}

func (t *StrReplaceEditorTool) Init(config map[string]string) {
}

func (t *StrReplaceEditorTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Operation to perform: \"str_replace\" or \"insert\".",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File to edit, relative to the repository root or absolute.",
			},
			"new_str": map[string]interface{}{
				"type":        "string",
				"description": "The new string to replace the old string with. If left empty it will delete the old string.",
			},
			"old_str": map[string]interface{}{
				"type":        "string",
				"description": "The old string to replace. It has to match exactly or the tool will error. New lines and spaces matter. Only used for str_replace.",
			},
			"old_str_start_line_number": map[string]interface{}{
				"type":        "integer",
				"description": "The 1-based start line number of the old string. Only used for str_replace. If command is str_replace, old_str_start_line_number, old_str_end_line_number, and old_str must be provided.",
			},
			"old_str_end_line_number": map[string]interface{}{
				"type":        "integer",
				"description": "The 1-based end line number of the old string inclusive. Only used for str_replace. If command is str_replace, old_str_start_line_number, old_str_end_line_number, and old_str must be provided.",
			},
			"insert_line": map[string]interface{}{
				"type":        "integer",
				"description": "The 1-based line number to insert the new string before. Only used for insert. If command is insert, insert_line and new_str must be provided.",
			},
		},
		"required": []interface{}{"command", "path"},
	}
}

type StrReplaceEditorToolInput struct {
	Command string `json:"command"` // "str_replace" or "insert"
	Path    string `json:"path"`

	OldStr1      string `json:"old_str"`
	OldStrStart1 int    `json:"old_str_start_line_number"`
	OldStrEnd1   int    `json:"old_str_end_line_number"`

	NewStr1 string `json:"new_str"`

	InsertLine1 int `json:"insert_line"`
}

type editOp struct {
	startIdx int
	endIdx   int
	text     string
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
	case "str_replace":
		return t.handleStrReplace(path, content, pos, totalLines, in)
	case "insert":
		return t.handleInsert(path, content, pos, totalLines, in)
	default:
		return "", fmt.Errorf("unsupported command: %q", in.Command)
	}
}

func (t *StrReplaceEditorTool) handleStrReplace(path, content string, pos []int, totalLines int, in StrReplaceEditorToolInput) (interface{}, error) {
	if in.OldStr1 == "" || in.OldStrStart1 <= 0 || in.OldStrEnd1 <= 0 {
		return "", fmt.Errorf("old_str and line numbers are required")
	}

	if in.OldStrEnd1 < in.OldStrStart1 {
		return fmt.Sprintf("end line before start line"), nil
	}

	if in.OldStrEnd1 > totalLines {
		return fmt.Sprintf("line range exceeds file length (total %d)", totalLines), nil
	}

	if in.OldStrEnd1-in.OldStrStart1+1 > maxLinesPerEdit {
		return fmt.Sprintf("line range spans %d lines, exceeds limit %d", in.OldStrEnd1-in.OldStrStart1+1, maxLinesPerEdit), nil
	}

	existing := content[pos[in.OldStrStart1-1]:pos[in.OldStrEnd1]]

	if strings.TrimSpace(existing) != strings.TrimSpace(in.OldStr1) {
		return fmt.Sprintf("content mismatch for lines %d-%d.\n\nFile content:\n%s", in.OldStrStart1, in.OldStrEnd1, existing), nil
	}

	var b strings.Builder
	b.WriteString(content[:pos[in.OldStrStart1-1]])
	b.WriteString(in.NewStr1)
	b.WriteString(content[pos[in.OldStrEnd1]:])

	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write updated file: %w", err)
	}

	return fmt.Sprintf("Applied str_replace edit to %s.", path), nil
}

func (t *StrReplaceEditorTool) handleInsert(path, content string, pos []int, totalLines int, in StrReplaceEditorToolInput) (interface{}, error) {
	if in.InsertLine1 == 0 {
		return "", fmt.Errorf("insert_line and new_str are required")
	}

	if in.InsertLine1 > totalLines {
		return fmt.Sprintf("insert_line=%d is out of range (0-%d)", in.InsertLine1, totalLines), nil
	}

	lines := strings.Count(in.NewStr1, "\n") + 1
	if lines > maxLinesPerEdit {
		return fmt.Sprintf("inserted text spans %d lines, exceeds limit %d", lines, maxLinesPerEdit), nil
	}

	insertIdx := pos[in.InsertLine1-1]

	var b strings.Builder
	b.WriteString(content[:insertIdx])
	b.WriteString(in.NewStr1)

	b.WriteString(content[insertIdx:])

	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write updated file: %w", err)
	}

	return fmt.Sprintf("Applied insert edit to %s.", path), nil
}
