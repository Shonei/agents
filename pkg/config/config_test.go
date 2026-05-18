package config

import (
	"os"
	"path/filepath"
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

func TestNormalizeAndValidateAgents(t *testing.T) {
	planner := Agent{Name: "planner", Model: "gemini-3.1-pro-preview"}
	builder := Agent{Name: "builder", Model: "gemini-3.1-pro-preview"}

	validRouter := func() Agent {
		return Agent{
			Name: "dev",
			Kind: AgentKindRouter,
			Classifier: &ClassifierConfig{
				Model:        "gemini-3.1-pro-preview",
				DefaultRoute: "planner",
			},
			Routes: []RouteConfig{
				{Agent: "planner", When: "exploring"},
				{Agent: "builder", When: "implementing"},
			},
		}
	}

	tests := []struct {
		name       string
		agents     map[string]Agent
		wantErrSub string
		check      func(t *testing.T, c *Config)
	}{
		{
			name: "valid router fills defaults",
			agents: map[string]Agent{
				"planner": planner,
				"builder": builder,
				"dev":     validRouter(),
			},
			check: func(t *testing.T, c *Config) {
				router := c.Agents["dev"]
				assert.Equal(t, AgentKindRouter, router.Kind)
				assert.Equal(t, ClassifierStrategySticky, router.Classifier.Strategy)
				assert.InDelta(t, DefaultConfidenceThreshold, router.Classifier.ConfidenceThreshold, 0.0001)

				plain := c.Agents["planner"]
				assert.Equal(t, AgentKindAgent, plain.Kind, "missing kind should normalize to %q", AgentKindAgent)
			},
		},
		{
			name: "unknown kind fails",
			agents: map[string]Agent{
				"weird": {Name: "weird", Kind: "supervisor"},
			},
			wantErrSub: "unknown kind",
		},
		{
			name: "router without classifier fails",
			agents: map[string]Agent{
				"planner": planner,
				"builder": builder,
				"dev": {
					Name:   "dev",
					Kind:   AgentKindRouter,
					Routes: []RouteConfig{{Agent: "planner"}, {Agent: "builder"}},
				},
			},
			wantErrSub: "missing classifier",
		},
		{
			name: "router default_route not in routes fails",
			agents: map[string]Agent{
				"planner": planner,
				"builder": builder,
				"dev": func() Agent {
					r := validRouter()
					r.Classifier.DefaultRoute = "missing"

					return r
				}(),
			},
			wantErrSub: "default_route",
		},
		{
			name: "router pointing to unknown agent fails",
			agents: map[string]Agent{
				"planner": planner,
				"dev": func() Agent {
					r := validRouter()
					r.Routes = []RouteConfig{
						{Agent: "planner"},
						{Agent: "ghost"},
					}

					return r
				}(),
			},
			wantErrSub: "unknown agent",
		},
		{
			name: "nested router fails",
			agents: map[string]Agent{
				"planner": planner,
				"builder": builder,
				"inner":   validRouter(),
				"dev": {
					Name: "dev",
					Kind: AgentKindRouter,
					Classifier: &ClassifierConfig{
						Model:        "gemini-3.1-pro-preview",
						DefaultRoute: "planner",
					},
					Routes: []RouteConfig{
						{Agent: "planner"},
						{Agent: "inner"},
					},
				},
			},
			wantErrSub: "nested routers",
		},
		{
			name: "router with single route fails",
			agents: map[string]Agent{
				"planner": planner,
				"dev": {
					Name: "dev",
					Kind: AgentKindRouter,
					Classifier: &ClassifierConfig{
						Model:        "gemini-3.1-pro-preview",
						DefaultRoute: "planner",
					},
					Routes: []RouteConfig{{Agent: "planner"}},
				},
			},
			wantErrSub: "at least 2 routes",
		},
		{
			name: "router with duplicate route fails",
			agents: map[string]Agent{
				"planner": planner,
				"dev": {
					Name: "dev",
					Kind: AgentKindRouter,
					Classifier: &ClassifierConfig{
						Model:        "gemini-3.1-pro-preview",
						DefaultRoute: "planner",
					},
					Routes: []RouteConfig{
						{Agent: "planner"},
						{Agent: "planner"},
					},
				},
			},
			wantErrSub: "declared more than once",
		},
		{
			name: "router with unsupported strategy fails",
			agents: map[string]Agent{
				"planner": planner,
				"builder": builder,
				"dev": func() Agent {
					r := validRouter()
					r.Classifier.Strategy = "per_turn"

					return r
				}(),
			},
			wantErrSub: "not supported",
		},
		{
			name: "router with out-of-range threshold fails",
			agents: map[string]Agent{
				"planner": planner,
				"builder": builder,
				"dev": func() Agent {
					r := validRouter()
					r.Classifier.ConfidenceThreshold = 2.0

					return r
				}(),
			},
			wantErrSub: "confidence_threshold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Agents: tt.agents}
			err := normalizeAndValidateAgents(cfg)

			if tt.wantErrSub != "" {
				assert.Error(t, err)
				if err != nil {
					assert.Contains(t, err.Error(), tt.wantErrSub)
				}

				return
			}

			assert.NoError(t, err)
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestReadOrCreateConfigRunsRouterValidation(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")

	yaml := `agents:
  dev:
    kind: router
    classifier:
      model: gemini-3.1-pro-preview
      default_route: ghost
    routes:
      - agent: ghost
        when: never
      - agent: phantom
        when: never
`

	assert.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	_, err := readOrCreateConfig(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}
