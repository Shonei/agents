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
	defaultKeepLastTurns = 4

	// summaryPrefix marks synthetic messages produced by compaction so they
	// are easy to identify in logs and audits.
	summaryPrefix = "[Previous conversation summary]\n"

	// maxEvictedBlockChars caps each serialized block when building the
	// summarizer prompt so a single huge tool result cannot blow the
	// summarizer's own context budget.
	maxEvictedBlockChars = 2000

	summarizerSystemPrompt = `You are a conversation summarizer for an autonomous coding agent.
Produce a concise but information-dense summary of the prior conversation.
Preserve:
- Decisions and conclusions reached
- File paths read, written, or discussed
- Tool calls made and their key outcomes (success/failure, what was produced)
- Errors encountered and how they were addressed
- Open questions or pending work
Drop pleasantries, repeated context, and verbose tool output.
Output plain text only. No markdown headings, no preamble.`
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
	for keep := defaultKeepLastTurns; keep >= 1; keep-- {
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

	summary, err := a.summarizeHistory(toEvict)
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
// boundaries (plain user-text messages with no tool_result blocks) and
// returns the index of the keepLastTurns-th such boundary from the end.
// That index becomes the start of the kept tail; everything before is safe
// to evict because it does not orphan a tool_use/tool_result pair.
// Returns -1 when there are not enough boundaries to make a safe cut.
func findCompactionCut(messages []InputMessage, keepLastTurns int) int {
	if keepLastTurns <= 0 {
		return -1
	}

	seen := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if !isUserTextBoundary(messages[i]) {
			continue
		}

		seen++
		if seen == keepLastTurns {
			return i
		}
	}

	return -1
}

// isUserTextBoundary reports whether a message is a "fresh" user message
// (plain text, no tool_result blocks) that can safely act as a cut point.
func isUserTextBoundary(msg InputMessage) bool {
	if msg.Role != RoleUser {
		return false
	}

	switch v := msg.Content.(type) {
	case string:
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

// summarizeHistory asks the underlying agent to produce a summary of the
// given messages. The summarizer call uses the same model but with no tools
// and a dedicated system prompt so the output is plain prose.
func (a *AI) summarizeHistory(toEvict []InputMessage) (string, error) {
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
// single oversized tool result cannot dominate the prompt.
func serializeForSummary(messages []InputMessage) string {
	var b strings.Builder

	for _, msg := range messages {
		b.WriteString(strings.ToUpper(msg.Role))
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
