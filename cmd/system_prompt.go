package cmd

import (
	"fmt"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/tools"
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
	agent := a.configFactory.GetAgent(agentName)

	if agent.IsRouter() {
		a.printRouter(agentName, agent)

		return
	}

	prompt, err := sdk.RenderPrompt(agent.SystemPrompts, tools.Tools())
	if err != nil {
		utils.NewExitError().WithMessage("failed to render prompt").WithReason(err).Done()
	}

	a.print(prompt)
}

// printRouter prints each sub-agent's rendered system prompt under a
// header so the user can see what every route will actually receive. A
// router itself has no system prompt of its own.
func (a *sPrompt) printRouter(name string, agent config.Agent) {
	a.print(fmt.Sprintf("# %s (router)\n", name))

	for _, route := range agent.Routes {
		sub := a.configFactory.GetAgent(route.Agent)

		rendered, err := sdk.RenderPrompt(sub.SystemPrompts, tools.Tools())
		if err != nil {
			utils.NewExitError().WithMessage("failed to render prompt for " + route.Agent).WithReason(err).Done()
		}

		a.print(fmt.Sprintf("\n## %s\n\n%s\n", route.Agent, rendered))
	}
}

func (a *sPrompt) print(prompt string) {
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
