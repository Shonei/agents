package sdk

import "encoding/json"

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

// AIResponse returns a JSON object the model can parse as a tool error.
// Message is encoded as a JSON string so quotes/newlines cannot break the payload.
func (a *AIError) AIResponse() string {
	payload, err := json.Marshal(map[string]string{
		"type":   "tool_error",
		"result": a.Message,
	})
	if err != nil {
		// json.Marshal only fails on unsupported types; both fields are strings.
		return `{"type":"tool_error","result":"failed to encode tool error"}`
	}

	return string(payload)
}
