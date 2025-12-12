package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// ChunkingStrategy defines the strategy used for chunking files.
type ChunkingStrategy string

const (
	// ChunkingStrategyNone reads the entire file as a single chunk.
	ChunkingStrategyNone ChunkingStrategy = "none"
	// ChunkingStrategyHeuristic splits the file based on heuristics (e.g. Go code structure or paragraphs).
	ChunkingStrategyHeuristic ChunkingStrategy = "heuristic"
	// ChunkingStrategyFixedSize splits the file into fixed-size chunks with overlap.
	ChunkingStrategyFixedSize ChunkingStrategy = "fixed-size"
)

const (
	DefaultChunkSize    = 2000
	DefaultChunkOverlap = 200
)

// Chunk reads a file and splits it into logical chunks based on the provided strategy.
// It returns the list of chunks or an error if the file cannot be read.
func Chunk(path string, strategy ChunkingStrategy) ([]string, error) {
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(contentBytes)

	if strategy == ChunkingStrategyNone {
		return []string{content}, nil
	}

	if strategy == ChunkingStrategyFixedSize {
		return chunkFixedSize(content, DefaultChunkSize, DefaultChunkOverlap), nil
	}

	ext := filepath.Ext(path)

	if ext == ".go" {
		return chunkGoCode(content), nil
	}

	// Default fallback: Split by double newline to approximate paragraphs/blocks
	return chunkDefault(content), nil
}

func chunkGoCode(content string) []string {
	lines := strings.Split(content, "\n")
	var chunks []string
	var currentChunk []string

	for _, line := range lines {
		// Check for top-level declarations (simple heuristic: starts at column 0)
		isTopLevel := false
		if len(line) > 0 && (line[0] >= 'a' && line[0] <= 'z') {
			if strings.HasPrefix(line, "func ") ||
				strings.HasPrefix(line, "type ") ||
				strings.HasPrefix(line, "const ") ||
				strings.HasPrefix(line, "var ") {
				isTopLevel = true
			}
		}

		if isTopLevel && len(currentChunk) > 0 {
			// Find how many lines at the end of currentChunk are comments/empty
			// and belong to this new block
			splitIndex := len(currentChunk)
			for i := len(currentChunk) - 1; i >= 0; i-- {
				l := strings.TrimSpace(currentChunk[i])
				if l == "" || strings.HasPrefix(l, "//") {
					splitIndex = i
				} else {
					break
				}
			}

			// Everything before splitIndex is the finished chunk
			if splitIndex > 0 {
				chunks = append(chunks, strings.Join(currentChunk[:splitIndex], "\n"))
			}

			// Everything from splitIndex is the start of the new chunk (comments)
			newChunkBase := currentChunk[splitIndex:]
			currentChunk = make([]string, len(newChunkBase))
			copy(currentChunk, newChunkBase)
		}

		currentChunk = append(currentChunk, line)
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, strings.Join(currentChunk, "\n"))
	}

	// Filter out empty chunks
	var finalChunks []string
	for _, c := range chunks {
		if strings.TrimSpace(c) != "" {
			finalChunks = append(finalChunks, c)
		}
	}

	return finalChunks
}

func chunkDefault(content string) []string {
	// Simple double-newline splitter, filtering empty results
	raw := strings.Split(content, "\n\n")
	var chunks []string
	for _, c := range raw {
		if strings.TrimSpace(c) != "" {
			chunks = append(chunks, c)
		}
	}

	return chunks
}

func chunkFixedSize(content string, size, overlap int) []string {
	if len(content) == 0 {
		return []string{}
	}
	if len(content) <= size {
		return []string{content}
	}

	var chunks []string
	start := 0
	for start < len(content) {
		end := start + size
		if end > len(content) {
			end = len(content)
		}
		chunks = append(chunks, content[start:end])

		if end == len(content) {
			break
		}

		start += size - overlap
	}
	return chunks
}
