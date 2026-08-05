package sdk

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIErrorAIResponse_EscapesSpecialCharacters(t *testing.T) {
	err := NewAIError(`path "foo/bar" failed` + "\nnext line")

	raw := err.AIResponse()

	var payload map[string]string
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))
	assert.Equal(t, "tool_error", payload["type"])
	assert.Equal(t, "path \"foo/bar\" failed\nnext line", payload["result"])
}

func TestAIErrorAIResponse_PlainMessage(t *testing.T) {
	raw := NewAIError("missing file").AIResponse()

	var payload map[string]string
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))
	assert.Equal(t, "tool_error", payload["type"])
	assert.Equal(t, "missing file", payload["result"])
}
