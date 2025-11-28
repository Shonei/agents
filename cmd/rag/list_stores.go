package rag

import (
	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/storage"
	"github.com/Shonei/agents/pkg/utils"
	"github.com/spf13/cobra"
)

// init registers the Store resource for pretty printing.
func init() {
	utils.RegisterResource(storage.Store{}, []string{"Name", "DocumentCount"})
}

type listStoresCommand struct {
	configFactory *config.ConfigFactory
}

// NewListStoresCommand implements the `agents rag stores` command.
func NewListStoresCommand(c *config.ConfigFactory) *cobra.Command {
	l := &listStoresCommand{
		configFactory: c,
	}

	cmd := &cobra.Command{
		Use:   "stores",
		Short: "List RAG stores in the local DB",
		Run:   l.Run,
	}

	return cmd
}

// Run lists all RAG stores and prints them using the configured output format.
func (l *listStoresCommand) Run(cmd *cobra.Command, args []string) {
	l.configFactory.LoadConfig()

	store := l.configFactory.GetDB()
	stores, err := store.ListStores()
	if err != nil {
		utils.NewExitError().WithMessage("failed to list stores").WithReason(err).Done()
	}

	l.configFactory.Print(stores)
}
