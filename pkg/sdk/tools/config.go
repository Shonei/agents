package tools

import "github.com/Shonei/agents/pkg/sdk"

var availableTools = []sdk.AITool{
	&FetchURLTool{},
	&TimeTool{},
	&WriteToFileTool{},
	&ViewFileTool{},
	&ListDirTool{},
	&BashTool{},
	&StrReplaceEditorTool{},
	&RagTool{},
	&MemoryTool{},
}

// availableServerTools holds tools executed by the model provider itself
// (e.g. Gemini google_search and url_context). They share the YAML tools
// list with regular AITool entries; engage resolves by name.
var availableServerTools = []sdk.ServerSideTool{
	&GoogleSearchTool{},
	&URLContextTool{},
}

func Tools() []sdk.AITool {
	return availableTools
}

func ServerTools() []sdk.ServerSideTool {
	return availableServerTools
}

func ToolNames() []string {
	names := make([]string, 0, len(availableTools)+len(availableServerTools))
	for _, tool := range availableTools {
		names = append(names, tool.Name())
	}
	for _, tool := range availableServerTools {
		names = append(names, tool.Name())
	}

	return names
}
