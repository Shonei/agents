package cmd

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/gemini"
	"github.com/Shonei/agents/pkg/sdk/tools"
	"github.com/Shonei/agents/pkg/utils"
)

type add struct {
	configFactory *config.ConfigFactory
	name          string
	systemPrompts string
	model         string
	tools         []string
}

var modelNames = []string{
	gemini.ModelGemini31ProPreview,
	gemini.ModelGemini31FlashLite,
	gemini.ModelGemini31FlashImagePreview,
}

func NewAdd(c *config.ConfigFactory) *cobra.Command {
	a := &add{
		configFactory: c,
	}

	cmd := &cobra.Command{
		Use:   "add --name <name> --system-prompt <system-prompt> --model <model> --tools <tools>",
		Short: "Add a new agent to your config",
		Run:   a.Run,
	}

	spb := &sdk.SystemPromptBuilder{}
	promptFunctions := spb.GetAvailableFunctions()

	flags := cmd.Flags()
	flags.StringVar(&a.name, "name", "", "Name of the agent")
	flags.StringVar(&a.systemPrompts, "system-prompt", "", "System prompts for the agent. You can use go template syntax to access functions like {{ .Cwd }}. Available functions: ["+strings.Join(promptFunctions, ", ")+"]")
	flags.StringVar(&a.model, "model", "", "Model to use for the agent ["+strings.Join(modelNames, " ")+"]")
	flags.StringSliceVar(&a.tools, "tools", []string{}, "Tools to use for the agent ["+strings.Join(tools.ToolNames(), " ")+"]")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("system-prompt")
	_ = cmd.MarkFlagRequired("model")

	return cmd
}

func (a *add) Run(cmd *cobra.Command, args []string) {
	a.configFactory.LoadConfig()

	if a.name == "" {
		utils.NewExitError().WithMessage("name is required").Done()
	}

	if a.systemPrompts == "" {
		utils.NewExitError().WithMessage("system prompts is required").Done()
	}

	if a.model == "" || !slices.Contains(modelNames, a.model) {
		utils.NewExitError().WithMessage("model is required and must be valid").Done()
	}

	toolCalls := []config.ToolCall{}
	for _, toolName := range a.tools {
		toolCalls = append(toolCalls, config.ToolCall{
			Name: toolName,
		})
	}

	a.configFactory.AddAgent(config.Agent{
		Name:          a.name,
		SystemPrompts: a.systemPrompts,
		Model:         a.model,
		Tools:         toolCalls,
	})
}
