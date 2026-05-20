package sdk

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type PlanStep struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type PlanState struct {
	mu          sync.RWMutex
	Title       string
	Description string
	Steps       map[string]*PlanStep
	nextID      int
	isCreated   bool
}

func NewPlanState() *PlanState {
	return &PlanState{
		Steps:  make(map[string]*PlanStep),
		nextID: 1,
	}
}

func (p *PlanState) Create(title, description string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Title = title
	p.Description = description
	p.Steps = make(map[string]*PlanStep)
	p.nextID = 1
	p.isCreated = true
}

func (p *PlanState) AddStep(description, status string) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isCreated {
		p.Title = "Default Plan"
		p.Description = "Auto-generated plan"
		p.isCreated = true
	}

	if status == "" {
		status = "pending"
	}

	id := strconv.Itoa(p.nextID)
	p.nextID++

	p.Steps[id] = &PlanStep{
		ID:          id,
		Description: description,
		Status:      status,
	}

	return id
}

func (p *PlanState) UpdateStep(id, description, status string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	step, exists := p.Steps[id]
	if !exists {
		return fmt.Errorf("step with id %s not found", id)
	}

	if description != "" {
		step.Description = description
	}
	if status != "" {
		step.Status = status
	}

	return nil
}

func (p *PlanState) RemoveStep(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.Steps[id]; !exists {
		return fmt.Errorf("step with id %s not found", id)
	}

	delete(p.Steps, id)

	return nil
}

func (p *PlanState) Format() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.isCreated {
		return "No active plan."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Plan: %s\n", p.Title)
	if p.Description != "" {
		fmt.Fprintf(&sb, "Description: %s\n", p.Description)
	}

	if len(p.Steps) == 0 {
		sb.WriteString("Steps: None\n")

		return sb.String()
	}

	sb.WriteString("Steps:\n")

	var ids []int
	for idStr := range p.Steps {
		id, _ := strconv.Atoi(idStr)
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		idStr := strconv.Itoa(id)
		step := p.Steps[idStr]
		fmt.Fprintf(&sb, "[%s] %s: %s\n", step.Status, step.ID, step.Description)
	}

	return sb.String()
}

type PlanSnapshot struct {
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Steps       []*PlanStep `json:"steps"`
}

func (p *PlanState) Snapshot() *PlanSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.isCreated {
		return nil
	}

	var ids []int
	for idStr := range p.Steps {
		id, _ := strconv.Atoi(idStr)
		ids = append(ids, id)
	}
	sort.Ints(ids)

	var steps []*PlanStep
	for _, id := range ids {
		idStr := strconv.Itoa(id)
		step := p.Steps[idStr]
		steps = append(steps, &PlanStep{
			ID:          step.ID,
			Description: step.Description,
			Status:      step.Status,
		})
	}

	return &PlanSnapshot{
		Title:       p.Title,
		Description: p.Description,
		Steps:       steps,
	}
}
