package utils

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/svent/sift/gitignore"
)

// SkipDirs contains directory names that should always be skipped during file collection.
var SkipDirs = map[string]bool{
	".git":           true,
	"node_modules":   true,
	".svn":           true,
	".hg":            true,
	"vendor":         true,
	"__pycache__":    true,
	".venv":          true,
	"venv":           true,
	".idea":          true,
	".vscode":        true,
	"dist":           true,
	"build":          true,
	".next":          true,
	".nuxt":          true,
	"target":         true,
	".terraform":     true,
	".cache":         true,
	"coverage":       true,
	".nyc_output":    true,
	".pytest_cache":  true,
	".mypy_cache":    true,
	".tox":           true,
	"eggs":           true,
	".eggs":          true,
	"*.egg-info":     true,
	".gradle":        true,
	".m2":            true,
	"Pods":           true,
	".dart_tool":     true,
	".pub-cache":     true,
}

type File struct {
	Path    string
	Content string
}

// CollectFiles walks the provided fullPath directory and returns files suitable for upload.
// It respects .gitignore rules in that directory. If dryRun is true, file contents are omitted.
// It skips common directories like .git, node_modules, vendor, etc.
func CollectFiles(fullPath string, dryRun bool) ([]File, error) {
	checker := gitignore.NewChecker()

	gitignorePath := filepath.Join(fullPath, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		// .gitignore exists, use it
		if err := checker.LoadBasePath(fullPath); err != nil {
			return nil, fmt.Errorf("failed to load .gitignore: %w", err)
		}
	}

	files := make([]File, 0, 128)
	err := filepath.WalkDir(fullPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Check if file should be ignored by .gitignore
		if checker != nil {
			fileInfo, statErr := entry.Info()
			if statErr != nil {
				return statErr
			}

			if checker.Check(path, fileInfo) {
				if entry.IsDir() {
					return filepath.SkipDir
				}

				return nil // skip this file
			}
		}

		if entry.IsDir() {
			if SkipDirs[entry.Name()] {
				return filepath.SkipDir
			}

			return nil
		}

		rel, relErr := filepath.Rel(fullPath, path)
		if relErr != nil {
			return relErr
		}

		if dryRun {
			files = append(files, File{Path: filepath.ToSlash(rel), Content: ""})

			return nil
		}

		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("failed to read file %s: %w", path, readErr)
		}

		files = append(files, File{Path: filepath.ToSlash(rel), Content: string(b)})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}
