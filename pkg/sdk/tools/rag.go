package tools

import (
	"fmt"
	"os"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/gemini"
	"github.com/Shonei/agents/pkg/storage"
)

func init() {
	RegisterTools(&RagTool{})
}

// RagTool is a tool for searching a RAG store
type RagTool struct {
	store  *storage.Storage
	gemini *gemini.Agent
}

func (r *RagTool) Name() string {
	return "rag"
}

func (r *RagTool) Description() string {
	return "Searches a RAG store for documents relevant to a query. The query is embedded and compared against stored documents using vector similarity. The more descriptive the query is the better."
}

func (r *RagTool) Init(_ map[string]string, c *config.ConfigFactory) {
	r.store = c.GetDB()

	geminiKey := c.GetGeminiAPIKey()
	r.gemini = gemini.NewAgent(
		gemini.WithAPIKey(geminiKey),
		gemini.WithEmbeddingDim(storage.SearchVectorSize),
	)
}

func (r *RagTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"search_query": map[string]interface{}{
				"type":        "string",
				"description": "The search query in natural language. The query will be embedded and compared against stored documents using vector similarity.",
				"example":     "Where is the implementation of the function foo?",
			},
			"include_content": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to include the file content in the response or just the metadata. Defaults to false. If you only include the metadata you can inspect the files before loading them into the context using other available tools.",
			},
			"result_limit": map[string]interface{}{
				"type":        "integer",
				"description": "The maximum number of results to return. Defaults to 5.",
			},
		},
		"required": []interface{}{"search_query"},
	}
}

type RagToolInput struct {
	SearchQuery    string `json:"search_query"`
	IncludeContent bool   `json:"include_content"`
	ResultLimit    int    `json:"result_limit"`
}

type RagToolResult struct {
	Distance float32           `json:"distance"`
	Path     string            `json:"path"`
	Content  string            `json:"content,omitempty"`
	Meta     map[string]string `json:"meta"`
}

func (r *RagTool) Call(input map[string]interface{}) (interface{}, error) {
	var in RagToolInput
	if err := mapstruct(input, &in); err != nil {
		return "", err
	}

	if in.SearchQuery == "" {
		return "", sdk.NewAIError("search_query is required for rag search")
	}

	limit := in.ResultLimit
	if limit <= 0 {
		limit = 5
	}

	storeName, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	vec, err := r.gemini.Embedding(in.SearchQuery)
	if err != nil {
		return "", fmt.Errorf("failed to create embedding for query: %w", err)
	}

	results, err := r.store.Search(vec, storeName, limit)
	if err != nil {
		return "", fmt.Errorf("failed to search store: %w", err)
	}

	out := make([]RagToolResult, 0, len(results))
	for _, d := range results {
		res := RagToolResult{
			Distance: d.Distance,
			Path:     d.Meta["path"],
			Meta:     d.Meta,
		}

		if in.IncludeContent {
			res.Content = d.Content
		}

		out = append(out, res)
	}

	return out, nil
}
