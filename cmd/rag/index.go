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
	configFactory    *config.ConfigFactory
	dirPath          string
	chunkingStrategy string
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
	flags.StringVar(&r.dirPath, "dir", ".", "Path to the directory to index. Files in a .gitignore file will be ignored.")
	flags.StringVar(&r.chunkingStrategy, "strategy", "none", "Chunking strategy to use (none, heuristic, fixed-size). Default is none.")

	return cmd
}

func (r *indexCommand) RunIndex(cmd *cobra.Command, args []string) {
	r.configFactory.LoadConfig()

	if !filepath.IsAbs(r.dirPath) {
		cwd, err := os.Getwd()
		if err != nil {
			utils.NewExitError().WithMessage("failed to get current directory").WithReason(err).Done()
		}

		r.dirPath = filepath.Join(cwd, r.dirPath)
	}

	storeName := r.dirPath
	store := r.configFactory.GetDB()

	files, err := utils.CollectFiles(r.dirPath, true)
	if err != nil {
		utils.NewExitError().WithMessage("failed to collect files").WithReason(err).Done()
	}

	geminiKey := r.configFactory.GetGeminiAPIKey()

	g := gemini.NewAgent(
		gemini.WithAPIKey(geminiKey),
		gemini.WithEmbeddingDim(storage.SearchVectorSize),
	)

	for _, file := range files {
		fullPath := filepath.Join(r.dirPath, file.Path)
		chunks, err := utils.Chunk(fullPath, utils.ChunkingStrategy(r.chunkingStrategy))
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Failed to chunk file %s: %v\n", file.Path, err)
			continue
		}

		for i, chunk := range chunks {
			fmt.Fprintf(cmd.OutOrStdout(), "Embedded file %s chunk %d into RAG\n", file.Path, i)

			fileMeta := map[string]string{
				"path":  file.Path,
				"size":  fmt.Sprintf("%d", len(chunk)),
				"ext":   filepath.Ext(file.Path),
				"chunk": fmt.Sprintf("%d", i),
			}

			vec, err := g.Embedding(chunk)
			if err != nil {
				utils.NewExitError().WithMessage("failed to create embedding").WithReason(err).Done()
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
