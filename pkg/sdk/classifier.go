package sdk

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// classifierSystemPrompt is the hardcoded system instruction for the
// routing classifier. We intentionally do not surface this via YAML: the
// classifier's job is mechanical and we want consistent behavior across
// all router agents.
const classifierSystemPrompt = `You are a routing classifier for a multi-agent system. Your only job is to pick which sub-agent should handle the user's next turn.

You will be shown:
- The available routes, each with a short description of when they apply.
- The currently active route.
- The recent conversation history (user and assistant text only).
- The user's new message.

Decision rules:
- Prefer to keep the current route. Routing should be sticky; only switch when the user's intent has clearly changed.
- Output a confidence in [0, 1]. The orchestrator only switches when confidence exceeds a configured threshold.
- Always emit your decision via the select_route function. Never reply in plain text.
- "reason" must be a single short sentence justifying the pick, grounded in the user's new message.`

// classifierToolName is the function name the classifier is forced to
// call. Kept private; the router builds the Tool definition from it.
const classifierToolName = "select_route"

// classifierHistoryWindow caps how many recent user+assistant text turns
// are shown to the classifier. Keeps the per-turn classification call
// cheap and bounded.
const classifierHistoryWindow = 6

// RouteSelection is the structured output the classifier produces for
// each user turn. Confidence is in [0, 1].
type RouteSelection struct {
	Route      string  `json:"route"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// classifier wraps an LLM Agent configured for structured route picks.
// It is intentionally narrow: no tool execution, no compaction, no
// history of its own.
type classifier struct {
	agent  Agent
	routes []RouteMeta
}

func newClassifier(agent Agent, routes []RouteMeta) *classifier {
	return &classifier{agent: agent, routes: routes}
}

// SelectRoute asks the classifier which sub-agent should handle userMsg
// given the current working history and the current active route. The
// returned latency measures only the model call.
func (c *classifier) SelectRoute(history []InputMessage, currentRoute, userMsg string) (RouteSelection, time.Duration, error) {
	if len(c.routes) == 0 {
		return RouteSelection{}, 0, fmt.Errorf("classifier has no routes configured")
	}

	prompt := renderClassifierPrompt(c.routes, currentRoute, history, userMsg)
	tool := c.buildTool()

	req := CreateMessageRequest{
		System:     classifierSystemPrompt,
		Messages:   []InputMessage{NewTextMessage(RoleUser, prompt)},
		Tools:      []Tool{tool},
		ToolChoice: &ToolChoice{Type: "tool", Name: classifierToolName},
	}

	start := time.Now()

	resp, err := c.agent.CreateMessage(req)

	elapsed := time.Since(start)

	if err != nil {
		return RouteSelection{}, elapsed, fmt.Errorf("classifier call failed: %w", err)
	}

	for _, block := range resp.Content {
		if block.Type != ContentTypeToolUse || block.Name != classifierToolName {
			continue
		}

		raw, err := json.Marshal(block.Input)
		if err != nil {
			return RouteSelection{}, elapsed, fmt.Errorf("classifier returned unparseable args: %w", err)
		}

		var sel RouteSelection
		if err := json.Unmarshal(raw, &sel); err != nil {
			return RouteSelection{}, elapsed, fmt.Errorf("classifier returned malformed select_route payload: %w", err)
		}

		if !c.isKnownRoute(sel.Route) {
			return RouteSelection{}, elapsed, fmt.Errorf("classifier picked unknown route %q", sel.Route)
		}

		if sel.Confidence < 0 {
			sel.Confidence = 0
		}

		if sel.Confidence > 1 {
			sel.Confidence = 1
		}

		return sel, elapsed, nil
	}

	return RouteSelection{}, elapsed, fmt.Errorf("classifier did not call %s", classifierToolName)
}

func (c *classifier) buildTool() Tool {
	names := make([]string, 0, len(c.routes))
	for _, r := range c.routes {
		names = append(names, r.Agent)
	}

	return NewTool(
		classifierToolName,
		"Pick which sub-agent should handle the user's next turn.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"route": map[string]interface{}{
					"type":        "string",
					"enum":        names,
					"description": "Name of the sub-agent that should handle the turn.",
				},
				"confidence": map[string]interface{}{
					"type":        "number",
					"description": "How confident you are in this pick, in [0, 1].",
				},
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "One short sentence justifying the pick.",
				},
			},
			"required": []string{"route", "confidence", "reason"},
		},
	)
}

func (c *classifier) isKnownRoute(name string) bool {
	for _, r := range c.routes {
		if r.Agent == name {
			return true
		}
	}

	return false
}

// renderClassifierPrompt builds the user-facing prompt body that the
// classifier sees. The hardcoded system prompt explains the task; this
// payload provides the per-turn context.
func renderClassifierPrompt(routes []RouteMeta, currentRoute string, history []InputMessage, userMsg string) string {
	var b strings.Builder

	b.WriteString("<routes>\n")
	for _, r := range routes {
		fmt.Fprintf(&b, "- %s: %s\n", r.Agent, r.When)
	}
	b.WriteString("</routes>\n\n")

	fmt.Fprintf(&b, "<current_route>%s</current_route>\n\n", currentRoute)

	b.WriteString("<recent_history>\n")
	b.WriteString(renderRecentHistory(history, classifierHistoryWindow))
	b.WriteString("</recent_history>\n\n")

	b.WriteString("<new_user_message>\n")
	b.WriteString(userMsg)
	b.WriteString("\n</new_user_message>")

	return b.String()
}

// renderRecentHistory walks `history` backwards collecting plain
// user/assistant text turns (no tool blocks, no thinking, no images),
// keeps at most `maxTurns` of them and emits them in chronological
// order. Designed to stay cheap regardless of how tool-heavy the
// underlying conversation was.
func renderRecentHistory(history []InputMessage, maxTurns int) string {
	if maxTurns <= 0 || len(history) == 0 {
		return ""
	}

	type turn struct {
		role string
		text string
	}

	collected := make([]turn, 0, maxTurns)
	for i := len(history) - 1; i >= 0 && len(collected) < maxTurns; i-- {
		text := plainTextOf(history[i])
		if text == "" {
			continue
		}

		collected = append(collected, turn{role: history[i].Role, text: text})
	}

	if len(collected) == 0 {
		return ""
	}

	var b strings.Builder
	for i := len(collected) - 1; i >= 0; i-- {
		fmt.Fprintf(&b, "%s: %s\n", strings.ToUpper(collected[i].role), collected[i].text)
	}

	return b.String()
}

// plainTextOf extracts the text portion of a message, ignoring tool_use,
// tool_result, thinking, and image blocks. Returns "" for messages that
// have no plain text.
func plainTextOf(msg InputMessage) string {
	switch v := msg.Content.(type) {
	case string:
		return v
	case []ContentBlock:
		var b strings.Builder
		for _, block := range v {
			if block.Type == ContentTypeText && block.Text != "" {
				if b.Len() > 0 {
					b.WriteString(" ")
				}

				b.WriteString(block.Text)
			}
		}

		return b.String()
	default:
		return ""
	}
}
