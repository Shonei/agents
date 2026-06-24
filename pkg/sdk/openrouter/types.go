package openrouter

// Constants for OpenRouter roles. OpenRouter follows the OpenAI chat
// completions shape, which has four message roles.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Content part type discriminators used in the multi-part content array.
const (
	ContentPartText     = "text"
	ContentPartImageURL = "image_url"
)

// ToolTypeWebSearch is the OpenRouter server-tool type that enables real-time
// web search. It is added to the request's tools array and executed
// server-side by OpenRouter; the model decides when to search.
const ToolTypeWebSearch = "openrouter:web_search"

// Common OpenRouter model identifiers. OpenRouter model IDs are always
// namespaced as "<provider>/<model>". These are convenience constants; any
// valid OpenRouter model slug can be passed via configuration instead.
const (
	// ModelKimiLatest is an alias that always resolves to the newest model in
	// Moonshot AI's Kimi family.
	ModelKimiLatest   = "moonshotai/kimi-k2.6"
	ModelKimiK2       = "moonshotai/kimi-k2.5"
	ModelClaudeOpus   = "anthropic/claude-opus-4.1"
	ModelClaudeSonnet = "anthropic/claude-sonnet-4.5"
	ModelGPT5         = "openai/gpt-5.2"
	ModelGeminiPro    = "google/gemini-3.1-pro-preview"
	ModelDeepSeekV3   = "deepseek/deepseek-chat"
)

// ChatCompletionRequest is the request body for POST /chat/completions.
type ChatCompletionRequest struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []Tool           `json:"tools,omitempty"`
	ToolChoice  any              `json:"tool_choice,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Reasoning   *ReasoningConfig `json:"reasoning,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

// ReasoningConfig enables and tunes extended reasoning ("thinking") for models
// that support it. OpenRouter returns the reasoning trace in the response
// message's Reasoning field.
type ReasoningConfig struct {
	Enabled bool   `json:"enabled,omitempty"`
	Effort  string `json:"effort,omitempty"` // "low", "medium", "high"
}

// Message is a single conversation message. Content is either a plain string
// or a []ContentPart for multi-part (text + image) content. Assistant messages
// that call tools carry them in ToolCalls; tool-result messages use the
// RoleTool role together with ToolCallID.
type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ContentPart is one element of a multi-part message content array.
type ContentPart struct {
	Type         string        `json:"type"`
	Text         string        `json:"text,omitempty"`
	ImageURL     *ImageURL     `json:"image_url,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// ImageURL carries an image as a URL. For inline image data we encode it as a
// data URL: "data:<mime>;base64,<data>".
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// CacheControl marks a content part as a prompt-cache breakpoint. OpenRouter
// requires explicit breakpoints for some providers (Anthropic, Gemini, Qwen)
// and ignores them for providers that cache automatically (OpenAI, Moonshot,
// DeepSeek, Grok), so it is always safe to set.
type CacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// Tool is an entry in the request's tools array. For local function tools Type
// is "function" and Function is set; for provider server tools (e.g. web
// search) Type is the server-tool id and Function is omitted.
type Tool struct {
	Type     string         `json:"type"`
	Function *ToolFunction  `json:"function,omitempty"`
	Params   map[string]any `json:"parameters,omitempty"`
}

// ToolFunction describes a callable function and its JSON-schema parameters.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToolCall is a function invocation requested by the assistant.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall holds the called function name and its arguments. Arguments is
// a JSON-encoded string (not an object), matching the OpenAI wire format.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolChoiceFunction is the object form of tool_choice used to force a
// specific function call.
type ToolChoiceFunction struct {
	Type     string                 `json:"type"` // "function"
	Function ToolChoiceFunctionName `json:"function"`
}

// ToolChoiceFunctionName names the function to force in a tool_choice object.
type ToolChoiceFunctionName struct {
	Name string `json:"name"`
}

// ChatCompletionResponse is the response body for a non-streaming completion.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice is a single completion candidate.
type Choice struct {
	Index        int             `json:"index"`
	Message      ResponseMessage `json:"message"`
	FinishReason string          `json:"finish_reason,omitempty"`
}

// ResponseMessage is the assistant message returned in a choice. Reasoning is
// populated for reasoning-capable models when reasoning is enabled.
type ResponseMessage struct {
	Role        string       `json:"role"`
	Content     string       `json:"content"`
	Reasoning   string       `json:"reasoning,omitempty"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	Annotations []Annotation `json:"annotations,omitempty"`
}

// Annotation is a citation attached to the assistant message, following the
// OpenAI annotation schema. Web-search results arrive as url_citation
// annotations.
type Annotation struct {
	Type        string       `json:"type"` // "url_citation"
	URLCitation *URLCitation `json:"url_citation,omitempty"`
}

// URLCitation is a single web source the model cited.
type URLCitation struct {
	URL     string `json:"url,omitempty"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content,omitempty"`
}

// Usage reports token counts. PromptTokensDetails surfaces cache hits when the
// provider reports them.
type Usage struct {
	PromptTokens       int                 `json:"prompt_tokens"`
	CompletionTokens   int                 `json:"completion_tokens"`
	TotalTokens        int                 `json:"total_tokens"`
	PromptTokensDetail *PromptTokensDetail `json:"prompt_tokens_details,omitempty"`
}

// PromptTokensDetail carries cache accounting for the prompt tokens.
type PromptTokensDetail struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}
