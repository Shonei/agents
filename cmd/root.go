package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Shonei/agents/cmd/config"
)

var rootCmd = &cobra.Command{
	Use:          "agents",
	Short:        "agents CLI",
	Long:         "agents CLI",
	SilenceUsage: true,
}

func Execute() {
	rootFlags := rootCmd.PersistentFlags()

	configFactory := config.NewConfigFactory()
	configFactory.AddFlags(rootFlags)

	add := NewAdd(configFactory)
	list := NewList(configFactory)
	engage := NewEngage(configFactory)
	rag := NewRAG(configFactory)

	rootCmd.AddCommand(add)
	rootCmd.AddCommand(list)
	rootCmd.AddCommand(engage)
	rootCmd.AddCommand(rag)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
