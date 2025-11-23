package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ViewFileTool struct{}

func (t *ViewFileTool) Name() string { return "view_file" }

func (t *ViewFileTool) Description() string {
	return "Reads a file and returns its contents with line numbers. Supports viewing a line range and regex search with surrounding context. Only works for files; use list_dir for directories."
}

func (t *ViewFileTool) Init(config map[string]string) {
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
				"minItems": 2,
				"maxItems": 2,
			},
			"search_query_regex": map[string]interface{}{
				"type":        "string",
				"description": "Optional single-line regex. When provided, only matching lines plus context are returned.",
				"example":     "MyType",
			},
			"case_sensitive": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether the regex search is case-sensitive. Defaults to false.",
			},
			"context_lines_before": map[string]interface{}{
				"type":        "integer",
				"description": "Number of context lines to include before each match. Defaults to 5.",
			},
			"context_lines_after": map[string]interface{}{
				"type":        "integer",
				"description": "Number of context lines to include after each match. Defaults to 5.",
			},
		},
		"required": []interface{}{"path"},
	}
}

type ViewFileToolInput struct {
	Path               string `json:"path"`
	ViewRange          []int  `json:"view_range"`
	SearchQueryRegex   string `json:"search_query_regex"`
	CaseSensitive      bool   `json:"case_sensitive"`
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
		// if no such file return not found to the AI

		return fmt.Sprintf("File not found: %s", path), nil
	}

	if info.IsDir() {
		return fmt.Sprintf("Path is a directory: %s", path), nil
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

	if in.SearchQueryRegex != "" {
		before, after := in.ContextLinesBefore, in.ContextLinesAfter
		if before <= 0 {
			before = 5
		}
		if after <= 0 {
			after = 5
		}

		pattern := in.SearchQueryRegex
		if !in.CaseSensitive {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return "", fmt.Errorf("invalid regex: %w", err)
		}

		ranges := collectMatchRanges(re, lines, start, end, before, after)
		if len(ranges) == 0 {
			return fmt.Sprintf("No matches for %q in %s (lines %d-%d).", in.SearchQueryRegex, path, start, end), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "File: %s\n", path)
		fmt.Fprintf(&b, "Search: %q (lines %d-%d)\n\n", in.SearchQueryRegex, start, end)

		prevEnd := 0
		for _, r := range ranges {
			if prevEnd != 0 && r.start > prevEnd+1 {
				b.WriteString("...\n")
			}
			for ln := r.start; ln <= r.end && ln <= total; ln++ {
				fmt.Fprintf(&b, "%6d  %s\n", ln, lines[ln-1])
			}
			prevEnd = r.end
		}
		return b.String(), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "File: %s\n", path)
	fmt.Fprintf(&b, "Lines: %d-%d (total %d)\n\n", start, end, total)
	for ln := start; ln <= end && ln <= total; ln++ {
		fmt.Fprintf(&b, "%6d  %s\n", ln, lines[ln-1])
	}
	return b.String(), nil
}

func resolveRange(viewRange []int, total int) (int, int, error) {
	start, end := 1, total
	if len(viewRange) > 0 {
		if len(viewRange) != 2 {
			return 0, 0, fmt.Errorf("view_range must have exactly 2 elements")
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
		return 0, 0, fmt.Errorf("invalid view_range: end before start")
	}
	return start, end, nil
}

func collectMatchRanges(re *regexp.Regexp, lines []string, start, end, before, after int) []lineRange {
	var ranges []lineRange
	var current *lineRange
	for ln := start; ln <= end && ln <= len(lines); ln++ {
		if !re.MatchString(lines[ln-1]) {
			continue
		}
		s := ln - before
		if s < start {
			s = start
		}
		e := ln + after
		if e > end {
			e = end
		}
		if current != nil && s <= current.end+1 {
			if e > current.end {
				current.end = e
			}
		} else {
			r := lineRange{start: s, end: e}
			ranges = append(ranges, r)
			current = &ranges[len(ranges)-1]
		}
	}
	return ranges
}
