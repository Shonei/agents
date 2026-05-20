package sdk

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Task struct {
	ID          string
	Description string
	Status      string
}

type TodoState struct {
	mu     sync.RWMutex
	tasks  map[string]*Task
	nextID int
}

var GlobalTodo = &TodoState{
	tasks:  make(map[string]*Task),
	nextID: 1,
}

func (t *TodoState) Add(description, status string) (*Task, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if description == "" {
		return nil, fmt.Errorf("task description is required")
	}

	id := strconv.Itoa(t.nextID)
	t.nextID++

	if status == "" {
		status = "pending"
	}

	task := &Task{
		ID:          id,
		Description: description,
		Status:      status,
	}
	t.tasks[id] = task

	return task, nil
}

func (t *TodoState) Update(id, description, status string) (*Task, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	task, exists := t.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task with id %s not found", id)
	}

	if description != "" {
		task.Description = description
	}
	if status != "" {
		task.Status = status
	}

	return task, nil
}

func (t *TodoState) Remove(id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if id == "" {
		return fmt.Errorf("id is required")
	}

	if _, exists := t.tasks[id]; !exists {
		return fmt.Errorf("task with id %s not found", id)
	}

	delete(t.tasks, id)

	return nil
}

func (t *TodoState) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.tasks = make(map[string]*Task)
	t.nextID = 1
}

func (t *TodoState) List() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.tasks) == 0 {
		return "Todo list is empty."
	}

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

	return sb.String()
}

func (t *TodoState) GetTasks() []*Task {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var ids []int
	for idStr := range t.tasks {
		id, _ := strconv.Atoi(idStr)
		ids = append(ids, id)
	}
	sort.Ints(ids)

	var tasks []*Task
	for _, id := range ids {
		idStr := strconv.Itoa(id)
		tasks = append(tasks, t.tasks[idStr])
	}

	return tasks
}
