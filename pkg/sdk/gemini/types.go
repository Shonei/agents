package gemini

// GenerateContentRequest represents the request body for generating content
type GenerateContentRequest struct {
	Contents          []Content         `json:"contents"`
	Tools             []Tool            `json:"tools,omitempty"`
	ToolConfig        *ToolConfig       `json:"toolConfig,omitempty"`
	GenerationConfig  *GenerationConfig `json:"generationConfig,omitempty"`
	SystemInstruction *Content          `json:"systemInstruction,omitempty"`
}

// Content represents a message in the conversation
type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role,omitempty"` // "user" or "model"
}

// Part represents a part of the content
type Part struct {
	Text             string            `json:"text,omitempty"`
	InlineData       *Blob             `json:"inlineData,omitempty"`
	FunctionCall     *FunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`
	ThoughtSignature string            `json:"thoughtSignature,omitempty"`
}

// Blob represents raw data (like images)
type Blob struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// FunctionCall represents a function call
type FunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args,omitempty"`
}

// FunctionResponse represents a function response
type FunctionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

// Tool represents a tool definition
type Tool struct {
	FunctionDeclarations []FunctionDeclaration `json:"functionDeclarations,omitempty"`
}

// FunctionDeclaration represents a function declaration
type FunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolConfig represents tool configuration
type ToolConfig struct {
	FunctionCallingConfig *FunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

// FunctionCallingConfig represents function calling configuration
type FunctionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"` // "AUTO", "ANY", "NONE"
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

// GenerationConfig represents generation configuration
type GenerationConfig struct {
	StopSequences   []string `json:"stopSequences,omitempty"`
	CandidateCount  int      `json:"candidateCount,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	Temperature     float64  `json:"temperature,omitempty"`
	TopP            float64  `json:"topP,omitempty"`
	TopK            int      `json:"topK,omitempty"`
}

// GenerateContentResponse represents the response from generating content
type GenerateContentResponse struct {
	Candidates    []Candidate    `json:"candidates"`
	UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`
}

// Candidate represents a generation candidate
type Candidate struct {
	Content       Content        `json:"content"`
	FinishReason  string         `json:"finishReason,omitempty"`
	SafetyRatings []SafetyRating `json:"safetyRatings,omitempty"`
	TokenCount    int            `json:"tokenCount,omitempty"`
	Index         int            `json:"index,omitempty"`
}

// SafetyRating represents a safety rating
type SafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

// UsageMetadata represents usage metadata
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// Constants
const (
	RoleUser             = "user"
	RoleModel            = "model"
	ModelGemini3         = "gemini-3-pro-preview"
	ModelGeminiEmbedding = "gemini-embedding-001"
)

// Embeddings

// EmbedContentRequest represents the request body for the embedContent endpoint.
type EmbedContentRequest struct {
	Content         *Content `json:"content,omitempty"`
	OutputDimension int      `json:"output_dimensionality,omitempty"`
}

// ContentEmbedding represents a single embedding vector.
type ContentEmbedding struct {
	Values []float32 `json:"values,omitempty"`
}

// EmbedContentResponse represents the response from the embedContent endpoint.
type EmbedContentResponse struct {
	Embedding *ContentEmbedding `json:"embedding,omitempty"`
}
