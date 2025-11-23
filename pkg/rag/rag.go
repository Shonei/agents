package rag

import (
	"fmt"

	"github.com/Shonei/agents/pkg/rag/storage"
	"github.com/Shonei/agents/pkg/sdk/gemini"
)

type RAG struct {
	g *gemini.Agent
	s *storage.Storage
}

func NewRAG(geminiAgent *gemini.Agent, storage *storage.Storage) *RAG {
	return &RAG{
		g: geminiAgent,
		s: storage,
	}
}

// AddContent takes raw content, creates an embedding using the Gemini agent,
// and stores the resulting vector plus content in the underlying storage.
func (r *RAG) AddContent(content string, meta map[string]string) error {
	vec, err := r.g.Embedding(content)
	if err != nil {
		return fmt.Errorf("failed to create embedding: %w", err)
	}

	doc := &storage.Document{
		Vec:     vec,
		Content: content,
		Meta:    meta,
	}

	if err := r.s.AddDocument(doc); err != nil {
		return fmt.Errorf("failed to store document: %w", err)
	}

	return nil
}
