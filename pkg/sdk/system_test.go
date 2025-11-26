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
