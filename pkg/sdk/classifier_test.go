package sdk

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderRecentHistorySkipsToolBlocks(t *testing.T) {
	history := []InputMessage{
		NewTextMessage(RoleUser, "u1"),
		{
			Role: RoleAssistant,
			Content: []ContentBlock{
				{Type: ContentTypeToolUse, Name: "bash", ID: "t1"},
			},
		},
		{
			Role: RoleUser,
			Content: []ContentBlock{
				NewToolResultContentBlock("t1", "", "ok", false),
			},
		},
		{
			Role: RoleAssistant,
			Content: []ContentBlock{
				{Type: ContentTypeText, Text: "a1"},
			},
		},
		NewTextMessage(RoleUser, "u2"),
	}

	out := renderRecentHistory(history, 4)

	assert.Contains(t, out, "USER: u1")
	assert.Contains(t, out, "ASSISTANT: a1")
	assert.Contains(t, out, "USER: u2")
	assert.NotContains(t, out, "bash", "tool_use blocks must be skipped")
	assert.NotContains(t, out, "ok", "tool_result blocks must be skipped")

	// Chronological order: u1 must come before u2.
	assert.Less(t, strings.Index(out, "USER: u1"), strings.Index(out, "USER: u2"))
}

func TestRenderRecentHistoryRespectsLimit(t *testing.T) {
	history := []InputMessage{
		NewTextMessage(RoleUser, "u1"),
		NewTextMessage(RoleAssistant, "a1"),
		NewTextMessage(RoleUser, "u2"),
		NewTextMessage(RoleAssistant, "a2"),
		NewTextMessage(RoleUser, "u3"),
	}

	out := renderRecentHistory(history, 2)

	assert.NotContains(t, out, "u1")
	assert.NotContains(t, out, "a1")
	assert.Contains(t, out, "a2")
	assert.Contains(t, out, "u3")
}

func TestRenderClassifierPromptContainsAllSections(t *testing.T) {
	routes := []RouteMeta{
		{Agent: "planner", When: "scoping"},
		{Agent: "builder", When: "implementing"},
	}

	history := []InputMessage{
		NewTextMessage(RoleUser, "how should we start?"),
		NewTextMessage(RoleAssistant, "a high level plan"),
	}

	out := renderClassifierPrompt(routes, "planner", history, "looks good, let's build it")

	assert.Contains(t, out, "<routes>")
	assert.Contains(t, out, "- planner: scoping")
	assert.Contains(t, out, "- builder: implementing")
	assert.Contains(t, out, "<current_route>planner</current_route>")
	assert.Contains(t, out, "USER: how should we start?")
	assert.Contains(t, out, "ASSISTANT: a high level plan")
	assert.Contains(t, out, "<new_user_message>")
	assert.Contains(t, out, "looks good, let's build it")
}
