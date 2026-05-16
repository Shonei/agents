package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGeminiAPIKey(t *testing.T) {
	c := NewConfigFactory()
	c.Config = &Config{
		GeminiAPIKey: "global-gemini-key",
	}

	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name: "Env var set",
			envVars: map[string]string{
				"GEMINI_API_KEY": "env-gemini-key",
			},
			expected: "env-gemini-key",
		},
		{
			name:     "Global config fallback",
			expected: "global-gemini-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			apiKey := c.GetGeminiAPIKey()
			assert.Equal(t, tt.expected, apiKey)
		})
	}
}
