package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Shonei/agents/cmd/tools"
)

// NewTools wires tools-related subcommands under `agents tools`.
func NewTools() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Tool debugging and inspection operations",
	}

	detailsCommand := tools.NewDetailsCommand()

	cmd.AddCommand(detailsCommand)

	return cmd
}

