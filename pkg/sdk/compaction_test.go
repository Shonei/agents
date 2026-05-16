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
			// boundaries from end: u3 (1st), u2 (2nd). Tool-result user msg
			// is skipped, so cut lands on u2 keeping the whole tool pair.
			want: 2,
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
