package utils

import (
	"bufio"
	"os"
	"strings"
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
