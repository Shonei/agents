package sdk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"

	"github.com/Shonei/agents/pkg/sdk/audit"
	"github.com/Shonei/agents/pkg/utils"
)

// handoffPrefix marks the synthetic user message injected at the head of
// a sub-agent's working history when control changes hands. Mirrors the
// summaryPrefix convention used by in-AI compaction.
const handoffPrefix = "[Handoff from %s]\n"

// RouteMeta is the runtime view of a single configured route. Kept in
// the order declared in YAML so the classifier sees them deterministically.
type RouteMeta struct {
	Agent string
	When  string
}

// RouterAI is a top-level Chatter that dispatches each user turn to one
// of several configured sub-agents based on a per-turn classifier. On a
// confident route change, it summarizes the prior conversation and
// presents the new sub-agent with a clean starting view consisting of
// the handoff summary followed by the user's latest message.
//
// One audit session is shared across the router and all sub-agents.
type RouterAI struct {
	name       string
	routes     map[string]*AI
	routeMeta  []RouteMeta
	classifier *classifier
	current    string
	threshold  float64
	history    []InputMessage
	audit      *audit.Audit
}

// NewRouterAI constructs a router. The caller is responsible for:
//   - Building each sub-agent *AI with system prompts already set via
//     SetSystemPromptSilent (so the audit session is not stomped).
//   - Pre-registering the audit user session (via aud.User) so all
//     downstream events land in one session.
//   - Constructing classifierAgent (typically a fresh gemini.Agent
//     configured for the model named in classifier.Model).
func NewRouterAI(
	name string,
	routes map[string]*AI,
	meta []RouteMeta,
	classifierAgent Agent,
	defaultRoute string,
	confidenceThreshold float64,
	aud *audit.Audit,
) *RouterAI {
	return &RouterAI{
		name:       name,
		routes:     routes,
		routeMeta:  meta,
		classifier: newClassifier(classifierAgent, meta),
		current:    defaultRoute,
		threshold:  confidenceThreshold,
		audit:      aud,
	}
}

// Audit exposes the shared audit handle so callers can register the
// user session before Chat() starts.
func (r *RouterAI) Audit() *audit.Audit {
	return r.audit
}

// SynthesizeRouterPrompt produces a stable, hashable description of the
// router's configuration that the audit logger can record as the
// session's "system prompt". Identical router configs produce identical
// hashes, which keeps the audit-viewer's session grouping useful.
//
// Each sub-agent's own rendered system prompt is included so that the
// session hash reflects what the agents will actually say, not just the
// router shell.
func SynthesizeRouterPrompt(name string, meta []RouteMeta, routes map[string]*AI, classifierModel string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "router: %s\n", name)
	fmt.Fprintf(&b, "classifier_model: %s\n", classifierModel)
	b.WriteString("routes:\n")

	keys := make([]string, 0, len(meta))
	for _, m := range meta {
		keys = append(keys, m.Agent)
	}
	sort.Strings(keys)

	for _, key := range keys {
		var when string
		for _, m := range meta {
			if m.Agent == key {
				when = m.When

				break
			}
		}

		fmt.Fprintf(&b, "- %s: %s\n", key, when)

		if sub, ok := routes[key]; ok {
			fmt.Fprintf(&b, "  system_prompt_hash: %s\n", shortHash(sub.systemPrompt))
		}
	}

	return b.String()
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))

	return hex.EncodeToString(sum[:])[:16]
}

// Chat owns the user-input loop and drives the routing/handoff state
// machine. Mirrors (*AI).Chat for symmetry with the non-router path.
func (r *RouterAI) Chat(initial string) (string, error) {
	fmt.Println("Chat started. Press Ctrl+C to exit.")

	if _, ok := r.routes[r.current]; !ok {
		return "", fmt.Errorf("router %q: default route %q is not registered", r.name, r.current)
	}

	msg := initial
	if msg == "" {
		input, err := utils.GatherUserContent()
		if err != nil {
			return "", err
		}

		if input == "" {
			return "", nil
		}

		msg = input
	}

	r.audit.LogEvent(audit.Event{
		Type:    audit.InitialMessageEvent,
		Content: msg,
	})

	for {
		if err := r.handleTurn(msg); err != nil {
			return "", err
		}

		next, err := utils.GatherUserContent()
		if err != nil {
			return "", err
		}

		if next == "" {
			break
		}

		r.audit.LogEvent(audit.Event{
			Type:    audit.UserMessageEvent,
			Content: next,
		})

		msg = next
	}

	return "", nil
}

// handleTurn classifies the message, optionally performs a handoff, and
// then runs the active sub-agent against the updated working history.
func (r *RouterAI) handleTurn(userMsg string) error {
	pick, latency, err := r.classifier.SelectRoute(r.history, r.current, userMsg)
	if err != nil {
		// Classifier failures are non-fatal: stick with the current
		// route, log the error as a low-confidence "stay", and proceed
		// so the user is not blocked. The audit log captures the reason.
		color.New(color.FgYellow).Printf("router: classifier error, staying on %q: %v\n", r.current, err)

		pick = RouteSelection{Route: r.current, Confidence: 0, Reason: "classifier error: " + err.Error()}
	}

	r.audit.LogEvent(audit.Event{
		Type: audit.RouteSelectionEvent,
		RouteSelection: routeSelectionAudit{
			Route:      pick.Route,
			Confidence: pick.Confidence,
			Reason:     pick.Reason,
			LatencyMs:  latency.Milliseconds(),
		},
	})

	if pick.Confidence >= r.threshold && pick.Route != r.current {
		if err := r.handoff(pick.Route); err != nil {
			return err
		}
	}

	r.history = append(r.history, NewTextMessage(RoleUser, userMsg))

	active, ok := r.routes[r.current]
	if !ok {
		return fmt.Errorf("router %q: active route %q is not registered", r.name, r.current)
	}

	updated, err := active.RunTurn(r.history)
	if err != nil {
		return err
	}

	r.history = updated

	return nil
}

// handoff summarizes the working history via the outgoing sub-agent and
// replaces the working history with a single synthetic user message
// carrying that summary. The user's actual new message is appended by
// handleTurn after this returns.
func (r *RouterAI) handoff(toRoute string) error {
	from := r.current

	prev, ok := r.routes[from]
	if !ok {
		return fmt.Errorf("router %q: outgoing route %q is not registered", r.name, from)
	}

	summary := ""
	if len(r.history) > 0 {
		s, err := prev.SummarizeMessages(r.history)
		if err != nil {
			return fmt.Errorf("handoff %s->%s: summarization failed: %w", from, toRoute, err)
		}

		summary = s
	}

	color.New(color.FgYellow, color.Bold).Printf("Handoff: %s -> %s\n", from, toRoute)

	r.audit.LogEvent(audit.Event{
		Type: audit.HandoffEvent,
		Handoff: handoffAudit{
			From:    from,
			To:      toRoute,
			Summary: summary,
		},
	})

	if summary == "" {
		r.history = nil
	} else {
		r.history = []InputMessage{
			NewTextMessage(RoleUser, fmt.Sprintf(handoffPrefix, from)+summary),
		}
	}

	r.current = toRoute

	return nil
}
