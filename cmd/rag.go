package cmd

import (
	"github.com/Shonei/agents/cmd/rag"
	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/config"
)

// Default embedding dimension for gemini-embedding-001.
// This should match the OutputDimension used when creating embeddings.
const geminiEmbeddingDim = 2048

type ragCommand struct {
	configFactory *config.ConfigFactory
	filePath      string
	dbPath        string
}

// NewRAG wires RAG-related subcommands under `agents rag`.
func NewRAG(c *config.ConfigFactory) *cobra.Command {
	r := &ragCommand{
		configFactory: c,
	}

	cmd := &cobra.Command{
		Use:   "rag",
		Short: "RAG (Retrieval-Augmented Generation) operations",
	}

	indexCommand := rag.NewIndexCommand()

	cmd.AddCommand(indexCommand)

	return cmd
}
