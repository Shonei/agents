package cmd

import (
	"github.com/Shonei/agents/cmd/rag"
	"github.com/spf13/cobra"
)

// NewRAG wires RAG-related subcommands under `agents rag`.
func NewRAG() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rag",
		Short: "RAG (Retrieval-Augmented Generation) operations",
	}

	indexCommand := rag.NewIndexCommand()

	cmd.AddCommand(indexCommand)

	return cmd
}
