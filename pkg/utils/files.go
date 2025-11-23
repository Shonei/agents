package utils

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/svent/sift/gitignore"
)

type File struct {
	Path    string
	Content string
}

// CollectFiles walks the provided fullPath directory and returns files suitable for upload.
// It respects .gitignore rules in that directory. If dryRun is true, file contents are omitted.
func CollectFiles(fullPath string, dryRun bool) ([]File, error) {
	var checker = gitignore.NewChecker()

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
