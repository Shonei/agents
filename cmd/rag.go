package cmd

import (
	"github.com/Shonei/agents/cmd/rag"
	"github.com/Shonei/agents/pkg/config"
	"github.com/spf13/cobra"
)

// NewRAG wires RAG-related subcommands under `agents rag`.
func NewRAG(c *config.ConfigFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rag",
		Short: "RAG (Retrieval-Augmented Generation) operations",
	}

	indexCommand := rag.NewIndexCommand(c)
	storesCommand := rag.NewListStoresCommand(c)

	cmd.AddCommand(indexCommand)
	cmd.AddCommand(storesCommand)

	return cmd
}
