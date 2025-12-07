package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/audit"
	"github.com/Shonei/agents/pkg/sdk/claude"
	"github.com/Shonei/agents/pkg/sdk/gemini"
	"github.com/Shonei/agents/pkg/sdk/tools"
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

	var aiSDK *sdk.AI
	switch {
	case strings.Contains(strings.ToLower(agent.Model), "claude"):
		aiSDK = sdk.NewAI(claude.NewAgent(
			claude.WithAPIKey(a.configFactory.GetAPIKey(agent)),
			claude.WithModel(agent.Model),
		), audit.NewAudit(a.configFactory.Config.AuditConfig))

	case strings.Contains(strings.ToLower(agent.Model), "gemini"):
		aiSDK = sdk.NewAI(gemini.NewAgent(
			gemini.WithAPIKey(a.configFactory.GetAPIKey(agent)),
			gemini.WithModel(agent.Model),
			gemini.WithThinking(),
		), audit.NewAudit(a.configFactory.Config.AuditConfig))
	default:
		utils.NewExitError().WithMessage(fmt.Sprintf("unsupported model: %s", agent.Model)).Done()

		return
	}

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
				aiSDK.RegisterTool(tool)

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

		aiSDK.SetSystemPrompt(rendered)
	}

	// Send the message to Claude
	response, err := aiSDK.Chat(a.prompt)
	if err != nil {
		utils.NewExitError().WithMessage("failed to engage agent").WithReason(err).Done()
	}

	// Print the response
	fmt.Println(response)
}
