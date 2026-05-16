package gemini

// Constants
const (
	RoleUser  = "user"
	RoleModel = "model"

	ModelGemini31ProPreview        = "gemini-3.1-pro-preview"
	ModelGemini31FlashLite         = "gemini-3.1-flash-lite"
	ModelGemini31FlashImagePreview = "gemini-3.1-flash-image-preview"
	ModelGeminiEmbedding           = "gemini-embedding-001"
)

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
	Thought          bool              `json:"thought,omitempty"`
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

// Tool represents a tool definition. Each tool entry can carry either local
// function declarations or a marker for a provider-executed (server-side)
// tool such as Google Search or URL Context.
type Tool struct {
	FunctionDeclarations []FunctionDeclaration `json:"functionDeclarations,omitempty"`
	GoogleSearch         *GoogleSearch         `json:"googleSearch,omitempty"`
	URLContext           *URLContext           `json:"urlContext,omitempty"`
}

// GoogleSearch is the marker payload for enabling Gemini's grounding-with-Google-Search
// server-side tool. It is intentionally empty; Gemini just needs the key to be present.
type GoogleSearch struct{}

// URLContext is the marker payload for enabling Gemini's URL context server-side tool.
type URLContext struct{}

// FunctionDeclaration represents a function declaration
type FunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolConfig represents tool configuration
type ToolConfig struct {
	FunctionCallingConfig            *FunctionCallingConfig `json:"functionCallingConfig,omitempty"`
	IncludeServerSideToolInvocations *bool                  `json:"includeServerSideToolInvocations,omitempty"`
}

// FunctionCallingConfig represents function calling configuration
type FunctionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"` // "AUTO", "ANY", "NONE"
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

// GenerationConfig represents generation configuration
type GenerationConfig struct {
	StopSequences      []string        `json:"stopSequences,omitempty"`
	CandidateCount     int             `json:"candidateCount,omitempty"`
	MaxOutputTokens    int             `json:"maxOutputTokens,omitempty"`
	Temperature        float64         `json:"temperature,omitempty"`
	TopP               float64         `json:"topP,omitempty"`
	TopK               int             `json:"topK,omitempty"`
	ThinkingConfig     *ThinkingConfig `json:"thinkingConfig,omitempty"`
	ResponseModalities []string        `json:"responseModalities,omitempty"`
}

type ThinkingConfig struct {
	IncludeThoughts bool `json:"includeThoughts,omitempty"`
	ThinkingBudget  int  `json:"thinkingBudget,omitempty"`
	ThinkingLevel   int  `json:"thinkingLevel,omitempty"`
}

// GenerateContentResponse represents the response from generating content
type GenerateContentResponse struct {
	Candidates    []Candidate    `json:"candidates"`
	UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`
	ResponseId    string         `json:"responseId,omitempty"`
}

// Candidate represents a generation candidate
type Candidate struct {
	Content           Content            `json:"content"`
	FinishReason      string             `json:"finishReason,omitempty"`
	SafetyRatings     []SafetyRating     `json:"safetyRatings,omitempty"`
	TokenCount        int                `json:"tokenCount,omitempty"`
	Index             int                `json:"index,omitempty"`
	GroundingMetadata *GroundingMetadata `json:"groundingMetadata,omitempty"`
}

// GroundingMetadata is Gemini's side-channel about server-side tool activity
// (google_search / url_context) that produced the response.
type GroundingMetadata struct {
	WebSearchQueries   []string            `json:"webSearchQueries,omitempty"`
	GroundingChunks    []GroundingChunk    `json:"groundingChunks,omitempty"`
	URLContextMetadata *URLContextMetadata `json:"urlContextMetadata,omitempty"`
}

// GroundingChunk is a single source the model leaned on while producing the answer.
type GroundingChunk struct {
	Web *GroundingChunkWeb `json:"web,omitempty"`
}

// GroundingChunkWeb describes a web result chunk.
type GroundingChunkWeb struct {
	URI   string `json:"uri,omitempty"`
	Title string `json:"title,omitempty"`
}

// URLContextMetadata describes which URLs the url_context tool actually retrieved.
type URLContextMetadata struct {
	URLMetadata []URLMetadataEntry `json:"urlMetadata,omitempty"`
}

// URLMetadataEntry is a single URL retrieval record reported by url_context.
type URLMetadataEntry struct {
	RetrievedURL       string `json:"retrievedUrl,omitempty"`
	URLRetrievalStatus string `json:"urlRetrievalStatus,omitempty"`
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
