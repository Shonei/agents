package tools

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Shonei/agents/pkg/config"
)

type TodoTool struct {
	mu     sync.Mutex
	tasks  map[string]*Task
	nextID int
}

type Task struct {
	ID          string
	Description string
	Status      string
}

func (t *TodoTool) Name() string {
	return "todo"
}

func (t *TodoTool) Description() string {
	return "A todo list to help manage tasks, plan, and track progress. Use this to break down complex problems and remember what needs to be done."
}

func (t *TodoTool) Init(config map[string]string, configFactory *config.ConfigFactory) {
	// No initialization needed
}

func (t *TodoTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "The action to perform: 'add', 'update', 'list', or 'remove'.",
				"enum":        []string{"add", "update", "list", "remove"},
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
	action, ok := input["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action is required and must be a string")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tasks == nil {
		t.tasks = make(map[string]*Task)
		t.nextID = 1
	}

	switch action {
	case "add":
		taskDesc, _ := input["task"].(string)
		if taskDesc == "" {
			return nil, fmt.Errorf("task description is required for 'add' action")
		}
		id := strconv.Itoa(t.nextID)
		t.nextID++
		status, _ := input["status"].(string)
		if status == "" {
			status = "pending"
		}
		t.tasks[id] = &Task{
			ID:          id,
			Description: taskDesc,
			Status:      status,
		}

		return fmt.Sprintf("Added task %s: %s (Status: %s)", id, taskDesc, status), nil

	case "update":
		id, _ := input["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("id is required for 'update' action")
		}
		task, exists := t.tasks[id]
		if !exists {
			return nil, fmt.Errorf("task with id %s not found", id)
		}
		if taskDesc, ok := input["task"].(string); ok && taskDesc != "" {
			task.Description = taskDesc
		}
		if status, ok := input["status"].(string); ok && status != "" {
			task.Status = status
		}

		return fmt.Sprintf("Updated task %s: %s (Status: %s)", task.ID, task.Description, task.Status), nil

	case "remove":
		id, _ := input["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("id is required for 'remove' action")
		}
		if _, exists := t.tasks[id]; !exists {
			return nil, fmt.Errorf("task with id %s not found", id)
		}
		delete(t.tasks, id)

		return fmt.Sprintf("Removed task %s", id), nil

	case "list":
		if len(t.tasks) == 0 {
			return "Todo list is empty.", nil
		}

		// Sort by ID for consistent output
		var ids []int
		for idStr := range t.tasks {
			id, _ := strconv.Atoi(idStr)
			ids = append(ids, id)
		}
		sort.Ints(ids)

		var sb strings.Builder
		sb.WriteString("Todo List:\n")
		for _, id := range ids {
			idStr := strconv.Itoa(id)
			task := t.tasks[idStr]
			fmt.Fprintf(&sb, "[%s] %s: %s\n", task.Status, task.ID, task.Description)
		}

		return sb.String(), nil

	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}
