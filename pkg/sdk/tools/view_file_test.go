package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestViewFile_readsFile(t *testing.T) {
	viewTool := &ViewFileTool{}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	path := filepath.Join(cwd, "testfiles", "view_file.txt")

	if err := os.WriteFile(path, []byte(testContest), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	input := map[string]interface{}{
		"path": path,
	}

	result, err := viewTool.Call(input)
	if err != nil {
		t.Fatalf("failed to view file: %v", err)
	}

	t.Log(result)
}
