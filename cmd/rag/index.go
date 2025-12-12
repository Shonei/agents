package rag

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

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
	flags.StringVar(&r.strategy, "strategy", "", "Indexing strategy to use. Defaults to node.")

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

	for _, file := range files {
		fileMeta := map[string]string{
			"path": file.Path,
			"size": fmt.Sprintf("%d", len(file.Content)),
			"ext":  filepath.Ext(file.Path),
		}

		vec, err := g.Embedding(file.Content)
		if err != nil {
			utils.NewExitError().WithMessage("failed to create embedding").WithReason(err).Done()
		}

		doc := &storage.Document{
			Content: file.Content,
			Meta:    fileMeta,
			Store:   storeName,
			Vec:     vec,
		}

		if err := store.AddDocument(doc); err != nil {
			utils.NewExitError().WithMessage("failed to store document").WithReason(err).Done()
		}
	}
}
