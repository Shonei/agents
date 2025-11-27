package gemini

import (
	"fmt"
	"net/http"
	"os"

	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

const (
	// DefaultModel is the default Gemini model to use
	DefaultModel = "gemini-3-pro-preview"
	// DefaultMaxTokens is the default maximum tokens for responses
	DefaultMaxTokens = 8192
	// EnvAPIKey is the environment variable name for the Gemini API key
	EnvAPIKey = "GEMINI_API_KEY" //nolint:gosec
)

type Agent struct {
	httpClient   *utils.HTTPBuilder
	apiKey       string
	model        string
	maxTokens    int
	embeddingDim int
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

func WithEmbeddingDim(dim int) AgentOption {
	return func(a *Agent) {
		a.embeddingDim = dim
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
		httpClient: utils.NewHTTPBuilder("https://generativelanguage.googleapis.com"),
		apiKey:     os.Getenv(EnvAPIKey),
		model:      DefaultModel,
		maxTokens:  DefaultMaxTokens,
	}

	for _, opt := range opts {
		opt(agent)
	}

	return agent
}

func (a *Agent) Embedding(message string) ([]float32, error) {
	req := &EmbedContentRequest{
		Content: &Content{
			Parts: []Part{{Text: message}},
		},
		OutputDimension: a.embeddingDim,
	}

	var resp EmbedContentResponse

	urlPath := fmt.Sprintf("/v1beta/models/%s:embedContent", ModelGeminiEmbedding)

	err := a.httpClient.
		New().
		WithMethod(http.MethodPost).
		WithHeader("x-goog-api-key", a.apiKey).
		WithPath(urlPath).
		JSONBody(req).
		Into(&resp).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding: %w", err)
	}

	if resp.Embedding == nil {
		return nil, fmt.Errorf("no embedding returned from Gemini API")
	}

	return resp.Embedding.Values, nil
}

// CreateMessage sends a message request to the Gemini API
func (a *Agent) CreateMessage(request sdk.CreateMessageRequest) (*sdk.MessageResponse, error) {
	if a.apiKey == "" {
		return nil, fmt.Errorf("API key is required. Set %s environment variable or use WithAPIKey option", EnvAPIKey)
	}

	// Convert SDK request to Gemini request
	geminiRequest, err := a.convertRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	geminiRequest.GenerationConfig = &GenerationConfig{
		ThinkingConfig: &ThinkingConfig{
			IncludeThoughts: true,
			// ThinkingBudget:  100,
			// ThinkingLevel:   1,
		},
		Temperature: 0.0,
		// MaxOutputTokens: request.MaxTokens,
	}

	var response GenerateContentResponse

	urlPath := fmt.Sprintf("/v1beta/models/%s:generateContent", a.model)

	err = a.httpClient.
		New().
		WithMethod(http.MethodPost).
		WithHeader("x-goog-api-key", a.apiKey).
		WithPath(urlPath).
		JSONBody(geminiRequest).
		Into(&response).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	// Convert Gemini response to SDK response
	return a.convertResponse(response)
}

func (a *Agent) convertRequest(req sdk.CreateMessageRequest) (*GenerateContentRequest, error) {
	geminiReq := &GenerateContentRequest{
		Contents: []Content{},
		GenerationConfig: &GenerationConfig{
			MaxOutputTokens: req.MaxTokens,
			Temperature:     0.0, // Default to 0 for deterministic output
		},
	}

	if req.Temperature != nil {
		geminiReq.GenerationConfig.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		geminiReq.GenerationConfig.TopP = *req.TopP
	}
	if req.TopK != nil {
		geminiReq.GenerationConfig.TopK = *req.TopK
	}

	// Convert System Prompt
	if req.System != nil {
		var systemText string
		switch v := req.System.(type) {
		case string:
			systemText = v
		case []sdk.ContentBlock:
			for _, block := range v {
				if block.Type == sdk.ContentTypeText {
					systemText += block.Text + "\n"
				}
			}
		}
		if systemText != "" {
			geminiReq.SystemInstruction = &Content{
				Parts: []Part{{Text: systemText}},
			}
		}
	}

	// Convert Messages
	for _, msg := range req.Messages {
		role := RoleUser
		if msg.Role == sdk.RoleAssistant {
			role = RoleModel
		}

		parts := []Part{}

		switch v := msg.Content.(type) {
		case string:
			parts = append(parts, Part{Text: v})
		case []sdk.ContentBlock:
			for _, block := range v {
				switch block.Type {
				case sdk.ContentTypeText:
					parts = append(parts, Part{Text: block.Text})
				case sdk.ContentTypeImage:
					if block.Source != nil {
						parts = append(parts, Part{
							InlineData: &Blob{
								MimeType: block.Source.MediaType,
								Data:     block.Source.Data,
							},
						})
					}

				case sdk.ContentTypeToolUse:
					parts = append(parts, Part{
						FunctionCall: &FunctionCall{
							Name: block.Name,
							Args: block.Input,
						},
						ThoughtSignature: block.ThoughtSignature,
					})
				case sdk.ContentTypeToolResult:
					// We need to parse the content back to map[string]interface{} if it's a JSON string
					var responseMap map[string]interface{}

					// If content is string, try to unmarshal it
					if strContent, ok := block.Content.(string); ok {
						responseMap = map[string]interface{}{"result": strContent}
					} else if mapContent, ok := block.Content.(map[string]interface{}); ok {
						responseMap = mapContent
					} else {
						// Fallback
						responseMap = map[string]interface{}{"result": block.Content}
					}

					parts = append(parts, Part{
						FunctionResponse: &FunctionResponse{
							Name:     block.ToolUseID, // We might need the name here, but SDK stores ID in ToolUseID for result.
							Response: responseMap,
						},
					})
				}
			}
		}

		geminiReq.Contents = append(geminiReq.Contents, Content{
			Role:  role,
			Parts: parts,
		})
	}

	// Convert Tools
	if len(req.Tools) > 0 {
		functionDecls := []FunctionDeclaration{}
		for _, t := range req.Tools {
			functionDecls = append(functionDecls, FunctionDeclaration{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			})
		}
		geminiReq.Tools = append(geminiReq.Tools, Tool{
			FunctionDeclarations: functionDecls,
		})
	}

	// Tool Choice
	if req.ToolChoice != nil {
		geminiReq.ToolConfig = &ToolConfig{
			FunctionCallingConfig: &FunctionCallingConfig{},
		}
		switch req.ToolChoice.Type {
		case "auto":
			geminiReq.ToolConfig.FunctionCallingConfig.Mode = "AUTO"
		case "any":
			geminiReq.ToolConfig.FunctionCallingConfig.Mode = "ANY"
		case "none":
			geminiReq.ToolConfig.FunctionCallingConfig.Mode = "NONE"
		case "tool":
			geminiReq.ToolConfig.FunctionCallingConfig.Mode = "ANY"
			geminiReq.ToolConfig.FunctionCallingConfig.AllowedFunctionNames = []string{req.ToolChoice.Name}
		}
	}

	return geminiReq, nil
}

func (a *Agent) convertResponse(resp GenerateContentResponse) (*sdk.MessageResponse, error) {
	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates returned")
	}

	candidate := resp.Candidates[0]

	contentBlocks := []sdk.ResponseContentBlock{}

	for _, part := range candidate.Content.Parts {
		if part.Thought {
			contentBlocks = append(contentBlocks, sdk.ResponseContentBlock{
				Type: sdk.ContentTypeThinking,
				Text: part.Text,
			})

			continue
		}

		if part.Text != "" {
			contentBlocks = append(contentBlocks, sdk.ResponseContentBlock{
				Type: sdk.ContentTypeText,
				Text: part.Text,
			})

			continue
		}

		if part.FunctionCall != nil {
			contentBlocks = append(contentBlocks, sdk.ResponseContentBlock{
				Type:             sdk.ContentTypeToolUse,
				ID:               part.FunctionCall.Name, // Gemini doesn't have a separate ID, so we use Name as ID
				Name:             part.FunctionCall.Name,
				Input:            part.FunctionCall.Args,
				ThoughtSignature: part.ThoughtSignature,
			})

			continue
		}
	}

	return &sdk.MessageResponse{
		ID:      resp.ResponseId,
		Type:    "message",
		Role:    sdk.RoleAssistant,
		Content: contentBlocks,
		Model:   a.model,
		Usage: sdk.Usage{
			InputTokens:  resp.UsageMetadata.PromptTokenCount,
			OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
		},
	}, nil
}
