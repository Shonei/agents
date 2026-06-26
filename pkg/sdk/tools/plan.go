package tools

import (
	"fmt"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/audit"
)

type PlanTool struct {
	state *sdk.PlanState
	audit *audit.Audit
}

func (t *PlanTool) Name() string {
	return "plan"
}

func (t *PlanTool) Description() string {
	return "A global plan management tool for agent coordination. Use 'create' to initialize a plan with a title and overall description. Use 'add_step' to append actionable tasks (default status 'pending'). Use 'update_step' to change step status ('pending', 'in_progress', 'completed', 'blocked') or refine task descriptions as work progresses. Use 'remove_step' to delete a step, and 'list' to view the formatted plan structure. The global plan state is injected into all collaborating agents' prompts via {{ .Plan }} to ensure seamless context tracking and progress alignment."
}

func (t *PlanTool) Init(config map[string]string, configFactory *config.ConfigFactory) {
	if t.state == nil {
		t.state = sdk.NewPlanState()
	}
}

func (t *PlanTool) SetAudit(audit *audit.Audit) {
	t.audit = audit
}

func (t *PlanTool) SetPlanState(state *sdk.PlanState) {
	t.state = state
}

func (t *PlanTool) TemplateKey() string {
	return "Plan"
}

func (t *PlanTool) TemplateData() any {
	if t.state == nil {
		return ""
	}

	return t.state.Format()
}

func (t *PlanTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "The planning action to execute. 'create': initialize a new global plan with title and description; 'add_step': add a new task to the plan; 'update_step': modify an existing task's description or update its status (e.g. to 'in_progress' or 'completed'); 'remove_step': delete a task by ID; 'list': return the fully formatted plan.",
				"enum":        []string{"create", "add_step", "update_step", "remove_step", "list"},
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "The overall title or objective of the plan. Required ONLY when action is 'create'.",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Required when action is 'create' (describing the overall project goal) or 'add_step' (describing the specific task step). Optional when action is 'update_step' to update a step's description.",
			},
			"id": map[string]interface{}{
				"type":        "string",
				"description": "The unique numerical identifier of the step (e.g., '1', '2'). Required ONLY for 'update_step' and 'remove_step' actions.",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "The execution status of the step. Allowed values: 'pending' (default when added), 'in_progress' (currently being worked on), 'completed' (fully implemented and verified), or 'blocked' (cannot proceed). Applicable when adding or updating steps.",
				"enum":        []string{"pending", "in_progress", "completed", "blocked"},
			},
		},
		"required": []string{"action"},
	}
}

func (t *PlanTool) Call(input map[string]interface{}) (interface{}, error) {
	defer func() {
		if t.audit != nil && t.state != nil {
			if snapshot := t.state.Snapshot(); snapshot != nil {
				t.audit.LogEvent(audit.Event{
					Type: audit.PlanEvent,
					Plan: snapshot,
				})
			}
		}
	}()

	action, ok := input["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action is required and must be a string")
	}

	switch action {
	case "create":
		title, _ := input["title"].(string)
		if title == "" {
			return nil, fmt.Errorf("title is required for 'create' action")
		}
		description, _ := input["description"].(string)

		t.state.Create(title, description)

		return fmt.Sprintf("Created plan: %s", title), nil

	case "add_step":
		description, _ := input["description"].(string)
		if description == "" {
			return nil, fmt.Errorf("description is required for 'add_step' action")
		}
		status, _ := input["status"].(string)

		id := t.state.AddStep(description, status)

		return fmt.Sprintf("Added step %s: %s", id, description), nil

	case "update_step":
		id, _ := input["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("id is required for 'update_step' action")
		}
		description, _ := input["description"].(string)
		status, _ := input["status"].(string)

		err := t.state.UpdateStep(id, description, status)
		if err != nil {
			return nil, err
		}

		return fmt.Sprintf("Updated step %s", id), nil

	case "remove_step":
		id, _ := input["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("id is required for 'remove_step' action")
		}

		err := t.state.RemoveStep(id)
		if err != nil {
			return nil, err
		}

		return fmt.Sprintf("Removed step %s", id), nil

	case "list":
		return t.state.Format(), nil

	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}
