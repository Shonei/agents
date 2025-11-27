package cmd

import (
	"fmt"

	"github.com/Shonei/agents/pkg/sdk"
	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/utils"
)

type sPrompt struct {
	configFactory *config.ConfigFactory
	pretty        bool
}

func NewSystemPrompt(c *config.ConfigFactory) *cobra.Command {
	a := &sPrompt{
		configFactory: c,
	}

	cmd := &cobra.Command{
		Use:   "prompt [agent_name]",
		Short: "A command that just prints out the system prompt for the agent. Good for reviewing the system prompt.",
		Run:   a.Run,
		Args:  cobra.ExactArgs(1),
	}

	flags := cmd.Flags()
	flags.BoolVar(&a.pretty, "pretty", false, "Render a pretty makrdown version of the prompt")

	return cmd
}

func (a *sPrompt) Run(cmd *cobra.Command, args []string) {
	a.configFactory.LoadConfig()

	agentName := args[0]

	// Get the agent configuration by name
	agent := a.configFactory.GetAgent(agentName)

	// Render the prompt
	prompt, err := sdk.RenderPrompt(agent.SystemPrompts)
	if err != nil {
		utils.NewExitError().WithMessage("failed to render prompt").WithReason(err).Done()
	}

	if a.pretty {
		out, err := glamour.Render(prompt, "dark")
		if err != nil {
			utils.NewExitError().WithMessage("failed to render prompt").WithReason(err).Done()
		}

		fmt.Println(out)

		return
	}

	fmt.Println(prompt)
}
