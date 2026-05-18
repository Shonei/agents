package sdk

// Chatter is the minimum surface needed to drive a top-level chat loop
// from the CLI. Both *AI and *RouterAI implement it so engage.go can
// dispatch to either without branching at the call site.
type Chatter interface {
	Chat(initial string) (string, error)
}

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
