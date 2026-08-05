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

func TestConvertRequest_ServerToolsIncludeInvocationDetails(t *testing.T) {
	a := NewAgent()

	req, err := a.convertRequest(sdk.CreateMessageRequest{
		Messages: []sdk.InputMessage{
			sdk.NewTextMessage(sdk.RoleUser, "Read https://example.com/docs"),
		},
		ServerTools: []sdk.ServerTool{{Name: sdk.ServerToolURLContext}},
	})
	require.NoError(t, err)
	require.Len(t, req.Tools, 1)
	require.NotNil(t, req.Tools[0].URLContext)
	require.NotNil(t, req.ToolConfig)
	require.NotNil(t, req.ToolConfig.IncludeServerSideToolInvocations)
	assert.True(t, *req.ToolConfig.IncludeServerSideToolInvocations)
	assert.Nil(t, req.ToolConfig.FunctionCallingConfig)
}

func TestConvertResponse_CapturesCandidateURLContextMetadata(t *testing.T) {
	a := NewAgent()

	resp, err := a.convertResponse(GenerateContentResponse{
		ResponseId: "r-url",
		Candidates: []Candidate{{
			Content: Content{
				Parts: []Part{{Text: "I read the docs."}},
			},
			URLContextMetadata: &URLContextMetadata{
				URLMetadata: []URLMetadataEntry{{
					RetrievedURL:       "https://example.com/docs",
					URLRetrievalStatus: "URL_RETRIEVAL_STATUS_SUCCESS",
				}},
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Content, 2)

	grounding := resp.Content[1]
	require.Equal(t, sdk.ContentTypeGrounding, grounding.Type)
	require.NotNil(t, grounding.Grounding)
	assert.Equal(t, []string{sdk.ServerToolURLContext}, grounding.Grounding.Tools)
	assert.Equal(t, []sdk.RetrievedURL{{
		URL:    "https://example.com/docs",
		Status: "URL_RETRIEVAL_STATUS_SUCCESS",
	}}, grounding.Grounding.RetrievedURLs)
}

func TestConvertResponse_MergesNestedAndCandidateURLContextMetadata(t *testing.T) {
	a := NewAgent()

	resp, err := a.convertResponse(GenerateContentResponse{
		ResponseId: "r-merge",
		Candidates: []Candidate{{
			Content: Content{
				Parts: []Part{{Text: "I read both docs."}},
			},
			GroundingMetadata: &GroundingMetadata{
				URLContextMetadata: &URLContextMetadata{
					URLMetadata: []URLMetadataEntry{{
						RetrievedURL:       "https://example.com/old-shape",
						URLRetrievalStatus: "URL_RETRIEVAL_STATUS_SUCCESS",
					}},
				},
			},
			URLContextMetadata: &URLContextMetadata{
				URLMetadata: []URLMetadataEntry{{
					RetrievedURL:       "https://example.com/current-shape",
					URLRetrievalStatus: "URL_RETRIEVAL_STATUS_SUCCESS",
				}},
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Content, 2)

	grounding := resp.Content[1].Grounding
	require.NotNil(t, grounding)
	assert.Equal(t, []string{sdk.ServerToolURLContext}, grounding.Tools)
	assert.Equal(t, []sdk.RetrievedURL{
		{URL: "https://example.com/old-shape", Status: "URL_RETRIEVAL_STATUS_SUCCESS"},
		{URL: "https://example.com/current-shape", Status: "URL_RETRIEVAL_STATUS_SUCCESS"},
	}, grounding.RetrievedURLs)
}
