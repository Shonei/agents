package sdk

import "encoding/json"

// Messages API Request Types

// CreateMessageRequest represents the request body for creating a message
type CreateMessageRequest struct {
	Messages      []InputMessage `json:"messages"`
	StopSequences []string       `json:"stop_sequences,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	System        SystemPrompt   `json:"system,omitempty"`
	ToolChoice    *ToolChoice    `json:"tool_choice,omitempty"`
	Tools         []Tool         `json:"tools,omitempty"`
}

// InputMessage represents a message in the conversation
type InputMessage struct {
	Role    string         `json:"role"` // "user" or "assistant"
	Content MessageContent `json:"content"`
}

// MessageContent can be either a string or an array of content blocks
type MessageContent interface{}

// UnmarshalJSON handles both string and array content
func (m *InputMessage) UnmarshalJSON(data []byte) error {
	type Alias InputMessage
	aux := &struct {
		Content json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(aux.Content, &str); err == nil {
		m.Content = str

		return nil
	}

	// Otherwise unmarshal as array of content blocks
	var blocks []ContentBlock
	if err := json.Unmarshal(aux.Content, &blocks); err != nil {
		return err
	}
	m.Content = blocks

	return nil
}

// SystemPrompt can be either a string or an array of text blocks
type SystemPrompt interface{}

// ContentBlock represents different types of content blocks
type ContentBlock struct {
	Type string `json:"type"`
	// Text content
	Text string `json:"text,omitempty"`
	// Image content
	Source *Blob `json:"source,omitempty"`
	// Tool use content
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
	// Tool result content
	ToolUseID        string      `json:"tool_use_id,omitempty"`
	Content          interface{} `json:"content,omitempty"`
	IsError          bool        `json:"is_error,omitempty"`
	ThoughtSignature string      `json:"thought_signature,omitempty"`
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

// Messages API Response Types

// MessageResponse represents the response from creating a message
type MessageResponse struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"` // "message"
	Role         string                 `json:"role"` // "assistant"
	Content      []ResponseContentBlock `json:"content"`
	Model        string                 `json:"model"`
	StopReason   *string                `json:"stop_reason"`
	StopSequence *string                `json:"stop_sequence"`
	Usage        Usage                  `json:"usage"`
}

// ResponseContentBlock represents different types of response content blocks
type ResponseContentBlock struct {
	Type string `json:"type"`

	// Text block
	Text string `json:"text,omitempty"`

	Blob *Blob `json:"blob,omitempty"`

	// Tool use block
	ID               string                 `json:"id,omitempty"`
	Name             string                 `json:"name,omitempty"`
	Input            map[string]interface{} `json:"input,omitempty"`
	ThoughtSignature string                 `json:"thought_signature,omitempty"`
}

type Blob struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

// Usage represents token usage information
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Content block type constants
const (
	ContentTypeText       = "text"
	ContentTypeImage      = "image"
	ContentTypeToolUse    = "tool_use"
	ContentTypeToolResult = "tool_result"
	ContentTypeThinking   = "thinking"
)

// Role constants
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Helper functions for creating common structures

// NewTextMessage creates a simple text message
func NewTextMessage(role, text string) InputMessage {
	return InputMessage{
		Role:    role,
		Content: text,
	}
}

// NewTextContentBlock creates a text content block
func NewTextContentBlock(text string) ContentBlock {
	return ContentBlock{
		Type: ContentTypeText,
		Text: text,
	}
}

// NewToolUseContentBlock creates a tool use content block
func NewToolUseContentBlock(id, name string, input map[string]interface{}) ContentBlock {
	return ContentBlock{
		Type:  ContentTypeToolUse,
		ID:    id,
		Name:  name,
		Input: input,
	}
}

// NewToolResultContentBlock creates a tool result content block
func NewToolResultContentBlock(toolUseID string, content interface{}, isError bool) ContentBlock {
	return ContentBlock{
		Type:      ContentTypeToolResult,
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	}
}

// NewTool creates a new tool definition
func NewTool(name, description string, inputSchema map[string]interface{}) Tool {
	return Tool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	}
}

// NewAutoToolChoice creates an auto tool choice
func NewAutoToolChoice() *ToolChoice {
	return &ToolChoice{
		Type: "auto",
	}
}

// GetTextContent extracts text content from response content blocks
func (r *MessageResponse) GetTextContent() string {
	var result string
	for _, block := range r.Content {
		if block.Type == ContentTypeText {
			result += block.Text
		}
	}

	return result
}

// GetToolUses extracts all tool use blocks from response
func (r *MessageResponse) GetToolUses() []ResponseContentBlock {
	var toolUses []ResponseContentBlock
	for _, block := range r.Content {
		if block.Type == ContentTypeToolUse {
			toolUses = append(toolUses, block)
		}
	}

	return toolUses
}
