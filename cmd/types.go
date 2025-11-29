package cmd

import (
	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/audit"
	"github.com/Shonei/agents/pkg/sdk/claude"
	"github.com/Shonei/agents/pkg/sdk/gemini"
	"github.com/Shonei/agents/pkg/sdk/tools"
)

const (
	Claude45 = claude.ModelClaude45
	Gemini3  = gemini.ModelGemini3
)

func Models() map[string]func(config.Agent, string, *audit.Audit) *sdk.AI {
	return map[string]func(config.Agent, string, *audit.Audit) *sdk.AI{
		Claude45: func(agent config.Agent, apiKey string, audit *audit.Audit) *sdk.AI {
			return sdk.NewAI(claude.NewAgent(
				claude.WithAPIKey(apiKey),
				claude.WithModel(claude.ModelClaude45),
			), audit)
		},
		Gemini3: func(agent config.Agent, apiKey string, audit *audit.Audit) *sdk.AI {
			return sdk.NewAI(gemini.NewAgent(
				gemini.WithAPIKey(apiKey),
				gemini.WithModel(gemini.ModelGemini3),
			), audit)
		},
	}
}

func Tools() []sdk.AITool {
	return []sdk.AITool{
		&tools.BashTool{},
		&tools.WriteToFileTool{},
		&tools.StrReplaceEditorTool{},
		&tools.ViewFileTool{},
		&tools.ListDirTool{},
		&tools.RagTool{},
		&tools.FetchURLTool{},
		&tools.AskUserTool{},
		&tools.TimeTool{},
		&tools.MemoryTool{},
	}
}

func ToolNames() []string {
	names := make([]string, 0, len(Tools()))
	for _, tool := range Tools() {
		names = append(names, tool.Name())
	}

	return names
}

func ModelNames() []string {
	names := make([]string, 0, len(Models()))
	for name := range Models() {
		names = append(names, name)
	}

	return names
}
