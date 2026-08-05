package gemini

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Shonei/agents/pkg/sdk"
)

func TestConvertResponse_UniqueFunctionCallIDs(t *testing.T) {
	a := NewAgent()

	resp, err := a.convertResponse(GenerateContentResponse{
		ResponseId: "r1",
		Candidates: []Candidate{{
			Content: Content{
				Parts: []Part{
					{FunctionCall: &FunctionCall{Name: "bash", Args: map[string]any{"command": "ls"}}},
					{FunctionCall: &FunctionCall{Name: "bash", Args: map[string]any{"command": "pwd"}}},
					{FunctionCall: &FunctionCall{Name: "view_file", Args: map[string]any{"path": "a.go"}}},
				},
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Content, 3)

	assert.Equal(t, "bash_1", resp.Content[0].ID)
	assert.Equal(t, "bash", resp.Content[0].Name)
	assert.Equal(t, "bash_2", resp.Content[1].ID)
	assert.Equal(t, "bash", resp.Content[1].Name)
	assert.Equal(t, "view_file_3", resp.Content[2].ID)
	assert.Equal(t, "view_file", resp.Content[2].Name)
}

func TestConvertRequest_FunctionResponseUsesToolName(t *testing.T) {
	a := NewAgent()

	req, err := a.convertRequest(sdk.CreateMessageRequest{
		Messages: []sdk.InputMessage{
			{
				Role: sdk.RoleAssistant,
				Content: []sdk.ContentBlock{
					{Type: sdk.ContentTypeToolUse, ID: "bash_1", Name: "bash", Input: map[string]any{"command": "ls"}},
				},
			},
			{
				Role: sdk.RoleUser,
				Content: []sdk.ContentBlock{
					sdk.NewToolResultContentBlock("bash_1", "bash", `{"ok":true}`, false),
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, req.Contents, 2)

	parts := req.Contents[1].Parts
	require.Len(t, parts, 1)
	require.NotNil(t, parts[0].FunctionResponse)
	assert.Equal(t, "bash", parts[0].FunctionResponse.Name)
	assert.Equal(t, true, parts[0].FunctionResponse.Response["ok"])
}

func TestConvertRequest_FunctionResponseFallsBackToToolUseID(t *testing.T) {
	a := NewAgent()

	req, err := a.convertRequest(sdk.CreateMessageRequest{
		Messages: []sdk.InputMessage{{
			Role: sdk.RoleUser,
			Content: []sdk.ContentBlock{{
				Type:      sdk.ContentTypeToolResult,
				ToolUseID: "legacy_name",
				Content:   "ok",
			}},
		}},
	})
	require.NoError(t, err)
	require.Len(t, req.Contents, 1)
	require.NotNil(t, req.Contents[0].Parts[0].FunctionResponse)
	assert.Equal(t, "legacy_name", req.Contents[0].Parts[0].FunctionResponse.Name)
}
