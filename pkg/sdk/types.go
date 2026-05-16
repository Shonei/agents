package sdk

// CreateMessageRequest represents the request body for creating a message.
type CreateMessageRequest struct {
	Messages   []InputMessage `json:"messages"`
	System     string         `json:"system,omitempty"`
	ToolChoice *ToolChoice    `json:"tool_choice,omitempty"`
	Tools      []Tool         `json:"tools,omitempty"`
}

// InputMessage represents a message in the conversation.
// Content is either a string (plain text) or a []ContentBlock for multi-part content.
type InputMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// ContentBlock represents a single content block in an InputMessage.
type ContentBlock struct {
	Type string `json:"type"`

	Text   string `json:"text,omitempty"`
	Source *Blob  `json:"source,omitempty"`

	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`

	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"`
	IsError   bool        `json:"is_error,omitempty"`

	ThoughtSignature string `json:"thought_signature,omitempty"`
}

// Tool represents a tool definition.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolChoice represents how the model should use tools.
type ToolChoice struct {
	Type string `json:"type"` // "auto", "any", "tool", or "none"
	Name string `json:"name,omitempty"`
}

// MessageResponse represents the response from creating a message.
type MessageResponse struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Role    string                 `json:"role"`
	Content []ResponseContentBlock `json:"content"`
	Model   string                 `json:"model"`
	Usage   Usage                  `json:"usage"`
}

// ResponseContentBlock represents a single content block in a MessageResponse.
type ResponseContentBlock struct {
	Type string `json:"type"`

	Text string `json:"text,omitempty"`
	Blob *Blob  `json:"blob,omitempty"`

	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`

	ThoughtSignature string `json:"thought_signature,omitempty"`
}

// Blob holds raw binary data (e.g. an image) along with its MIME type.
type Blob struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

// Usage represents token usage information.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Content block type constants.
const (
	ContentTypeText       = "text"
	ContentTypeImage      = "image"
	ContentTypeToolUse    = "tool_use"
	ContentTypeToolResult = "tool_result"
	ContentTypeThinking   = "thinking"
)

// Role constants.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// NewTextMessage creates a simple text message.
func NewTextMessage(role, text string) InputMessage {
	return InputMessage{
		Role:    role,
		Content: text,
	}
}

// NewToolResultContentBlock creates a tool result content block.
func NewToolResultContentBlock(toolUseID string, content interface{}, isError bool) ContentBlock {
	return ContentBlock{
		Type:      ContentTypeToolResult,
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	}
}

// NewTool creates a new tool definition.
func NewTool(name, description string, inputSchema map[string]interface{}) Tool {
	return Tool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	}
}

// NewAutoToolChoice creates an auto tool choice.
func NewAutoToolChoice() *ToolChoice {
	return &ToolChoice{Type: "auto"}
}

// GetTextContent concatenates the text from all text content blocks in the response.
func (r *MessageResponse) GetTextContent() string {
	var result string
	for _, block := range r.Content {
		if block.Type == ContentTypeText {
			result += block.Text
		}
	}

	return result
}
