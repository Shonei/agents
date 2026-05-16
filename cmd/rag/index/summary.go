package index

import (
	"regexp"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/gemini"
)

type SummaryStrategy struct {
	gc *gemini.Agent
}

func NewSummaryStrategy(c *config.ConfigFactory) (*SummaryStrategy, error) {
	geminiKey := c.GetGeminiAPIKey()
	g := gemini.NewAgent(
		gemini.WithAPIKey(geminiKey),
		gemini.WithModel(gemini.ModelGemini31FlashLite),
	)

	return &SummaryStrategy{
		gc: g,
	}, nil
}

func (s *SummaryStrategy) Summarize(content string) ([]string, error) {
	sysPrompt := `You are an expert software engineer specialized in code analysis and documentation.
The provided text is primarily source code. Your task is to generate a structured summary suitable for a RAG (Retrieval-Augmented Generation) system.
This summary will be used to index the code for semantic search, so it is critical that you capture the intent, functionality, and key logic of the code.

Break the content into logical, self-contained sections.
For each section:
1. Identify the relevant block of original code (e.g., a function, a struct definition, or a logical block).
2. Write a detailed summary that explains:
    - What the code does.
    - How it works.
    - Key inputs and outputs.
    - Important implementation details.
3. Wrap each section in a <chunk> tag.

Structure each chunk as follows:
<chunk>
Original Content:
[Insert the relevant excerpt from original text here]

Summary:
[Insert the detailed technical summary here]
</chunk>

Do not include any text outside the <chunk> tags.`

	req := sdk.CreateMessageRequest{
		System: sysPrompt,
		Messages: []sdk.InputMessage{
			sdk.NewTextMessage(sdk.RoleUser, content),
		},
	}

	resp, err := s.gc.CreateMessage(req)
	if err != nil {
		return nil, err
	}

	response := resp.GetTextContent()

	return extractChunks(response), nil
}

func extractChunks(text string) []string {
	re := regexp.MustCompile(`(?s)<chunk>\s*(.*?)\s*</chunk>`)
	matches := re.FindAllStringSubmatch(text, -1)

	var chunks []string
	for _, match := range matches {
		if len(match) > 1 {
			chunks = append(chunks, match[1])
		}
	}

	if len(chunks) == 0 && len(text) > 0 {
		return []string{text}
	}

	return chunks
}
