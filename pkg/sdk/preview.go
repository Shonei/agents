package sdk

import (
	"fmt"
)

// CompactionPreview is the full result of running one compaction pass over a
// history, exposed for inspection rather than applied to a live conversation.
type CompactionPreview struct {
	// KeepTurns is the number of trailing turn boundaries preserved verbatim.
	KeepTurns int
	// Pinned is the head kept out of the eviction range entirely — the original
	// user request, when the history starts with one.
	Pinned []InputMessage
	// Cut is the index within the body (after Pinned) that was split at.
	Cut int
	// Evicted are the messages fed to the summarizer and dropped.
	Evicted []InputMessage
	// Kept is the tail preserved verbatim.
	Kept []InputMessage
	// SummarizerInput is the serialized transcript the summarizer saw.
	SummarizerInput string
	// Summary is the summarizer's output.
	Summary string
	// Rebuilt is the history the agent would see on its next turn.
	Rebuilt []InputMessage
}

// HandoffPreview is the result of generating a router handoff briefing from a
// history, exposed for inspection rather than applied to a live conversation.
type HandoffPreview struct {
	// From is the outgoing agent name used in the handoff prefix.
	From string
	// SummarizerInput is the serialized transcript the summarizer saw.
	SummarizerInput string
	// Summary is the summarizer's output.
	Summary string
	// Rebuilt is the history the incoming sub-agent would start from.
	Rebuilt []InputMessage
}

// PreviewCompaction runs a single compaction pass over the given history and
// returns everything about it, without consulting the token budget and without
// mutating any live conversation state. keepTurns of zero falls back to the
// agent's configured value, then to defaultKeepLastTurns.
//
// This is the same cut-selection and rebuild logic maybeCompact uses, so the
// shapes it reports are the shapes a real run would produce.
func (a *AI) PreviewCompaction(messages []InputMessage, keepTurns int) (*CompactionPreview, error) {
	preview, err := a.PlanCompaction(messages, keepTurns)
	if err != nil {
		return nil, err
	}

	summary, err := a.SummarizeForCompaction("", preview.Evicted)
	if err != nil {
		return nil, err
	}

	preview.Summary = summary
	// Uses the same builder as the live path so the preview cannot drift from
	// what a real compaction would produce.
	preview.Rebuilt = buildCompacted(preview.Pinned, summary, preview.Kept)

	return preview, nil
}

// PlanCompaction resolves the cut point and the summarizer input for a
// compaction pass without calling the model. Summary and Rebuilt are left
// empty. Useful for inspecting what a pass would evict at no cost.
func (a *AI) PlanCompaction(messages []InputMessage, keepTurns int) (*CompactionPreview, error) {
	if keepTurns <= 0 {
		keepTurns = a.agent.MaxContextTurns()
	}

	if keepTurns <= 0 {
		keepTurns = defaultKeepLastTurns
	}

	// Mirrors splitCompactionHead's first-pass behaviour without touching this
	// AI's rolling-summary state: previews are stateless by design, so they
	// always describe a first compaction pass.
	var pinned []InputMessage

	body := messages
	if len(messages) > 0 && isUserTextBoundary(messages[0]) {
		pinned = messages[:1]
		body = messages[1:]
	}

	cut := computeCompactionCut(body, keepTurns)
	if cut <= 0 {
		return nil, fmt.Errorf("no safe compaction cut found in %d messages (need at least one complete turn boundary before the kept tail)", len(messages))
	}

	evicted := body[:cut]

	return &CompactionPreview{
		KeepTurns:       keepTurns,
		Pinned:          pinned,
		Cut:             cut,
		Evicted:         evicted,
		Kept:            body[cut:],
		SummarizerInput: serializeForSummary(evicted),
	}, nil
}

// PreviewHandoff generates a handoff briefing from the given history exactly as
// RouterAI.handoff would, and returns the history the incoming sub-agent would
// start from. Unlike compaction, handoff evicts the entire history.
func (a *AI) PreviewHandoff(from string, messages []InputMessage) (*HandoffPreview, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("cannot preview a handoff from an empty history")
	}

	summary, err := a.SummarizeForHandoff(messages)
	if err != nil {
		return nil, err
	}

	return &HandoffPreview{
		From:            from,
		SummarizerInput: serializeForSummary(messages),
		Summary:         summary,
		Rebuilt: []InputMessage{
			NewTextMessage(RoleUser, fmt.Sprintf(handoffPrefix, from)+summary),
		},
	}, nil
}

// CompactionSystemPrompt returns the prompt that drives compaction, where the
// same agent continues and the summary is its own memory.
func CompactionSystemPrompt() string {
	return compactionSystemPrompt
}

// HandoffSystemPrompt returns the prompt that drives a router handoff, where a
// different agent takes over and must be briefed from scratch.
func HandoffSystemPrompt() string {
	return handoffSystemPrompt
}
