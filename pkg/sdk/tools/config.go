package tools

import "github.com/Shonei/agents/pkg/sdk"

func Tools() []sdk.AITool {
	return []sdk.AITool{
		&FetchURLTool{},
		&BrowseURLTool{},
		&FirecrawlFetchTool{},
		&IngestAPISpecTool{},
		&TimeTool{},
		&WriteToFileTool{},
		&DeleteFileTool{},
		&ViewFileTool{},
		&ListDirTool{},
		&BashTool{},
		&StrReplaceEditorTool{},
		&RagTool{},
		&MemoryTool{},
		&GithubPRDetailsTool{},
		&GithubPRDiffTool{},
		&GithubPRCommentsTool{},
		&GitCheckoutPRTool{},
		&GithubPRReviewTool{},
		&TodoTool{},
		&PlanTool{},
	}
}

func ServerTools() []sdk.ServerSideTool {
	return []sdk.ServerSideTool{
		&GoogleSearchTool{},
		&URLContextTool{},
		&WebSearchTool{},
		&WebFetchTool{},
	}
}

func ToolNames() []string {
	tools := Tools()
	serverTools := ServerTools()
	names := make([]string, 0, len(tools)+len(serverTools))
	for _, tool := range tools {
		names = append(names, tool.Name())
	}
	for _, tool := range serverTools {
		names = append(names, tool.Name())
	}

	return names
}
