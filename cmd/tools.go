package cmd

import (
	"github.com/Shonei/agents/pkg/config"
	"github.com/spf13/cobra"

	"github.com/Shonei/agents/cmd/tools"
)

// NewTools wires tools-related subcommands under `agents tools`.
func NewTools(c *config.ConfigFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Tool debugging and inspection operations",
	}

	detailsCommand := tools.NewDetailsCommand()
	executeCommand := tools.NewExecuteCommand(c)

	cmd.AddCommand(executeCommand)
	cmd.AddCommand(detailsCommand)

	return cmd
}
