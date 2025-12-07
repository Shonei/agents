package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/tools"
	"github.com/Shonei/agents/pkg/utils"
)

type detailsCommand struct{}

// NewDetailsCommand implements the `agents tools details <tool_name>` command.
func NewDetailsCommand() *cobra.Command {
	d := &detailsCommand{}

	cmd := &cobra.Command{
		Use:   "details <tool_name>",
		Short: "Show details of a tool including its description and input schema",
		Long:  "Show details of a tool including its description and input schema. Available tools: " + strings.Join(tools.ToolNames(), ", "),
		Run:   d.Run,
		Args:  cobra.ExactArgs(1),
	}

	return cmd
}

// Run prints the tool's description and input schema.
func (d *detailsCommand) Run(cmd *cobra.Command, args []string) {
	toolName := args[0]

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

	fmt.Printf("Tool: %s\n\n", foundTool.Name())
	fmt.Printf("Description:\n%s\n\n", foundTool.Description())

	schema := foundTool.InputSchema()
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		utils.NewExitError().WithMessage("failed to marshal input schema").WithReason(err).Done()
	}

	fmt.Printf("Input Schema:\n%s\n", string(schemaJSON))
}
