package cmd

import (
	"github.com/Shonei/agents/cmd/config"
	"github.com/Shonei/agents/pkg/utils"
	"github.com/spf13/cobra"
)

func init() {
	utils.RegisterResource(config.Agent{}, []string{"Name", "Model", "Tools.Name"})
}

type list struct {
	configFactory *config.ConfigFactory
}

func NewList(c *config.ConfigFactory) *cobra.Command {
	a := &list{
		configFactory: c,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists all configured agents in the config file.",
		Run:   a.Run,
	}

	return cmd
}

func (l *list) Run(cmd *cobra.Command, args []string) {
	l.configFactory.LoadConfig()

	agents := make([]config.Agent, 0, len(l.configFactory.Config.Agents))
	for _, a := range l.configFactory.Config.Agents {
		agents = append(agents, a)
	}

	l.configFactory.Print(agents)
}
