package sdk

import (
	"os"
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
