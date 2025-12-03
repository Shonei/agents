package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

type ListDirTool struct{}

func (t *ListDirTool) Name() string { return "list_dir" }

func (t *ListDirTool) Description() string {
	return "Explores a directory and returns metadata for files and subdirectories, including relative paths, file types, sizes, and child counts. Can optionally recurse into subdirectories."
}

func (t *ListDirTool) Init(config map[string]string, _ *config.ConfigFactory) {
}

func (t *ListDirTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Directory to explore. Can be relative to the current working directory or absolute. Defaults to current directory if omitted.",
				"example":     "./go/agents",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to recursively list subdirectories. Defaults to true.",
			},
			"max_depth": map[string]interface{}{
				"type":        "integer",
				"description": "Optional maximum recursion depth (>=1). If omitted or 0, a safe default is used to avoid huge listings.",
				"example":     5,
			},
		},
	}
}

type ListDirToolInput struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
	MaxDepth  int    `json:"max_depth"`
}

type DirectoryEntry struct {
	Path       string           `json:"path"`
	Type       string           `json:"type"`
	Size       int64            `json:"size,omitempty"`
	ChildCount int              `json:"child_count,omitempty"`
	Children   []DirectoryEntry `json:"children,omitempty"`
}

const defaultListDirMaxDepth = 5

func (t *ListDirTool) Call(input map[string]interface{}) (interface{}, error) {
	var toolInput ListDirToolInput
	if err := mapstruct(input, &toolInput); err != nil {
		return "", err
	}

	if toolInput.Path == "" {
		toolInput.Path = "."
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	if !filepath.IsAbs(toolInput.Path) {
		toolInput.Path = filepath.Join(cwd, toolInput.Path)
	}

	rootAbs, err := filepath.Abs(toolInput.Path)
	if err != nil {
		return "", sdk.NewAIError(fmt.Sprintf("failed to resolve path: %s", toolInput.Path)).WithReason(err)
	}

	info, err := os.Stat(rootAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", sdk.NewAIError(fmt.Sprintf("path does not exist: %s", rootAbs))
		}

		return "", fmt.Errorf("failed to stat path: %w", err)
	}

	if !info.IsDir() {
		return "", sdk.NewAIError(fmt.Sprintf("path is not a directory: %s", rootAbs))
	}

	recursive := toolInput.Recursive
	maxDepth := toolInput.MaxDepth
	if maxDepth < 0 {
		maxDepth = 0
	}

	if recursive && maxDepth == 0 {
		maxDepth = defaultListDirMaxDepth
	}

	entries, err := listDirectoryEntries(rootAbs, rootAbs, recursive, maxDepth, 1)
	if err != nil {
		return "", err
	}

	return map[string]interface{}{
		"root":    rootAbs,
		"entries": entries,
	}, nil
}

func listDirectoryEntries(root, dir string, recursive bool, maxDepth, depth int) ([]DirectoryEntry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	result := make([]DirectoryEntry, 0, len(dirEntries))

	for _, entry := range dirEntries {
		fullPath := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("failed to get info for %s: %w", fullPath, err)
		}

		rel, err := filepath.Rel(root, fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to compute relative path for %s: %w", fullPath, err)
		}

		e := DirectoryEntry{Path: filepath.ToSlash(rel)}

		if info.IsDir() {
			e.Type = "directory"

			childrenEntries, err := os.ReadDir(fullPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read directory %s: %w", fullPath, err)
			}
			e.ChildCount = len(childrenEntries)

			if recursive && (maxDepth == 0 || depth < maxDepth) {
				children, err := listDirectoryEntries(root, fullPath, recursive, maxDepth, depth+1)
				if err != nil {
					return nil, err
				}
				e.Children = children
			}
		} else {
			e.Type = "file"
			e.Size = info.Size()
		}

		result = append(result, e)
	}

	return result, nil
}
