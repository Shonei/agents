package sdk

import "testing"

func TestFindCompactionCut(t *testing.T) {
	userText := func(s string) InputMessage {
		return NewTextMessage(RoleUser, s)
	}

	assistantText := func(s string) InputMessage {
		return InputMessage{
			Role: RoleAssistant,
			Content: []ContentBlock{
				{Type: ContentTypeText, Text: s},
			},
		}
	}

	assistantToolUse := func(name, id string) InputMessage {
		return InputMessage{
			Role: RoleAssistant,
			Content: []ContentBlock{
				{Type: ContentTypeToolUse, Name: name, ID: id},
			},
		}
	}

	userToolResult := func(id, content string) InputMessage {
		return InputMessage{
			Role: RoleUser,
			Content: []ContentBlock{
				NewToolResultContentBlock(id, content, false),
			},
		}
	}

	tests := []struct {
		name          string
		messages      []InputMessage
		keepLastTurns int
		want          int
	}{
		{
			name: "all text, K=2 keeps last two user messages",
			messages: []InputMessage{
				userText("u1"),
				assistantText("a1"),
				userText("u2"),
				assistantText("a2"),
				userText("u3"),
				assistantText("a3"),
			},
			keepLastTurns: 2,
			want:          2, // start of u2
		},
		{
			name: "tool pair must not be split",
			messages: []InputMessage{
				userText("u1"),
				assistantText("a1"),
				userText("u2"),
				assistantToolUse("bash", "t1"),
				userToolResult("t1", "ok"),
				assistantText("a2 after tool"),
				userText("u3"),
				assistantText("a3"),
			},
			keepLastTurns: 2,
			// boundaries from end: u3 (1st), end of tool round (2nd).
			// Cut after the tool_result keeps the whole tool pair in tail.
			want: 5,
		},
		{
			name: "K larger than available boundaries returns -1",
			messages: []InputMessage{
				userText("u1"),
				assistantText("a1"),
				userText("u2"),
			},
			keepLastTurns: 5,
			want:          -1,
		},
		{
			name: "exactly K boundaries returns 0 (nothing safe to evict)",
			messages: []InputMessage{
				userText("u1"),
				assistantText("a1"),
				userText("u2"),
				assistantText("a2"),
			},
			keepLastTurns: 2,
			want:          0,
		},
		{
			name:          "empty history",
			messages:      nil,
			keepLastTurns: 4,
			want:          -1,
		},
		{
			name: "K=0 always returns -1",
			messages: []InputMessage{
				userText("u1"),
				userText("u2"),
			},
			keepLastTurns: 0,
			want:          -1,
		},
		{
			name: "multiple consecutive tool pairs",
			messages: []InputMessage{
				userText("u1"),
				assistantToolUse("bash", "t1"),
				userToolResult("t1", "r1"),
				assistantToolUse("bash", "t2"),
				userToolResult("t2", "r2"),
				assistantText("a1"),
				userText("u2"),
				assistantText("a2"),
				userText("u3"),
				assistantText("a3"),
			},
			keepLastTurns: 2,
			// boundaries from end: u3, u2. Cut at u2 index (6).
			want: 6,
		},
		{
			name: "tool result boundary allows cut inside a long turn",
			messages: []InputMessage{
				userText("u1"),
				assistantToolUse("bash", "t1"),
				userToolResult("t1", "r1"),
				assistantToolUse("bash", "t2"),
				userToolResult("t2", "r2"),
				assistantText("a1"),
			},
			keepLastTurns: 1,
			// Only boundary is the end of the last tool round; cut after it.
			want: 5,
		},
		{
			name: "synthetic summary is not a boundary",
			messages: []InputMessage{
				NewTextMessage(RoleUser, summaryPrefix+"older summary"),
				userText("u1"),
				assistantText("a1"),
				userText("u2"),
				assistantText("a2"),
			},
			keepLastTurns: 2,
			// boundaries from end: u2, u1. Summary is ignored.
			want: 1,
		},
		{
			name: "trailing tool result does not count as boundary",
			messages: []InputMessage{
				userText("u1"),
				assistantText("a1"),
				userText("u2"),
				assistantToolUse("bash", "t1"),
				userToolResult("t1", "ok"),
			},
			keepLastTurns: 1,
			want:          2, // u2 is the only valid boundary
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findCompactionCut(tt.messages, tt.keepLastTurns)
			if got != tt.want {
				t.Errorf("findCompactionCut() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestIsUserTextBoundary(t *testing.T) {
	tests := []struct {
		name string
		msg  InputMessage
		want bool
	}{
		{
			name: "user string message",
			msg:  NewTextMessage(RoleUser, "hello"),
			want: true,
		},
		{
			name: "assistant string message",
			msg:  NewTextMessage(RoleAssistant, "hi"),
			want: false,
		},
		{
			name: "user text content block",
			msg: InputMessage{
				Role:    RoleUser,
				Content: []ContentBlock{{Type: ContentTypeText, Text: "hi"}},
			},
			want: true,
		},
		{
			name: "user tool_result block is not a boundary",
			msg: InputMessage{
				Role: RoleUser,
				Content: []ContentBlock{
					NewToolResultContentBlock("t1", "result", false),
				},
			},
			want: false,
		},
		{
			name: "user with mixed text + tool_result is not a boundary",
			msg: InputMessage{
				Role: RoleUser,
				Content: []ContentBlock{
					{Type: ContentTypeText, Text: "hi"},
					NewToolResultContentBlock("t1", "result", false),
				},
			},
			want: false,
		},
		{
			name: "user with empty content blocks",
			msg: InputMessage{
				Role:    RoleUser,
				Content: []ContentBlock{},
			},
			want: false,
		},
		{
			name: "synthetic summary message is not a boundary",
			msg:  NewTextMessage(RoleUser, summaryPrefix+"older summary"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUserTextBoundary(tt.msg)
			if got != tt.want {
				t.Errorf("isUserTextBoundary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsToolResultBoundary(t *testing.T) {
	assistantToolUse := func(name, id string) InputMessage {
		return InputMessage{
			Role: RoleAssistant,
			Content: []ContentBlock{
				{Type: ContentTypeToolUse, Name: name, ID: id},
			},
		}
	}

	userToolResult := func(id, content string) InputMessage {
		return InputMessage{
			Role: RoleUser,
			Content: []ContentBlock{
				NewToolResultContentBlock(id, content, false),
			},
		}
	}

	tests := []struct {
		name     string
		messages []InputMessage
		i        int
		want     bool
	}{
		{
			name: "pure tool_result with following message",
			messages: []InputMessage{
				assistantToolUse("bash", "t1"),
				userToolResult("t1", "ok"),
				NewTextMessage(RoleAssistant, "done"),
			},
			i:    1,
			want: true,
		},
		{
			name: "trailing tool_result cannot be cut after",
			messages: []InputMessage{
				assistantToolUse("bash", "t1"),
				userToolResult("t1", "ok"),
			},
			i:    1,
			want: false,
		},
		{
			name: "user text is not a tool_result boundary",
			messages: []InputMessage{
				NewTextMessage(RoleUser, "hi"),
				NewTextMessage(RoleAssistant, "hello"),
			},
			i:    0,
			want: false,
		},
		{
			name: "mixed content is not a tool_result boundary",
			messages: []InputMessage{
				assistantToolUse("bash", "t1"),
				{
					Role: RoleUser,
					Content: []ContentBlock{
						{Type: ContentTypeText, Text: "hi"},
						NewToolResultContentBlock("t1", "ok", false),
					},
				},
				NewTextMessage(RoleAssistant, "done"),
			},
			i:    1,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isToolResultBoundary(tt.messages, tt.i)
			if got != tt.want {
				t.Errorf("isToolResultBoundary() = %v, want %v", got, tt.want)
			}
		})
	}
}
