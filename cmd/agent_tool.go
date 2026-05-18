package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

type agentTool struct {
	agent           *sdk.AI
	name            string
	description     string
	parentSessionID string
}

func (t *agentTool) Name() string {
	return t.name
}

func (t *agentTool) Description() string {
	return t.description
}

func (t *agentTool) Init(config map[string]string, configFactory *config.ConfigFactory) {
	// Initialization is handled during creation in buildAgent
}

func (t *agentTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "The prompt or task description to send to the sub-agent.",
			},
		},
		"required": []string{"prompt"},
	}
}

func (t *agentTool) Call(input map[string]interface{}) (interface{}, error) {
	prompt, ok := input["prompt"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'prompt' in input")
	}

	// Run the sub-agent once
	result, err := t.agent.RunOnce(prompt)
	if err != nil {
		return nil, fmt.Errorf("sub-agent error: %w", err)
	}

	// Try to parse as JSON in case it contains images
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err == nil {
		return parsed, nil
	}

	return result, nil
}

func (t *agentTool) SetAudit(parentSessionID string) {
	t.parentSessionID = parentSessionID
	// We don't set the audit on the agent here because it's already set during buildAgent
}
