package rag

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk/gemini"
	"github.com/Shonei/agents/pkg/storage"
	"github.com/Shonei/agents/pkg/utils"
	"github.com/spf13/cobra"
)

type searchCommand struct {
	configFactory *config.ConfigFactory
	store         string
	limit         int
}

type searchResult struct {
	Distance float32           `json:"distance"`
	Path     string            `json:"path"`
	Content  string            `json:"content"`
	Meta     map[string]string `json:"meta"`
}

func init() {
	utils.RegisterResource(searchResult{}, []string{"Distance", "Path"})
}

// NewSearchCommand implements the `agents rag search` command.
func NewSearchCommand(c *config.ConfigFactory) *cobra.Command {
	s := &searchCommand{
		configFactory: c,
		limit:         5,
	}

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search a RAG store for relevant documents",
		Run:   s.Run,
	}

	flags := cmd.Flags()
	flags.StringVar(&s.store, "store", "", "Name of the RAG store to search (defaults to current working directory path)")
	flags.IntVar(&s.limit, "limit", 5, "Maximum number of results to return")

	return cmd
}

func (s *searchCommand) Run(cmd *cobra.Command, args []string) {
	s.configFactory.LoadConfig()

	fmt.Print("Search query: ")
	query, err := utils.ReadUserInput()
	if err != nil {
		utils.NewExitError().WithMessage("failed to read search query").WithReason(err).Done()
	}

	if s.store == "" {
		// By default we use the current working directory as store name,
		// which matches the behavior of the index command (store name = abs dir).
		cwd, err := os.Getwd()
		if err != nil {
			utils.NewExitError().WithMessage("failed to get current directory").WithReason(err).Done()
		}

		abs, err := filepath.Abs(cwd)
		if err != nil {
			utils.NewExitError().WithMessage("failed to resolve current directory").WithReason(err).Done()
		}

		s.store = abs
	}

	store := s.configFactory.GetDB()

	geminiKey := s.configFactory.GetGeminiAPIKey()
	g := gemini.NewAgent(
		gemini.WithAPIKey(geminiKey),
		gemini.WithEmbeddingDim(storage.SearchVectorSize),
	)

	vec, err := g.Embedding(query)
	if err != nil {
		utils.NewExitError().WithMessage("failed to create embedding for query").WithReason(err).Done()
	}

	results, err := store.Search(vec, s.store, s.limit)
	if err != nil {
		utils.NewExitError().WithMessage("failed to search store").WithReason(err).Done()
	}

	searchResults := make([]searchResult, len(results))
	for i, r := range results {
		searchResults[i] = searchResult{
			Distance: r.Distance,
			Path:     r.Meta["path"],
			Content:  r.Content,
			Meta:     r.Meta,
		}
	}

	s.configFactory.Print(searchResults)
}
