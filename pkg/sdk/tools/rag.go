package tools

import (
	"fmt"
	"strings"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/rag"
)

// RagTool is a tool for searching a RAG store
type RagTool struct {
	DBPath       string `yaml:"db_path"`
	EmbeddingDim int    `yaml:"embedding_dim"`
	rag          *rag.RAG
}

func (r *RagTool) Name() string {
	return "rag"
}

func (r *RagTool) Description() string {
	return "The RAG tool allows you to search the local code base for relevant information. It will return the most relevant files and their paths and optionally the file content."
}

func (r *RagTool) Init(config map[string]string, _ *config.ConfigFactory) {
	// Initialize configuration from the provided map. These keys are expected
	// to be wired from the agent's YAML/tool config, e.g.:
	//
	//   tools:
	//     - name: rag
	//       config:
	//         db_path: agents.db
	//         embedding_dim: "2048"
	//
	// If values are missing we fall back to sensible defaults.

	// Default DB path used by the rag CLI command.
	// dbPath := config["db_path"]
	// if dbPath == "" {
	// 	utils.NewExitError().WithMessage("db_path is required for RAG tool").Done()
	// }
	//
	// dimStr, ok := config["embedding_dim"]
	// if !ok || dimStr == "" {
	// 	utils.NewExitError().WithMessage("embedding_dim is required for RAG tool").Done()
	// }
	//
	// embeddingDim, err := strconv.Atoi(dimStr)
	// if err != nil || embeddingDim <= 0 {
	// 	utils.NewExitError().WithMessage("embedding_dim must be a positive integer").Done()
	// }
	//
	// apiKey := config["gemini_api_key"]
	// if apiKey == "" {
	// 	utils.NewExitError().WithMessage("gemini_api_key is required for RAG tool").Done()
	// }
	//
	// g := gemini.NewAgent(
	// 	gemini.WithAPIKey(apiKey),
	// 	gemini.WithEmbeddingDim(embeddingDim),
	// )
	//
	// if _, err := os.Stat(dbPath); err != nil {
	// 	utils.NewExitError().WithMessage("failed to stat RAG DB").WithReason(err).Done()
	// }
	//
	// store, err := storage.NewRAG(dbPath, embeddingDim)
	// if err != nil {
	// 	utils.NewExitError().WithMessage("failed to initialize RAG storage").WithReason(err).Done()
	// }
	//
	// r.DBPath = dbPath
	// r.EmbeddingDim = embeddingDim
	// r.rag = rag.NewRAG(g, store)
}

func (r *RagTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"search_query": map[string]interface{}{
				"type":        "string",
				"description": "The search query in natural language.",
				"example":     "Where is the implementation of the function foo?",
			},
			"include_content": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to include the file content in the response. Defaults to false.",
			},
		},
		"required": []interface{}{"search_query"},
	}
}

type RagToolInput struct {
	SearchQuery    string `json:"search_query"`
	IncludeContent bool   `json:"include_content"`
}

func (r *RagTool) Call(input map[string]interface{}) (interface{}, error) {
	var in RagToolInput
	if err := mapstruct(input, &in); err != nil {
		return "", err
	}
	if strings.TrimSpace(in.SearchQuery) == "" {
		return "", fmt.Errorf("search_query is required")
	}

	// Default to top-5 results, mirroring cmd/rag.go.
	const topK = 5
	results, err := r.rag.Search(in.SearchQuery, topK)
	if err != nil {
		return "", fmt.Errorf("failed to search RAG store: %w", err)
	}

	if len(results) == 0 {
		return "No results found", nil
	}

	// Format a human-friendly response for the model.
	var b strings.Builder
	for i, doc := range results {
		path := doc.Meta["path"]
		fmt.Fprintf(&b, "Result %d:\n", i+1)
		if path != "" {
			fmt.Fprintf(&b, "Path: %s\n", path)
		}
		if in.IncludeContent {
			b.WriteString("---\n")
			b.WriteString(doc.Content)
			b.WriteString("\n")
		}
		b.WriteString("---\n")
	}

	return b.String(), nil
}
