package sdk

import "strings"

type AIError struct {
	Message string
	Reason  error
}

func NewAIError(message string) *AIError {
	return &AIError{
		Message: message,
	}
}

func (a *AIError) WithReason(reason error) *AIError {
	a.Reason = reason

	return a
}

func (a *AIError) Error() string {
	return a.Message
}

func (a *AIError) AIResponse() string {
	var s strings.Builder

	s.WriteString(`{"type": "tool_error", "result": "`)
	s.WriteString(a.Message)
	s.WriteString(`"}`)

	return s.String()
}
