package tools

import (
	"bufio"
	"fmt"
	"os"

	"github.com/fatih/color"
)

// AskUserTool allows the agent to ask the user a question
type AskUserTool struct{}

func (a *AskUserTool) Name() string {
	return "ask_user"
}

func (a *AskUserTool) Description() string {
	return "Asks the user a question and waits for their response. Use this when you need clarification or more information."
}

func (a *AskUserTool) Init(config map[string]string) {
}

func (a *AskUserTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"question": map[string]interface{}{
				"type":        "string",
				"description": "The question to ask the user",
				"example":     "What is the name of the file you want me to edit?",
			},
		},
		"required": []interface{}{"question"},
	}
}

func (a *AskUserTool) Call(input map[string]interface{}) (interface{}, error) {
	question, ok := input["question"].(string)
	if !ok {
		return "", fmt.Errorf("question must be a string")
	}

	color.New(color.FgYellow, color.Bold).Printf("\nQuestion from Agent: %s\n", question)
	fmt.Print(color.New(color.Bold).Sprint("Your Answer: "))

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return scanner.Text(), nil
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading input: %w", err)
	}

	return "", fmt.Errorf("no input received")
}
