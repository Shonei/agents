package rag

import "github.com/Shonei/agents/pkg/rag/storage"

// Search embeds the query using the Gemini agent and searches the underlying
// storage, returning the top-k most similar documents.
func (r *RAG) Search(query string, k int) ([]storage.Document, error) {
	if k <= 0 {
		return []storage.Document{}, nil
	}

	vec, err := r.g.Embedding(query)
	if err != nil {
		return nil, err
	}

	return r.s.Search(vec, k)
}
