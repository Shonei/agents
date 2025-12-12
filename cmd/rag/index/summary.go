package index

import (
	"github.com/Shonei/agents/pkg/config"
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

func (s *SummaryStrategy) Summarize(content string) (string, error) {
	return "", nil
}
