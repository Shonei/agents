package rag

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Shonei/agents/cmd/rag/index"
	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/utils"
)

type summaryCommand struct {
	configFactory *config.ConfigFactory
	file          string
}

func NewSummaryCommand(c *config.ConfigFactory) *cobra.Command {
	r := &summaryCommand{
		configFactory: c,
	}

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Test the summary strategy on a specific file",
		Run:   r.RunSummary,
	}

	flags := cmd.Flags()
	flags.StringVar(&r.file, "file", "", "Path to the file to summarize")

	return cmd
}

func (r *summaryCommand) RunSummary(cmd *cobra.Command, args []string) {
	r.configFactory.LoadConfig()

	if r.file == "" {
		utils.NewExitError().WithMessage("file path is required").Done()
	}

	content, err := os.ReadFile(r.file)
	if err != nil {
		utils.NewExitError().WithMessage("failed to read file").WithReason(err).Done()
	}

	strategy, err := index.NewSummaryStrategy(r.configFactory)
	if err != nil {
		utils.NewExitError().WithMessage("failed to create summary strategy").WithReason(err).Done()
	}

	chunks, err := strategy.Summarize(r.file, string(content))
	if err != nil {
		utils.NewExitError().WithMessage("failed to summarize content").WithReason(err).Done()
	}

	for i, chunk := range chunks {
		fmt.Printf("--- Chunk %d ---\n%s\n\n", i+1, chunk.Content)
	}
}
