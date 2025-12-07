package sdk

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk/audit"
	"github.com/Shonei/agents/pkg/utils"
)

// AI is an agentic wrapper for the Agent
// You can register tools and the AI will use them when appropriate
// When chatting with the AI, it will use the tools you registered
type AI struct {
	agent        Agent
	tools        []AITool
	systemPrompt string
	audit        *audit.Audit
}

type AITool interface {
	Name() string
	Description() string
	Init(config map[string]string, configFactory *config.ConfigFactory)
	InputSchema() map[string]interface{}
	Call(input map[string]interface{}) (interface{}, error)
}

func NewAI(agent Agent, audit *audit.Audit) *AI {
	return &AI{
		agent: agent,
		audit: audit,
	}
}

func (a *AI) RegisterTool(tool AITool) {
	a.tools = append(a.tools, tool)
}

func (a *AI) SetSystemPrompt(prompt string) {
	a.audit.User(prompt)

	a.systemPrompt = prompt
}

func (a *AI) Chat(message string) (string, error) {
	fmt.Println("Chat started. Press Ctrl+C to exit.")

	if message == "" {
		input, err := utils.GatherUserContent()
		if err != nil {
			return "", err
		}

		if input == "" {
			return "", nil
		}

		message = input
	}

	tools := []Tool{}
	for _, tool := range a.tools {
		tools = append(tools, NewTool(tool.Name(), tool.Description(), tool.InputSchema()))
	}

	// Initial message
	history := []InputMessage{
		NewTextMessage(RoleUser, message),
	}

	a.audit.LogEvent(audit.Event{
		Type:    audit.InitialMessageEvent,
		Content: message,
	})

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
		nextMessage, err := utils.GatherUserContent()
		if err != nil {
			return "", err
		}

		if nextMessage == "" {
			break
		}

		a.audit.LogEvent(audit.Event{
			Type:    audit.UserMessageEvent,
			Content: nextMessage,
		})

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
			Messages:   messages,
			Tools:      c.tools,
			ToolChoice: NewAutoToolChoice(),
			System:     a.systemPrompt,
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
			switch block.Type {
			case ContentTypeText:
				a.audit.LogEvent(audit.Event{
					Type:    audit.AssistantMessageEvent,
					Content: block.Text,
				})
			case ContentTypeToolUse:
				a.audit.LogEvent(audit.Event{
					Type: audit.FunctionCallEvent,
					FunctionCall: functionCallAudit{
						Name:  block.Name,
						Input: block.Input,
					},
				})
			}

			assistantContent = append(assistantContent, ContentBlock{
				Type:             block.Type,
				Text:             block.Text,
				ID:               block.ID,
				Name:             block.Name,
				Input:            block.Input,
				ThoughtSignature: block.ThoughtSignature,
				Source:           block.Blob,
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
			if block.Type == ContentTypeThinking {
				color.New(color.FgHiBlue, color.Italic).Print("Thinking: ")
				// Render markdown
				out, err := glamour.Render(block.Text, "dark")
				if err != nil {
					// Fallback to plain text if rendering fails
					fmt.Println(block.Text)
				} else {
					fmt.Print(out)
				}

				continue
			}

			if block.Type == ContentTypeImage {
				cwd, err := os.Getwd()
				if err != nil {
					utils.NewExitError().WithMessage("failed to get current directory").WithReason(err).Done()
				}

				imageBytes, err := base64.StdEncoding.DecodeString(block.Blob.Data)
				if err != nil {
					utils.NewExitError().WithMessage("failed to decode image").WithReason(err).Done()
				}

				randSuffix := utils.RandomString(5)

				imageFormat := "png"
				contentTypeParts := strings.Split(block.Blob.MimeType, "/")
				if len(contentTypeParts) == 2 {
					imageFormat = contentTypeParts[1]
				}

				// Write the image to a file
				fileName := fmt.Sprintf("image_%s.%s", randSuffix, imageFormat)

				err = os.WriteFile(filepath.Join(cwd, fileName), imageBytes, 0o600)
				if err != nil {
					utils.NewExitError().WithMessage("failed to write image to file").WithReason(err).Done()
				}

				color.New(color.FgBlue, color.Bold).Print("CLI:\n")
				fmt.Printf("\tImage saved to %s\n", fileName)

				continue
			}

			if block.Type == ContentTypeText {
				color.New(color.FgBlue, color.Bold).Print("Assistant: ")

				// Render markdown
				out, err := glamour.Render(block.Text, "dark")
				if err != nil {
					// Fallback to plain text if rendering fails
					fmt.Println(block.Text)
				} else {
					fmt.Print(out)
				}

				continue
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
			found = true

			// print some generic information of the tool calls we make
			color.New(color.FgCyan, color.Bold).Print("Tool Call: ")
			color.Cyan("%s", tool.Name())
			inputBytes, _ := json.Marshal(toolCall.Input)
			color.Cyan("Tool Input:  %s\n", inputBytes[:min(len(inputBytes), 100)])

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

			break
		}
	}
	if !found {
		return nil, fmt.Errorf("tool not found: '%s'", toolCall.Name)
	}

	for _, response := range toolResults {
		a.audit.LogEvent(audit.Event{
			Type: audit.FunctionResponseEvent,
			FunctionResponse: functionResponseAudit{
				Name:     response.ToolUseID,
				Response: response.Content,
			},
		})
	}

	return toolResults, nil
}

type functionCallAudit struct {
	Name  string `json:"name"`
	Input any    `json:"input"`
}

type functionResponseAudit struct {
	Name     string `json:"name"`
	Response any    `json:"response"`
}
