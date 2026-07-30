// Package provider maps an agent's configured model string onto a concrete
// provider-backed sdk.Agent. It exists so that every entry point (engage,
// context previews, future commands) resolves models the same way instead of
// each re-implementing the dispatch.
package provider

import (
	"fmt"
	"strings"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/gemini"
	"github.com/Shonei/agents/pkg/sdk/openrouter"
)

// New constructs the right provider-backed sdk.Agent for the given agent
// config based on its model string. OpenRouter model IDs are namespaced as
// "<provider>/<model>" (and so contain a slash); native Gemini IDs contain
// "gemini". Anything else is unsupported.
func New(c *config.ConfigFactory, agent config.Agent) (sdk.Agent, error) {
	switch {
	case strings.Contains(agent.Model, "/"):
		return newOpenRouter(c, agent), nil
	case strings.Contains(strings.ToLower(agent.Model), "gemini"):
		return newGemini(c, agent), nil
	default:
		return nil, fmt.Errorf("unsupported model: %s", agent.Model)
	}
}

func newGemini(c *config.ConfigFactory, agent config.Agent) sdk.Agent {
	opts := []gemini.AgentOption{
		gemini.WithAPIKey(c.GetGeminiAPIKey()),
		gemini.WithModel(agent.Model),
	}

	if agent.ThinkingEnabled {
		opts = append(opts, gemini.WithThinking())
	}

	if agent.MaxTokens != nil {
		opts = append(opts, gemini.WithMaxTokens(*agent.MaxTokens))
	}

	if agent.MaxContextTokens != nil {
		opts = append(opts, gemini.WithMaxContextTokens(*agent.MaxContextTokens))
	}

	if agent.MaxContextTurns != nil {
		opts = append(opts, gemini.WithMaxContextTurns(*agent.MaxContextTurns))
	}

	if agent.Temperature != nil {
		opts = append(opts, gemini.WithTemperature(*agent.Temperature))
	}

	if agent.ResponseModalities != nil {
		opts = append(opts, gemini.WithResponseModalities(agent.ResponseModalities))
	}

	return gemini.NewAgent(opts...)
}

func newOpenRouter(c *config.ConfigFactory, agent config.Agent) sdk.Agent {
	opts := []openrouter.AgentOption{
		openrouter.WithAPIKey(c.GetOpenRouterAPIKey()),
		openrouter.WithModel(agent.Model),
	}

	if agent.ThinkingEnabled {
		opts = append(opts, openrouter.WithThinking())
	}

	if agent.MaxTokens != nil {
		opts = append(opts, openrouter.WithMaxTokens(*agent.MaxTokens))
	}

	if agent.MaxContextTokens != nil {
		opts = append(opts, openrouter.WithMaxContextTokens(*agent.MaxContextTokens))
	}

	if agent.MaxContextTurns != nil {
		opts = append(opts, openrouter.WithMaxContextTurns(*agent.MaxContextTurns))
	}

	if agent.Temperature != nil {
		opts = append(opts, openrouter.WithTemperature(*agent.Temperature))
	}

	return openrouter.NewAgent(opts...)
}
