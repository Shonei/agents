package utils

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type ExitError struct {
	ExitCode int
	Message  string
	Reason   error
}

func NewExitError() *ExitError {
	return &ExitError{
		ExitCode: 1,
	}
}

// WithCode sets the exit code for the error
func (e *ExitError) WithCode(code int) *ExitError {
	e.ExitCode = code

	return e
}

// WithMessage sets the message for the error
func (e *ExitError) WithMessage(message string) *ExitError {
	e.Message = message

	return e
}

// WithReason sets the reason for the error
func (e *ExitError) WithReason(reason error) *ExitError {
	e.Reason = reason

	return e
}

// Done is a convenience method to exit the application with the error
func (e *ExitError) Done() {
	CheckError(e)
}

func (e *ExitError) Error() string {
	return e.Message
}

func (e *ExitError) Code() int {
	return e.ExitCode
}

// CheckError handles the error nad calls os.Exit
// If the error is not an ExitError, it is printed to stderr and the application exits with code 1
// If the error is nil, nothing happens
func CheckError(err error) {
	if err == nil {
		return
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		if exitErr.Code() == 0 {
			os.Exit(0)
		}

		messageBuilder := strings.Builder{}

		if exitErr.Message != "" {
			messageBuilder.WriteString("Message: \n  " + strings.ReplaceAll(exitErr.Message, "\n", "\n  "))
			if !strings.HasSuffix(messageBuilder.String(), "\n") {
				messageBuilder.WriteString("\n")
			}
		}

		if exitErr.Reason != nil {
			// I want the message to look like this:
			// Message:
			//   something went wrong
			// Reason:
			//   something else went wrong
			if len(messageBuilder.String()) > 1 && !strings.HasSuffix(messageBuilder.String(), "\n") {
				messageBuilder.WriteString("\n")
			}

			messageBuilder.WriteString("Reason:\n  ")
			messageBuilder.WriteString(strings.ReplaceAll(exitErr.Reason.Error(), "\n", "\n  "))

			if !strings.HasSuffix(messageBuilder.String(), "\n") {
				messageBuilder.WriteString("\n")
			}
		}

		fmt.Fprint(os.Stderr, messageBuilder.String())
		os.Exit(exitErr.Code())
	}

	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
