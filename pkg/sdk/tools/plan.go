package tools

import (
	"fmt"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

type PlanTool struct{}

func (t *PlanTool) Name() string {
	return "plan"
}

func (t *PlanTool) Description() string {
	return "A planning tool to create and manage a global plan. Use this to establish a shared plan across agents. The plan state is injected into system prompts via {{ .Plan }}."
}

func (t *PlanTool) Init(config map[string]string, configFactory *config.ConfigFactory) {
	// No initialization needed
}

func (t *PlanTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "The action to perform: 'create', 'add_step', 'update_step', 'remove_step', or 'list'.",
				"enum":        []string{"create", "add_step", "update_step", "remove_step", "list"},
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "The title of the plan. Required for 'create'.",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "The description of the plan or step. Required for 'create' and 'add_step', optional for 'update_step'.",
			},
			"id": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the step. Required for 'update_step' and 'remove_step'.",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "The status of the step: 'pending', 'in_progress', 'completed', 'blocked'.",
				"enum":        []string{"pending", "in_progress", "completed", "blocked"},
			},
		},
		"required": []string{"action"},
	}
}

func (t *PlanTool) Call(input map[string]interface{}) (interface{}, error) {
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

		sdk.GlobalPlan.Create(title, description)

		return fmt.Sprintf("Created plan: %s", title), nil

	case "add_step":
		description, _ := input["description"].(string)
		if description == "" {
			return nil, fmt.Errorf("description is required for 'add_step' action")
		}
		status, _ := input["status"].(string)

		id := sdk.GlobalPlan.AddStep(description, status)

		return fmt.Sprintf("Added step %s: %s", id, description), nil

	case "update_step":
		id, _ := input["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("id is required for 'update_step' action")
		}
		description, _ := input["description"].(string)
		status, _ := input["status"].(string)

		err := sdk.GlobalPlan.UpdateStep(id, description, status)
		if err != nil {
			return nil, err
		}

		return fmt.Sprintf("Updated step %s", id), nil

	case "remove_step":
		id, _ := input["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("id is required for 'remove_step' action")
		}

		err := sdk.GlobalPlan.RemoveStep(id)
		if err != nil {
			return nil, err
		}

		return fmt.Sprintf("Removed step %s", id), nil

	case "list":
		return sdk.GlobalPlan.Format(), nil

	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}
