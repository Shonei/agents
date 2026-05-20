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
	planState     *sdk.PlanState
	todoState     *sdk.TodoState
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

	a.planState = sdk.NewPlanState()
	a.todoState = sdk.NewTodoState()

	agentName := args[0]
	agent := a.configFactory.GetAgent(agentName)
	auditLogger := a.createLogger()

	var chatter sdk.Chatter
	if agent.IsRouter() {
		chatter = a.buildRouter(agentName, agent, auditLogger)
	} else {
		chatter = a.buildAgent(agent, auditLogger, false, 0, "")
	}

	response, err := chatter.Chat(a.prompt)
	if err != nil {
		utils.NewExitError().WithMessage("failed to engage agent").WithReason(err).Done()
	}

	fmt.Println(response)
}

// buildAgent constructs a fully-wired *sdk.AI for a plain agent config.
// When silentPrompt is true the system prompt is set without registering
// a new audit user session — used for sub-agents owned by a router so
// the whole router conversation lives in a single audit session.
func (a *engage) buildAgent(agent config.Agent, auditLogger audit.Logger, silentPrompt bool, depth int, parentSessionID string) *sdk.AI {
	if !strings.Contains(strings.ToLower(agent.Model), "gemini") {
		utils.NewExitError().WithMessage(fmt.Sprintf("unsupported model: %s", agent.Model)).Done()
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

	aiAudit := audit.NewAudit(auditLogger)
	aiSDK := sdk.NewAI(gemini.NewAgent(opts...), aiAudit)
	aiSDK.SetHideThinking(a.configFactory.Config.HideThinking)
	aiSDK.SetHideGrounding(a.configFactory.Config.HideGrounding)

	// If this is a sub-agent, set it to quiet mode to suppress terminal output
	if depth > 0 {
		aiSDK.SetQuiet(true)
	}

	for _, toolName := range agent.Tools {
		if toolName.Config == nil {
			toolName.Config = make(map[string]string)
		}

		if tool := findAITool(toolName.Name); tool != nil {
			tool.Init(toolName.Config, a.configFactory)

			if awareTool, ok := tool.(sdk.AuditAwareTool); ok {
				awareTool.SetAudit(aiAudit)
			}
			if awareTool, ok := tool.(sdk.PlanAwareTool); ok {
				awareTool.SetPlanState(a.planState)
			}
			if awareTool, ok := tool.(sdk.TodoAwareTool); ok {
				awareTool.SetTodoState(a.todoState)
			}

			aiSDK.RegisterTool(tool)

			continue
		}

		if st := findServerTool(toolName.Name); st != nil {
			st.Init(toolName.Config, a.configFactory)
			aiSDK.RegisterServerTool(st)

			continue
		}

		// Check if the tool is another agent
		if subAgentConfig := a.configFactory.GetAgent(toolName.Name); subAgentConfig.Model != "" {
			if depth >= 3 {
				fmt.Printf("Warning: Skipping agent tool '%s' to prevent excessive recursion (depth >= 3)\n", toolName.Name)

				continue
			}

			// Create a new logger for the sub-agent so it gets its own session
			subLogger := a.createLogger()
			subAgent := a.buildAgent(subAgentConfig, subLogger, false, depth+1, aiAudit.SessionID())

			agentTool := &agentTool{
				agent:       subAgent,
				name:        toolName.Name,
				description: subAgentConfig.Description,
			}

			if agentTool.description == "" {
				agentTool.description = fmt.Sprintf("An AI agent specialized in %s", toolName.Name)
			}

			aiSDK.RegisterTool(agentTool)

			continue
		}

		utils.NewExitError().WithMessage(fmt.Sprintf("unsupported tool: %s", toolName.Name)).Done()
	}

	if agent.SystemPrompts != "" {
		rendered, err := sdk.RenderPrompt(agent.SystemPrompts, aiSDK.Tools())
		if err != nil {
			utils.NewExitError().WithMessage("failed to render prompt").WithReason(err).Done()
		}

		if silentPrompt {
			aiSDK.SetSystemPromptSilent(rendered)
		} else {
			aiAudit.User(rendered, parentSessionID)
			aiSDK.SetSystemPromptSilent(rendered)
		}
	}

	return aiSDK
}

// buildRouter constructs a *sdk.RouterAI for a router agent config.
// Sub-agents are built with silent prompts so all events land in the
// single audit session registered here at the router level.
func (a *engage) buildRouter(name string, agent config.Agent, auditLogger audit.Logger) *sdk.RouterAI {
	routes := make(map[string]*sdk.AI, len(agent.Routes))

	for _, route := range agent.Routes {
		sub := a.configFactory.GetAgent(route.Agent)
		routes[route.Agent] = a.buildAgent(sub, auditLogger, true, 0, "")
	}

	meta := make([]sdk.RouteMeta, 0, len(agent.Routes))
	for _, route := range agent.Routes {
		meta = append(meta, sdk.RouteMeta{Agent: route.Agent, When: route.When})
	}

	classifierAgent := gemini.NewAgent(
		gemini.WithAPIKey(a.configFactory.GetGeminiAPIKey()),
		gemini.WithModel(agent.Classifier.Model),
	)

	routerAudit := audit.NewAudit(auditLogger)
	routerAudit.User(sdk.SynthesizeRouterPrompt(name, meta, routes, agent.Classifier.Model), "")

	return sdk.NewRouterAI(
		name,
		routes,
		meta,
		classifierAgent,
		agent.Classifier.DefaultRoute,
		agent.Classifier.ConfidenceThreshold,
		routerAudit,
	)
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
