package cmd

import (
	"fmt"

	"github.com/Shonei/agents/pkg/sdk/claude"
	"github.com/Shonei/agents/pkg/sdk/gemini"
	"github.com/Shonei/agents/pkg/sdk/tools"
	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/audit"
	"github.com/Shonei/agents/pkg/utils"
)

type engage struct {
	configFactory *config.ConfigFactory
	prompt        string
}

func Models() map[string]func(config.Agent, string, *audit.Audit) *sdk.AI {
	return map[string]func(config.Agent, string, *audit.Audit) *sdk.AI{
		claude.ModelClaude45: func(agent config.Agent, apiKey string, audit *audit.Audit) *sdk.AI {
			return sdk.NewAI(claude.NewAgent(
				claude.WithAPIKey(apiKey),
				claude.WithModel(claude.ModelClaude45),
			), audit)
		},
		gemini.ModelGemini3: func(agent config.Agent, apiKey string, audit *audit.Audit) *sdk.AI {
			return sdk.NewAI(gemini.NewAgent(
				gemini.WithAPIKey(apiKey),
				gemini.WithModel(gemini.ModelGemini3),
				gemini.WithThinking(),
			), audit)
		},
	}
}

func NewEngage(c *config.ConfigFactory) *cobra.Command {
	a := &engage{
		configFactory: c,
	}

	cmd := &cobra.Command{
		Use:   "engage [agent_name]",
		Short: "Command to engage the agent and get the results",
		Run:   a.Run,
		Args:  cobra.ExactArgs(1),
	}

	flags := cmd.Flags()
	flags.StringVar(&a.prompt, "prompt", "", "The prompt to send to the agent")

	return cmd
}

func (a *engage) Run(cmd *cobra.Command, args []string) {
	a.configFactory.LoadConfig()
	agentName := args[0]

	// Get the agent configuration by name
	agent := a.configFactory.GetAgent(agentName)

	// Resolve the model factory
	modelFactory, ok := Models()[agent.Model]
	if !ok {
		utils.NewExitError().WithMessage(fmt.Sprintf("unsupported model: %s", agent.Model)).Done()
	}

	apiKey := a.configFactory.GetAPIKey(agent)

	// Initialize the agent
	ai := modelFactory(agent, apiKey, audit.NewAudit(a.configFactory.Config.AuditConfig))

	// Register tools
	for _, toolName := range agent.Tools {
		found := false

		for _, tool := range tools.Tools() {
			if tool.Name() == toolName.Name {
				found = true

				if toolName.Config == nil {
					toolName.Config = make(map[string]string)
				}

				tool.Init(toolName.Config, a.configFactory)
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
