package utils

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/svent/sift/gitignore"
)

// binarySniffSize is the number of bytes inspected when deciding whether a
// file is binary. Git uses 8000; we follow the same convention.
const binarySniffSize = 8000

// SkipDirs contains directory names that should always be skipped during file collection.
var SkipDirs = map[string]bool{
	".git":          true,
	"node_modules":  true,
	".svn":          true,
	".hg":           true,
	"vendor":        true,
	"__pycache__":   true,
	".venv":         true,
	"venv":          true,
	".idea":         true,
	".vscode":       true,
	"dist":          true,
	"build":         true,
	".next":         true,
	".nuxt":         true,
	"target":        true,
	".terraform":    true,
	".cache":        true,
	"coverage":      true,
	".nyc_output":   true,
	".pytest_cache": true,
	".mypy_cache":   true,
	".tox":          true,
	"eggs":          true,
	".eggs":         true,
	"*.egg-info":    true,
	".gradle":       true,
	".m2":           true,
	"Pods":          true,
	".dart_tool":    true,
	".pub-cache":    true,
}

type File struct {
	Path    string
	Content string
}

// IsBinaryFile reports whether the file at path looks like binary data. The
// detection mirrors the heuristic used by Git: read up to binarySniffSize
// bytes and treat the file as binary if it contains a NUL byte.
func IsBinaryFile(path string) (bool, error) {
	f, err := os.Open(path) // #nosec G304 - caller-supplied path is intentional
	if err != nil {
		return false, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, binarySniffSize)

	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("failed to read file: %w", err)
	}

	return bytes.IndexByte(buf[:n], 0) >= 0, nil
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

		isBin, binErr := IsBinaryFile(path)
		if binErr != nil {
			return fmt.Errorf("failed to detect binary file %s: %w", path, binErr)
		}

		if isBin {
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
