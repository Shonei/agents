package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

const (
	// Max lines to display without warning
	maxLinesWarningThreshold = 1000
	// Max lines to display at all
	maxLinesHardLimit = 10000
)

// isBinary checks if the file content appears to be binary
func isBinary(data []byte) bool {
	// Check first 8KB for null bytes or high percentage of non-text bytes
	checkSize := 8192
	if len(data) < checkSize {
		checkSize = len(data)
	}

	nullCount := 0
	nonPrintable := 0

	for i := 0; i < checkSize; i++ {
		b := data[i]
		if b == 0 {
			nullCount++
		}
		// Count non-printable characters (excluding common whitespace)
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			nonPrintable++
		}
	}

	// If we find null bytes or >30% non-printable, consider it binary
	if nullCount > 0 || (float64(nonPrintable)/float64(checkSize)) > 0.3 {
		return true
	}

	return false
}

// formatFileSize formats bytes into human-readable format
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// lineNumberWidth calculates the width needed for line numbers
func lineNumberWidth(totalLines int) int {
	width := 1
	for n := totalLines; n >= 10; n /= 10 {
		width++
	}
	if width < 4 {
		width = 4
	}

	return width
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

	// Check if file is binary
	if isBinary(data) {
		return "", sdk.NewAIError(fmt.Sprintf("Cannot display binary file: %s (size: %s). Use a different tool for binary files.", path, formatFileSize(info.Size())))
	}

	lines := strings.Split(string(data), "\n")
	total := len(lines)

	// Handle empty files
	if total == 0 || (total == 1 && lines[0] == "") {
		var b strings.Builder
		fmt.Fprintf(&b, "<filePath>%s</filePath>\n", path)
		fmt.Fprintf(&b, "<fileInfo>\n")
		fmt.Fprintf(&b, "  Size: %s\n", formatFileSize(info.Size()))
		fmt.Fprintf(&b, "  Modified: %s\n", info.ModTime().Format(time.RFC3339))
		fmt.Fprintf(&b, "</fileInfo>\n")
		fmt.Fprintf(&b, "<viewRange>0 lines (empty file)</viewRange>\n")
		fmt.Fprintf(&b, "<content>\n")
		fmt.Fprintf(&b, "  (empty file)\n")
		fmt.Fprintf(&b, "</content>\n")

		return b.String(), nil
	}

	start, end, err := resolveRange(in.ViewRange, total)
	if err != nil {
		return "", err
	}

	// Check if file is very large
	var warnings []string
	if total > maxLinesHardLimit {
		if len(in.ViewRange) == 0 {
			// No range specified, truncate to hard limit
			end = maxLinesHardLimit
			warnings = append(warnings, fmt.Sprintf("⚠ File has %d lines. Truncated to first %d lines. Use view_range parameter to view specific sections.", total, maxLinesHardLimit))
		}
	} else if total > maxLinesWarningThreshold && len(in.ViewRange) == 0 {
		warnings = append(warnings, fmt.Sprintf("⚠ Large file (%d lines). Consider using view_range parameter to view specific sections.", total))
	}

	var b strings.Builder

	// File path
	fmt.Fprintf(&b, "<filePath>%s</filePath>\n", path)

	// File metadata
	fmt.Fprintf(&b, "<fileInfo>\n")
	fmt.Fprintf(&b, "  Size: %s\n", formatFileSize(info.Size()))
	fmt.Fprintf(&b, "  Lines: %d\n", total)
	fmt.Fprintf(&b, "  Modified: %s\n", info.ModTime().Format(time.RFC3339))
	fmt.Fprintf(&b, "</fileInfo>\n")

	// Warnings if any
	if len(warnings) > 0 {
		fmt.Fprintf(&b, "<warnings>\n")
		for _, w := range warnings {
			fmt.Fprintf(&b, "  %s\n", w)
		}
		fmt.Fprintf(&b, "</warnings>\n")
	}

	// View range with context indicators
	fmt.Fprintf(&b, "<viewRange>%d-%d of %d</viewRange>\n", start, end, total)

	// Calculate line number width dynamically
	lineWidth := lineNumberWidth(total)
	lineFormat := fmt.Sprintf("%%%dd  %%s\n", lineWidth)

	fmt.Fprintf(&b, "<content>\n")

	// Context indicator: lines above
	if start > 1 {
		fmt.Fprintf(&b, "  ⋮ (%d lines above)\n", start-1)
	}

	// Display the actual content
	for ln := start; ln <= end && ln <= total; ln++ {
		fmt.Fprintf(&b, lineFormat, ln, lines[ln-1])
	}

	// Context indicator: lines below
	if end < total {
		fmt.Fprintf(&b, "  ⋮ (%d lines below)\n", total-end)
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
