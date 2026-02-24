package claude

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

const (
	// DefaultAPIVersion is the default Anthropic API version
	DefaultAPIVersion = "2023-06-01"
	// DefaultModel is the default Claude model to use
	DefaultModel = ModelClaude46
	// DefaultMaxTokens is the default maximum tokens for responses
	DefaultMaxTokens = 10_000
	// EnvAPIKey is the environment variable name for the Anthropic API key
	EnvAPIKey = "ANTHROPIC_API_KEY" //nolint:gosec
)

type Agent struct {
	httpClient      *utils.HTTPBuilder
	apiKey          string
	apiVersion      string
	model           string
	maxTokens       int
	thinkingEnabled bool
	temperature     float64
}

// AgentOption is a functional option for configuring the Agent
type AgentOption func(*Agent)

// WithAPIKey sets the API key for the agent
func WithAPIKey(apiKey string) AgentOption {
	return func(a *Agent) {
		a.apiKey = apiKey
	}
}

func WithThinking() AgentOption {
	return func(a *Agent) {
		a.thinkingEnabled = true
	}
}

// WithModel sets the default model for the agent
func WithModel(model string) AgentOption {
	return func(a *Agent) {
		a.model = model
	}
}

// WithMaxTokens sets the default max tokens for the agent
func WithMaxTokens(maxTokens int) AgentOption {
	return func(a *Agent) {
		a.maxTokens = maxTokens
	}
}

func WithTemperature(temperature float64) AgentOption {
	return func(a *Agent) {
		a.temperature = temperature
	}
}

// Model returns the model configured for the agent
func (a *Agent) Model() string {
	return a.model
}

// MaxTokens returns the max tokens configured for the agent
func (a *Agent) MaxTokens() int {
	return a.maxTokens
}

func retry(attempt int, resp *http.Response) (int, bool) {
	if attempt > 3 {
		return 0, false
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		// try and get header
		retryAfterHeader := resp.Header.Get("Retry-After")
		if retryAfterHeader != "" {
			retryAfter, err := strconv.Atoi(retryAfterHeader)
			if err == nil {
				// +1 just in case. We are already waiting minutes 1 second won't hurt
				return retryAfter + 1, true
			}
		}

		return 15, true
	}

	return 0, false
}

// NewAgent creates a new Agent with the given options
func NewAgent(opts ...AgentOption) *Agent {
	agent := &Agent{
		httpClient:      utils.NewHTTPBuilder("https://api.anthropic.com"),
		apiKey:          os.Getenv(EnvAPIKey),
		apiVersion:      DefaultAPIVersion,
		model:           DefaultModel,
		maxTokens:       DefaultMaxTokens,
		temperature:     0.0,
		thinkingEnabled: false,
	}

	for _, opt := range opts {
		opt(agent)
	}

	return agent
}

// CreateMessage sends a message request to the Claude API
func (a *Agent) CreateMessage(request sdk.CreateMessageRequest) (*sdk.MessageResponse, error) {
	if a.apiKey == "" {
		return nil, fmt.Errorf("API key is required. Set %s environment variable or use WithAPIKey option", EnvAPIKey)
	}

	// Convert SDK request to internal Claude request type
	internalReq := a.convertRequest(request)

	var internalResp MessageResponse

	err := a.httpClient.
		New().
		WithMethod(http.MethodPost).
		WithPath("/v1/messages").
		WithHeader("x-api-key", a.apiKey).
		WithHeader("anthropic-version", a.apiVersion).
		JSONBody(internalReq).
		WithRetry(retry).
		Into(&internalResp).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	// Convert internal Claude response back to SDK type
	return a.convertResponse(internalResp), nil
}

func (a *Agent) convertRequest(req sdk.CreateMessageRequest) CreateMessageRequest {
	messages := []InputMessage{}
	for _, msg := range req.Messages {
		content := []ContentBlock{}

		switch v := msg.Content.(type) {
		case string:
			content = append(content, ContentBlock{Type: ContentTypeText, Text: v})
			msg.Content = content
		case []sdk.ContentBlock:
			for _, block := range v {
				switch block.Type {
				case sdk.ContentTypeText:
					content = append(content, ContentBlock{Type: ContentTypeText, Text: block.Text})
				case sdk.ContentTypeToolUse:
					content = append(content, ContentBlock{Type: ContentTypeToolUse, ID: block.ID, Name: block.Name, Input: block.Input})
				case sdk.ContentTypeToolResult:
					content = append(content, ContentBlock{Type: ContentTypeToolResult, ToolUseID: block.ToolUseID, Content: block.Content, IsError: block.IsError})
				case sdk.ContentTypeThinking:
					content = append(content, ContentBlock{Type: ContentTypeThinking, Thinking: block.Text, Signature: block.ThoughtSignature})
				}
			}
			msg.Content = content
		}

		messages = append(messages, InputMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	toolChoices := ToolChoice{}
	if req.ToolChoice != nil {
		toolChoices = ToolChoice{
			Type: req.ToolChoice.Type,
			Name: req.ToolChoice.Name,
		}
	}

	tools := []Tool{}
	for _, tool := range req.Tools {
		tools = append(tools, Tool{
			Type:        tool.Type,
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	claudeReq := CreateMessageRequest{
		Model:         a.model,
		Messages:      messages,
		MaxTokens:     a.maxTokens,
		StopSequences: req.StopSequences,
		Stream:        req.Stream,
		System:        req.System,
		Temperature:   &a.temperature,
		ToolChoice:    &toolChoices,
		Tools:         tools,
	}

	if a.thinkingEnabled {
		claudeReq.Thinking = &ThinkingConfig{Type: "enabled", BudgetTokens: 2000}
	}

	return claudeReq
}

func (a *Agent) convertResponse(resp MessageResponse) *sdk.MessageResponse {
	uage := sdk.Usage{
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}

	content := []sdk.ResponseContentBlock{}
	for _, block := range resp.Content {
		switch block.Type {
		case ContentTypeText:
			content = append(content, sdk.ResponseContentBlock{
				Type: sdk.ContentTypeText,
				Text: block.Text,
			})
		case ContentTypeToolUse:
			content = append(content, sdk.ResponseContentBlock{
				Type:             sdk.ContentTypeToolUse,
				ID:               block.ID,
				Name:             block.Name,
				Input:            block.Input,
				ThoughtSignature: block.ThoughtSignature,
			})
		case ContentTypeThinking:
			content = append(content, sdk.ResponseContentBlock{
				Type:             sdk.ContentTypeThinking,
				Text:             block.Thinking,
				ThoughtSignature: block.Signature,
			})
		default:
			content = append(content, sdk.ResponseContentBlock{
				Type:             block.Type,
				Text:             block.Text,
				ID:               block.ID,
				Name:             block.Name,
				Input:            block.Input,
				ThoughtSignature: block.ThoughtSignature,
			})
		}
	}

	return &sdk.MessageResponse{
		ID:           resp.ID,
		Type:         resp.Type,
		Role:         resp.Role,
		Content:      content,
		Model:        resp.Model,
		StopReason:   resp.StopReason,
		StopSequence: resp.StopSequence,
		Usage:        uage,
	}
}
