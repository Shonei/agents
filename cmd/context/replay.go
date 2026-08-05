// Package context provides commands for inspecting recorded conversations and
// replaying the context transforms (compaction, handoff) over them.
package context

import (
	"encoding/json"
	"fmt"

	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/audit"
	"github.com/Shonei/agents/pkg/storage"
)

// Replay is a history reconstructed from audit events, plus the caveats that
// apply to it.
type Replay struct {
	Messages []sdk.InputMessage
	// Warnings describes information the audit log does not carry, so callers
	// can tell the user how far to trust the reconstruction.
	Warnings []string
	// Skipped counts events that are not part of the conversation history
	// (grounding, plan, todo, route_selection), keyed by event type.
	Skipped map[string]int
	// Compactions is the number of compaction events already recorded in this
	// session, i.e. times the live run compacted.
	Compactions int
}

// ReplayEvents reconstructs an sdk history from a session's audit events.
//
// This is a reconstruction, not the original: the audit log records a
// function_call's name and input but not its tool_use ID, and does not record
// thinking blocks at all. Tool calls are therefore paired with their results
// positionally (FIFO, which matches the order processTools emits them) and given
// synthetic IDs. Message roles, turn boundaries, and block structure — the
// things compaction and handoff actually key off — are preserved faithfully.
func ReplayEvents(events []storage.AuditEvent) *Replay {
	r := &Replay{
		Skipped: make(map[string]int),
	}

	var (
		pendingAssistant []sdk.ContentBlock
		pendingResults   []sdk.ContentBlock
		openCallIDs      []string
		callSeq          int
		sawThinkingGap   bool
	)

	flushAssistant := func() {
		if len(pendingAssistant) == 0 {
			return
		}

		r.Messages = append(r.Messages, sdk.InputMessage{
			Role:    sdk.RoleAssistant,
			Content: pendingAssistant,
		})
		pendingAssistant = nil
	}

	flushResults := func() {
		if len(pendingResults) == 0 {
			return
		}

		r.Messages = append(r.Messages, sdk.InputMessage{
			Role:    sdk.RoleUser,
			Content: pendingResults,
		})
		pendingResults = nil
	}

	for _, ev := range events {
		switch ev.Type {
		case audit.InitialMessageEvent, audit.UserMessageEvent:
			flushResults()
			flushAssistant()
			r.Messages = append(r.Messages, sdk.NewTextMessage(sdk.RoleUser, ev.Content.String))

		case audit.AssistantMessageEvent:
			flushResults()
			pendingAssistant = append(pendingAssistant, sdk.ContentBlock{
				Type: sdk.ContentTypeText,
				Text: ev.Content.String,
			})

		case audit.FunctionCallEvent:
			flushResults()

			callSeq++
			id := fmt.Sprintf("replay_call_%d", callSeq)
			openCallIDs = append(openCallIDs, id)

			name, input := decodeFunctionCall(ev.Payload.String)
			pendingAssistant = append(pendingAssistant, sdk.ContentBlock{
				Type:  sdk.ContentTypeToolUse,
				ID:    id,
				Name:  name,
				Input: input,
			})

		case audit.FunctionResponseEvent:
			flushAssistant()

			id := ""
			if len(openCallIDs) > 0 {
				id = openCallIDs[0]
				openCallIDs = openCallIDs[1:]
			}

			pendingResults = append(pendingResults, sdk.ContentBlock{
				Type:      sdk.ContentTypeToolResult,
				ToolUseID: id,
				Content:   decodeFunctionResponse(ev.Payload.String),
			})

		case audit.CompactionEvent:
			// A compaction that already happened in the live run. The audited
			// history keeps every event either side of it, so replaying the
			// full transcript shows more than the agent actually saw.
			r.Compactions++

		default:
			r.Skipped[ev.Type]++
		}

		if ev.Type == audit.AssistantMessageEvent || ev.Type == audit.FunctionCallEvent {
			sawThinkingGap = true
		}
	}

	flushAssistant()
	flushResults()

	if sawThinkingGap {
		r.Warnings = append(r.Warnings,
			"thinking blocks are not recorded in the audit log, so the replayed history omits them")
	}

	if callSeq > 0 {
		r.Warnings = append(r.Warnings,
			"tool_use IDs are not recorded; calls were paired with results positionally and given synthetic IDs")
	}

	if len(openCallIDs) > 0 {
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("%d tool call(s) have no recorded result (session likely ended mid-turn)", len(openCallIDs)))
	}

	if r.Compactions > 0 {
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("this session compacted %d time(s) during the live run; the replay shows the full transcript, not the truncated history the agent saw", r.Compactions))
	}

	return r
}

// decodeFunctionCall pulls the tool name and input out of a function_call
// payload, which the audit logger writes as {"name":…,"input":…}.
func decodeFunctionCall(payload string) (string, map[string]interface{}) {
	var wire struct {
		Name  string                 `json:"name"`
		Input map[string]interface{} `json:"input"`
	}

	if payload == "" {
		return "unknown", nil
	}

	if err := json.Unmarshal([]byte(payload), &wire); err != nil {
		return "unknown", nil
	}

	if wire.Name == "" {
		wire.Name = "unknown"
	}

	return wire.Name, wire.Input
}

// decodeFunctionResponse pulls the result out of a function_response payload,
// which the audit logger writes as {"name":<tool_use_id>,"response":…}.
func decodeFunctionResponse(payload string) string {
	var wire struct {
		Response any `json:"response"`
	}

	if payload == "" {
		return ""
	}

	if err := json.Unmarshal([]byte(payload), &wire); err != nil {
		return payload
	}

	switch v := wire.Response.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}

		return string(b)
	}
}
