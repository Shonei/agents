package context

import (
	"fmt"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/provider"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/audit"
	"github.com/Shonei/agents/pkg/storage"
)

// loaded bundles everything the compact/handoff previews need from a recorded
// session.
type loaded struct {
	session *storage.AuditSession
	replay  *Replay
}

// loadSession resolves a session ID (or unique prefix) and replays its events
// into an sdk history.
func loadSession(c *config.ConfigFactory, id string) (*loaded, error) {
	db := c.GetDB()

	session, err := db.GetAuditSession(id)
	if err != nil {
		return nil, err
	}

	events, err := db.GetAuditEvents(session.ID)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("session %s has no recorded events", session.ID)
	}

	replay := ReplayEvents(events)
	if len(replay.Messages) == 0 {
		return nil, fmt.Errorf("session %s produced no replayable messages (only non-history events)", session.ID)
	}

	return &loaded{session: session, replay: replay}, nil
}

// summarizerAI builds a bare sdk.AI used only to run the summarizer. It is
// wired to a noop audit logger on purpose: previewing must never write events
// into the audit log it is reading from.
//
// agentName selects a configured agent (so its model and max_context_turns
// apply); modelOverride bypasses the config entirely.
func summarizerAI(c *config.ConfigFactory, agentName, modelOverride string) (*sdk.AI, config.Agent, error) {
	var agentCfg config.Agent

	switch {
	case modelOverride != "":
		agentCfg = config.Agent{Name: "(--model)", Model: modelOverride}
	case agentName != "":
		agentCfg = c.GetAgent(agentName)
	default:
		return nil, agentCfg, fmt.Errorf("pass --agent <name> to borrow a configured agent's model, or --model <id> to name one directly")
	}

	modelAgent, err := provider.New(c, agentCfg)
	if err != nil {
		return nil, agentCfg, err
	}

	ai := sdk.NewAI(modelAgent, audit.NewAudit(audit.NewNoopLogger()))
	ai.SetQuiet(true)

	return ai, agentCfg, nil
}
