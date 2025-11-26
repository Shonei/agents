package cmd

import (
	"fmt"

	"github.com/Shonei/agents/pkg/sdk"
	"github.com/spf13/cobra"

	"github.com/Shonei/agents/cmd/config"
	"github.com/Shonei/agents/pkg/utils"
)

type engage struct {
	configFactory *config.ConfigFactory
	prompt        string
}

func NewEngage(c *config.ConfigFactory) *cobra.Command {
	a := &engage{
		configFactory: c,
	}

	cmd := &cobra.Command{
		Use:   "engage [agent_name] --prompt <prompt>",
		Short: "Command to engage the agent and get the results",
		Run:   a.Run,
		Args:  cobra.ExactArgs(1),
	}

	flags := cmd.Flags()
	flags.StringVar(&a.prompt, "prompt", "", "The prompt to send to the agent")

	_ = cmd.MarkFlagRequired("prompt")

	return cmd
}

func (a *engage) Run(cmd *cobra.Command, args []string) {
	a.configFactory.LoadConfig()

	// Validate that a prompt was provided
	if len(args) == 0 {
		utils.NewExitError().WithMessage("prompt is required").Done()
	}

	agentName := args[0]

	// Get the agent configuration by name
	agent := a.configFactory.GetAgent(agentName)

	// Resolve the model factory
	modelFactory, ok := Models()[agent.Model]
	if !ok {
		utils.NewExitError().WithMessage(fmt.Sprintf("unsupported model: %s", agent.Model)).Done()
	}

	apiKey := a.configFactory.GetAPIKey(agent)
	geminiKey := a.configFactory.GetGeminiAPIKey()

	// Initialize the agent
	ai := modelFactory(agent, apiKey)

	// Register tools
	for _, toolName := range agent.Tools {
		found := false

		for _, tool := range Tools() {
			if tool.Name() == toolName.Name {
				found = true

				if toolName.Config == nil {
					toolName.Config = make(map[string]string)
				}
				toolName.Config["gemini_api_key"] = geminiKey

				tool.Init(toolName.Config)
				ai.RegisterTool(tool)

				break
			}
		}

		if !found {
			// if we can't find a tool error
			utils.NewExitError().WithMessage(fmt.Sprintf("unsupported tool: %s", toolName.Name)).Done()
		}
	}

	// Add system prompt if configured
	if agent.SystemPrompts != "" {
		rendered, err := sdk.RenderPrompt(agent.SystemPrompts)
		if err != nil {
			utils.NewExitError().WithMessage("failed to render prompt").WithReason(err).Done()
		}

		ai.SetSystemPrompt(rendered)
	}

	// Send the message to Claude
	response, err := ai.Chat(a.prompt)
	if err != nil {
		utils.NewExitError().WithMessage("failed to engage agent").WithReason(err).Done()
	}

	// Print the response
	fmt.Println(response)
}
