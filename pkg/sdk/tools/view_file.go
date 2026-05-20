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
	return "Reads a file from the local filesystem.\n" +
		"\n" +
		"Usage:\n" +
		"- You can optionally specify `offset` and `limit` (handy for long files), but it's recommended to read the whole file by not providing these parameters.\n" +
		"- Lines in the output are numbered starting at 1, using the following format: LINE_NUMBER|LINE_CONTENT.\n" +
		"- You have the capability to call multiple tools in a single response. It is always better to speculatively read multiple files as a batch that are potentially useful.\n" +
		"- If you only need a slice (e.g. after `rg -n` or `grep -n` returns a line number), use `offset` and `limit` to read around it — not `sed -n 'N,Mp'`.\n" +
		"- If the file is binary, an error will be returned.\n" +
		"- If the file is larger than 10000 lines and no slice is requested, the output is truncated to the first 10000 lines with a warning.\n" +
		"- If you read a file that exists but has empty contents, you will receive a single-line header indicating the file is empty.\n" +
		"- For directories, use `list_dir`."
}

func (t *ViewFileTool) Init(_ map[string]string, _ *config.ConfigFactory) {
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
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "The line number to start reading from (1-indexed). Only provide if the file is too large to read at once, or if you only need a slice.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "The number of lines to read. Only provide if the file is too large to read at once, or if you only need a slice.",
			},
		},
		"required": []interface{}{"path"},
	}
}

type ViewFileToolInput struct {
	Path   string `json:"path"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

const (
	// Max lines to display without warning
	maxLinesWarningThreshold = 1000
	// Max lines to display at all (without offset/limit)
	maxLinesHardLimit = 10000
)

// isBinary checks if the file content appears to be binary.
func isBinary(data []byte) bool {
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

		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			nonPrintable++
		}
	}

	if nullCount > 0 || (float64(nonPrintable)/float64(checkSize)) > 0.3 {
		return true
	}

	return false
}

// formatFileSize formats bytes into human-readable format.
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

// lineNumberWidth calculates the width needed for line numbers.
// Pads to a minimum of 6 so the gutter looks consistent across files
// and matches the conventional `      N|content` layout that downstream
// LLMs are most familiar with.
func lineNumberWidth(totalLines int) int {
	width := 1
	for n := totalLines; n >= 10; n /= 10 {
		width++
	}

	if width < 6 {
		width = 6
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

	// displayPath is what the caller sees echoed back. We deliberately
	// preserve the path they passed in (relative or absolute) so the
	// tool's output doesn't leak the local absolute path when the agent
	// asked for a project-relative one. Internal stat/read still uses
	// the resolved absolute path.
	displayPath := in.Path

	path := in.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
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

	if isBinary(data) {
		return "", sdk.NewAIError(fmt.Sprintf("Cannot display binary file: %s (size: %s). Use a different tool for binary files.", path, formatFileSize(info.Size())))
	}

	lines := strings.Split(string(data), "\n")
	total := len(lines)

	// Empty file: collapse to a single-line header. Same shape as the
	// non-empty case, just without the content block.
	if total == 0 || (total == 1 && lines[0] == "") {
		return fmt.Sprintf("%s (%s, modified %s) — empty file\n",
			displayPath,
			formatFileSize(info.Size()),
			info.ModTime().Format(time.RFC3339),
		), nil
	}

	sliceProvided := in.Offset > 0 || in.Limit > 0

	start, end, err := resolveSlice(in.Offset, in.Limit, total)
	if err != nil {
		return "", err
	}

	var warnings []string

	switch {
	case !sliceProvided && total > maxLinesHardLimit:
		end = maxLinesHardLimit
		warnings = append(warnings, fmt.Sprintf("⚠ File has %d lines; output truncated to first %d. Use `offset`/`limit` to view further sections.", total, maxLinesHardLimit))
	case !sliceProvided && total > maxLinesWarningThreshold:
		warnings = append(warnings, fmt.Sprintf("⚠ Large file (%d lines). Consider using `offset`/`limit` to view a specific section.", total))
	}

	var b strings.Builder

	fmt.Fprintf(&b, "%s (%s, %d lines, modified %s) — lines %d-%d of %d\n",
		displayPath,
		formatFileSize(info.Size()),
		total,
		info.ModTime().Format(time.RFC3339),
		start, end, total,
	)

	for _, w := range warnings {
		fmt.Fprintln(&b, w)
	}

	b.WriteString("\n")

	lineWidth := lineNumberWidth(total)
	lineFormat := fmt.Sprintf("%%%dd|%%s\n", lineWidth)

	if start > 1 {
		fmt.Fprintf(&b, "  ⋮ (%d lines above)\n", start-1)
	}

	for ln := start; ln <= end && ln <= total; ln++ {
		fmt.Fprintf(&b, lineFormat, ln, lines[ln-1])
	}

	if end < total {
		fmt.Fprintf(&b, "  ⋮ (%d lines below)\n", total-end)
	}

	return b.String(), nil
}

// resolveSlice converts the (offset, limit) input pair into a concrete
// 1-based inclusive [start, end] window over a file of `total` lines.
// Offset 0 or unset means "start at line 1". Limit 0 or unset means
// "read to end of file".
func resolveSlice(offset, limit, total int) (int, int, error) {
	if total == 0 {
		return 1, 0, nil
	}

	start := offset
	if start < 1 {
		start = 1
	}

	if start > total {
		return 0, 0, sdk.NewAIError(fmt.Sprintf("offset %d is past end of file (%d lines)", offset, total))
	}

	end := total
	if limit > 0 {
		end = start + limit - 1
		if end > total {
			end = total
		}
	}

	return start, end, nil
}
