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
		gemini.WithModel(gemini.ModelGemini25FlashLite),
	)

	return &SummaryStrategy{
		gc: g,
	}, nil
}

func (s *SummaryStrategy) Summarize(content string) ([]string, error) {
	sysPrompt := `You are an expert at summarizing technical content. 
Please analyze the provided text and generate a structured summary.
Break the summary into logical, self-contained sections.

Wrap each section in a <chunk> tag.
Example:
<chunk>
First key point or section summary...
</chunk>
<chunk>
Second key point or section summary...
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
