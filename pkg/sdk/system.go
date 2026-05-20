package sdk

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"text/template"
	"time"
)

// SystemPromptBuilder is a type passed to a go template that includes helper functions for building system prompts
// it includes tools like time and cwd
type SystemPromptBuilder struct{}

func (s *SystemPromptBuilder) Cwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	return cwd
}

func (s *SystemPromptBuilder) Now() string {
	return time.Now().Format(time.RFC3339)
}

func (s *SystemPromptBuilder) OSInfo() string {
	info := fmt.Sprintf("OS: %s, Architecture: %s", runtime.GOOS, runtime.GOARCH)

	return info
}

func (s *SystemPromptBuilder) Shell() string {
	return os.Getenv("SHELL")
}

func (s *SystemPromptBuilder) DirList(depth int) string {
	dirs, err := listDir(".", depth, depth)
	if err != nil {
		return ""
	}

	sb := strings.Builder{}
	sb.WriteString("<currentDir>" + s.Cwd() + "</currentDir>\n")
	sb.WriteString("<dirList>\n")
	for _, dir := range dirs {
		fmt.Fprintf(&sb, "- %s\n", dir)
	}
	sb.WriteString("</dirList>")

	return sb.String()
}

func (s *SystemPromptBuilder) RepoContext() string {
	files := []string{
		"AGENTS.md",
		".cursorrules",
		"CURSOR.md",
		"COPILOT.md",
		".github/copilot-instructions.md",
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err == nil {
			return fmt.Sprintf("<repository_instructions source=\"%s\">\n%s\n</repository_instructions>", file, string(content))
		}
	}

	return ""
}

func (s *SystemPromptBuilder) Plan() string {
	return GlobalPlan.Format()
}

// GetAvailableFunctions returns an array of methods attached to this struct
// This is only used for the human documentation when building prompts
// and using reflections is fun :shrug:
func (s *SystemPromptBuilder) GetAvailableFunctions() []string {
	t := reflect.TypeOf(s)
	methods := t.NumMethod()

	availableMethods := []string{}

	for i := 0; i < methods; i++ {
		method := t.Method(i)
		if method.Name == "GetAvailableFunctions" {
			continue // skip method used for docs
		}

		funcSignature := strings.Builder{}
		funcSignature.WriteString(method.Name)
		funcSignature.WriteString("(")

		funcType := method.Func.Type()
		for j := 1; j < funcType.NumIn(); j++ {
			if j > 1 {
				funcSignature.WriteString(", ")
			}
			funcSignature.WriteString(funcType.In(j).String())
		}

		funcSignature.WriteString(") ")

		for j := 0; j < funcType.NumOut(); j++ {
			if j > 0 {
				funcSignature.WriteString(", ")
			}
			funcSignature.WriteString(funcType.Out(j).String())
		}

		availableMethods = append(availableMethods, funcSignature.String())
	}

	return availableMethods
}

func RenderPrompt(prompt string) (string, error) {
	pt, err := template.New("system_prompt").Parse(prompt)
	if err != nil {
		return "", fmt.Errorf("failed to parse prompt: %w", err)
	}

	var b strings.Builder
	if err := pt.Execute(&b, &SystemPromptBuilder{}); err != nil {
		return "", fmt.Errorf("failed to render prompt: %w", err)
	}

	return b.String(), nil
}

func listDir(dir string, depth int, maxDepth int) ([]string, error) {
	if depth < 0 {
		return nil, fmt.Errorf("invalid depth: %d", depth)
	}

	if depth == 0 {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			if depth < maxDepth {
				subEntries, err := listDir(entry.Name(), depth-1, maxDepth)
				if err != nil {
					return nil, err
				}
				result = append(result, subEntries...)
			}

			continue
		}

		result = append(result, entry.Name())
	}

	return result, nil
}
