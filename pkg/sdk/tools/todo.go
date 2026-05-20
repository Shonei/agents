package tools

import (
	"fmt"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/audit"
)

type TodoTool struct {
	state *sdk.TodoState
	audit *audit.Audit
}

func (t *TodoTool) Name() string {
	return "todo"
}

func (t *TodoTool) Description() string {
	return "A todo list to help manage tasks, plan, and track progress. Use this to break down complex problems and remember what needs to be done."
}

func (t *TodoTool) Init(config map[string]string, configFactory *config.ConfigFactory) {
	if t.state == nil {
		t.state = sdk.NewTodoState()
	}
}

func (t *TodoTool) SetAudit(audit *audit.Audit) {
	t.audit = audit
}

func (t *TodoTool) SetTodoState(state *sdk.TodoState) {
	t.state = state
}

func (t *TodoTool) TemplateKey() string {
	return "Todo"
}

func (t *TodoTool) TemplateData() any {
	return func() string {
		if t.state == nil {
			return ""
		}

		return t.state.List()
	}
}

func (t *TodoTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "The action to perform: 'add', 'update', 'list', 'remove', or 'reset'.",
				"enum":        []string{"add", "update", "list", "remove", "reset"},
			},
			"id": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the task. Required for 'update' and 'remove'.",
			},
			"task": map[string]interface{}{
				"type":        "string",
				"description": "The description of the task. Required for 'add', optional for 'update'.",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "The status of the task: 'pending', 'in_progress', 'completed', 'blocked'.",
				"enum":        []string{"pending", "in_progress", "completed", "blocked"},
			},
		},
		"required": []string{"action"},
	}
}

func (t *TodoTool) Call(input map[string]interface{}) (interface{}, error) {
	defer func() {
		if t.audit != nil && t.state != nil {
			t.audit.LogEvent(audit.Event{
				Type: audit.TodoEvent,
				Todo: t.state.GetTasks(),
			})
		}
	}()

	action, ok := input["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action is required and must be a string")
	}

	switch action {
	case "add":
		taskDesc, _ := input["task"].(string)
		status, _ := input["status"].(string)
		task, err := t.state.Add(taskDesc, status)
		if err != nil {
			return nil, err
		}

		return fmt.Sprintf("Added task %s: %s (Status: %s)", task.ID, task.Description, task.Status), nil

	case "update":
		id, _ := input["id"].(string)
		taskDesc, _ := input["task"].(string)
		status, _ := input["status"].(string)
		task, err := t.state.Update(id, taskDesc, status)
		if err != nil {
			return nil, err
		}

		return fmt.Sprintf("Updated task %s: %s (Status: %s)", task.ID, task.Description, task.Status), nil

	case "remove":
		id, _ := input["id"].(string)
		err := t.state.Remove(id)
		if err != nil {
			return nil, err
		}

		return fmt.Sprintf("Removed task %s", id), nil

	case "list":
		return t.state.List(), nil

	case "reset":
		t.state.Reset()

		return "Todo list has been reset.", nil

	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}
