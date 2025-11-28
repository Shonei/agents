package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

type ViewFileTool struct{}

func (t *ViewFileTool) Name() string { return "view_file" }

func (t *ViewFileTool) Description() string {
	return "Reads a file and returns its contents with line numbers. Supports viewing a line range. Only works for files; use list_dir for directories."
}

func (t *ViewFileTool) Init(config map[string]string, _ *config.ConfigFactory) {
}

func (t *ViewFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File to view, relative to the repository root or absolute.",
				"example":     "pkg/sdk/workflows/workflows.go",
			},
			"view_range": map[string]interface{}{
				"type":        "array",
				"description": "Optional 1-based inclusive [start_line, end_line]. Use -1 as end_line to mean 'to end of file'.",
				"items": map[string]interface{}{
					"type": "integer",
				},
			},
		},
		"required": []interface{}{"path"},
	}
}

type ViewFileToolInput struct {
	Path               string `json:"path"`
	ViewRange          []int  `json:"view_range"`
	ContextLinesBefore int    `json:"context_lines_before"`
	ContextLinesAfter  int    `json:"context_lines_after"`
}

type lineRange struct {
	start int
	end   int
}

func (t *ViewFileTool) Call(input map[string]interface{}) (interface{}, error) {
	var in ViewFileToolInput
	if err := mapstruct(input, &in); err != nil {
		return "", err
	}

	if in.Path == "" {
		return "", fmt.Errorf("path is required")
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
		if ok := os.IsNotExist(err); ok {
			return "", sdk.NewAIError(fmt.Sprintf("failed to stat path: %s", path)).WithReason(err)
		}

		return "", fmt.Errorf("failed to stat path: %w", err)
	}

	if info.IsDir() {
		return "", sdk.NewAIError(fmt.Sprintf("path is a directory: %s", path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	total := len(lines)

	start, end, err := resolveRange(in.ViewRange, total)
	if err != nil {
		return "", err
	}

	var b strings.Builder

	fmt.Fprintf(&b, "<filePath>%s</filePath>\n", path)
	fmt.Fprintf(&b, "<viewRange>%d-%d of %d</viewRange>\n", start, end, total)
	fmt.Fprintf(&b, "<content>\n")

	for ln := start; ln <= end && ln <= total; ln++ {
		fmt.Fprintf(&b, "%6d  %s\n", ln, lines[ln-1])
	}

	fmt.Fprintf(&b, "</content>\n")

	return b.String(), nil
}

func resolveRange(viewRange []int, total int) (int, int, error) {
	start, end := 1, total
	if len(viewRange) > 0 {
		if len(viewRange) != 2 {
			return 0, 0, sdk.NewAIError("view_range must have exactly 2 elements")
		}

		start = viewRange[0]
		end = viewRange[1]

		if start < 1 {
			start = 1
		}

		if end == -1 || end > total {
			end = total
		}
	}

	if total == 0 {
		return 1, 0, nil
	}

	if end < start {
		return 0, 0, sdk.NewAIError("invalid view_range: end before start")
	}

	return start, end, nil
}
