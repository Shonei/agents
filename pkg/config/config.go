package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/Shonei/agents/pkg/sdk/audit"
	"github.com/Shonei/agents/pkg/storage"
	"github.com/Shonei/agents/pkg/utils"
)

var (
	defaultConfigPath  = "~/agents/config.yaml"
	configEnvOverwrite = "AGENTS_CONFIG"
)

func init() {
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		defaultConfigPath = homeDir + "/agents/config.yaml"
	}

	if envPath := os.Getenv(configEnvOverwrite); envPath != "" {
		defaultConfigPath = envPath
	}
}

type Agent struct {
	Name          string `yaml:"name"`
	Description   string `yaml:"description,omitempty"`
	SystemPrompts string `yaml:"system_prompt"`
	Model         string `yaml:"model"`

	// we will deal with this later
	MaxTokens          *int       `yaml:"max_tokens"`
	MaxContextTokens   *int       `yaml:"max_context_tokens"`
	Temperature        *float64   `yaml:"temperature"`
	ResponseModalities []string   `yaml:"response_modalities"`
	ThinkingEnabled    bool       `yaml:"thinking_enabled"`
	Tools              []ToolCall `yaml:"tools"`

	// Router-only fields. Populated when Kind == AgentKindRouter.
	Kind       string            `yaml:"kind,omitempty"`
	Classifier *ClassifierConfig `yaml:"classifier,omitempty"`
	Routes     []RouteConfig     `yaml:"routes,omitempty"`
}

// Agent kinds. An empty Kind is treated as AgentKindAgent for backward
// compatibility with the original single-agent configs.
const (
	AgentKindAgent  = "agent"
	AgentKindRouter = "router"
)

// ClassifierConfig configures the cheap-ish per-turn classifier that
// powers a router agent. Only the sticky strategy is supported in v1.
type ClassifierConfig struct {
	Model               string  `yaml:"model"`
	Strategy            string  `yaml:"strategy"`
	DefaultRoute        string  `yaml:"default_route"`
	ConfidenceThreshold float64 `yaml:"confidence_threshold"`
}

// RouteConfig declares a single sub-agent route inside a router agent.
// Agent is the name of another agent in the same config; When is a short
// natural-language hint shown to the classifier.
type RouteConfig struct {
	Agent string `yaml:"agent"`
	When  string `yaml:"when"`
}

type ToolCall struct {
	Name   string            `yaml:"name"`
	Config map[string]string `yaml:"config"`
}

// DefaultConfidenceThreshold is the fallback confidence threshold applied
// to a router when the YAML omits it. Picked to bias towards stability:
// the classifier must be reasonably sure before we incur a handoff.
const DefaultConfidenceThreshold = 0.7

// ClassifierStrategySticky is the only strategy supported in v1. The
// router re-classifies every turn but only swaps the active sub-agent
// when the classifier confidently picks a different route.
const ClassifierStrategySticky = "sticky"

// IsRouter reports whether the agent is configured as a router rather
// than a plain LLM-backed agent.
func (a *Agent) IsRouter() bool {
	return a.Kind == AgentKindRouter
}

type Config struct {
	GeminiAPIKey  string            `yaml:"gemini_api_key"`
	GitHubToken   string            `yaml:"github_token"`
	Agents        map[string]Agent  `yaml:"agents"`
	AuditConfig   audit.AuditConfig `yaml:"audit"`
	DBPath        string            `yaml:"db_path"`
	HideThinking  bool              `yaml:"hide_thinking"`
	HideGrounding bool              `yaml:"hide_grounding"`
}

func NewConfigFactory() *ConfigFactory {
	return &ConfigFactory{}
}

type ConfigFactory struct {
	configPath    string
	outputFormat  string
	hideThinking  bool
	hideGrounding bool
	db            *storage.Storage
	Config        *Config
}

func (c *ConfigFactory) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&c.configPath, "config", "c", defaultConfigPath, "config file (default is "+defaultConfigPath+")")
	flags.StringVarP(&c.outputFormat, "output", "o", "table", "output format (yaml, json, table)")
	flags.BoolVar(&c.hideThinking, "hide-thinking", false, "hide thinking blocks from output")
	flags.BoolVar(&c.hideGrounding, "hide-grounding", false, "hide grounding summary (server-side tool sources) from output")
}

func (c *ConfigFactory) LoadConfig() {
	if c.configPath == "" {
		fmt.Fprintln(os.Stderr, "config path is empty")
		os.Exit(1)
	}

	var err error
	c.Config, err = readOrCreateConfig(c.configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("failed to load config: %v", err))
		os.Exit(1)
	}

	if c.hideThinking {
		c.Config.HideThinking = true
	}

	if c.hideGrounding {
		c.Config.HideGrounding = true
	}

	if c.Config.DBPath == "" {
		return
	}

	db, err := storage.NewStorage(c.Config.DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("failed to open DB: %v", err))
		os.Exit(1)
	}

	c.db = db
}

func (c *ConfigFactory) AddAgent(agent Agent) {
	if c.Config.Agents == nil {
		c.Config.Agents = make(map[string]Agent)
	}

	c.Config.Agents[agent.Name] = agent

	c.SaveConfig()
}

func (c *ConfigFactory) GetAgent(name string) Agent {
	agent, ok := c.Config.Agents[name]
	if !ok {
		utils.NewExitError().WithMessage(fmt.Sprintf("agent '%s' not found in config", name)).Done()
	}

	return agent
}

func (c *ConfigFactory) GetGeminiAPIKey() string {
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key
	}

	return c.Config.GeminiAPIKey
}

func (c *ConfigFactory) GetGitHubToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	return c.Config.GitHubToken
}

func (c *ConfigFactory) SaveConfig() {
	b, err := yaml.Marshal(c.Config)
	if err != nil {
		utils.NewExitError().WithMessage("failed to marshal CLI config").WithReason(err).Done()
	}

	err = os.WriteFile(c.configPath, b, 0o600)
	if err != nil {
		utils.NewExitError().WithMessage("unable to persist CLI config").WithReason(err).Done()
	}
}

func (c *ConfigFactory) GetDB() *storage.Storage {
	if c.db == nil {
		utils.NewExitError().WithMessage("DB not initialized, Did you remember to set 'db_path' in your config?").Done()
	}

	return c.db
}

func (c *ConfigFactory) Print(resource any) {
	utils.Print(resource, c.outputFormat)
}

func readOrCreateConfig(configPath string) (*Config, error) {
	config := Config{}

	b, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			createErr := os.MkdirAll(filepath.Dir(configPath), 0o600)
			if createErr != nil {
				return nil, fmt.Errorf("failed to create config directory: %v", createErr)
			}

			createErr = os.WriteFile(configPath, []byte{}, 0o600)
			if createErr != nil {
				return nil, fmt.Errorf("failed to create config file: %v", createErr)
			}

			return &config, nil
		}

		return nil, err
	}

	err = yaml.Unmarshal(b, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	if err := normalizeAndValidateAgents(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// normalizeAndValidateAgents fills in defaults on router agents and
// rejects configurations that would blow up later at runtime (unknown
// routes, missing classifier block, nested routers, etc.). Single-agent
// configs are left untouched.
func normalizeAndValidateAgents(c *Config) error {
	for name, agent := range c.Agents {
		if agent.Kind == "" {
			agent.Kind = AgentKindAgent
		}

		switch agent.Kind {
		case AgentKindAgent:
			// nothing to validate for plain agents beyond what the SDK
			// already enforces when it tries to build them.
		case AgentKindRouter:
			if err := validateRouterAgent(name, &agent, c.Agents); err != nil {
				return err
			}
		default:
			return fmt.Errorf("agent %q: unknown kind %q (expected %q or %q)",
				name, agent.Kind, AgentKindAgent, AgentKindRouter)
		}

		c.Agents[name] = agent
	}

	return nil
}

func validateRouterAgent(name string, agent *Agent, all map[string]Agent) error {
	if agent.Classifier == nil {
		return fmt.Errorf("router %q: missing classifier block", name)
	}

	if agent.Classifier.Model == "" {
		return fmt.Errorf("router %q: classifier.model is required", name)
	}

	if agent.Classifier.Strategy == "" {
		agent.Classifier.Strategy = ClassifierStrategySticky
	}

	if agent.Classifier.Strategy != ClassifierStrategySticky {
		return fmt.Errorf("router %q: classifier.strategy %q is not supported (only %q in v1)",
			name, agent.Classifier.Strategy, ClassifierStrategySticky)
	}

	if agent.Classifier.ConfidenceThreshold == 0 {
		agent.Classifier.ConfidenceThreshold = DefaultConfidenceThreshold
	}

	if agent.Classifier.ConfidenceThreshold < 0 || agent.Classifier.ConfidenceThreshold > 1 {
		return fmt.Errorf("router %q: classifier.confidence_threshold must be in [0,1], got %v",
			name, agent.Classifier.ConfidenceThreshold)
	}

	if len(agent.Routes) < 2 {
		return fmt.Errorf("router %q: at least 2 routes are required, got %d", name, len(agent.Routes))
	}

	seen := make(map[string]struct{}, len(agent.Routes))
	for i, route := range agent.Routes {
		if route.Agent == "" {
			return fmt.Errorf("router %q: routes[%d].agent is required", name, i)
		}

		if route.Agent == name {
			return fmt.Errorf("router %q: route refers back to itself", name)
		}

		target, ok := all[route.Agent]
		if !ok {
			return fmt.Errorf("router %q: routes[%d] points to unknown agent %q", name, i, route.Agent)
		}

		if target.Kind == AgentKindRouter {
			return fmt.Errorf("router %q: routes[%d] points to another router %q (nested routers are not supported)",
				name, i, route.Agent)
		}

		if _, dup := seen[route.Agent]; dup {
			return fmt.Errorf("router %q: route %q is declared more than once", name, route.Agent)
		}

		seen[route.Agent] = struct{}{}
	}

	if _, ok := seen[agent.Classifier.DefaultRoute]; !ok {
		return fmt.Errorf("router %q: classifier.default_route %q is not one of the configured routes",
			name, agent.Classifier.DefaultRoute)
	}

	return nil
}
