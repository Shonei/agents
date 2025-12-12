package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/config"
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
	imageGen := NewImageGen(configFactory)

	rag := NewRAG(configFactory)
	prompt := NewSystemPrompt(configFactory)
	tools := NewTools(configFactory)

	rootCmd.AddCommand(add)
	rootCmd.AddCommand(list)
	rootCmd.AddCommand(engage)
	rootCmd.AddCommand(imageGen)
	rootCmd.AddCommand(rag)
	rootCmd.AddCommand(prompt)
	rootCmd.AddCommand(tools)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
