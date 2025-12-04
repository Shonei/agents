package tools

import "github.com/Shonei/agents/pkg/sdk"

var availableTools = []sdk.AITool{}

func RegisterTools(tools sdk.AITool) {
	availableTools = append(availableTools, tools)
}

func Tools() []sdk.AITool {
	return availableTools
}

func ToolNames() []string {
	names := make([]string, 0, len(Tools()))
	for _, tool := range Tools() {
		names = append(names, tool.Name())
	}

	return names
}
