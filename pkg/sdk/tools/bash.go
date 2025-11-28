package tools

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/fatih/color"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

// BashTool is given an input command and executes it
type BashTool struct{}

func (b *BashTool) Name() string {
	return "bash"
}

func (b *BashTool) Description() string {
	return "Given an input command, executes it and return the output and the exit code wrapped in XML tags"
}

func (b *BashTool) Init(config map[string]string, _ *config.ConfigFactory) {
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

	userAsk(command)

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

func userAsk(command string) {
	// Ask end user for permission first
	color.New(color.FgYellow, color.Bold).Println("\nYou are about to execute the following command:")
	color.Cyan("  %s", command)
	fmt.Print(color.New(color.Bold).Sprint("Do you want to continue? (y/N): "))

	var answer string
	_, err := fmt.Scanln(&answer)
	if err != nil {
		utils.NewExitError().WithMessage("failed to read user input").WithReason(err).Done()
	}

	if answer != "y" {
		color.Red("user aborted")
		os.Exit(0)
	}

	color.Green("Executing...\n\n")
}
