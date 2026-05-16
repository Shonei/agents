package tools

import (
	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

// URLContextTool enables Gemini's URL context server-side tool. When this
// tool is declared, Gemini will fetch URLs it finds in the prompt and ground
// the response on their contents. The SDK never invokes Call on it.
type URLContextTool struct{}

func (u *URLContextTool) Name() string {
	return sdk.ServerToolURLContext
}

func (u *URLContextTool) Kind() string {
	return sdk.ServerToolURLContext
}

func (u *URLContextTool) Init(_ map[string]string, _ *config.ConfigFactory) {
}
