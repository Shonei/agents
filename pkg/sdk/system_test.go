package sdk

import (
	"os"
	"slices"
	"testing"
)

func Test_SystemPromptBuilder(t *testing.T) {
	builder := SystemPromptBuilder{}

	functions := builder.GetAvailableFunctions()
	expect := []string{
		"Cwd() string",
		"DirList(int) string",
		"Now() string",
		"RepoContext() string",
	}

	for _, f := range expect {
		if !slices.Contains(functions, f) {
			t.Errorf("expected function %s not found", f)
		}
	}
}

func Test_RenderPrompt_Cwd(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	rendered, err := RenderPrompt("{{ .Cwd }}")
	if err != nil {
		t.Fatalf("RenderPrompt returned error: %v", err)
	}

	if rendered != cwd {
		t.Errorf("expected Cwd to be %q, got %q", cwd, rendered)
	}
}

func Test_RepoContext(t *testing.T) {
	// Create a temporary directory to run the test in
	tempDir, err := os.MkdirTemp("", "repocontext_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to the temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	builder := SystemPromptBuilder{}

	// Test 1: No files exist
	if got := builder.RepoContext(); got != "" {
		t.Errorf("expected empty string when no files exist, got %q", got)
	}

	// Test 2: Create a lower priority file
	err = os.WriteFile(".cursorrules", []byte("cursor rules content"), 0o600)
	if err != nil {
		t.Fatalf("failed to write .cursorrules: %v", err)
	}

	expectedCursor := "<repository_instructions source=\".cursorrules\">\ncursor rules content\n</repository_instructions>"
	if got := builder.RepoContext(); got != expectedCursor {
		t.Errorf("expected %q, got %q", expectedCursor, got)
	}

	// Test 3: Create a higher priority file
	err = os.WriteFile("AGENTS.md", []byte("agents md content"), 0o600)
	if err != nil {
		t.Fatalf("failed to write AGENTS.md: %v", err)
	}

	expectedAgents := "<repository_instructions source=\"AGENTS.md\">\nagents md content\n</repository_instructions>"
	if got := builder.RepoContext(); got != expectedAgents {
		t.Errorf("expected %q, got %q", expectedAgents, got)
	}
}
