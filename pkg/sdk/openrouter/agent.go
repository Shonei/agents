package openrouter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

const (
	// BaseURL is the OpenRouter API base URL.
	BaseURL = "https://openrouter.ai/api/v1"
	// DefaultModel is the model used when none is configured.
	DefaultModel = ModelKimiLatest
	// DefaultMaxTokens is the default maximum tokens for responses.
	DefaultMaxTokens = 8192
	// EnvAPIKey is the environment variable name for the OpenRouter API key.
	EnvAPIKey = "OPENROUTER_API_KEY" //nolint:gosec
)

type Agent struct {
	httpClient       *utils.HTTPBuilder
	apiKey           string
	model            string
	maxTokens        int
	maxContextTokens int
	temperature      float64
	thinkingEnabled  bool
	cachingEnabled   bool
}

// AgentOption is a functional option for configuring the Agent.
type AgentOption func(*Agent)

// WithAPIKey sets the API key for the agent.
func WithAPIKey(apiKey string) AgentOption {
	return func(a *Agent) {
		a.apiKey = apiKey
	}
}

// WithThinking enables extended reasoning for models that support it.
func WithThinking() AgentOption {
	return func(a *Agent) {
		a.thinkingEnabled = true
	}
}

// WithModel sets the model for the agent (e.g. "moonshotai/kimi-k2.5").
func WithModel(model string) AgentOption {
	return func(a *Agent) {
		a.model = model
	}
}

// WithMaxTokens sets the default max output tokens for the agent.
func WithMaxTokens(maxTokens int) AgentOption {
	return func(a *Agent) {
		a.maxTokens = maxTokens
	}
}

// WithMaxContextTokens sets the input-context token budget at which the chat
// loop will trigger conversation compaction. A value of 0 disables compaction.
func WithMaxContextTokens(maxContextTokens int) AgentOption {
	return func(a *Agent) {
		a.maxContextTokens = maxContextTokens
	}
}

// WithTemperature sets the sampling temperature.
func WithTemperature(temperature float64) AgentOption {
	return func(a *Agent) {
		a.temperature = temperature
	}
}

// WithCaching toggles prompt-cache breakpoints. Caching is on by default; pass
// false to disable it.
func WithCaching(enabled bool) AgentOption {
	return func(a *Agent) {
		a.cachingEnabled = enabled
	}
}

// Model returns the model configured for the agent.
func (a *Agent) Model() string {
	return a.model
}

// MaxTokens returns the max tokens configured for the agent.
func (a *Agent) MaxTokens() int {
	return a.maxTokens
}

// MaxContextTokens returns the input-context token budget configured for the
// agent. A value of 0 disables conversation compaction.
func (a *Agent) MaxContextTokens() int {
	return a.maxContextTokens
}

// NewAgent creates a new OpenRouter Agent with the given options. Prompt
// caching is enabled by default.
func NewAgent(opts ...AgentOption) *Agent {
	agent := &Agent{
		httpClient:     utils.NewHTTPBuilder(BaseURL),
		apiKey:         os.Getenv(EnvAPIKey),
		model:          DefaultModel,
		maxTokens:      DefaultMaxTokens,
		temperature:    0.0,
		cachingEnabled: true,
	}

	for _, opt := range opts {
		opt(agent)
	}

	return agent
}

// CreateMessage sends a message request to the OpenRouter API.
func (a *Agent) CreateMessage(request sdk.CreateMessageRequest) (*sdk.MessageResponse, error) {
	if a.apiKey == "" {
		return nil, fmt.Errorf("API key is required. Set %s environment variable or use WithAPIKey option", EnvAPIKey)
	}

	openRouterRequest, err := a.convertRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	var response ChatCompletionResponse

	err = a.httpClient.
		New().
		WithMethod(http.MethodPost).
		WithHeader("Authorization", "Bearer "+a.apiKey).
		WithHeader("X-Title", "agents").
		WithPath("/chat/completions").
		WithRetry(retry).
		JSONBody(openRouterRequest).
		Into(&response).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	return a.convertResponse(response)
}

func (a *Agent) convertRequest(req sdk.CreateMessageRequest) (*ChatCompletionRequest, error) {
	openRouterReq := &ChatCompletionRequest{
		Model:       a.model,
		Messages:    []Message{},
		MaxTokens:   a.maxTokens,
		Temperature: a.temperature,
	}

	if a.thinkingEnabled {
		openRouterReq.Reasoning = &ReasoningConfig{Enabled: true}
	}

	// System prompt becomes a leading system-role message. When caching is on
	// it is the highest-value, most stable breakpoint, so mark it.
	if req.System != "" {
		sys := Message{Role: RoleSystem, Content: req.System}
		if a.cachingEnabled {
			markCacheable(&sys)
		}
		openRouterReq.Messages = append(openRouterReq.Messages, sys)
	}

	for _, msg := range req.Messages {
		role := RoleUser
		if msg.Role == sdk.RoleAssistant {
			role = RoleAssistant
		}

		switch v := msg.Content.(type) {
		case string:
			openRouterReq.Messages = append(openRouterReq.Messages, Message{Role: role, Content: v})
		case []sdk.ContentBlock:
			openRouterReq.Messages = append(openRouterReq.Messages, convertBlocks(role, v)...)
		}
	}

	// Move the conversation-tail cache breakpoint to the final message so the
	// growing prefix (system + tools + earlier turns) stays cached across the
	// agentic loop.
	if a.cachingEnabled && len(openRouterReq.Messages) > 0 {
		markLastCacheable(openRouterReq.Messages)
	}

	// Local function tools.
	for _, t := range req.Tools {
		openRouterReq.Tools = append(openRouterReq.Tools, Tool{
			Type: "function",
			Function: &ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	// Provider-executed server tools. OpenRouter runs these itself and feeds
	// the results back to the model. Only web search is supported today;
	// other kinds (e.g. Gemini's google_search) are ignored here.
	for _, st := range req.ServerTools {
		if st.Name == sdk.ServerToolWebSearch {
			openRouterReq.Tools = append(openRouterReq.Tools, Tool{Type: ToolTypeWebSearch})
		}
	}

	// Tool choice only applies when local tools are declared.
	if req.ToolChoice != nil && len(req.Tools) > 0 {
		switch req.ToolChoice.Type {
		case "auto":
			openRouterReq.ToolChoice = "auto"
		case "any":
			openRouterReq.ToolChoice = "required"
		case "none":
			openRouterReq.ToolChoice = "none"
		case "tool":
			openRouterReq.ToolChoice = ToolChoiceFunction{
				Type:     "function",
				Function: ToolChoiceFunctionName{Name: req.ToolChoice.Name},
			}
		}
	}

	return openRouterReq, nil
}

// convertBlocks turns a single SDK multi-part message into one or more
// OpenRouter messages. Text/image blocks accumulate into the parent message;
// tool_use blocks attach as tool_calls on an assistant message; tool_result
// blocks each become their own RoleTool message (OpenAI wire requirement).
func convertBlocks(role string, blocks []sdk.ContentBlock) []Message {
	var out []Message
	var parts []ContentPart
	var toolCalls []ToolCall

	flush := func() {
		if len(parts) == 0 && len(toolCalls) == 0 {
			return
		}
		m := Message{Role: role}
		if len(parts) > 0 {
			m.Content = parts
		}
		if len(toolCalls) > 0 {
			m.ToolCalls = toolCalls
		}
		out = append(out, m)
		parts = nil
		toolCalls = nil
	}

	for _, block := range blocks {
		switch block.Type {
		case sdk.ContentTypeText:
			parts = append(parts, ContentPart{Type: ContentPartText, Text: block.Text})
		case sdk.ContentTypeImage:
			if block.Source != nil {
				parts = append(parts, ContentPart{
					Type:     ContentPartImageURL,
					ImageURL: &ImageURL{URL: dataURL(block.Source)},
				})
			}
		case sdk.ContentTypeToolUse:
			args, err := json.Marshal(block.Input)
			if err != nil {
				args = []byte("{}")
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: FunctionCall{
					Name:      block.Name,
					Arguments: string(args),
				},
			})
		case sdk.ContentTypeToolResult:
			// Tool results must be standalone tool-role messages, so flush any
			// pending assistant content first to preserve ordering.
			flush()
			out = append(out, Message{
				Role:       RoleTool,
				ToolCallID: block.ToolUseID,
				Content:    stringifyToolResult(block.Content),
			})
		}
		// ContentTypeThinking is intentionally dropped on the way back in:
		// OpenRouter does not accept reasoning content as input.
	}

	flush()

	return out
}

func (a *Agent) convertResponse(resp ChatCompletionResponse) (*sdk.MessageResponse, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	msg := resp.Choices[0].Message

	contentBlocks := []sdk.ResponseContentBlock{}

	if msg.Reasoning != "" {
		contentBlocks = append(contentBlocks, sdk.ResponseContentBlock{
			Type: sdk.ContentTypeThinking,
			Text: msg.Reasoning,
		})
	}

	if msg.Content != "" {
		contentBlocks = append(contentBlocks, sdk.ResponseContentBlock{
			Type: sdk.ContentTypeText,
			Text: msg.Content,
		})
	}

	for _, tc := range msg.ToolCalls {
		var input map[string]any
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				input = map[string]any{}
			}
		}

		contentBlocks = append(contentBlocks, sdk.ResponseContentBlock{
			Type:  sdk.ContentTypeToolUse,
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	// Web-search citations arrive as message annotations; surface them as a
	// grounding block so sources display the same way Gemini grounding does.
	if grounding := convertAnnotations(msg.Annotations); grounding != nil {
		contentBlocks = append(contentBlocks, sdk.ResponseContentBlock{
			Type:      sdk.ContentTypeGrounding,
			Grounding: grounding,
		})
	}

	var usage sdk.Usage
	if resp.Usage != nil {
		usage = sdk.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
	}

	model := resp.Model
	if model == "" {
		model = a.model
	}

	return &sdk.MessageResponse{
		ID:      resp.ID,
		Type:    "message",
		Role:    sdk.RoleAssistant,
		Content: contentBlocks,
		Model:   model,
		Usage:   usage,
	}, nil
}

// dataURL encodes an SDK image blob as a base64 data URL for image_url parts.
func dataURL(b *sdk.Blob) string {
	return fmt.Sprintf("data:%s;base64,%s", b.MimeType, b.Data)
}

// convertAnnotations maps OpenRouter url_citation annotations onto the
// provider-agnostic sdk.GroundingMetadata shape. Returns nil when there are no
// usable citations.
func convertAnnotations(annotations []Annotation) *sdk.GroundingMetadata {
	out := &sdk.GroundingMetadata{}

	for _, ann := range annotations {
		if ann.URLCitation == nil || ann.URLCitation.URL == "" {
			continue
		}
		out.Sources = append(out.Sources, sdk.GroundingSource{
			Title:   ann.URLCitation.Title,
			URI:     ann.URLCitation.URL,
			Snippet: ann.URLCitation.Content,
		})
	}

	if len(out.Sources) == 0 {
		return nil
	}

	return out
}

// stringifyToolResult renders a tool result content value as the plain string
// OpenRouter expects for a tool-role message. JSON-encodable values are
// marshalled; strings pass through unchanged.
func stringifyToolResult(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", v)
	}
}

// markCacheable places a cache_control breakpoint on a message, converting
// string content to a single text part when needed. Returns true if a
// breakpoint was set.
func markCacheable(m *Message) bool {
	switch c := m.Content.(type) {
	case string:
		if c == "" {
			return false
		}
		m.Content = []ContentPart{{Type: ContentPartText, Text: c, CacheControl: ephemeral()}}

		return true
	case []ContentPart:
		for i := len(c) - 1; i >= 0; i-- {
			if c[i].Type == ContentPartText {
				c[i].CacheControl = ephemeral()

				return true
			}
		}
	}

	return false
}

// markLastCacheable sets a cache breakpoint on the last message that can carry
// one, scanning from the tail. Tool-role messages are skipped so the
// breakpoint lands on conversational text.
func markLastCacheable(messages []Message) {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleTool {
			continue
		}
		if markCacheable(&messages[i]) {
			return
		}
	}
}

func ephemeral() *CacheControl {
	return &CacheControl{Type: "ephemeral"}
}

func retry(attempt int, resp *http.Response) (int, bool) {
	if attempt > 3 {
		return 0, false
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return 15, true
	}

	return 0, false
}
