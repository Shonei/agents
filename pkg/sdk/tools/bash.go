package tools

import (
	"fmt"
	"os/exec"

	"github.com/fatih/color"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

// BashTool is given an input command and executes it
type BashTool struct {
	requireConfirmation bool
}

func (b *BashTool) Name() string {
	return "bash"
}

func (b *BashTool) Description() string {
	return "Given an input command, executes it and return the output and the exit code wrapped in XML tags. The user will be prompted to confirm the execution and he can choose to skip it."
}

func (b *BashTool) Init(config map[string]string, _ *config.ConfigFactory) {
	b.requireConfirmation = true
	if val, ok := config["require_confirmation"]; ok && val == "false" {
		b.requireConfirmation = false
	}
}

func (b *BashTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The bash command you want to execute.",
				"example":     "ls -la",
			},
		},
		"required": []interface{}{"command"},
	}
}

func (b *BashTool) Call(input map[string]interface{}) (interface{}, error) {
	command, ok := input["command"].(string)
	if !ok {
		return "", sdk.NewAIError("command must be a string")
	}

	if b.requireConfirmation {
		color.New(color.FgYellow, color.Bold).Println("\nYou are about to execute the following command:")
		color.Cyan("  %s", command)
		answer, _ := utils.AskUserConfirmation()
		switch answer {
		case utils.ToolExecutionYes:
			// continue
		case utils.ToolExecutionSkip:
			return "<exitcode>1</exitcode><output>Skipped by user</output>", nil
		case utils.ToolExecutionAbort:
			utils.NewExitError().WithMessage("tool execution aborted by user").Done()
		case utils.ToolExecutionUnknown:
			utils.NewExitError().WithMessage("unknown user choice").Done()
		}
	} else {
		color.New(color.FgYellow, color.Bold).Println("\nExecuting command (auto-confirmed):")
		color.Cyan("  %s", command)
	}

	// Execute the command
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// check if it's an exit code error and return a valid AI response
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Sprintf("<exitcode>%d</exitcode><output>%s</output>", exitErr.ExitCode(), string(output)), nil
		}

		return "", err
	}

	return fmt.Sprintf("<exitcode>0</exitcode><output>%s</output>", string(output)), nil
}
