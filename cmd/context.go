package cmd

import (
	"github.com/spf13/cobra"

	contextcmd "github.com/Shonei/agents/cmd/context"
	"github.com/Shonei/agents/pkg/config"
)

// NewContext wires conversation-context subcommands under `agents context`.
func NewContext(c *config.ConfigFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Inspect recorded conversations and the context transforms applied to them",
		Long: "Inspect recorded conversations and replay the two context transforms over them.\n\n" +
			"`compact` continues the same agent with its system prompt intact.\n" +
			"`handoff` briefs a different agent that shares none of the history.\n\n" +
			"Both read from the audit log, so they need audit.type: database and db_path configured.",
	}

	cmd.AddCommand(contextcmd.NewListCommand(c))
	cmd.AddCommand(contextcmd.NewCompactCommand(c))
	cmd.AddCommand(contextcmd.NewHandoffCommand(c))

	return cmd
}
