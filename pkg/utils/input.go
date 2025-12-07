package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

// ReadMultilineInput reads input from stdin until an empty line is entered.
// It returns the combined input joined by newlines.
func ReadMultilineInput() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}

// ReadUserInput reads a single line of input from the user with a prompt
func ReadUserInput() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := scanner.Text()

	return input, scanner.Err()
}

// ToolExecutionChoice represents the user's choice for tool execution
type ToolExecutionChoice int

const (
	ToolExecutionUnknown ToolExecutionChoice = iota
	ToolExecutionYes
	ToolExecutionSkip
	ToolExecutionAbort
)

func GatherUserContent() (string, error) {
	var userInput string

	fmt.Print("\n> ")
	for {
		nextMessage, err := ReadUserInput()
		if err != nil {
			return "", fmt.Errorf("error reading input: %w", err)
		}

		if nextMessage == "" {
			break
		}

		userInput += nextMessage + "\n"
		fmt.Print("> ")
	}

	return userInput, nil
}

// AskUserConfirmation prompts the user for confirmation to execute a tool
func AskUserConfirmation() (ToolExecutionChoice, error) {
	color.New(color.FgYellow, color.Bold).Print("Execute this tool? ")
	color.New(color.FgYellow, color.Bold).Print("[Y]es / [S]kip / [A]bort: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := strings.ToLower(strings.TrimSpace(scanner.Text()))

	if err := scanner.Err(); err != nil {
		return ToolExecutionAbort, err
	}

	answer := ToolExecutionUnknown

	switch input {
	case "y", "yes":
		answer = ToolExecutionYes
	case "s", "skip":
		answer = ToolExecutionSkip
	case "a", "abort":
		answer = ToolExecutionAbort
	}

	// I just want a new line after this
	fmt.Println()

	return answer, nil
}
