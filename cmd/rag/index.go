package rag

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/Shonei/agents/cmd/rag/index"
	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk/gemini"
	"github.com/Shonei/agents/pkg/storage"
	"github.com/Shonei/agents/pkg/utils"
)

type indexCommand struct {
	configFactory *config.ConfigFactory
	dirPath       string
	file          string
	strategy      string
}

// NewIndexCommand implements the `agents rag index` command.
func NewIndexCommand(c *config.ConfigFactory) *cobra.Command {
	r := &indexCommand{
		configFactory: c,
	}

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index a folder into the local RAG store",
		Run:   r.RunIndex,
	}

	flags := cmd.Flags()

	flags.StringVar(&r.dirPath, "dir", "", "Path to the directory to index. Files in a .gitignore file will be ignored. If both --dir and --file are set an error will be returned.")
	flags.StringVar(&r.file, "file", "", "Path to a specific file to index. If both --dir and --file are set an error will be returned.")
	flags.StringVar(&r.strategy, "strategy", "", "Indexing strategy to use. Defaults to 'none'. Available strategies: 'none', 'summary'.")

	cmd.AddCommand(NewSummaryCommand(c))

	return cmd
}

func (r *indexCommand) RunIndex(cmd *cobra.Command, args []string) {
	r.configFactory.LoadConfig()

	if r.dirPath != "" && r.file != "" {
		utils.NewExitError().WithMessage("both --dir and --file were set. Please choose one or the other.").Done()
	}

	if r.dirPath != "" {
		r.indexDir()

		return
	}

	if r.file != "" {
		r.indexFile()

		return
	}

	// Default to current directory if no path provided
	r.dirPath = "."
	r.indexDir()
}

func (r *indexCommand) indexFile() {
	if !filepath.IsAbs(r.file) {
		cwd, err := os.Getwd()
		if err != nil {
			utils.NewExitError().WithMessage("failed to get current directory").WithReason(err).Done()
		}

		r.file = filepath.Join(cwd, r.file)
	}

	storeName := r.file

	store := r.configFactory.GetDB()
	geminiKey := r.configFactory.GetGeminiAPIKey()
	g := gemini.NewAgent(
		gemini.WithAPIKey(geminiKey),
		gemini.WithEmbeddingDim(storage.SearchVectorSize),
	)

	file, err := os.ReadFile(r.file)
	if err != nil {
		utils.NewExitError().WithMessage("failed to read file").WithReason(err).Done()
	}

	fileMeta := map[string]string{
		"path": r.file,
		"size": fmt.Sprintf("%d", len(file)),
		"ext":  filepath.Ext(r.file),
	}

	vec, err := g.Embedding(string(file))
	if err != nil {
		utils.NewExitError().WithMessage("failed to create embedding").WithReason(err).Done()
	}

	doc := &storage.Document{
		Content: string(file),
		Meta:    fileMeta,
		Store:   storeName,
		Vec:     vec,
	}

	if err := store.AddDocument(doc); err != nil {
		utils.NewExitError().WithMessage("failed to store document").WithReason(err).Done()
	}
}

func (r *indexCommand) indexDir() {
	if !filepath.IsAbs(r.dirPath) {
		cwd, err := os.Getwd()
		if err != nil {
			utils.NewExitError().WithMessage("failed to get current directory").WithReason(err).Done()
		}

		r.dirPath = filepath.Join(cwd, r.dirPath)
	}

	storeName := r.dirPath
	store := r.configFactory.GetDB()

	files, err := utils.CollectFiles(r.dirPath, false)
	if err != nil {
		utils.NewExitError().WithMessage("failed to collect files").WithReason(err).Done()
	}

	geminiKey := r.configFactory.GetGeminiAPIKey()

	g := gemini.NewAgent(
		gemini.WithAPIKey(geminiKey),
		gemini.WithEmbeddingDim(storage.SearchVectorSize),
	)

	strategy, err := r.getStrategy()
	if err != nil {
		utils.NewExitError().WithMessage("failed to get strategy").WithReason(err).Done()
	}

	for _, file := range files {
		// check if file is already indexed

		metaSearch, err := store.MetaSearch(map[string]string{
			"path": file.Path,
		}, storeName)
		if err != nil {
			utils.NewExitError().WithMessage("failed to search meta").WithReason(err).Done()
		}

		if len(metaSearch) > 0 {
			fmt.Printf("File %s already indexed\n", file.Path)

			continue
		}

		chunks, err := strategy(file.Content)
		if err != nil {
			utils.NewExitError().WithMessage("failed to chunk content").WithReason(err).Done()
		}

		fmt.Printf("Indexing file %s\n", file.Path)

		for i, chunk := range chunks {
			vec, err := g.Embedding(chunk)
			if err != nil {
				utils.NewExitError().WithMessage("failed to create embedding").WithReason(err).Done()
			}

			fileMeta := map[string]string{
				"path":         file.Path,
				"indexed_at":   time.Now().Format(time.RFC3339),
				"size":         fmt.Sprintf("%d", len(file.Content)),
				"ext":          filepath.Ext(file.Path),
				"chunk":        fmt.Sprintf("%d", i),
				"total_chunks": fmt.Sprintf("%d", len(chunks)),
				"strategy":     r.strategy,
				"file_content": file.Content,
			}

			doc := &storage.Document{
				Content: chunk,
				Meta:    fileMeta,
				Store:   storeName,
				Vec:     vec,
			}

			if err := store.AddDocument(doc); err != nil {
				utils.NewExitError().WithMessage("failed to store document").WithReason(err).Done()
			}
		}
	}
}

func (r *indexCommand) getStrategy() (func(string) ([]string, error), error) {
	if r.strategy == "" || r.strategy == "none" {
		return func(s string) ([]string, error) {
			return []string{s}, nil
		}, nil
	}

	if r.strategy == "summary" {
		strats, err := index.NewSummaryStrategy(r.configFactory)
		if err != nil {
			return nil, fmt.Errorf("failed to create summary strategy: %w", err)
		}

		return strats.Summarize, nil
	}

	return nil, fmt.Errorf("unknown strategy: %s", r.strategy)
}
