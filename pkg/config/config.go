package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Shonei/agents/pkg/sdk/audit"
	"github.com/Shonei/agents/pkg/storage"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/Shonei/agents/pkg/utils"
)

var (
	defaultConfigPath  = "~/.agents/config.yaml"
	configEnvOverwrite = "AGENTS_CONFIG"
)

func init() {
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		defaultConfigPath = homeDir + "/.agents/config.yaml"
	}

	if envPath := os.Getenv(configEnvOverwrite); envPath != "" {
		defaultConfigPath = envPath
	}
}

type Agent struct {
	Name          string `yaml:"name"`
	SystemPrompts string `yaml:"system_prompt"`
	Model         string `yaml:"model"`
	// we will deal with this later
	// MaxTokens     int      `yaml:"max_tokens"`
	// Temperature   float64  `yaml:"temperature"`
	Tools []ToolCall `yaml:"tools"`
}

type ToolCall struct {
	Name   string            `yaml:"name"`
	Config map[string]string `yaml:"config"`
}

type Config struct {
	ClaudeAPIKey string            `yaml:"claude_api_key"`
	GeminiAPIKey string            `yaml:"gemini_api_key"`
	Agents       map[string]Agent  `yaml:"agents"`
	AuditConfig  audit.AuditConfig `yaml:"audit"`
	DBPath       string            `yaml:"db_path"`
}

func NewConfigFactory() *ConfigFactory {
	return &ConfigFactory{}
}

type ConfigFactory struct {
	configPath   string
	outputFormat string
	db           *storage.Storage
	Config       *Config
}

func (c *ConfigFactory) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&c.configPath, "config", "c", defaultConfigPath, "config file (default is "+defaultConfigPath+")")
	flags.StringVarP(&c.outputFormat, "output", "o", "table", "output format (yaml, json, table)")
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

func (c *ConfigFactory) GetAPIKey(agent Agent) string {
	if strings.Contains(strings.ToLower(agent.Model), "claude") {
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			return key
		}

		return c.Config.ClaudeAPIKey
	}

	if strings.Contains(strings.ToLower(agent.Model), "gemini") {
		return c.GetGeminiAPIKey()
	}

	return ""
}

func (c *ConfigFactory) GetGeminiAPIKey() string {
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key
	}

	return c.Config.GeminiAPIKey
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
		utils.NewExitError().WithMessage("DB not initialized").Done()
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

	return &config, nil
}
