package claude

import "github.com/Shonei/agents/pkg/sdk"

// Messages API Request Types

// CreateMessageRequest represents the request body for creating a message
// This is the internal Claude wire type; it deliberately mirrors sdk.CreateMessageRequest
// so we can use simple type conversions in the agent while keeping the types distinct.
type CreateMessageRequest struct {
	Model         string          `json:"model"`
	Messages      []InputMessage  `json:"messages"`
	MaxTokens     int             `json:"max_tokens"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	System        SystemPrompt    `json:"system,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	Thinking      *ThinkingConfig `json:"thinking,omitempty"`
	ToolChoice    *ToolChoice     `json:"tool_choice,omitempty"`
	Tools         []Tool          `json:"tools,omitempty"`
	TopK          *int            `json:"top_k,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
}

// InputMessage represents a message in the conversation
type InputMessage struct {
	Role    string         `json:"role"` // "user" or "assistant"
	Content MessageContent `json:"content"`
}

// SystemPrompt can be either a string or an array of text blocks
type SystemPrompt interface{}

// MessageContent can be either a string or an array of content blocks
type MessageContent interface{}

// CacheControl for prompt caching
type CacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// ImageSource represents an image source
type ImageSource struct {
	Type      string `json:"type"` // "base64" or "url"
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// Tool represents a tool definition
type Tool struct {
	Type        string                 `json:"type,omitempty"` // "custom" or null
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolChoice represents how the model should use tools
type ToolChoice struct {
	Type                   string `json:"type"` // "auto", "any", "tool", or "none"
	Name                   string `json:"name,omitempty"`
	DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
}

// ThinkingConfig for extended thinking
type ThinkingConfig struct {
	Type         string `json:"type"` // "enabled" or "disabled"
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// Metadata for request metadata
type Metadata struct {
	UserID string `json:"user_id,omitempty"`
}

// ContainerParams for container configuration
type ContainerParams struct {
	ID     string        `json:"id,omitempty"`
	Skills []SkillParams `json:"skills,omitempty"`
}

// SkillParams for skill specification
type SkillParams struct {
	SkillID string `json:"skill_id"`
	Type    string `json:"type"` // "anthropic" or "custom"
	Version string `json:"version,omitempty"`
}

// ContextManagementConfig for context management
type ContextManagementConfig struct {
	Strategy *ContextManagementStrategy `json:"strategy,omitempty"`
}

// ContextManagementStrategy defines context management strategy
type ContextManagementStrategy struct {
	Type    string                    `json:"type"` // "auto" or "manual"
	Trigger *ContextManagementTrigger `json:"trigger,omitempty"`
	Keep    *ContextManagementKeep    `json:"keep,omitempty"`
}

// ContextManagementTrigger defines when to trigger context management
type ContextManagementTrigger struct {
	Type  string `json:"type"` // "tool_uses" or "thinking_turns"
	Value int    `json:"value"`
}

// ContextManagementKeep defines what to keep during context management
type ContextManagementKeep struct {
	Type  string `json:"type"` // "tool_uses"
	Value int    `json:"value"`
}

// RequestMCPServerURL for MCP server definition
type RequestMCPServerURL struct {
	URL string `json:"url"`
}

// Messages API Response Types

// MessageResponse represents the response from creating a message.
// This is the internal Claude wire type; it mirrors sdk.MessageResponse so
// we can use simple type conversions in the agent while keeping the types distinct.
type MessageResponse struct {
	ID           string                     `json:"id"`
	Type         string                     `json:"type"` // "message"
	Role         string                     `json:"role"` // "assistant"
	Content      []sdk.ResponseContentBlock `json:"content"`
	Model        string                     `json:"model"`
	StopReason   *string                    `json:"stop_reason"`
	StopSequence *string                    `json:"stop_sequence"`
	Usage        sdk.Usage                  `json:"usage"`
}

// ResponseCitation represents a citation in the response
type ResponseCitation struct {
	Type       string `json:"type"`
	CitedText  string `json:"cited_text,omitempty"`
	StartIndex int    `json:"start_index,omitempty"`
	EndIndex   int    `json:"end_index,omitempty"`
	// Web search citation fields
	EncryptedIndex string  `json:"encrypted_index,omitempty"`
	Title          *string `json:"title,omitempty"`
	URL            string  `json:"url,omitempty"`
}

// CacheCreation represents cache creation breakdown
type CacheCreation struct {
	Tokens map[string]int `json:"tokens,omitempty"`
}

// ServerToolUsage represents server tool usage
type ServerToolUsage struct {
	WebSearchRequests int `json:"web_search_requests"`
	WebFetchRequests  int `json:"web_fetch_requests"`
}

// ResponseContextManagement represents context management response
type ResponseContextManagement struct {
	Applied *ContextManagementApplied `json:"applied,omitempty"`
}

// ContextManagementApplied represents applied context management
type ContextManagementApplied struct {
	Type string `json:"type"`
}

// Container represents container information
type Container struct {
	ID     string  `json:"id"`
	Skills []Skill `json:"skills,omitempty"`
}

// Skill represents a loaded skill
type Skill struct {
	SkillID string `json:"skill_id"`
	Type    string `json:"type"` // "anthropic" or "custom"
	Version string `json:"version"`
}

// Error Response Types

// ErrorResponse represents an error response
type ErrorResponse struct {
	Type      string `json:"type"` // "error"
	Error     Error  `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

// Error represents the error details
type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Model constants
const (
	ModelClaude45 = "claude-sonnet-4-5-20250929"
)

// NewTextMessage creates a simple text message
func NewTextMessage(role, text string) InputMessage {
	return InputMessage{
		Role:    role,
		Content: text,
	}
}
