package sdk

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Shonei/agents/pkg/config"
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

	rendered, err := RenderPrompt("{{ .Cwd }}", nil)
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

type mockContributor struct {
	key  string
	data any
}

func (m *mockContributor) Name() string                                                       { return "mock" }
func (m *mockContributor) Description() string                                                { return "mock" }
func (m *mockContributor) Init(config map[string]string, configFactory *config.ConfigFactory) {}
func (m *mockContributor) InputSchema() map[string]interface{}                                { return nil }
func (m *mockContributor) Call(input map[string]interface{}) (interface{}, error)             { return nil, nil }
func (m *mockContributor) TemplateKey() string                                                { return m.key }
func (m *mockContributor) TemplateData() any                                                  { return m.data }

func TestRenderPrompt_ValidationAndCapitalization(t *testing.T) {
	// 1. Capitalization test (lowercase "my_var" becomes "My_var")
	contrib1 := &mockContributor{
		key:  "my_var",
		data: func() string { return "capitalized_success" },
	}
	rendered, err := RenderPrompt("{{ .My_var }}", []AITool{contrib1})
	if err != nil {
		t.Fatalf("expected no error for capitalized key rendering, got: %v", err)
	}
	if rendered != "capitalized_success" {
		t.Errorf("expected 'capitalized_success', got %q", rendered)
	}

	// 2. Conflict with existing method test ("cwd" becomes "Cwd", conflicts with Cwd() method)
	contrib2 := &mockContributor{
		key:  "cwd",
		data: func() string { return "conflict" },
	}
	_, err = RenderPrompt("{{ .Cwd }}", []AITool{contrib2})
	if err == nil {
		t.Error("expected error due to conflict with existing Cwd method, got nil")
	} else if !strings.Contains(err.Error(), "conflicts with existing SystemPromptBuilder") {
		t.Errorf("expected conflict error message, got: %v", err)
	}

	// 3. Duplicate key test (both lowercase and uppercase versions capitalized to "Custom")
	contrib3a := &mockContributor{
		key:  "custom",
		data: "val1",
	}
	contrib3b := &mockContributor{
		key:  "Custom",
		data: "val2",
	}
	_, err = RenderPrompt("{{ .Custom }}", []AITool{contrib3a, contrib3b})
	if err == nil {
		t.Error("expected error due to duplicate template keys, got nil")
	} else if !strings.Contains(err.Error(), "duplicate template key") {
		t.Errorf("expected duplicate error message, got: %v", err)
	}
}
