package tools

import (
	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

// WebSearchTool is a provider-executed web-search tool. It is declared on the
// request and the provider performs the search itself, returning a grounded,
// cited answer. The SDK never invokes Call on it. Currently supported by the
// OpenRouter provider (mapped to the "openrouter:web_search" server tool);
// providers that do not support it simply ignore the declaration.
type WebSearchTool struct{}

func (w *WebSearchTool) Name() string {
	return sdk.ServerToolWebSearch
}

func (w *WebSearchTool) Kind() string {
	return sdk.ServerToolWebSearch
}

func (w *WebSearchTool) Init(_ map[string]string, _ *config.ConfigFactory) {
}
