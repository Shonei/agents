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
// a sub-agent's working history when control changes hands.
const handoffPrefix = "[Handoff from %s]\n"

// handoffSystemPrompt drives a router handoff: a DIFFERENT agent takes over,
// with its own system prompt, its own tools, and none of this conversation. It
// therefore has to re-establish the goal and constraints from scratch, and a
// directive ("do this next") is correct here — it is a briefing, not a memory.
// Contrast compactionSystemPrompt, where the same agent continues.
const handoffSystemPrompt = `You are briefing a different agent that is taking over this work. It has its own system prompt, its own tools, and none of this conversation: what you write is the only thing it will know about what came before. Write for that agent, not for a human reviewer.

Use these labeled sections, in this order. Omit a label only if it would be empty. No other headings, no preamble, no closing remarks.

GOAL: The user's original request and any constraints they imposed (libraries to use or avoid, files off-limits, style or acceptance criteria). Quote constraints verbatim when wording matters.
PROGRESS: What has been completed, in order, with the concrete artifact for each step (file path, symbol name, command, value). Mark anything still in flight as "in progress".
FILES: Paths read, created, modified, or deleted, each with a one-phrase note on the change.
KEY FINDINGS: Facts learned from tool output that future steps depend on — symbol locations, API shapes, schema fields, exact error messages, exit codes, configuration values. Preserve identifiers, numbers, paths, and error strings verbatim.
DEAD ENDS: Approaches tried and abandoned, with the reason, so they are not retried.
NEXT: The single most immediate action the incoming agent should take, specific enough to act on without further inference. Then any other pending work.

Rules:
- Never invent facts. If a detail is not in the source, omit it rather than guess.
- Preserve file paths, symbol names, command strings, and error messages byte-for-byte.
- If a shared plan was recorded via the plan tool, reference it rather than restating it — the incoming agent reads the same plan state, and two descriptions of the work can disagree.
- Lines prefixed with "(thinking)" are the outgoing agent's internal reasoning; use them to infer intent but do not treat them as ground truth.
- Lines under a "TOOL_RESULT:" role are tool output, not user speech; attribute them accordingly.
- Drop pleasantries, restated context, and prose around tool output; keep the load-bearing details inside it.
- Target under ~600 words. If you must cut, cut from PROGRESS before GOAL or NEXT.`

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
		if err := r.handoff(pick); err != nil {
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
// handleTurn after this returns. The pick is threaded through purely so
// the on-terminal log can show the classifier's reason for the switch.
func (r *RouterAI) handoff(pick RouteSelection) error {
	from := r.current
	toRoute := pick.Route

	prev, ok := r.routes[from]
	if !ok {
		return fmt.Errorf("router %q: outgoing route %q is not registered", r.name, from)
	}

	next, ok := r.routes[toRoute]
	if !ok {
		return fmt.Errorf("router %q: incoming route %q is not registered", r.name, toRoute)
	}

	summary := ""
	if len(r.history) > 0 {
		s, err := prev.SummarizeForHandoff(r.history)
		if err != nil {
			return fmt.Errorf("handoff %s->%s: summarization failed: %w", from, toRoute, err)
		}

		summary = s
	}

	logHandoff(from, toRoute, pick)

	r.audit.LogEvent(audit.Event{
		Type: audit.HandoffEvent,
		Handoff: handoffAudit{
			From:         from,
			To:           toRoute,
			Summary:      summary,
			SystemPrompt: next.SystemPrompt(),
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

// logHandoff prints a one-line, human-readable record of a route switch.
// The full structured record (including the generated summary) still
// lands in the audit log via HandoffEvent; this is purely for the user
// running the CLI to see what just happened and why.
func logHandoff(from, to string, pick RouteSelection) {
	reason := strings.TrimSpace(pick.Reason)
	if reason == "" {
		color.New(color.FgYellow, color.Bold).Printf("Handoff: %s -> %s (confidence %.2f)\n", from, to, pick.Confidence)

		return
	}

	color.New(color.FgYellow, color.Bold).Printf("Handoff: %s -> %s (confidence %.2f) -- %s\n",
		from, to, pick.Confidence, reason)
}
