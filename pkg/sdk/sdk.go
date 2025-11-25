package sdk

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"
)

// AI is an agentic wrapper for the Agent
// You can register tools and the AI will use them when appropriate
// When chatting with the AI, it will use the tools you registered
type AI struct {
	agent        Agent
	tools        []AITool
	systemPrompt string
}

type AITool interface {
	Name() string
	Description() string
	Init(config map[string]string)
	InputSchema() map[string]interface{}
	Call(input map[string]interface{}) (interface{}, error)
}

func NewAI(agent Agent) *AI {
	return &AI{
		agent: agent,
	}
}

func (a *AI) RegisterTool(tool AITool) {
	a.tools = append(a.tools, tool)
}

func (a *AI) SetSystemPrompt(prompt string) {
	a.systemPrompt = prompt
}

func (a *AI) Chat(message string) (string, error) {
	tools := []Tool{}
	for _, tool := range a.tools {
		tools = append(tools, NewTool(tool.Name(), tool.Description(), tool.InputSchema()))
	}

	// Initial message
	history := []InputMessage{
		NewTextMessage(RoleUser, message),
	}

	fmt.Println("Chat started. Press Ctrl+C to exit.")

	for {
		// Process current history
		updatedHistory, _, err := a.chat(chatPayload{
			tools:    tools,
			messages: history,
		})
		if err != nil {
			return "", err
		}

		// Update history with the full conversation from this turn
		history = updatedHistory

		// Prompt for next user input
		fmt.Print("\n> ")
		var nextMessage string
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			nextMessage = scanner.Text()
			if nextMessage == "" {
				break
			}
		} else {
			if err := scanner.Err(); err != nil {
				return "", fmt.Errorf("error reading input: %w", err)
			}

			break
		}

		history = append(history, NewTextMessage(RoleUser, nextMessage))
	}

	return "", nil // Reached when user input is empty which is treated as an exit condition
}

type chatPayload struct {
	message  string
	tools    []Tool
	messages []InputMessage
}

func (a *AI) chat(c chatPayload) ([]InputMessage, *MessageResponse, error) {
	// Build messages list
	messages := c.messages
	if len(messages) == 0 {
		messages = []InputMessage{
			NewTextMessage(RoleUser, c.message),
		}
	}

	// Loop until we get a response without tool calls
	for {
		request := CreateMessageRequest{
			Model:      a.agent.Model(),
			MaxTokens:  a.agent.MaxTokens(),
			Messages:   messages,
			Tools:      c.tools,
			ToolChoice: NewAutoToolChoice(),
		}

		response, err := a.agent.CreateMessage(request)
		if err != nil {
			return nil, nil, err
		}

		// log usage
		fmt.Printf("Usage: %d input tokens, %d output tokens\n", response.Usage.InputTokens, response.Usage.OutputTokens)

		hasToolCalls := false

		// Convert response content to ContentBlocks for the conversation history
		assistantContent := []ContentBlock{}
		for _, block := range response.Content {
			assistantContent = append(assistantContent, ContentBlock{
				Type:             block.Type,
				Text:             block.Text,
				ID:               block.ID,
				Name:             block.Name,
				Input:            block.Input,
				ThoughtSignature: block.ThoughtSignature,
			})

			if block.Type == ContentTypeToolUse {
				hasToolCalls = true
			}
		}

		// Add assistant's response with tool uses to messages
		// We add it even if there are no tool calls, because we need to record the assistant's reply
		messages = append(messages, InputMessage{
			Role:    RoleAssistant,
			Content: assistantContent,
		})

		// Print non-tool-use blocks
		for _, block := range response.Content {
			if block.Type != ContentTypeToolUse {
				color.New(color.FgBlue, color.Bold).Print("Assistant: ")

				// Render markdown
				out, err := glamour.Render(block.Text, "dark")
				if err != nil {
					// Fallback to plain text if rendering fails
					fmt.Println(block.Text)
				} else {
					fmt.Print(out)
				}
			}
		}

		// no tools to process so we're done
		if !hasToolCalls {
			return messages, response, nil
		}

		for _, block := range response.Content {
			if block.Type != ContentTypeToolUse {
				continue
			}

			toolResults, err := a.processTools(block)
			if err != nil {
				return nil, nil, err
			}

			// Add tool results to messages
			messages = append(messages, InputMessage{
				Role:    RoleUser,
				Content: toolResults,
			})
		}
	}
}

func (a *AI) processTools(toolCall ResponseContentBlock) ([]ContentBlock, error) {
	// Execute tools and collect results
	toolResults := []ContentBlock{}

	found := false
	for _, tool := range a.tools {
		if tool.Name() == toolCall.Name {
			// print some generic information of the tool calls we make
			color.New(color.FgCyan, color.Bold).Print("Tool Call: ")
			color.Cyan("%s", tool.Name())
			inputBytes, _ := json.Marshal(toolCall.Input)
			color.Cyan("Tool Input:  %s\n\n", inputBytes[:min(len(inputBytes), 100)])
			// color.Cyan("Tool Input:  %s\n\n", inputBytes)

			result, err := tool.Call(toolCall.Input)
			if err != nil {
				var aiError *AIError
				if errors.As(err, &aiError) {
					toolResults = append(toolResults, NewToolResultContentBlock(toolCall.ID, aiError.AIResponse(), true))

					continue
				}

				return nil, err
			}

			// Convert result to JSON string for API compatibility
			// The Anthropic API requires tool result content to be a string or array of content blocks
			var resultContent string
			switch v := result.(type) {
			case string:
				resultContent = v
			default:
				// Convert to JSON string
				jsonBytes, err := json.Marshal(result)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal tool result: %w", err)
				}
				resultContent = string(jsonBytes)
			}

			toolResults = append(toolResults, NewToolResultContentBlock(toolCall.ID, resultContent, false))
			found = true

			break
		}
	}
	if !found {
		return nil, fmt.Errorf("tool not found: %s", toolCall.Name)
	}

	return toolResults, nil
}
