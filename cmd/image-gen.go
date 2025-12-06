package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/gemini"
	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/utils"
)

type imageGenCmd struct {
	configFactory *config.ConfigFactory
}

func NewImageGen(c *config.ConfigFactory) *cobra.Command {
	a := &imageGenCmd{
		configFactory: c,
	}

	cmd := &cobra.Command{
		Use:   "image-gen",
		Short: "Command to experiment with image generation using a pre-configured agent.",
		Run:   a.Run,
		Args:  cobra.ExactArgs(0),
	}

	return cmd
}

func (a *imageGenCmd) Run(cmd *cobra.Command, args []string) {
	a.configFactory.LoadConfig()

	geminiKey := a.configFactory.GetGeminiAPIKey()
	g := gemini.NewAgent(
		gemini.WithAPIKey(geminiKey),
		gemini.WithModel(gemini.ModelGeminiImageGen),
		gemini.WithImageGen(),
	)

	fmt.Print("\n> ")
	input, err := utils.ReadUserInput()
	if err != nil {
		utils.NewExitError().WithMessage("error reading input").WithReason(err).Done()
	}

	response, err := g.CreateMessage(sdk.CreateMessageRequest{
		Messages: []sdk.InputMessage{
			sdk.NewTextMessage(sdk.RoleUser, input),
		},
	})
	if err != nil {
		utils.NewExitError().WithMessage("failed to generate image").WithReason(err).Done()
	}

	for _, block := range response.Content {
		switch block.Type {
		case sdk.ContentTypeImage:
			cwd, err := os.Getwd()
			if err != nil {
				utils.NewExitError().WithMessage("failed to get current directory").WithReason(err).Done()
			}

			imageBytes, err := base64.StdEncoding.DecodeString(block.Blob.Data)
			if err != nil {
				utils.NewExitError().WithMessage("failed to decode image").WithReason(err).Done()
			}

			randSuffix := utils.RandomString(5)

			imageFormat := "png"
			contentTypeParts := strings.Split(block.Blob.MimeType, "/")
			if len(contentTypeParts) == 2 {
				imageFormat = contentTypeParts[1]
			}

			// Write the image to a file
			err = os.WriteFile(filepath.Join(cwd, fmt.Sprintf("image_%s.%s", randSuffix, imageFormat)), imageBytes, 0o600)
			if err != nil {
				utils.NewExitError().WithMessage("failed to write image to file").WithReason(err).Done()
			}
		case sdk.ContentTypeText:
			color.New(color.FgBlue, color.Bold).Print("Assistant: ")

			// Render markdown
			out, err := glamour.Render(block.Text, "dark")
			if err != nil {
				// Fallback to plain text if rendering fails
				fmt.Println(block.Text)
			} else {
				fmt.Print(out)
			}
		}
	}

	fmt.Printf("Usage: %d input tokens, %d output tokens\n", response.Usage.InputTokens, response.Usage.OutputTokens)
}
