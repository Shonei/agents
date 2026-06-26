package tools

import (
	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

// WebFetchTool is a provider-executed web-fetch tool. It is declared on the
// request and the provider fetches the page itself, returning the page content
// for the model to use. The SDK never invokes Call on it. Currently supported
// by the OpenRouter provider (mapped to the "openrouter:web_fetch" server
// tool); providers that do not support it simply ignore the declaration.
type WebFetchTool struct{}

func (w *WebFetchTool) Name() string {
	return sdk.ServerToolWebFetch
}

func (w *WebFetchTool) Kind() string {
	return sdk.ServerToolWebFetch
}

func (w *WebFetchTool) Init(_ map[string]string, _ *config.ConfigFactory) {
}
