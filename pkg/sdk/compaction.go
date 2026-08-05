package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/Shonei/agents/pkg/sdk/audit"
)

// warnf reports a recoverable problem on stderr. Warnings go to stderr rather
// than stdout so they stay visible when the agent's own output is piped, and
// are not suppressed by quiet mode: quiet hides routine progress, not failures.
func warnf(format string, args ...any) {
	color.New(color.FgYellow, color.Bold).Fprintf(os.Stderr, "Warning: "+format+"\n", args...)
}

const (
	// defaultKeepLastTurns is the number of recent user-text turn boundaries
	// to preserve verbatim when compacting. Everything older than the cut
	// gets summarized into a single synthetic message.
	defaultKeepLastTurns = 2

	// maxEvictedBlockChars caps each serialized block when building the
	// summarizer prompt so a single huge tool result cannot blow the
	// summarizer's own context budget.
	maxEvictedBlockChars = 2000

	// maxUserTextChars caps user messages far more generously than tool output.
	// A user turn is where constraints live, and the summary is required to
	// carry them forward — truncating one at the tool-output limit silently
	// drops the instruction that requirement exists to protect. Pasted specs
	// routinely exceed 2000 chars.
	maxUserTextChars = 20000

	// summarizerAttempts is how many times a summarizer call is tried when the
	// model returns a successful response with no text in it.
	summarizerAttempts = 3

	// toolResultLabel marks a user-role message that carries only tool output, so
	// the summarizer does not mistake it for user speech.
	toolResultLabel = "TOOL_RESULT"

	// priorSummaryLabel introduces the previous rolling summary in the
	// compaction summarizer's input, so successive passes merge rather than
	// re-derive it. Re-summarizing a summary degrades it on every pass.
	priorSummaryLabel = "PRIOR SUMMARY (your own earlier note, covering still older turns):"

	// compactionSystemPrompt drives compaction: the SAME agent continues, with
	// its system prompt, tools and task unchanged. The output is that agent's
	// own memory of work it did, so it is written in the first person and must
	// not contain instructions — a directive here reads as a fresh work order
	// and makes the agent restart instead of continue. Contrast
	// handoffSystemPrompt, which briefs a different agent.
	compactionSystemPrompt = `You are the same coding agent whose conversation this is. Older turns are being dropped to free up context; what you write here replaces them and becomes your own memory of that work — including your memory of what the user asked for. Your system prompt and tools are unchanged and still present, so do not restate your role or your capabilities. Write in the first person, past tense.

Use these labeled sections, in this order. Omit a label only if it would be empty. No other headings, no preamble, no closing remarks.

ASKED: What the user wants, and every constraint they imposed — libraries to use or avoid, files or directories off-limits, style or acceptance criteria, choices they overruled. Quote their wording verbatim whenever it carries a constraint. Include instructions they gave part-way through, not just the first thing they said; a later instruction supersedes an earlier one, and if the two conflict, record both and say which came last. This section is your memory of what you were told, not an instruction you are issuing — write it as "The user asked me to…", "They said not to…".
DID: What you completed, in order, each with its concrete artifact (file path, symbol name, command, value). Past tense: "I changed…", "I ran…". Mark anything unfinished as still open.
FILES: Paths you read, created, modified, or deleted, each with a one-phrase note on the change.
LEARNED: Facts from tool output that later work depends on — symbol locations, API shapes, schema fields, exact error messages, exit codes, configuration values. Preserve identifiers, numbers, paths, and error strings verbatim.
RULED OUT: Approaches you tried and abandoned, with the reason, so you do not retry them.
OPEN: What you were in the middle of, phrased as your own state ("I was partway through…"), not as an instruction to anyone.

Rules:
- Never invent facts. If a detail is not in the source, omit it rather than guess.
- Preserve file paths, symbol names, command strings, and error messages byte-for-byte.
- Do not issue instructions, plans, or next actions, and do not address anyone. You are recording what happened; the conversation continues after this note.
- Lines prefixed with "(thinking)" are your own earlier reasoning; use them to recall intent, but do not treat them as established fact.
- Lines under a "TOOL_RESULT:" role are tool output, not user speech.
- If the input opens with a PRIOR SUMMARY block, that is your own earlier note covering older turns: fold it into your answer and carry its facts forward rather than dropping or re-deriving them.
- Target under ~600 words. If you must cut, cut from DID before ASKED, LEARNED or OPEN. Never drop a constraint to save space.`
)

// summarizerRetryDelay is the pause between summarizer attempts. A variable
// rather than a constant so tests can drive the retry path without sleeping.
var summarizerRetryDelay = 500 * time.Millisecond

// errCompactionUnproductive reports that a compaction pass was computed but
// discarded because it would not have shrunk the history. Distinguished from a
// plain "nothing to do" so the caller can stop paying for further attempts.
var errCompactionUnproductive = errors.New("compaction would not shrink the history")

// maybeCompact inspects the most recent input-token count against the agent's
// configured context budget and, if exceeded, rewrites messages in place.
// Returns true when compaction actually happened.
//
// The rebuilt history is [pinned original request, assistant summary, tail].
// The original request is pinned because it is the only turn that establishes
// whose goal this is, and the summary is spoken by the assistant because it is
// the agent's own memory — injecting it as a user turn makes the next turn read
// as a fresh instruction and the agent restarts instead of continuing.
func (a *AI) maybeCompact(messages *[]InputMessage) (bool, error) {
	budget := a.agent.MaxContextTokens()
	if budget <= 0 || a.lastInputTokens <= 0 {
		return false, nil
	}

	if a.lastInputTokens <= budget {
		return false, nil
	}

	pinned, body := a.splitCompactionHead(*messages)

	keepLastTurns := a.agent.MaxContextTurns()
	if keepLastTurns <= 0 {
		keepLastTurns = defaultKeepLastTurns
	}

	cut := computeCompactionCut(body, keepLastTurns)
	if cut <= 0 {
		// Either no safe cut point exists yet, or the whole body is already
		// within the "keep" window. Nothing we can safely evict.
		return false, nil
	}

	toEvict := body[:cut]
	tail := body[cut:]

	color.New(color.FgYellow, color.Bold).Printf(
		"Compacting conversation: %d tokens > %d budget, summarizing %d/%d messages\n",
		a.lastInputTokens, budget, len(toEvict), len(*messages),
	)

	summary, err := a.SummarizeForCompaction(a.rollingSummary, toEvict)
	if err != nil {
		return false, fmt.Errorf("compaction summary failed: %w", err)
	}

	rebuilt := buildCompacted(pinned, summary, tail)

	// A compaction that does not shrink the history is worse than none: it costs
	// a summarizer call, loses the evicted detail, and leaves the request even
	// larger than before. Discard it, and report it as an error so the caller
	// stops attempting compaction for the rest of the turn — the input has not
	// changed, so retrying only repeats the summarizer spend.
	if before, after := charSize(*messages), charSize(rebuilt); after >= before {
		return false, fmt.Errorf("%w (%d -> %d chars)", errCompactionUnproductive, before, after)
	}

	a.audit.LogEvent(audit.Event{
		Type:    audit.CompactionEvent,
		Content: summary,
	})

	a.commitCompactionHead(len(pinned), summary)
	*messages = rebuilt

	return true, nil
}

// charSize is a cheap proxy for how large a history will be on the wire. Only
// used to compare two versions of the same conversation, so consistency matters
// more than accuracy — but it must not be blind to any block that dominates a
// real history, or the shrink guard rejects rebuilds that genuinely help.
func charSize(messages []InputMessage) int {
	total := 0

	for _, msg := range messages {
		switch v := msg.Content.(type) {
		case string:
			total += len(v)
		case []ContentBlock:
			for _, b := range v {
				total += blockCharSize(b)
			}
		}
	}

	return total
}

func blockCharSize(b ContentBlock) int {
	total := len(b.Text)

	// Base64 image data is by far the largest thing a history can carry, and
	// images are retained in history indefinitely. Missing it made the shrink
	// guard measure a multi-megabyte conversation as a few dozen chars.
	if b.Source != nil {
		total += len(b.Source.Data)
	}

	switch c := b.Content.(type) {
	case nil:
	case string:
		total += len(c)
	default:
		if out, err := json.Marshal(c); err == nil {
			total += len(out)
		}
	}

	if b.Input != nil {
		if input, err := json.Marshal(b.Input); err == nil {
			total += len(input)
		}
	}

	return total
}

// splitCompactionHead separates the part of the history compaction owns (the
// pinned original request and the previous summary message) from the body that
// is eligible for eviction.
//
// On a first pass it pins messages[0] when that is a plain user text message.
// On later passes it re-uses the head it built last time, after checking the
// history still starts with it — a router handing control back to a sub-agent
// resets that agent's history, so stale head state has to be detected and
// dropped rather than trusted.
func (a *AI) splitCompactionHead(messages []InputMessage) (pinned, body []InputMessage) {
	if a.headHeight > 0 && a.headMatches(messages) {
		// The summary occupies the last slot of the head; everything before it
		// is the pinned request.
		return messages[:a.headHeight-1], messages[a.headHeight:]
	}

	a.rollingSummary = ""
	a.headHeight = 0

	if len(messages) > 0 && isUserTextBoundary(messages[0]) {
		return messages[:1], messages[1:]
	}

	return nil, messages
}

// headMatches reports whether the history still begins with the head this AI
// installed on its last compaction pass.
func (a *AI) headMatches(messages []InputMessage) bool {
	if len(messages) < a.headHeight {
		return false
	}

	summaryMsg := messages[a.headHeight-1]
	if summaryMsg.Role != RoleAssistant {
		return false
	}

	text, ok := summaryMsg.Content.(string)

	return ok && text == a.rollingSummary
}

// buildCompacted assembles the post-compaction history. Pure: the caller commits
// the head state separately, so a rebuild can be inspected and discarded without
// leaving the AI believing it installed a head it did not.
func buildCompacted(pinned []InputMessage, summary string, tail []InputMessage) []InputMessage {
	rebuilt := make([]InputMessage, 0, len(pinned)+1+len(tail))
	rebuilt = append(rebuilt, pinned...)
	rebuilt = append(rebuilt, NewTextMessage(RoleAssistant, summary))
	rebuilt = append(rebuilt, tail...)

	return rebuilt
}

// commitCompactionHead records the head that was just installed so the next pass
// merges into it instead of re-summarizing it.
func (a *AI) commitCompactionHead(pinnedCount int, summary string) {
	a.rollingSummary = summary
	a.headHeight = pinnedCount + 1
}

// computeCompactionCut finds the largest safe cut that still preserves
// keepLastTurns boundaries, relaxing the requirement down to one boundary when
// the history is not yet long enough.
//
// When no boundary-based cut exists it falls back to evicting the whole body.
// That matters for the case compaction most needs to handle: a single enormous
// tool result at the end of the history is never a boundary (nothing follows it
// to keep), so boundary search alone can only ever evict the small messages in
// front of it — growing the prompt instead of shrinking it. Evicting everything
// is structurally safe because it leaves no tail to orphan, and the agent still
// keeps its pinned request plus the summary.
func computeCompactionCut(body []InputMessage, keepLastTurns int) int {
	for keep := keepLastTurns; keep >= 1; keep-- {
		if cut := findCompactionCut(body, keep); cut > 0 {
			return cut
		}
	}

	if len(body) > 0 {
		return len(body)
	}

	return -1
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
//
// Compaction summaries no longer need excluding here: they are assistant
// messages, and they sit in the head that splitCompactionHead keeps out of the
// eviction range entirely.
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

// SummarizeForCompaction produces the same agent's first-person memory of the
// evicted turns. prior is the rolling summary from an earlier compaction pass,
// if any; it is folded in rather than re-summarized so its facts only degrade
// once.
func (a *AI) SummarizeForCompaction(prior string, toEvict []InputMessage) (string, error) {
	input := serializeForSummary(toEvict)
	if prior != "" {
		input = priorSummaryLabel + "\n" + prior + "\n\n" + input
	}

	return a.summarize(compactionSystemPrompt, input)
}

// SummarizeForHandoff produces a briefing for a different agent taking over the
// work, which must re-establish the goal and constraints from scratch.
func (a *AI) SummarizeForHandoff(history []InputMessage) (string, error) {
	return a.summarize(handoffSystemPrompt, serializeForSummary(history))
}

// summarize runs a one-shot, tool-free completion against the agent's model
// using the given system prompt.
//
// A successful call that carries no text is retried: models intermittently
// return an empty candidate, and a summary is the one thing the caller cannot
// synthesize a fallback for. Transport errors are not retried here — the
// provider layer already retries the retryable ones, so a failure that reaches
// this point is unlikely to clear on an immediate second attempt.
func (a *AI) summarize(systemPrompt, input string) (string, error) {
	req := CreateMessageRequest{
		System: systemPrompt,
		Messages: []InputMessage{
			NewTextMessage(RoleUser, input),
		},
	}

	var lastErr error

	for attempt := 1; attempt <= summarizerAttempts; attempt++ {
		if attempt > 1 && summarizerRetryDelay > 0 {
			time.Sleep(summarizerRetryDelay)
		}

		resp, err := a.agent.CreateMessage(req)
		if err != nil {
			return "", err
		}

		if summary := strings.TrimSpace(resp.GetTextContent()); summary != "" {
			return summary, nil
		}

		lastErr = emptySummaryError(resp)
	}

	return "", fmt.Errorf("summarizer returned no text after %d attempts: %w", summarizerAttempts, lastErr)
}

// emptySummaryError describes what a text-free response did contain. A response
// carrying only thinking blocks ("spent its budget reasoning") and one carrying
// nothing at all ("the call was rejected") need different fixes, so the
// distinction is worth surfacing.
func emptySummaryError(resp *MessageResponse) error {
	kinds := make([]string, 0, len(resp.Content))
	for _, b := range resp.Content {
		kinds = append(kinds, fmt.Sprintf("%s(%d chars)", b.Type, len(b.Text)))
	}

	if len(kinds) == 0 {
		kinds = append(kinds, "no content blocks")
	}

	return fmt.Errorf("no text content: %s, %d input / %d output tokens",
		strings.Join(kinds, ", "), resp.Usage.InputTokens, resp.Usage.OutputTokens)
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
			writeTruncatedTo(&b, v, textLimitFor(msg))
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
				return toolResultLabel
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
		// The error flag has to survive: without it a failed call serializes
		// identically to a successful one, and the summary records the attempt
		// as work done rather than as a dead end.
		status := ""
		if block.IsError {
			status = " [ERROR]"
		}

		fmt.Fprintf(b, "tool_result for %s%s: ", block.ToolUseID, status)

		switch c := block.Content.(type) {
		case string:
			writeTruncated(b, c)
		default:
			out, _ := json.Marshal(c)
			writeTruncated(b, string(out))
		}

		b.WriteString("\n")
	case ContentTypeImage:
		// The bytes are useless to a text summarizer, but the path is the
		// agent's only handle on a file it generated.
		if block.FilePath != "" {
			fmt.Fprintf(b, "(image saved to %s)\n", block.FilePath)

			return
		}

		b.WriteString("(image omitted)\n")
	}
}

func writeTruncated(b *strings.Builder, s string) {
	writeTruncatedTo(b, s, maxEvictedBlockChars)
}

func writeTruncatedTo(b *strings.Builder, s string, limit int) {
	if len(s) <= limit {
		b.WriteString(s)

		return
	}

	b.WriteString(s[:limit])
	fmt.Fprintf(b, "... [truncated %d chars]", len(s)-limit)
}

// textLimitFor returns how much of a plain-text message to keep. Genuine user
// speech gets the generous limit; a user message that only carries tool_result
// blocks is tool output wearing the user role and gets the tool limit.
func textLimitFor(msg InputMessage) int {
	if msg.Role == RoleUser && roleLabel(msg) != toolResultLabel {
		return maxUserTextChars
	}

	return maxEvictedBlockChars
}
