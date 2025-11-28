package rag

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/utils"
	"github.com/spf13/cobra"
)

type deleteStoreCommand struct {
	configFactory *config.ConfigFactory
	store         string
}

// NewDeleteStoreCommand implements the `agents rag delete-store` command.
func NewDeleteStoreCommand(c *config.ConfigFactory) *cobra.Command {
	d := &deleteStoreCommand{
		configFactory: c,
	}

	cmd := &cobra.Command{
		Use:     "delete-store",
		Aliases: []string{"ds", "delete"},
		Short:   "Delete a RAG store and all its documents",
		Run:     d.Run,
	}

	flags := cmd.Flags()
	flags.StringVar(&d.store, "store", "", "Name of the RAG store to delete (defaults to current working directory path)")

	return cmd
}

// Run deletes the specified RAG store from the local DB.
func (d *deleteStoreCommand) Run(cmd *cobra.Command, args []string) {
	d.configFactory.LoadConfig()

	if d.store == "" {
		cwd, err := os.Getwd()
		if err != nil {
			utils.NewExitError().WithMessage("failed to get current directory").WithReason(err).Done()
		}

		abs, err := filepath.Abs(cwd)
		if err != nil {
			utils.NewExitError().WithMessage("failed to resolve current directory").WithReason(err).Done()
		}

		d.store = abs
	}

	store := d.configFactory.GetDB()

	if err := store.DeleteStore(d.store); err != nil {
		utils.NewExitError().WithMessage("failed to delete store").WithReason(err).Done()
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deleted RAG store %s\n", d.store)
}
