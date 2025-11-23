package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAPIKey(t *testing.T) {
	c := NewConfigFactory()
	c.Config = &Config{
		ClaudeAPIKey: "global-claude-key",
		GeminiAPIKey: "global-gemini-key",
	}

	tests := []struct {
		name     string
		agent    Agent
		envVars  map[string]string
		expected string
	}{
		{
			name: "Claude Env Var set",
			agent: Agent{
				Model: "claude-3-5-sonnet",
			},
			envVars: map[string]string{
				"ANTHROPIC_API_KEY": "env-claude-key",
			},
			expected: "env-claude-key",
		},
		{
			name: "Claude Global Config",
			agent: Agent{
				Model: "claude-3-5-sonnet",
			},
			expected: "global-claude-key",
		},
		{
			name: "Gemini Env Var set",
			agent: Agent{
				Model: "gemini-2.0-flash",
			},
			envVars: map[string]string{
				"GEMINI_API_KEY": "env-gemini-key",
			},
			expected: "env-gemini-key",
		},
		{
			name: "Gemini Global Config",
			agent: Agent{
				Model: "gemini-2.0-flash",
			},
			expected: "global-gemini-key",
		},
		{
			name: "Unknown Model",
			agent: Agent{
				Model: "unknown-model",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			apiKey := c.GetAPIKey(tt.agent)
			assert.Equal(t, tt.expected, apiKey)
		})
	}
}
