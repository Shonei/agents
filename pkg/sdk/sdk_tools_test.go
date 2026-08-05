package sdk

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk/audit"
)

type stubTool struct {
	name string
	err  error
	out  interface{}
}

func (s *stubTool) Name() string { return s.name }
func (s *stubTool) Description() string {
	return "stub"
}

func (s *stubTool) Init(map[string]string, *config.ConfigFactory) {}
func (s *stubTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{"type": "object"}
}

func (s *stubTool) Call(map[string]interface{}) (interface{}, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.out, nil
}

func newTestAI(tools ...AITool) *AI {
	ai := NewAI(nil, audit.NewAudit(audit.NewNoopLogger()))
	for _, tool := range tools {
		ai.RegisterTool(tool)
	}

	return ai
}

func TestProcessTools_UnknownToolIsSoftError(t *testing.T) {
	ai := newTestAI()

	results, err := ai.processTools(ResponseContentBlock{
		Type: ContentTypeToolUse,
		ID:   "call_1",
		Name: "does_not_exist",
	})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.True(t, results[0].IsError)
	assert.Equal(t, "call_1", results[0].ToolUseID)
	assert.Equal(t, "does_not_exist", results[0].Name)

	var payload map[string]string
	require.NoError(t, json.Unmarshal([]byte(results[0].Content.(string)), &payload))
	assert.Equal(t, "tool_error", payload["type"])
	assert.Contains(t, payload["result"], "does_not_exist")
}

func TestProcessTools_PlainErrorIsSoft(t *testing.T) {
	ai := newTestAI(&stubTool{name: "boom", err: errors.New(`bad "arg"`)})

	results, err := ai.processTools(ResponseContentBlock{
		Type:  ContentTypeToolUse,
		ID:    "call_2",
		Name:  "boom",
		Input: map[string]interface{}{},
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].IsError)
	assert.Equal(t, "boom", results[0].Name)

	var payload map[string]string
	require.NoError(t, json.Unmarshal([]byte(results[0].Content.(string)), &payload))
	assert.Equal(t, `bad "arg"`, payload["result"])
}

func TestProcessTools_AIErrorIsSoft(t *testing.T) {
	ai := newTestAI(&stubTool{name: "view", err: NewAIError("not found")})

	results, err := ai.processTools(ResponseContentBlock{
		Type: ContentTypeToolUse,
		ID:   "call_3",
		Name: "view",
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].IsError)

	var payload map[string]string
	require.NoError(t, json.Unmarshal([]byte(results[0].Content.(string)), &payload))
	assert.Equal(t, "not found", payload["result"])
}

func TestProcessTools_SuccessIncludesName(t *testing.T) {
	ai := newTestAI(&stubTool{name: "echo", out: "ok"})

	results, err := ai.processTools(ResponseContentBlock{
		Type: ContentTypeToolUse,
		ID:   "call_4",
		Name: "echo",
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].IsError)
	assert.Equal(t, "echo", results[0].Name)
	assert.Equal(t, "ok", results[0].Content)
}
