package claude

import (
	"fmt"
	"net/http"
	"os"

	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

const (
	// DefaultAPIVersion is the default Anthropic API version
	DefaultAPIVersion = "2023-06-01"
	// DefaultModel is the default Claude model to use
	DefaultModel = ModelClaude45
	// DefaultMaxTokens is the default maximum tokens for responses
	DefaultMaxTokens = 1024
	// EnvAPIKey is the environment variable name for the Anthropic API key
	EnvAPIKey = "ANTHROPIC_API_KEY" //nolint:gosec
)

type Agent struct {
	httpClient *utils.HTTPBuilder
	apiKey     string
	apiVersion string
	model      string
	maxTokens  int
}

// AgentOption is a functional option for configuring the Agent
type AgentOption func(*Agent)

// WithAPIKey sets the API key for the agent
func WithAPIKey(apiKey string) AgentOption {
	return func(a *Agent) {
		a.apiKey = apiKey
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

// Model returns the model configured for the agent
func (a *Agent) Model() string {
	return a.model
}

// MaxTokens returns the max tokens configured for the agent
func (a *Agent) MaxTokens() int {
	return a.maxTokens
}

// NewAgent creates a new Agent with the given options
func NewAgent(opts ...AgentOption) *Agent {
	agent := &Agent{
		httpClient: utils.NewHTTPBuilder("https://api.anthropic.com"),
		apiKey:     os.Getenv(EnvAPIKey),
		apiVersion: DefaultAPIVersion,
		model:      DefaultModel,
		maxTokens:  DefaultMaxTokens,
	}

	for _, opt := range opts {
		opt(agent)
	}

	return agent
}

// SendMessage sends a simple text message to Claude and returns the response
func (a *Agent) SendMessage(message string) (*sdk.MessageResponse, error) {
	if a.apiKey == "" {
		return nil, fmt.Errorf("API key is required. Set %s environment variable or use WithAPIKey option", EnvAPIKey)
	}

	// Create the request
	request := sdk.CreateMessageRequest{
		Model:     a.model,
		MaxTokens: a.maxTokens,
		Messages: []sdk.InputMessage{
			sdk.NewTextMessage(sdk.RoleUser, message),
		},
	}

	return a.CreateMessage(request)
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
		Into(&internalResp).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	// Convert internal Claude response back to SDK type
	return a.convertResponse(internalResp), nil
}

func (a *Agent) convertRequest(req sdk.CreateMessageRequest) CreateMessageRequest {
	return CreateMessageRequest{
		Model:         req.Model,
		Messages:      req.Messages,
		MaxTokens:     req.MaxTokens,
		StopSequences: req.StopSequences,
		Stream:        req.Stream,
		System:        req.System,
		Temperature:   req.Temperature,
		ToolChoice:    req.ToolChoice,
		Tools:         req.Tools,
		TopK:          req.TopK,
		TopP:          req.TopP,
	}
}

func (a *Agent) convertResponse(resp MessageResponse) *sdk.MessageResponse {
	return &sdk.MessageResponse{
		ID:           resp.ID,
		Type:         resp.Type,
		Role:         resp.Role,
		Content:      resp.Content,
		Model:        resp.Model,
		StopReason:   resp.StopReason,
		StopSequence: resp.StopSequence,
		Usage:        resp.Usage,
	}
}
