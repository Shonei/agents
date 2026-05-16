package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/audit"
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

func (a *engage) createLogger() audit.Logger {
	c := a.configFactory.Config.AuditConfig

	if !c.Enabled {
		return audit.NewNoopLogger()
	}

	switch c.AuditType {
	case audit.AuditTypeDatabase:
		store := a.configFactory.GetDB()

		return audit.NewDBLogger(store)
	case audit.AuditTypeFile:
		logger, err := audit.NewFileLogger(c.AuditPath)
		if err != nil {
			utils.NewExitError().WithMessage("failed to create file logger").WithReason(err).Done()
		}

		return logger
	default:
		utils.NewExitError().WithMessage("unsupported audit type: " + c.AuditType).Done()

		return audit.NewNoopLogger()
	}
}

func (a *engage) Run(cmd *cobra.Command, args []string) {
	a.configFactory.LoadConfig()
	agentName := args[0]

	// Get the agent configuration by name
	agent := a.configFactory.GetAgent(agentName)

	auditLogger := a.createLogger()

	if !strings.Contains(strings.ToLower(agent.Model), "gemini") {
		utils.NewExitError().WithMessage(fmt.Sprintf("unsupported model: %s", agent.Model)).Done()

		return
	}

	opts := []gemini.AgentOption{
		gemini.WithAPIKey(a.configFactory.GetGeminiAPIKey()),
		gemini.WithModel(agent.Model),
	}

	if agent.ThinkingEnabled {
		opts = append(opts, gemini.WithThinking())
	}

	if agent.MaxTokens != nil {
		opts = append(opts, gemini.WithMaxTokens(*agent.MaxTokens))
	}

	if agent.MaxContextTokens != nil {
		opts = append(opts, gemini.WithMaxContextTokens(*agent.MaxContextTokens))
	}

	if agent.Temperature != nil {
		opts = append(opts, gemini.WithTemperature(*agent.Temperature))
	}

	if agent.ResponseModalities != nil {
		opts = append(opts, gemini.WithResponseModalities(agent.ResponseModalities))
	}

	aiSDK := sdk.NewAI(gemini.NewAgent(opts...), audit.NewAudit(auditLogger))
	aiSDK.SetHideThinking(a.configFactory.Config.HideThinking)
	aiSDK.SetHideGrounding(a.configFactory.Config.HideGrounding)

	// Register tools. A YAML tool entry may resolve to either a local AITool
	// or a provider-executed ServerSideTool (e.g. google_search, url_context).
	for _, toolName := range agent.Tools {
		if toolName.Config == nil {
			toolName.Config = make(map[string]string)
		}

		if tool := findAITool(toolName.Name); tool != nil {
			tool.Init(toolName.Config, a.configFactory)
			aiSDK.RegisterTool(tool)

			continue
		}

		if st := findServerTool(toolName.Name); st != nil {
			st.Init(toolName.Config, a.configFactory)
			aiSDK.RegisterServerTool(st)

			continue
		}

		utils.NewExitError().WithMessage(fmt.Sprintf("unsupported tool: %s", toolName.Name)).Done()
	}

	// Add system prompt if configured
	if agent.SystemPrompts != "" {
		rendered, err := sdk.RenderPrompt(agent.SystemPrompts)
		if err != nil {
			utils.NewExitError().WithMessage("failed to render prompt").WithReason(err).Done()
		}

		aiSDK.SetSystemPrompt(rendered)
	}

	response, err := aiSDK.Chat(a.prompt)
	if err != nil {
		utils.NewExitError().WithMessage("failed to engage agent").WithReason(err).Done()
	}

	// Print the response
	fmt.Println(response)
}

func findAITool(name string) sdk.AITool {
	for _, tool := range tools.Tools() {
		if tool.Name() == name {
			return tool
		}
	}

	return nil
}

func findServerTool(name string) sdk.ServerSideTool {
	for _, st := range tools.ServerTools() {
		if st.Name() == name {
			return st
		}
	}

	return nil
}
