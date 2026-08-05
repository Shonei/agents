package sdk

import (
	"errors"
	"strings"
	"testing"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk/audit"
)

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
			name: "assistant summary at the head is not a boundary",
			messages: []InputMessage{
				NewTextMessage(RoleAssistant, "older summary"),
				userText("u1"),
				assistantText("a1"),
				userText("u2"),
				assistantText("a2"),
			},
			keepLastTurns: 2,
			// boundaries from end: u2, u1. The summary is an assistant message
			// and so can never be a cut point.
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

func TestSummarizeForCompactionUsesCompactionPrompt(t *testing.T) {
	ai, fa := newScriptedAI("m", textResponse("I changed foo.go."))

	summary, err := ai.SummarizeForCompaction("", []InputMessage{
		NewTextMessage(RoleUser, "do the thing"),
	})
	if err != nil {
		t.Fatalf("SummarizeForCompaction() error = %v", err)
	}

	if summary != "I changed foo.go." {
		t.Errorf("summary = %q", summary)
	}

	if len(fa.requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(fa.requests))
	}

	if fa.requests[0].System != compactionSystemPrompt {
		t.Error("compaction did not use compactionSystemPrompt")
	}

	if fa.requests[0].System == handoffSystemPrompt {
		t.Error("compaction must not share the handoff prompt")
	}
}

func TestSummarizeForHandoffUsesHandoffPrompt(t *testing.T) {
	ai, fa := newScriptedAI("m", textResponse("GOAL: ship it"))

	if _, err := ai.SummarizeForHandoff([]InputMessage{
		NewTextMessage(RoleUser, "do the thing"),
	}); err != nil {
		t.Fatalf("SummarizeForHandoff() error = %v", err)
	}

	if fa.requests[0].System != handoffSystemPrompt {
		t.Error("handoff did not use handoffSystemPrompt")
	}

	if fa.requests[0].System == compactionSystemPrompt {
		t.Error("handoff must not share the compaction prompt")
	}
}

func TestSummarizeForCompactionMergesPriorSummary(t *testing.T) {
	ai, fa := newScriptedAI("m", textResponse("merged"))

	if _, err := ai.SummarizeForCompaction("older note", []InputMessage{
		NewTextMessage(RoleUser, "newer turn"),
	}); err != nil {
		t.Fatalf("SummarizeForCompaction() error = %v", err)
	}

	input, ok := fa.requests[0].Messages[0].Content.(string)
	if !ok {
		t.Fatal("summarizer input was not a string")
	}

	if !strings.Contains(input, priorSummaryLabel) {
		t.Error("prior summary was not labeled in the summarizer input")
	}

	if !strings.Contains(input, "older note") {
		t.Error("prior summary text was not carried into the summarizer input")
	}

	if !strings.Contains(input, "newer turn") {
		t.Error("newly evicted turns missing from the summarizer input")
	}
}

func TestRebuildCompactedShape(t *testing.T) {
	ai, _ := newScriptedAI("m")

	pinned := []InputMessage{NewTextMessage(RoleUser, "the original request")}
	tail := []InputMessage{NewTextMessage(RoleAssistant, "recent work")}

	got := buildCompacted(pinned, "my memory", tail)

	if len(got) != 3 {
		t.Fatalf("rebuilt %d messages, want 3", len(got))
	}

	if got[0].Role != RoleUser || got[0].Content != "the original request" {
		t.Error("original request was not pinned verbatim at the head")
	}

	// The whole point of the split: the summary is the agent's own memory, so it
	// must be spoken by the assistant, not injected as a user instruction.
	if got[1].Role != RoleAssistant {
		t.Errorf("summary injected as role=%s, want %s", got[1].Role, RoleAssistant)
	}

	if got[1].Content != "my memory" {
		t.Errorf("summary content = %v", got[1].Content)
	}

	// buildCompacted is pure: state is only recorded when the caller commits,
	// so a rebuild can be discarded without corrupting the head.
	if ai.rollingSummary != "" || ai.headHeight != 0 {
		t.Error("buildCompacted mutated head state; it must be pure")
	}

	ai.commitCompactionHead(len(pinned), "my memory")

	if ai.rollingSummary != "my memory" {
		t.Error("rolling summary state not recorded on commit")
	}

	if ai.headHeight != 2 {
		t.Errorf("headHeight = %d, want 2", ai.headHeight)
	}
}

func TestSplitCompactionHeadPinsFirstRequest(t *testing.T) {
	ai, _ := newScriptedAI("m")

	messages := []InputMessage{
		NewTextMessage(RoleUser, "original"),
		NewTextMessage(RoleAssistant, "work"),
		NewTextMessage(RoleUser, "more"),
	}

	pinned, body := ai.splitCompactionHead(messages)

	if len(pinned) != 1 || pinned[0].Content != "original" {
		t.Fatalf("pinned = %v, want the original request", pinned)
	}

	if len(body) != 2 {
		t.Errorf("body has %d messages, want 2", len(body))
	}
}

func TestSplitCompactionHeadReusesInstalledHead(t *testing.T) {
	ai, _ := newScriptedAI("m")

	pinned := []InputMessage{NewTextMessage(RoleUser, "original")}
	rebuilt := buildCompacted(pinned, "note one", []InputMessage{
		NewTextMessage(RoleAssistant, "work"),
	})
	ai.commitCompactionHead(len(pinned), "note one")

	rebuilt = append(rebuilt, NewTextMessage(RoleUser, "next turn"))

	gotPinned, body := ai.splitCompactionHead(rebuilt)

	if len(gotPinned) != 1 || gotPinned[0].Content != "original" {
		t.Errorf("pinned = %v, want the original request", gotPinned)
	}

	// The previous summary must be excluded from the body, otherwise it gets
	// re-summarized every pass and degrades.
	for _, m := range body {
		if m.Content == "note one" {
			t.Fatal("previous summary leaked into the evictable body")
		}
	}

	if len(body) != 2 {
		t.Errorf("body has %d messages, want 2", len(body))
	}
}

func TestSplitCompactionHeadResetsOnStaleState(t *testing.T) {
	ai, _ := newScriptedAI("m")

	// Install a head, then hand the AI a completely different history, as a
	// router does when it routes back to a sub-agent whose history was reset.
	ai.commitCompactionHead(1, "note")

	fresh := []InputMessage{
		NewTextMessage(RoleUser, "brand new request"),
		NewTextMessage(RoleAssistant, "work"),
	}

	pinned, body := ai.splitCompactionHead(fresh)

	if ai.rollingSummary != "" || ai.headHeight != 0 {
		t.Error("stale head state was not reset")
	}

	if len(pinned) != 1 || pinned[0].Content != "brand new request" {
		t.Errorf("pinned = %v, want the new request", pinned)
	}

	if len(body) != 1 {
		t.Errorf("body has %d messages, want 1", len(body))
	}
}

// toolUseResponse drives RunTurn through another loop iteration.
func toolUseResponse(name, id string) *MessageResponse {
	return &MessageResponse{
		Role: RoleAssistant,
		Content: []ResponseContentBlock{
			{Type: ContentTypeToolUse, ID: id, Name: name, Input: map[string]interface{}{}},
		},
	}
}

// noopTool satisfies a tool_use so the loop can continue.
type noopTool struct{}

func (*noopTool) Name() string                                  { return "bash" }
func (*noopTool) Description() string                           { return "noop" }
func (*noopTool) Init(map[string]string, *config.ConfigFactory) {}

func (*noopTool) InputSchema() map[string]interface{} { return map[string]interface{}{} }

func (*noopTool) Call(map[string]interface{}) (interface{}, error) { return "ok", nil }

// noSummarizerDelay removes the retry pause so the retry path can be tested
// without sleeping.
func noSummarizerDelay(t *testing.T) {
	t.Helper()

	original := summarizerRetryDelay
	summarizerRetryDelay = 0

	t.Cleanup(func() { summarizerRetryDelay = original })
}

// emptyResponse is a successful call that carries no text, which is the
// intermittent provider behaviour the summarizer retries.
func emptyResponse() *MessageResponse {
	return &MessageResponse{Role: RoleAssistant}
}

func TestSummarizeRetriesEmptyResponse(t *testing.T) {
	noSummarizerDelay(t)

	ai, fa := newScriptedAI("m", emptyResponse(), textResponse("I changed foo.go."))

	summary, err := ai.SummarizeForCompaction("", []InputMessage{
		NewTextMessage(RoleUser, "do the thing"),
	})
	if err != nil {
		t.Fatalf("SummarizeForCompaction() error = %v, want a retry to succeed", err)
	}

	if summary != "I changed foo.go." {
		t.Errorf("summary = %q", summary)
	}

	if len(fa.requests) != 2 {
		t.Errorf("made %d calls, want 2 (one empty, one retried)", len(fa.requests))
	}
}

func TestSummarizeGivesUpAfterAttempts(t *testing.T) {
	noSummarizerDelay(t)

	ai, fa := newScriptedAI("m",
		emptyResponse(), emptyResponse(), emptyResponse(), emptyResponse())

	_, err := ai.SummarizeForCompaction("", []InputMessage{
		NewTextMessage(RoleUser, "do the thing"),
	})
	if err == nil {
		t.Fatal("SummarizeForCompaction() succeeded, want an error")
	}

	if !strings.Contains(err.Error(), "after 3 attempts") {
		t.Errorf("error does not report the attempt count: %v", err)
	}

	// The diagnostic detail from the last attempt must survive the wrapping.
	if !strings.Contains(err.Error(), "no content blocks") {
		t.Errorf("error lost the response diagnostic: %v", err)
	}

	if len(fa.requests) != summarizerAttempts {
		t.Errorf("made %d calls, want %d", len(fa.requests), summarizerAttempts)
	}
}

func TestSummarizeDoesNotRetryTransportError(t *testing.T) {
	noSummarizerDelay(t)

	fa := &fakeAgent{name: "m", err: errors.New("401 unauthorized")}
	ai := NewAI(fa, audit.NewAudit(audit.NewNoopLogger()))

	_, err := ai.SummarizeForHandoff([]InputMessage{NewTextMessage(RoleUser, "hi")})
	if err == nil {
		t.Fatal("SummarizeForHandoff() succeeded, want an error")
	}

	// The provider layer already retries what is retryable; hammering a hard
	// failure three more times only multiplies the latency.
	if len(fa.requests) != 1 {
		t.Errorf("made %d calls, want 1", len(fa.requests))
	}
}

// TestRunTurnSurvivesCompactionFailure is the point of the change: compaction
// is an optimization against a self-imposed budget, so losing it must not end
// the conversation.
func TestRunTurnSurvivesCompactionFailure(t *testing.T) {
	noSummarizerDelay(t)

	fa := &fakeAgent{
		name:             "m",
		maxContextTokens: 10,
		responses: []*MessageResponse{
			// Every summarizer attempt comes back empty...
			emptyResponse(), emptyResponse(), emptyResponse(),
			// ...and the turn itself still has to complete.
			textResponse("done anyway"),
		},
	}

	ai := NewAI(fa, audit.NewAudit(audit.NewNoopLogger()))
	ai.SetQuiet(true)
	ai.lastInputTokens = 1000 // over the budget, so compaction is attempted

	history := []InputMessage{
		NewTextMessage(RoleUser, "u1"),
		NewTextMessage(RoleAssistant, "a1"),
		NewTextMessage(RoleUser, "u2"),
		NewTextMessage(RoleAssistant, "a2"),
		NewTextMessage(RoleUser, "u3"),
	}

	updated, err := ai.RunTurn(history)
	if err != nil {
		t.Fatalf("RunTurn() error = %v, want the turn to survive a failed compaction", err)
	}

	last := updated[len(updated)-1]
	if last.Role != RoleAssistant {
		t.Fatalf("last message role = %s, want %s", last.Role, RoleAssistant)
	}

	blocks, ok := last.Content.([]ContentBlock)
	if !ok || len(blocks) == 0 || blocks[0].Text != "done anyway" {
		t.Errorf("turn did not complete normally, last message = %#v", last.Content)
	}

	// 3 summarizer attempts + 1 real turn, and no further compaction attempts
	// once it was known to be unavailable.
	if len(fa.requests) != 4 {
		t.Errorf("made %d calls, want 4", len(fa.requests))
	}
}

// TestComputeCompactionCutFallsBackToFullEviction covers B1: an enormous final
// tool result can never be a boundary, so boundary search alone would only evict
// the small messages in front of it.
func TestComputeCompactionCutFallsBackToFullEviction(t *testing.T) {
	body := []InputMessage{
		{Role: RoleAssistant, Content: []ContentBlock{{Type: ContentTypeToolUse, ID: "t1", Name: "bash"}}},
		{Role: RoleUser, Content: []ContentBlock{NewToolResultContentBlock("t1", strings.Repeat("x", 900_000), false)}},
	}

	cut := computeCompactionCut(body, 2)
	if cut != len(body) {
		t.Errorf("computeCompactionCut() = %d, want %d (evict everything)", cut, len(body))
	}

	// Evicting the whole body leaves no tail, so nothing can be orphaned.
	if len(body[cut:]) != 0 {
		t.Error("full eviction should leave an empty tail")
	}
}

func TestComputeCompactionCutEmptyBody(t *testing.T) {
	if cut := computeCompactionCut(nil, 2); cut > 0 {
		t.Errorf("computeCompactionCut(nil) = %d, want <= 0", cut)
	}
}

// TestMaybeCompactSkipsNonShrinkingRebuild is the other half of B1: even with
// full eviction available, a rebuild that would not shrink the history must be
// discarded rather than applied.
func TestMaybeCompactSkipsNonShrinkingRebuild(t *testing.T) {
	noSummarizerDelay(t)

	// The summarizer returns a note far larger than the history it replaces.
	huge := strings.Repeat("y", 50_000)
	fa := &fakeAgent{name: "m", maxContextTokens: 10, responses: []*MessageResponse{textResponse(huge)}}

	ai := NewAI(fa, audit.NewAudit(audit.NewNoopLogger()))
	ai.lastInputTokens = 1000

	messages := []InputMessage{
		NewTextMessage(RoleUser, "original"),
		NewTextMessage(RoleAssistant, "a1"),
		NewTextMessage(RoleUser, "u2"),
		NewTextMessage(RoleAssistant, "a2"),
		NewTextMessage(RoleUser, "u3"),
	}
	before := charSize(messages)

	compacted, err := ai.maybeCompact(&messages)

	// Reported as an error, not a silent no-op, so RunTurn stops paying for
	// further attempts against an unchanged history.
	if !errors.Is(err, errCompactionUnproductive) {
		t.Fatalf("maybeCompact() error = %v, want errCompactionUnproductive", err)
	}

	// Guard against passing vacuously: the summarizer must actually have run,
	// otherwise this proves nothing about the shrink check.
	if len(fa.requests) != 1 {
		t.Fatalf("summarizer ran %d times, want 1 (compaction must have engaged)", len(fa.requests))
	}

	if compacted {
		t.Error("maybeCompact() applied a rebuild that does not shrink the history")
	}

	if charSize(messages) != before {
		t.Error("history was mutated despite the rebuild being rejected")
	}

	// A discarded rebuild must not leave the AI believing it installed a head.
	if ai.rollingSummary != "" || ai.headHeight != 0 {
		t.Error("head state was committed for a discarded rebuild")
	}
}

// TestSerializeForSummaryPreservesErrorsAndPaths covers B6: a failed tool call
// used to serialize identically to a successful one, so the summary recorded it
// as work done rather than as a dead end.
func TestSerializeForSummaryPreservesErrorsAndPaths(t *testing.T) {
	messages := []InputMessage{
		NewTextMessage(RoleUser, "run the tests"),
		{Role: RoleAssistant, Content: []ContentBlock{
			{Type: ContentTypeToolUse, ID: "t1", Name: "bash", Input: map[string]interface{}{"command": "make test"}},
		}},
		{Role: RoleUser, Content: []ContentBlock{
			NewToolResultContentBlock("t1", "FAIL: TestFoo", true),
		}},
		{Role: RoleAssistant, Content: []ContentBlock{
			{Type: ContentTypeImage, FilePath: "/tmp/image_abcde.png"},
		}},
	}

	out := serializeForSummary(messages)

	if !strings.Contains(out, "[ERROR]") {
		t.Errorf("failed tool result is indistinguishable from a success:\n%s", out)
	}

	if !strings.Contains(out, "FAIL: TestFoo") {
		t.Error("tool result content missing")
	}

	if !strings.Contains(out, "/tmp/image_abcde.png") {
		t.Errorf("image path dropped, agent loses the handle to its own artifact:\n%s", out)
	}

	// Tool output must not be attributed to the user.
	if !strings.Contains(out, "TOOL_RESULT:") {
		t.Error("tool results should be labeled as tool output, not user speech")
	}
}

// TestCompactionPromptRecordsUserConstraints covers B2: the prompt must require
// the summary to carry the user's task and constraints, since later user turns
// are evicted and only the first is pinned.
func TestCompactionPromptRecordsUserConstraints(t *testing.T) {
	prompt := CompactionSystemPrompt()

	if !strings.Contains(prompt, "ASKED:") {
		t.Error("compaction prompt has no section for what the user asked")
	}

	for _, want := range []string{"constraint", "part-way through", "verbatim"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("compaction prompt does not mention %q", want)
		}
	}

	// The old prompt told the summarizer the task was still in the conversation
	// and to omit it. That is what silently dropped mid-conversation
	// instructions.
	if strings.Contains(prompt, "do not restate your role, your instructions, or the task") {
		t.Error("compaction prompt still suppresses the task it now has to record")
	}

	// It must stay a recollection, not become a work order again.
	if !strings.Contains(prompt, "Do not issue instructions") {
		t.Error("compaction prompt lost its no-directives rule")
	}
}

// TestCharSizeCountsImageBlobs covers the regression the shrink guard first
// shipped with: base64 image data dominates a real history, and missing it made
// a multi-megabyte conversation measure as a few dozen chars, so every rebuild
// looked like a size increase and compaction disabled itself.
func TestCharSizeCountsImageBlobs(t *testing.T) {
	blob := strings.Repeat("A", 4_000_000)

	messages := []InputMessage{
		{Role: RoleAssistant, Content: []ContentBlock{
			{Type: ContentTypeImage, Source: &Blob{MimeType: "image/png", Data: blob}},
		}},
	}

	if got := charSize(messages); got < len(blob) {
		t.Errorf("charSize() = %d, want at least %d (image blob must be counted)", got, len(blob))
	}
}

func TestCharSizeCountsNonStringToolResult(t *testing.T) {
	messages := []InputMessage{
		{Role: RoleUser, Content: []ContentBlock{
			NewToolResultContentBlock("t1", map[string]any{"key": strings.Repeat("v", 5000)}, false),
		}},
	}

	if got := charSize(messages); got < 5000 {
		t.Errorf("charSize() = %d, want at least 5000 (structured tool result must be counted)", got)
	}
}

// TestRunTurnStopsRetryingUnproductiveCompaction covers the cost regression: a
// rejected rebuild left compaction enabled, so every loop iteration paid for
// another summarizer call against an unchanged history.
func TestRunTurnStopsRetryingUnproductiveCompaction(t *testing.T) {
	noSummarizerDelay(t)

	huge := strings.Repeat("y", 50_000)
	fa := &fakeAgent{
		name:             "m",
		maxContextTokens: 10,
		responses: []*MessageResponse{
			textResponse(huge),            // summarizer: rebuild rejected
			toolUseResponse("bash", "t1"), // turn 1: drives a second iteration
			textResponse("done"),          // turn 2: ends the turn
		},
	}

	ai := NewAI(fa, audit.NewAudit(audit.NewNoopLogger()))
	ai.SetQuiet(true)
	ai.lastInputTokens = 1000

	ai.RegisterTool(&noopTool{})

	if _, err := ai.RunTurn([]InputMessage{
		NewTextMessage(RoleUser, "original"),
		NewTextMessage(RoleAssistant, "a1"),
		NewTextMessage(RoleUser, "u2"),
		NewTextMessage(RoleAssistant, "a2"),
		NewTextMessage(RoleUser, "u3"),
	}); err != nil {
		t.Fatalf("RunTurn() error = %v", err)
	}

	// 1 summarizer + 2 model turns. A second summarizer call would mean the
	// rejection was not remembered.
	if len(fa.requests) != 3 {
		t.Errorf("made %d calls, want 3 (one summarizer attempt only)", len(fa.requests))
	}
}

// TestSerializeForSummaryKeepsLongUserInstructions covers the half of B2 a
// prompt cannot fix: a constraint longer than the tool-output cap was truncated
// away before the summarizer ever saw it.
func TestSerializeForSummaryKeepsLongUserInstructions(t *testing.T) {
	filler := strings.Repeat("context that does not matter. ", 120) // > 2000 chars
	instruction := filler + " Use approach B, and never touch vendor/."

	out := serializeForSummary([]InputMessage{
		NewTextMessage(RoleUser, instruction),
	})

	if !strings.Contains(out, "Use approach B, and never touch vendor/.") {
		t.Error("the constraint at the end of a long user message was truncated away")
	}

	// Tool output keeps the tight cap: it is the bulk of a history and is not
	// where constraints live.
	toolOut := serializeForSummary([]InputMessage{
		{Role: RoleUser, Content: []ContentBlock{
			NewToolResultContentBlock("t1", strings.Repeat("z", 5000), false),
		}},
	})

	if !strings.Contains(toolOut, "[truncated") {
		t.Error("large tool output should still be truncated")
	}
}
