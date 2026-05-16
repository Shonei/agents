package sdk

// Agent defines the interface for an LLM agent
type Agent interface {
	// Model returns the model name configured for the agent
	Model() string

	// MaxTokens returns the max tokens configured for the agent
	MaxTokens() int

	// MaxContextTokens returns the input-context token budget that triggers
	// conversation compaction. A value of 0 disables compaction.
	MaxContextTokens() int

	// CreateMessage sends a message request to the LLM API
	CreateMessage(request CreateMessageRequest) (*MessageResponse, error)
}
