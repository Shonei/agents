package sdk

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"

	"github.com/Shonei/agents/pkg/sdk/audit"
)

const (
	// defaultKeepLastTurns is the number of recent user-text turn boundaries
	// to preserve verbatim when compacting. Everything older than the cut
	// gets summarized into a single synthetic message.
	defaultKeepLastTurns = 2

	// summaryPrefix marks synthetic messages produced by compaction so they
	// are easy to identify in logs and audits.
	summaryPrefix = "[Previous conversation summary]\n"

	// maxEvictedBlockChars caps each serialized block when building the
	// summarizer prompt so a single huge tool result cannot blow the
	// summarizer's own context budget.
	maxEvictedBlockChars = 2000

	summarizerSystemPrompt = `You are compacting a coding-agent conversation. Your output REPLACES the prior messages: the same agent (or a peer agent with similar tools) will read it as its only memory of what happened and must be able to resume work without seeing the original. Write for that future agent, not for a human reviewer.

Use these labeled sections, in this order. Omit a label only if it would be empty. No other headings, no preamble, no closing remarks.

GOAL: The user's original request and any constraints they imposed (libraries to use or avoid, files off-limits, style or acceptance criteria). Quote constraints verbatim when wording matters.
PROGRESS: What has been completed, in order, with the concrete artifact for each step (file path, symbol name, command, value). Mark anything still in flight as "in progress".
FILES: Paths read, created, modified, or deleted, each with a one-phrase note on the change.
KEY FINDINGS: Facts learned from tool output that future steps depend on — symbol locations, API shapes, schema fields, exact error messages, exit codes, configuration values. Preserve identifiers, numbers, paths, and error strings verbatim.
DEAD ENDS: Approaches tried and abandoned, with the reason, so they are not retried.
NEXT: The single most immediate action the agent should take, specific enough to act on without further inference. Then any other pending work.

Rules:
- Never invent facts. If a detail is not in the source, omit it rather than guess.
- Preserve file paths, symbol names, command strings, and error messages byte-for-byte.
- Lines prefixed with "(thinking)" are the prior agent's internal reasoning; use them to infer intent but do not treat them as ground truth.
- Lines under a "TOOL_RESULT:" role are tool output, not user speech; attribute them accordingly.
- Drop pleasantries, restated context, and prose around tool output; keep the load-bearing details inside it.
- Target under ~600 words. If you must cut, cut from PROGRESS before GOAL or NEXT.`
)

// maybeCompact inspects the most recent input-token count against the agent's
// configured context budget and, if exceeded, rewrites messages in place to
// replace the older prefix with a single synthetic summary message. Returns
// true when compaction actually happened.
func (a *AI) maybeCompact(messages *[]InputMessage) (bool, error) {
	budget := a.agent.MaxContextTokens()
	if budget <= 0 || a.lastInputTokens <= 0 {
		return false, nil
	}

	if a.lastInputTokens <= budget {
		return false, nil
	}

	cut := -1
	keepLastTurns := a.agent.MaxContextTurns()
	if keepLastTurns <= 0 {
		keepLastTurns = defaultKeepLastTurns
	}

	for keep := keepLastTurns; keep >= 1; keep-- {
		cut = findCompactionCut(*messages, keep)
		if cut > 0 {
			break
		}
	}

	if cut <= 0 {
		// Either no safe cut point exists yet, or the entire history is
		// already within the "keep" window. Nothing we can safely evict.
		return false, nil
	}

	toEvict := (*messages)[:cut]
	tail := (*messages)[cut:]

	color.New(color.FgYellow, color.Bold).Printf(
		"Compacting conversation: %d tokens > %d budget, summarizing %d/%d messages\n",
		a.lastInputTokens, budget, len(toEvict), len(*messages),
	)

	summary, err := a.SummarizeMessages(toEvict)
	if err != nil {
		return false, fmt.Errorf("compaction summary failed: %w", err)
	}

	a.audit.LogEvent(audit.Event{
		Type:    audit.CompactionEvent,
		Content: summary,
	})

	rebuilt := make([]InputMessage, 0, 1+len(tail))
	rebuilt = append(rebuilt, NewTextMessage(RoleUser, summaryPrefix+summary))
	rebuilt = append(rebuilt, tail...)
	*messages = rebuilt

	return true, nil
}

// findCompactionCut walks backwards through messages looking for clean turn
// boundaries. A boundary is either a plain user-text message (cut before it)
// or the end of a complete tool-call round, i.e. a user tool_result message
// followed by at least one later message (cut after it). It returns the cut
// index that preserves keepLastTurns boundaries from the end. Returns -1
// when there are not enough boundaries to make a safe cut.
func findCompactionCut(messages []InputMessage, keepLastTurns int) int {
	if keepLastTurns <= 0 {
		return -1
	}

	seen := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if isUserTextBoundary(messages[i]) {
			seen++
			if seen == keepLastTurns {
				return i
			}

			continue
		}

		if isToolResultBoundary(messages, i) {
			seen++
			if seen == keepLastTurns {
				return i + 1
			}
		}
	}

	return -1
}

// isToolResultBoundary reports whether messages[i] is a user message that
// contains only tool_result blocks and is followed by at least one later
// message. Cutting after such a message keeps the corresponding tool_use in
// the evicted prefix and starts the kept tail with the assistant's response,
// which is structurally safe.
func isToolResultBoundary(messages []InputMessage, i int) bool {
	if i+1 >= len(messages) {
		return false
	}

	msg := messages[i]
	if msg.Role != RoleUser {
		return false
	}

	blocks, ok := msg.Content.([]ContentBlock)
	if !ok || len(blocks) == 0 {
		return false
	}

	for _, b := range blocks {
		if b.Type != ContentTypeToolResult {
			return false
		}
	}

	return true
}

// isUserTextBoundary reports whether a message is a "fresh" user message
// (plain text, no tool_result blocks) that can safely act as a cut point.
// Synthetic summary messages produced by compaction are excluded so they
// can be re-summarized in later compaction passes.
func isUserTextBoundary(msg InputMessage) bool {
	if msg.Role != RoleUser {
		return false
	}

	switch v := msg.Content.(type) {
	case string:
		if strings.HasPrefix(v, summaryPrefix) {
			return false
		}

		return true
	case []ContentBlock:
		for _, b := range v {
			if b.Type == ContentTypeToolResult {
				return false
			}
		}

		return len(v) > 0
	default:
		return false
	}
}

// SummarizeMessages asks the underlying agent to produce a plain-prose
// summary of the given messages. Used both by the in-AI compaction loop
// and by RouterAI when generating a handoff summary. The call uses the
// same model but with no tools and a dedicated system prompt.
func (a *AI) SummarizeMessages(toEvict []InputMessage) (string, error) {
	serialized := serializeForSummary(toEvict)

	req := CreateMessageRequest{
		System: summarizerSystemPrompt,
		Messages: []InputMessage{
			NewTextMessage(RoleUser, serialized),
		},
	}

	resp, err := a.agent.CreateMessage(req)
	if err != nil {
		return "", err
	}

	summary := strings.TrimSpace(resp.GetTextContent())
	if summary == "" {
		return "", fmt.Errorf("summarizer returned no text content")
	}

	return summary, nil
}

// serializeForSummary renders a slice of messages into a compact text form
// suitable for feeding to the summarizer. Each block is truncated so a
// single oversized tool result cannot dominate the prompt. User messages
// that carry only tool_result blocks are relabeled as TOOL_RESULT so the
// summarizer does not mistake tool output for user speech.
func serializeForSummary(messages []InputMessage) string {
	var b strings.Builder

	for _, msg := range messages {
		b.WriteString(roleLabel(msg))
		b.WriteString(":\n")

		switch v := msg.Content.(type) {
		case string:
			writeTruncated(&b, v)
			b.WriteString("\n")
		case []ContentBlock:
			for _, block := range v {
				writeBlock(&b, block)
			}
		}

		b.WriteString("\n")
	}

	return b.String()
}

// roleLabel returns the section header to use for a message in the
// serialized summary input. User messages whose content is exclusively
// tool_result blocks are labeled TOOL_RESULT so the summarizer treats
// them as tool output rather than user speech.
func roleLabel(msg InputMessage) string {
	if msg.Role == RoleUser {
		if blocks, ok := msg.Content.([]ContentBlock); ok && len(blocks) > 0 {
			allToolResults := true
			for _, b := range blocks {
				if b.Type != ContentTypeToolResult {
					allToolResults = false

					break
				}
			}

			if allToolResults {
				return "TOOL_RESULT"
			}
		}
	}

	return strings.ToUpper(msg.Role)
}

func writeBlock(b *strings.Builder, block ContentBlock) {
	switch block.Type {
	case ContentTypeText:
		writeTruncated(b, block.Text)
		b.WriteString("\n")
	case ContentTypeThinking:
		b.WriteString("(thinking) ")
		writeTruncated(b, block.Text)
		b.WriteString("\n")
	case ContentTypeToolUse:
		input, _ := json.Marshal(block.Input)
		fmt.Fprintf(b, "tool_use %s(", block.Name)
		writeTruncated(b, string(input))
		b.WriteString(")\n")
	case ContentTypeToolResult:
		fmt.Fprintf(b, "tool_result for %s: ", block.ToolUseID)

		switch c := block.Content.(type) {
		case string:
			writeTruncated(b, c)
		default:
			out, _ := json.Marshal(c)
			writeTruncated(b, string(out))
		}

		b.WriteString("\n")
	case ContentTypeImage:
		b.WriteString("(image omitted)\n")
	}
}

func writeTruncated(b *strings.Builder, s string) {
	if len(s) <= maxEvictedBlockChars {
		b.WriteString(s)

		return
	}

	b.WriteString(s[:maxEvictedBlockChars])
	fmt.Fprintf(b, "... [truncated %d chars]", len(s)-maxEvictedBlockChars)
}
