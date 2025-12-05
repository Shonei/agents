package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/tools"
	"github.com/Shonei/agents/pkg/utils"
	"github.com/spf13/cobra"
)

type executeCommand struct{}

// NewExecuteCommand implements the `agents tools execute <tool_name> <params>...` command.
func NewExecuteCommand() *cobra.Command {
	e := &executeCommand{}

	cmd := &cobra.Command{
		Use:   "execute <tool_name> [params...]",
		Short: "Execute a tool with the given parameters",
		Long: `Execute a tool with the given parameters.

Parameters are specified as key:value pairs. For nested maps, use dot notation in the key.

Examples:
  agents tools execute fetch_url url:https://example.com
  agents tools execute view_file path:./README.md
  agents tools execute view_file path:./file.go view_range.0:1 view_range.1:10

Available tools: ` + strings.Join(tools.ToolNames(), ", "),
		Run:  e.Run,
		Args: cobra.MinimumNArgs(1),
	}

	return cmd
}

// Run executes the tool with the parsed parameters.
func (e *executeCommand) Run(cmd *cobra.Command, args []string) {
	toolName := args[0]
	params := args[1:]

	var foundTool sdk.AITool
	for _, tool := range tools.Tools() {
		if tool.Name() == toolName {
			foundTool = tool
			break
		}
	}

	if foundTool == nil {
		utils.NewExitError().WithMessage(fmt.Sprintf("tool not found: %s. Available tools: %s", toolName, strings.Join(tools.ToolNames(), ", "))).Done()
	}

	// Parse parameters into a map
	input := parseParams(params)

	// Call the tool
	result, err := foundTool.Call(input)
	if err != nil {
		utils.NewExitError().WithMessage("tool execution failed").WithReason(err).Done()
	}

	// Print the result
	switch v := result.(type) {
	case string:
		fmt.Println(v)
	default:
		jsonResult, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Printf("%v\n", result)
		} else {
			fmt.Println(string(jsonResult))
		}
	}
}

// parseParams parses key:value pairs into a nested map.
// Keys can use dot notation for nested maps (e.g., "view_range.0:1").
func parseParams(params []string) map[string]interface{} {
	result := make(map[string]interface{})

	for _, param := range params {
		// Split on first colon only
		idx := strings.Index(param, ":")
		if idx == -1 {
			// this results in an os exit
			utils.NewExitError().WithMessage(fmt.Sprintf("invalid param: %s", param)).Done()
		}

		key := param[:idx]
		value := param[idx+1:]

		// Handle nested keys with dot notation
		parts := strings.Split(key, ".")
		setNestedValue(result, parts, value)
	}

	return result
}

// setNestedValue sets a value in a nested map structure based on the key parts.
func setNestedValue(m map[string]interface{}, parts []string, value string) {
	if len(parts) == 1 {
		m[parts[0]] = value
		return
	}

	key := parts[0]
	if _, exists := m[key]; !exists {
		m[key] = make(map[string]interface{})
	}

	if nested, ok := m[key].(map[string]interface{}); ok {
		setNestedValue(nested, parts[1:], value)
	}
}
