package sdk

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Shonei/agents/pkg/sdk/audit"
)

// fakeAgent is a minimal Agent that returns scripted responses in order.
// Each call to CreateMessage records the request and pops the next
// scripted response. Useful for asserting what each layer (classifier,
// sub-agent, summarizer) was asked to do.
type fakeAgent struct {
	name      string
	responses []*MessageResponse
	requests  []CreateMessageRequest
}

func (f *fakeAgent) Model() string         { return f.name }
func (f *fakeAgent) MaxTokens() int        { return 1024 }
func (f *fakeAgent) MaxContextTokens() int { return 0 }

func (f *fakeAgent) CreateMessage(req CreateMessageRequest) (*MessageResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.responses) == 0 {
		return &MessageResponse{Role: RoleAssistant}, nil
	}

	resp := f.responses[0]
	f.responses = f.responses[1:]

	return resp, nil
}

func newScriptedAI(name string, scripted ...*MessageResponse) (*AI, *fakeAgent) {
	fa := &fakeAgent{name: name, responses: scripted}
	ai := NewAI(fa, audit.NewAudit(audit.NewNoopLogger()))

	return ai, fa
}

func textResponse(text string) *MessageResponse {
	return &MessageResponse{
		Role:    RoleAssistant,
		Content: []ResponseContentBlock{{Type: ContentTypeText, Text: text}},
	}
}

func routeResponse(route string, confidence float64, reason string) *MessageResponse {
	return &MessageResponse{
		Role: RoleAssistant,
		Content: []ResponseContentBlock{{
			Type: ContentTypeToolUse,
			Name: classifierToolName,
			Input: map[string]interface{}{
				"route":      route,
				"confidence": confidence,
				"reason":     reason,
			},
		}},
	}
}

func TestRouterStaysOnLowConfidence(t *testing.T) {
	plannerAI, plannerAgent := newScriptedAI("planner", textResponse("planner reply"))
	builderAI, builderAgent := newScriptedAI("builder")

	classifierAgent := &fakeAgent{
		name:      "classifier",
		responses: []*MessageResponse{routeResponse("builder", 0.4, "weak signal")},
	}

	router := NewRouterAI(
		"dev",
		map[string]*AI{"planner": plannerAI, "builder": builderAI},
		[]RouteMeta{{Agent: "planner", When: "exploring"}, {Agent: "builder", When: "implementing"}},
		classifierAgent,
		"planner",
		0.7,
		audit.NewAudit(audit.NewNoopLogger()),
	)

	err := router.handleTurn("looks good, let's build it")
	assert.NoError(t, err)

	assert.Equal(t, "planner", router.current, "low confidence should not switch routes")
	assert.Len(t, plannerAgent.requests, 1, "active route handles the turn")
	assert.Len(t, builderAgent.requests, 0, "non-active route is untouched")
	assert.Len(t, classifierAgent.requests, 1)
}

func TestRouterSwitchesOnConfidentPick(t *testing.T) {
	plannerAI, plannerAgent := newScriptedAI("planner",
		textResponse("acknowledged"),        // initial planner turn
		textResponse("summary of planning"), // outgoing-agent summarization on handoff
	)
	builderAI, builderAgent := newScriptedAI("builder", textResponse("building..."))

	classifierAgent := &fakeAgent{
		name: "classifier",
		responses: []*MessageResponse{
			routeResponse("planner", 0.95, "still scoping"),
			routeResponse("builder", 0.9, "user approved plan"),
		},
	}

	router := NewRouterAI(
		"dev",
		map[string]*AI{"planner": plannerAI, "builder": builderAI},
		[]RouteMeta{{Agent: "planner", When: "exploring"}, {Agent: "builder", When: "implementing"}},
		classifierAgent,
		"planner",
		0.7,
		audit.NewAudit(audit.NewNoopLogger()),
	)

	assert.NoError(t, router.handleTurn("how should we structure this?"))
	assert.Equal(t, "planner", router.current)
	assert.Len(t, plannerAgent.requests, 1)

	assert.NoError(t, router.handleTurn("looks good, let's build it"))
	assert.Equal(t, "builder", router.current)

	// planner saw: turn 1 user message AND the summarization request on handoff.
	assert.Len(t, plannerAgent.requests, 2, "planner is asked to summarize during handoff")

	// builder sees a clean view starting with the handoff summary.
	assert.Len(t, builderAgent.requests, 1)
	builderReq := builderAgent.requests[0]
	assert.Equal(t, 2, len(builderReq.Messages), "builder sees [handoff_summary, user_msg]")

	first, ok := builderReq.Messages[0].Content.(string)
	assert.True(t, ok, "handoff message should be plain text")
	assert.True(t, strings.HasPrefix(first, "[Handoff from planner]"),
		"expected handoff prefix, got %q", first)
	assert.Contains(t, first, "summary of planning")

	last, ok := builderReq.Messages[1].Content.(string)
	assert.True(t, ok)
	assert.Equal(t, "looks good, let's build it", last)
}

func TestRouterHandoffWithEmptyHistorySkipsSummary(t *testing.T) {
	plannerAI, _ := newScriptedAI("planner")
	builderAI, builderAgent := newScriptedAI("builder", textResponse("hi from builder"))

	classifierAgent := &fakeAgent{
		name:      "classifier",
		responses: []*MessageResponse{routeResponse("builder", 0.99, "user wants builder")},
	}

	router := NewRouterAI(
		"dev",
		map[string]*AI{"planner": plannerAI, "builder": builderAI},
		[]RouteMeta{{Agent: "planner", When: "exploring"}, {Agent: "builder", When: "implementing"}},
		classifierAgent,
		"planner",
		0.7,
		audit.NewAudit(audit.NewNoopLogger()),
	)

	assert.NoError(t, router.handleTurn("just go build it"))
	assert.Equal(t, "builder", router.current)

	// builder receives only the user message — no synthetic handoff prefix.
	assert.Len(t, builderAgent.requests, 1)
	req := builderAgent.requests[0]
	assert.Len(t, req.Messages, 1, "with empty prior history nothing to summarize")
	msg, _ := req.Messages[0].Content.(string)
	assert.Equal(t, "just go build it", msg)
}

func TestRouterClassifierErrorStaysOnCurrent(t *testing.T) {
	plannerAI, plannerAgent := newScriptedAI("planner", textResponse("ok"))
	builderAI, _ := newScriptedAI("builder")

	// Classifier produces no responses → CreateMessage returns an empty
	// response, the classifier's SelectRoute fails, the router falls back
	// to current route.
	classifierAgent := &fakeAgent{name: "classifier"}

	router := NewRouterAI(
		"dev",
		map[string]*AI{"planner": plannerAI, "builder": builderAI},
		[]RouteMeta{{Agent: "planner", When: "exploring"}, {Agent: "builder", When: "implementing"}},
		classifierAgent,
		"planner",
		0.7,
		audit.NewAudit(audit.NewNoopLogger()),
	)

	assert.NoError(t, router.handleTurn("hi"))
	assert.Equal(t, "planner", router.current)
	assert.Len(t, plannerAgent.requests, 1)
}

func TestSynthesizeRouterPromptIsDeterministic(t *testing.T) {
	plannerAI, _ := newScriptedAI("planner")
	plannerAI.SetSystemPromptSilent("plan stuff")

	builderAI, _ := newScriptedAI("builder")
	builderAI.SetSystemPromptSilent("build stuff")

	meta := []RouteMeta{
		{Agent: "planner", When: "exploring"},
		{Agent: "builder", When: "implementing"},
	}

	out1 := SynthesizeRouterPrompt("dev", meta, map[string]*AI{"planner": plannerAI, "builder": builderAI}, "gemini-3.1-pro-preview")
	out2 := SynthesizeRouterPrompt("dev", meta, map[string]*AI{"builder": builderAI, "planner": plannerAI}, "gemini-3.1-pro-preview")

	assert.Equal(t, out1, out2, "ordering of routes map should not affect hash")
	assert.Contains(t, out1, "router: dev")
	assert.Contains(t, out1, "classifier_model: gemini-3.1-pro-preview")
}
