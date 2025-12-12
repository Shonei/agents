package tools

import (
	"fmt"
	"time"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/gemini"
	"github.com/Shonei/agents/pkg/storage"
)

type MemoryTool struct {
	store  *storage.Storage
	gemini *gemini.Agent
}

func (m *MemoryTool) Name() string {
	return "memory"
}

func (m *MemoryTool) Description() string {
	return "The memory tool allows you to store and retrieve information from a persistent memory store. Use 'store' to save content and 'retrieve' to find relevant information. You can store any relevant information you want to remember. And you should store information you are asked to store."
}

func (m *MemoryTool) Init(_ map[string]string, c *config.ConfigFactory) {
	m.store = c.GetDB()
	geminiKey := c.GetGeminiAPIKey()
	m.gemini = gemini.NewAgent(
		gemini.WithAPIKey(geminiKey),
		gemini.WithEmbeddingDim(storage.SearchVectorSize),
	)
}

func (m *MemoryTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The command to execute: 'store' or 'retrieve'.",
				"enum":        []string{"store", "retrieve"},
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to store. Required for 'store' command. Only used for 'store' command. This will be embedded and compared against search queries using vector similarity.",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "The query to search for. Required for 'retrieve' command. The query will be embedded and compared against stored documents using vector similarity.",
			},
		},
		"required": []interface{}{"command"},
	}
}

type MemoryToolInput struct {
	Command string `json:"command"`
	Content string `json:"content"`
	Query   string `json:"query"`
}

func (m *MemoryTool) Call(input map[string]interface{}) (interface{}, error) {
	var in MemoryToolInput
	if err := mapstruct(input, &in); err != nil {
		return "", err
	}

	switch in.Command {
	case "store":
		if in.Content == "" {
			return "", sdk.NewAIError("content is required for store command")
		}

		return m.storeContent(in.Content)
	case "retrieve":
		if in.Query == "" {
			return "", sdk.NewAIError("query is required for retrieve command")
		}

		return m.retrieveContent(in.Query)
	}

	return "", sdk.NewAIError(fmt.Sprintf("unknown command: %s", in.Command))
}

func (m *MemoryTool) storeContent(content string) (string, error) {
	vec, err := m.gemini.Embedding(content)
	if err != nil {
		return "", fmt.Errorf("failed to create embedding for content: %w", err)
	}

	doc := &storage.Document{
		Vec:     vec,
		Content: content,
		Store:   "memory",
		Meta: map[string]string{
			"created_at": time.Now().Format(time.RFC3339),
		},
	}

	if err := m.store.AddDocument(doc); err != nil {
		return "", fmt.Errorf("failed to store document: %w", err)
	}

	return "Content stored successfully", nil
}

func (m *MemoryTool) retrieveContent(query string) (interface{}, error) {
	vec, err := m.gemini.Embedding(query)
	if err != nil {
		return "", fmt.Errorf("failed to create embedding for query: %w", err)
	}

	// Limit to 5 results for now
	results, err := m.store.Search(vec, "memory", 5)
	if err != nil {
		return "", fmt.Errorf("failed to search memory: %w", err)
	}

	out := make([]map[string]interface{}, 0, len(results))
	for _, d := range results {
		out = append(out, map[string]interface{}{
			"content":  d.Content,
			"meta":     d.Meta,
			"distance": d.Distance,
		})
	}

	return out, nil
}
