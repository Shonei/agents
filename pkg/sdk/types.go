package sdk

// CreateMessageRequest represents the request body for creating a message.
type CreateMessageRequest struct {
	Messages    []InputMessage `json:"messages"`
	System      string         `json:"system,omitempty"`
	ToolChoice  *ToolChoice    `json:"tool_choice,omitempty"`
	Tools       []Tool         `json:"tools,omitempty"`
	ServerTools []ServerTool   `json:"server_tools,omitempty"`
}

// ServerTool represents a tool that is executed by the provider rather than
// by the local SDK loop. The Name field carries a provider-recognised kind
// (see the ServerTool* constants below).
type ServerTool struct {
	Name string `json:"name"`
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

	Text     string `json:"text,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	Source   *Blob  `json:"source,omitempty"`

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

	// Grounding is populated for ContentTypeGrounding blocks. It carries the
	// provider-reported side-channel information about server-side tool
	// executions (e.g. Gemini google_search / url_context).
	Grounding *GroundingMetadata `json:"grounding,omitempty"`
}

// GroundingMetadata is a provider-agnostic summary of server-side tool
// activity that accompanies an assistant response.
type GroundingMetadata struct {
	// Tools lists the server-side tools that contributed to this grounding
	// event (e.g. "google_search", "url_context", "web_search", "web_fetch").
	// Populated by the provider adapter when it can tell which tools ran.
	Tools            []string          `json:"tools,omitempty"`
	WebSearchQueries []string          `json:"web_search_queries,omitempty"`
	Sources          []GroundingSource `json:"sources,omitempty"`
	RetrievedURLs    []RetrievedURL    `json:"retrieved_urls,omitempty"`
}

// GroundingSource describes a single web result the provider used while
// generating the answer.
type GroundingSource struct {
	Title   string `json:"title,omitempty"`
	URI     string `json:"uri,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

// RetrievedURL describes a URL the provider fetched via a URL-context style
// tool, along with the retrieval status it reported.
type RetrievedURL struct {
	URL    string `json:"url"`
	Status string `json:"status,omitempty"`
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
	ContentTypeGrounding  = "grounding"
)

// Server-side tool kind constants. These are the canonical names used both
// in the YAML `tools:` list and on the wire inside CreateMessageRequest.ServerTools.
const (
	ServerToolGoogleSearch = "google_search"
	ServerToolURLContext   = "url_context"
	// ServerToolWebSearch is a provider-agnostic web-search server tool. It is
	// currently handled by the OpenRouter provider (mapped to the
	// "openrouter:web_search" server tool); providers that do not support it
	// ignore it.
	ServerToolWebSearch = "web_search"
	// ServerToolWebFetch is a provider-agnostic web-fetch server tool. It is
	// currently handled by the OpenRouter provider (mapped to the
	// "openrouter:web_fetch" server tool); providers that do not support it
	// ignore it.
	ServerToolWebFetch = "web_fetch"
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
// name is the tool's declared name (required by providers like Gemini on
// functionResponse); toolUseID is the per-call identifier (OpenAI tool_call_id).
func NewToolResultContentBlock(toolUseID, name string, content interface{}, isError bool) ContentBlock {
	return ContentBlock{
		Type:      ContentTypeToolResult,
		ToolUseID: toolUseID,
		Name:      name,
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
