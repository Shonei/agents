package openrouter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Shonei/agents/pkg/sdk"
)

func TestConvertRequest_SystemAndCaching(t *testing.T) {
	a := NewAgent(WithModel("anthropic/claude-sonnet-4.5"))

	req, err := a.convertRequest(sdk.CreateMessageRequest{
		System:   "you are helpful",
		Messages: []sdk.InputMessage{sdk.NewTextMessage(sdk.RoleUser, "hello")},
	})
	require.NoError(t, err)
	require.Len(t, req.Messages, 2)

	// System prompt is a leading system-role message with a cache breakpoint.
	assert.Equal(t, RoleSystem, req.Messages[0].Role)
	sysParts, ok := req.Messages[0].Content.([]ContentPart)
	require.True(t, ok, "cached system content should be a parts array")
	require.Len(t, sysParts, 1)
	require.NotNil(t, sysParts[0].CacheControl)
	assert.Equal(t, "ephemeral", sysParts[0].CacheControl.Type)

	// The tail breakpoint lands on the final (user) message.
	tailParts, ok := req.Messages[1].Content.([]ContentPart)
	require.True(t, ok)
	require.NotNil(t, tailParts[0].CacheControl)
}

func TestConvertRequest_CachingDisabled(t *testing.T) {
	a := NewAgent(WithCaching(false))

	req, err := a.convertRequest(sdk.CreateMessageRequest{
		System:   "sys",
		Messages: []sdk.InputMessage{sdk.NewTextMessage(sdk.RoleUser, "hi")},
	})
	require.NoError(t, err)

	// With caching off the system prompt stays a plain string and no
	// breakpoints are added.
	assert.Equal(t, "sys", req.Messages[0].Content)
}

func TestConvertRequest_ImageBecomesDataURL(t *testing.T) {
	a := NewAgent(WithCaching(false))

	req, err := a.convertRequest(sdk.CreateMessageRequest{
		Messages: []sdk.InputMessage{{
			Role: sdk.RoleUser,
			Content: []sdk.ContentBlock{{
				Type:   sdk.ContentTypeImage,
				Source: &sdk.Blob{MimeType: "image/png", Data: "QUJD"},
			}},
		}},
	})
	require.NoError(t, err)
	require.Len(t, req.Messages, 1)

	parts := req.Messages[0].Content.([]ContentPart)
	require.Len(t, parts, 1)
	assert.Equal(t, ContentPartImageURL, parts[0].Type)
	require.NotNil(t, parts[0].ImageURL)
	assert.Equal(t, "data:image/png;base64,QUJD", parts[0].ImageURL.URL)
}

func TestConvertRequest_ToolUseAndToolResult(t *testing.T) {
	a := NewAgent(WithCaching(false))

	req, err := a.convertRequest(sdk.CreateMessageRequest{
		Messages: []sdk.InputMessage{
			{
				Role: sdk.RoleAssistant,
				Content: []sdk.ContentBlock{
					{Type: sdk.ContentTypeText, Text: "let me check"},
					{Type: sdk.ContentTypeToolUse, ID: "call_1", Name: "get_time", Input: map[string]any{"tz": "utc"}},
				},
			},
			{
				Role: sdk.RoleUser,
				Content: []sdk.ContentBlock{
					sdk.NewToolResultContentBlock("call_1", "get_time", "12:00", false),
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, req.Messages, 2)

	// Assistant message keeps its text and carries the tool call.
	asst := req.Messages[0]
	assert.Equal(t, RoleAssistant, asst.Role)
	require.Len(t, asst.ToolCalls, 1)
	assert.Equal(t, "call_1", asst.ToolCalls[0].ID)
	assert.Equal(t, "get_time", asst.ToolCalls[0].Function.Name)
	assert.JSONEq(t, `{"tz":"utc"}`, asst.ToolCalls[0].Function.Arguments)

	// Tool result is its own tool-role message keyed by tool_call_id.
	tool := req.Messages[1]
	assert.Equal(t, RoleTool, tool.Role)
	assert.Equal(t, "call_1", tool.ToolCallID)
	assert.Equal(t, "12:00", tool.Content)
}

func TestConvertRequest_ToolChoice(t *testing.T) {
	a := NewAgent(WithCaching(false))

	tools := []sdk.Tool{sdk.NewTool("get_time", "", map[string]any{})}

	cases := map[string]any{
		"auto": "auto",
		"any":  "required",
		"none": "none",
	}
	for in, want := range cases {
		req, err := a.convertRequest(sdk.CreateMessageRequest{
			Messages:   []sdk.InputMessage{sdk.NewTextMessage(sdk.RoleUser, "hi")},
			Tools:      tools,
			ToolChoice: &sdk.ToolChoice{Type: in},
		})
		require.NoError(t, err)
		assert.Equal(t, want, req.ToolChoice, "tool choice %q", in)
	}

	req, err := a.convertRequest(sdk.CreateMessageRequest{
		Messages:   []sdk.InputMessage{sdk.NewTextMessage(sdk.RoleUser, "hi")},
		Tools:      tools,
		ToolChoice: &sdk.ToolChoice{Type: "tool", Name: "get_time"},
	})
	require.NoError(t, err)
	forced, ok := req.ToolChoice.(ToolChoiceFunction)
	require.True(t, ok)
	assert.Equal(t, "get_time", forced.Function.Name)
}

func TestConvertRequest_WebSearchServerTool(t *testing.T) {
	a := NewAgent(WithCaching(false))

	req, err := a.convertRequest(sdk.CreateMessageRequest{
		Messages:    []sdk.InputMessage{sdk.NewTextMessage(sdk.RoleUser, "what's new today?")},
		ServerTools: []sdk.ServerTool{{Name: sdk.ServerToolWebSearch}},
	})
	require.NoError(t, err)

	require.Len(t, req.Tools, 1)
	assert.Equal(t, ToolTypeWebSearch, req.Tools[0].Type)
	assert.Nil(t, req.Tools[0].Function)
	// No tool_choice is forced for server-only tool sets.
	assert.Nil(t, req.ToolChoice)
}

func TestConvertResponse_WebSearchAnnotations(t *testing.T) {
	a := NewAgent()

	resp, err := a.convertResponse(ChatCompletionResponse{
		Choices: []Choice{{
			Message: ResponseMessage{
				Role:    RoleAssistant,
				Content: "Here is the answer.",
				Annotations: []Annotation{{
					Type: "url_citation",
					URLCitation: &URLCitation{
						URL:     "https://example.com/news",
						Title:   "Big News",
						Content: "excerpt",
					},
				}},
			},
		}},
	})
	require.NoError(t, err)

	require.Len(t, resp.Content, 2)
	assert.Equal(t, sdk.ContentTypeText, resp.Content[0].Type)
	g := resp.Content[1]
	assert.Equal(t, sdk.ContentTypeGrounding, g.Type)
	require.NotNil(t, g.Grounding)
	require.Len(t, g.Grounding.Sources, 1)
	assert.Equal(t, "https://example.com/news", g.Grounding.Sources[0].URI)
	assert.Equal(t, "Big News", g.Grounding.Sources[0].Title)
}

func TestConvertResponse(t *testing.T) {
	a := NewAgent(WithModel("moonshotai/kimi-k2.5"))

	resp, err := a.convertResponse(ChatCompletionResponse{
		ID:    "gen-1",
		Model: "moonshotai/kimi-k2.5",
		Choices: []Choice{{
			Message: ResponseMessage{
				Role:      RoleAssistant,
				Reasoning: "thinking...",
				Content:   "the answer",
				ToolCalls: []ToolCall{{
					ID:       "call_9",
					Type:     "function",
					Function: FunctionCall{Name: "lookup", Arguments: `{"q":"x"}`},
				}},
			},
		}},
		Usage: &Usage{PromptTokens: 10, CompletionTokens: 5},
	})
	require.NoError(t, err)

	require.Len(t, resp.Content, 3)
	assert.Equal(t, sdk.ContentTypeThinking, resp.Content[0].Type)
	assert.Equal(t, "thinking...", resp.Content[0].Text)
	assert.Equal(t, sdk.ContentTypeText, resp.Content[1].Type)
	assert.Equal(t, sdk.ContentTypeToolUse, resp.Content[2].Type)
	assert.Equal(t, "call_9", resp.Content[2].ID)
	assert.Equal(t, "lookup", resp.Content[2].Name)
	assert.Equal(t, map[string]any{"q": "x"}, resp.Content[2].Input)

	assert.Equal(t, sdk.RoleAssistant, resp.Role)
	assert.Equal(t, 10, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
}
