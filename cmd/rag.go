package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Shonei/agents/cmd/config"
	ragpkg "github.com/Shonei/agents/pkg/rag"
	"github.com/Shonei/agents/pkg/rag/storage"
	"github.com/Shonei/agents/pkg/sdk/gemini"
	"github.com/Shonei/agents/pkg/utils"
	"github.com/spf13/cobra"
)

// Default embedding dimension for gemini-embedding-001.
// This should match the OutputDimension used when creating embeddings.
const geminiEmbeddingDim = 2048

type ragCommand struct {
	configFactory *config.ConfigFactory
	filePath      string
	dbPath        string
}

// NewRAG wires RAG-related subcommands under `agents rag`.
func NewRAG(c *config.ConfigFactory) *cobra.Command {
	r := &ragCommand{
		configFactory: c,
	}

	cmd := &cobra.Command{
		Use:   "rag",
		Short: "RAG (Retrieval-Augmented Generation) operations",
	}

	embedCmd := &cobra.Command{
		Use:   "embed --folder <path>",
		Short: "Embed a folder into the local RAG store",
		Run:   r.RunEmbed,
	}

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the local RAG store for relevant content",
		Args:  cobra.MinimumNArgs(1),
		Run:   r.RunSearch,
	}

	embedFlags := embedCmd.Flags()
	embedFlags.StringVar(&r.filePath, "folder", "", "Path to the folder to embed")
	embedFlags.StringVar(&r.dbPath, "db", "agents.db", "Path to the DuckDB database file for RAG storage")

	searchFlags := searchCmd.Flags()
	searchFlags.StringVar(&r.dbPath, "db", "agents.db", "Path to the DuckDB database file for RAG storage")

	_ = embedCmd.MarkFlagRequired("file")

	cmd.AddCommand(embedCmd)
	cmd.AddCommand(searchCmd)

	return cmd
}

func (r *ragCommand) RunEmbed(cmd *cobra.Command, args []string) {
	r.configFactory.LoadConfig()

	if r.filePath == "" {
		utils.NewExitError().WithMessage("file is required").Done()
	}

	files, err := utils.CollectFiles(r.filePath, false)
	if err != nil {
		utils.NewExitError().WithMessage("failed to collect files").WithReason(err).Done()
	}

	apiKey := getGeminiAPIKey(r.configFactory)
	if apiKey == "" {
		utils.NewExitError().WithMessage("Gemini API key is required. Set GEMINI_API_KEY or gemini_api_key in config.").Done()
	}

	g := gemini.NewAgent(
		gemini.WithAPIKey(apiKey),
		gemini.WithEmbeddingDim(geminiEmbeddingDim),
	)

	if err := ensureDir(r.dbPath); err != nil {
		utils.NewExitError().WithMessage("failed to prepare RAG storage directory").WithReason(err).Done()
	}

	store, err := storage.NewRAG(r.dbPath, geminiEmbeddingDim)
	if err != nil {
		utils.NewExitError().WithMessage("failed to initialize RAG storage").WithReason(err).Done()
	}

	ragEngine := ragpkg.NewRAG(g, store)

	for _, file := range files {
		fmt.Fprintf(cmd.OutOrStdout(), "Embedded file %s into RAG store %s\n", file.Path, r.dbPath)

		fileMeta := map[string]string{
			"path": file.Path,
			"size": fmt.Sprintf("%d", len(file.Content)),
			"ext":  filepath.Ext(file.Path),
		}

		if err := ragEngine.AddContent(file.Content, fileMeta); err != nil {
			utils.NewExitError().WithMessage("failed to embed content").WithReason(err).Done()
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Embedded file %s into RAG store %s\n", r.filePath, r.dbPath)
}

func (r *ragCommand) RunSearch(cmd *cobra.Command, args []string) {
	r.configFactory.LoadConfig()

	query := args[0]
	if query == "" {
		utils.NewExitError().WithMessage("query is required").Done()
	}

	apiKey := getGeminiAPIKey(r.configFactory)
	if apiKey == "" {
		utils.NewExitError().WithMessage("Gemini API key is required. Set GEMINI_API_KEY or gemini_api_key in config.").Done()
	}

	g := gemini.NewAgent(
		gemini.WithAPIKey(apiKey),
		gemini.WithEmbeddingDim(geminiEmbeddingDim),
	)

	if err := ensureDir(r.dbPath); err != nil {
		utils.NewExitError().WithMessage("failed to prepare RAG storage directory").WithReason(err).Done()
	}

	store, err := storage.NewRAG(r.dbPath, geminiEmbeddingDim)
	if err != nil {
		utils.NewExitError().WithMessage("failed to initialize RAG storage").WithReason(err).Done()
	}

	ragEngine := ragpkg.NewRAG(g, store)

	results, err := ragEngine.Search(query, 5)
	if err != nil {
		utils.NewExitError().WithMessage("failed to search RAG store").WithReason(err).Done()
	}

	if len(results) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No results found")
		return
	}

	for i, doc := range results {
		fmt.Fprintf(cmd.OutOrStdout(), "Result %d:\n", i+1)
		fmt.Fprintln(cmd.OutOrStdout(), "Path: ", doc.Meta["path"])
		fmt.Fprintln(cmd.OutOrStdout(), "---")
	}
}

func getGeminiAPIKey(c *config.ConfigFactory) string {
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key
	}

	if c != nil && c.Config != nil {
		return c.Config.GeminiAPIKey
	}

	return ""
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, os.ModePerm)
}
