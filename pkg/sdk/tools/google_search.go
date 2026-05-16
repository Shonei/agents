package tools

import (
	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

// GoogleSearchTool is a Gemini-executed grounding tool. It is declared on
// the request and the model performs the search itself, returning the
// grounded answer plus groundingMetadata. The SDK never invokes Call on it.
type GoogleSearchTool struct{}

func (g *GoogleSearchTool) Name() string {
	return sdk.ServerToolGoogleSearch
}

func (g *GoogleSearchTool) Kind() string {
	return sdk.ServerToolGoogleSearch
}

func (g *GoogleSearchTool) Init(_ map[string]string, _ *config.ConfigFactory) {
}
