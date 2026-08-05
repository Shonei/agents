package gemini

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

const (
	// DefaultModel is the default Gemini model to use
	DefaultModel = ModelGemini31ProPreview
	// DefaultMaxTokens is the default maximum tokens for responses
	DefaultMaxTokens = 8192
	// EnvAPIKey is the environment variable name for the Gemini API key
	EnvAPIKey = "GEMINI_API_KEY" //nolint:gosec
)

type Agent struct {
	httpClient         *utils.HTTPBuilder
	apiKey             string
	model              string
	maxTokens          int
	maxContextTokens   int
	maxContextTurns    int
	temperature        float64
	embeddingDim       int
	responseModalities []string
	thinkingEnabled    bool
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

// WithMaxContextTokens sets the input-context token budget at which the chat
// loop will trigger conversation compaction. A value of 0 disables compaction.
func WithMaxContextTokens(maxContextTokens int) AgentOption {
	return func(a *Agent) {
		a.maxContextTokens = maxContextTokens
	}
}

// WithMaxContextTurns sets the number of recent user turns to preserve verbatim
// when compacting. A value of 0 uses the default.
func WithMaxContextTurns(maxContextTurns int) AgentOption {
	return func(a *Agent) {
		a.maxContextTurns = maxContextTurns
	}
}

func WithTemperature(temperature float64) AgentOption {
	return func(a *Agent) {
		a.temperature = temperature
	}
}

func WithResponseModalities(modalities []string) AgentOption {
	return func(a *Agent) {
		a.responseModalities = modalities
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

// MaxContextTokens returns the input-context token budget configured for the
// agent. A value of 0 disables conversation compaction.
func (a *Agent) MaxContextTokens() int {
	return a.maxContextTokens
}

// MaxContextTurns returns the number of recent user turns to preserve verbatim
// when compacting. A value of 0 means the caller should use its default.
func (a *Agent) MaxContextTurns() int {
	return a.maxContextTurns
}

// NewAgent creates a new Agent with the given options
func NewAgent(opts ...AgentOption) *Agent {
	agent := &Agent{
		httpClient:  utils.NewHTTPBuilder("https://generativelanguage.googleapis.com"),
		apiKey:      os.Getenv(EnvAPIKey),
		model:       DefaultModel,
		maxTokens:   DefaultMaxTokens,
		temperature: 0.0,
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
		WithRetry(retry).
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

	// Add to the config convertRequest already built rather than replacing it:
	// replacing it discarded the configured max_tokens and silently capped every
	// response at DefaultMaxTokens.
	if geminiRequest.GenerationConfig == nil {
		geminiRequest.GenerationConfig = &GenerationConfig{}
	}

	if geminiRequest.GenerationConfig.MaxOutputTokens <= 0 {
		geminiRequest.GenerationConfig.MaxOutputTokens = DefaultMaxTokens
	}

	geminiRequest.GenerationConfig.Temperature = a.temperature
	geminiRequest.GenerationConfig.ResponseModalities = a.responseModalities

	if a.thinkingEnabled {
		geminiRequest.GenerationConfig.ThinkingConfig = &ThinkingConfig{
			IncludeThoughts: true,
		}
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
			MaxOutputTokens: a.maxTokens,
			Temperature:     a.temperature,
		},
	}

	if req.System != "" {
		geminiReq.SystemInstruction = &Content{
			Parts: []Part{{Text: req.System}},
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
								MimeType: block.Source.MimeType,
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
						var parsedMap map[string]interface{}
						if err := json.Unmarshal([]byte(strContent), &parsedMap); err == nil {
							responseMap = parsedMap
						} else {
							responseMap = map[string]interface{}{"result": strContent}
						}
					} else if mapContent, ok := block.Content.(map[string]interface{}); ok {
						responseMap = mapContent
					} else {
						// Fallback
						responseMap = map[string]interface{}{"result": block.Content}
					}

					// Gemini functionResponse.name must be the function name, not
					// the per-call id. Fall back to ToolUseID for older history
					// where id and name were the same string.
					name := block.Name
					if name == "" {
						name = block.ToolUseID
					}

					parts = append(parts, Part{
						FunctionResponse: &FunctionResponse{
							Name:     name,
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

	// Convert server-side tools (executed by Gemini, not by the SDK loop)
	for _, st := range req.ServerTools {
		switch st.Name {
		case sdk.ServerToolGoogleSearch:
			geminiReq.Tools = append(geminiReq.Tools, Tool{GoogleSearch: &GoogleSearch{}})
		case sdk.ServerToolURLContext:
			geminiReq.Tools = append(geminiReq.Tools, Tool{URLContext: &URLContext{}})
		}
	}

	// Tool Choice
	// Gemini's functionCallingConfig governs the local function-declaration
	// tools only; sending it when none are declared (server tools only)
	// causes the API to reject the request, so guard on req.Tools here.
	if len(req.ServerTools) > 0 {
		geminiReq.ToolConfig = ensureToolConfig(geminiReq.ToolConfig)
		geminiReq.ToolConfig.IncludeServerSideToolInvocations = new(true)
	}
	if req.ToolChoice != nil && len(req.Tools) > 0 {
		geminiReq.ToolConfig = ensureToolConfig(geminiReq.ToolConfig)
		geminiReq.ToolConfig.FunctionCallingConfig = &FunctionCallingConfig{}
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

func ensureToolConfig(toolConfig *ToolConfig) *ToolConfig {
	if toolConfig != nil {
		return toolConfig
	}

	return &ToolConfig{}
}

func (a *Agent) convertResponse(resp GenerateContentResponse) (*sdk.MessageResponse, error) {
	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates returned")
	}

	candidate := resp.Candidates[0]

	contentBlocks := []sdk.ResponseContentBlock{}
	funcCallIndex := 0

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
			// Gemini has no separate tool-call id. Synthesize a unique one so
			// parallel calls to the same function do not collide.
			funcCallIndex++
			callID := fmt.Sprintf("%s_%d", part.FunctionCall.Name, funcCallIndex)

			contentBlocks = append(contentBlocks, sdk.ResponseContentBlock{
				Type:             sdk.ContentTypeToolUse,
				ID:               callID,
				Name:             part.FunctionCall.Name,
				Input:            part.FunctionCall.Args,
				ThoughtSignature: part.ThoughtSignature,
			})

			continue
		}

		if part.InlineData != nil {
			contentBlocks = append(contentBlocks, sdk.ResponseContentBlock{
				Type: sdk.ContentTypeImage,
				Blob: &sdk.Blob{
					MimeType: part.InlineData.MimeType,
					Data:     part.InlineData.Data,
				},
			})

			continue
		}
	}

	if grounding := convertGrounding(candidate.GroundingMetadata, candidate.URLContextMetadata); grounding != nil {
		contentBlocks = append(contentBlocks, sdk.ResponseContentBlock{
			Type:      sdk.ContentTypeGrounding,
			Grounding: grounding,
		})
	}

	return &sdk.MessageResponse{
		ID:      resp.ResponseId,
		Type:    "message",
		Role:    sdk.RoleAssistant,
		Content: contentBlocks,
		Model:   a.model,
		Usage:   usageOf(resp.UsageMetadata),
	}, nil
}

// usageOf tolerates a response without usageMetadata: the field is optional on
// the wire, and losing token counts must degrade compaction rather than panic.
func usageOf(m *UsageMetadata) sdk.Usage {
	if m == nil {
		return sdk.Usage{}
	}

	return sdk.Usage{
		InputTokens:  m.PromptTokenCount,
		OutputTokens: m.CandidatesTokenCount,
	}
}

// convertGrounding maps Gemini's groundingMetadata payload onto the
// provider-agnostic sdk.GroundingMetadata shape. Returns nil when the
// candidate carries no useful grounding info.
func convertGrounding(g *GroundingMetadata, urlContext *URLContextMetadata) *sdk.GroundingMetadata {
	if g == nil && urlContext == nil {
		return nil
	}

	out := &sdk.GroundingMetadata{}
	if g != nil {
		out.WebSearchQueries = g.WebSearchQueries

		for _, chunk := range g.GroundingChunks {
			if chunk.Web == nil {
				continue
			}
			out.Sources = append(out.Sources, sdk.GroundingSource{
				Title: chunk.Web.Title,
				URI:   chunk.Web.URI,
			})
		}

		appendURLMetadata(out, g.URLContextMetadata)
	}
	appendURLMetadata(out, urlContext)

	if len(out.WebSearchQueries) == 0 && len(out.Sources) == 0 && len(out.RetrievedURLs) == 0 {
		return nil
	}

	// Infer which Gemini server tools produced this side-channel. Search
	// queries / grounding chunks come from google_search; retrieved_urls come
	// from url_context. Both can appear on the same candidate.
	if len(out.WebSearchQueries) > 0 || len(out.Sources) > 0 {
		out.Tools = append(out.Tools, sdk.ServerToolGoogleSearch)
	}
	if len(out.RetrievedURLs) > 0 {
		out.Tools = append(out.Tools, sdk.ServerToolURLContext)
	}

	return out
}

func appendURLMetadata(out *sdk.GroundingMetadata, metadata *URLContextMetadata) {
	if metadata == nil {
		return
	}

	seen := make(map[string]struct{}, len(out.RetrievedURLs))
	for _, u := range out.RetrievedURLs {
		seen[u.URL+"|"+u.Status] = struct{}{}
	}
	for _, u := range metadata.URLMetadata {
		if u.RetrievedURL == "" {
			continue
		}
		key := u.RetrievedURL + "|" + u.URLRetrievalStatus
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out.RetrievedURLs = append(out.RetrievedURLs, sdk.RetrievedURL{
			URL:    u.RetrievedURL,
			Status: u.URLRetrievalStatus,
		})
	}
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
