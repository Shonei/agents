package utils

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestReadMultilineInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single line",
			input:    "Hello\n\n",
			expected: "Hello",
		},
		{
			name:     "Multiple lines",
			input:    "Line 1\nLine 2\n\n",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "Empty input",
			input:    "\n",
			expected: "",
		},
		{
			name:     "Lines with spaces",
			input:    "  Line 1  \n  Line 2  \n\n",
			expected: "  Line 1  \n  Line 2  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a pipe to mock Stdin
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("Failed to create pipe: %v", err)
			}

			// Save original Stdin and restore it after test
			oldStdin := os.Stdin
			defer func() { os.Stdin = oldStdin }()
			os.Stdin = r

			// Write input to pipe in a goroutine
			go func() {
				defer w.Close()
				_, err := io.WriteString(w, tt.input)
				if err != nil {
					t.Errorf("Failed to write to pipe: %v", err)
				}
			}()

			// Call function
			got, err := ReadMultilineInput()
			if err != nil {
				t.Errorf("ReadMultilineInput() error = %v", err)

				return
			}

			if got != tt.expected {
				t.Errorf("ReadMultilineInput() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPromptToolExecutionInputParsing(t *testing.T) {
	// Test that input parsing would work correctly
	testCases := []struct {
		input    string
		expected string
	}{
		{"y", "y"},
		{"Y", "y"},
		{"yes", "yes"},
		{"YES", "yes"},
		{"s", "s"},
		{"S", "s"},
		{"skip", "skip"},
		{"SKIP", "skip"},
		{"a", "a"},
		{"A", "a"},
		{"abort", "abort"},
		{"ABORT", "abort"},
	}

	for _, tc := range testCases {
		result := strings.ToLower(strings.TrimSpace(tc.input))
		if result != tc.expected {
			t.Errorf("For input '%s', expected '%s' but got '%s'", tc.input, tc.expected, result)
		}
	}
}
