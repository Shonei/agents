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
	agent           Agent
	tools           []AITool
	serverTools     []ServerSideTool
	systemPrompt    string
	audit           *audit.Audit
	lastInputTokens int
	hideThinking    bool
	hideGrounding   bool
	quiet           bool
	maxTurns        int
}

type AITool interface {
	Name() string
	Description() string
	Init(config map[string]string, configFactory *config.ConfigFactory)
	InputSchema() map[string]interface{}
	Call(input map[string]interface{}) (interface{}, error)
}

// AuditAwareTool is an optional interface for tools that need to log events
// to the same audit session as their parent agent.
type AuditAwareTool interface {
	SetAudit(parentSessionID string)
}

// ServerSideTool is a tool that is executed by the model provider rather than
// by the local SDK loop. Implementations only need to declare their
// YAML-facing name and the provider-recognised kind to send on the wire.
type ServerSideTool interface {
	Name() string
	Kind() string
	Init(config map[string]string, configFactory *config.ConfigFactory)
}

func NewAI(agent Agent, audit *audit.Audit) *AI {
	return &AI{
		agent:    agent,
		audit:    audit,
		maxTurns: 120, // Default max turns to prevent infinite loops
	}
}

func (a *AI) RegisterTool(tool AITool) {
	a.tools = append(a.tools, tool)
}

func (a *AI) RegisterServerTool(tool ServerSideTool) {
	a.serverTools = append(a.serverTools, tool)
}

func (a *AI) SetHideThinking(hide bool) {
	a.hideThinking = hide
}

func (a *AI) SetHideGrounding(hide bool) {
	a.hideGrounding = hide
}

func (a *AI) SetQuiet(quiet bool) {
	a.quiet = quiet
}

func (a *AI) SetMaxTurns(turns int) {
	a.maxTurns = turns
}

func (a *AI) SetSystemPrompt(prompt string) {
	a.audit.User(prompt, "")

	a.systemPrompt = prompt
}

// SetSystemPromptSilent sets the agent's system prompt without registering
// a new audit "user" session. Use this when the audit session is owned by
// a parent (e.g. RouterAI registers the session once and sets each
// sub-agent's prompt silently so all sub-agent events land in the same
// session).
func (a *AI) SetSystemPromptSilent(prompt string) {
	a.systemPrompt = prompt
}

// SystemPrompt returns the agent's currently configured system prompt.
// Used by composite agents (e.g. RouterAI) to capture the incoming
// sub-agent's prompt in the audit trail when control changes hands.
func (a *AI) SystemPrompt() string {
	return a.systemPrompt
}

// Audit exposes the underlying audit handle so composite agents (e.g.
// RouterAI) can log events through the same audit channel without
// constructing a parallel one.
func (a *AI) Audit() *audit.Audit {
	return a.audit
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

	history := []InputMessage{
		NewTextMessage(RoleUser, message),
	}

	a.audit.LogEvent(audit.Event{
		Type:    audit.InitialMessageEvent,
		Content: message,
	})

	for {
		updatedHistory, err := a.RunTurn(history)
		if err != nil {
			return "", err
		}

		history = updatedHistory

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

// RunOnce runs a single non-interactive prompt through the standard tool
// loop and returns the assistant's final text. Useful for sub-agent calls
// (agent-as-tool) and other one-shot, no-stdin scenarios.
func (a *AI) RunOnce(prompt string) (string, error) {
	a.audit.LogEvent(audit.Event{
		Type:    audit.InitialMessageEvent,
		Content: prompt,
	})

	history, err := a.RunTurn([]InputMessage{NewTextMessage(RoleUser, prompt)})
	if err != nil {
		return "", err
	}

	if len(history) == 0 {
		return "", nil
	}

	last := history[len(history)-1]
	if last.Role != RoleAssistant {
		return "", nil
	}

	switch v := last.Content.(type) {
	case string:
		return v, nil
	case []ContentBlock:
		var b strings.Builder
		var images []string
		for _, block := range v {
			if block.Type == ContentTypeText {
				b.WriteString(block.Text)
			} else if block.Type == ContentTypeImage && block.FilePath != "" {
				images = append(images, block.FilePath)
			}
		}

		if len(images) > 0 {
			result := map[string]interface{}{
				"text":   b.String(),
				"images": images,
			}
			jsonBytes, _ := json.Marshal(result)

			return string(jsonBytes), nil
		}

		return b.String(), nil
	}

	return "", nil
}

// RunTurn drives the inner request/tool-loop for a single user turn.
// `history` must already end with the new user message. It returns the
// updated history with the assistant reply (and any tool_use / tool_result
// blocks generated during the turn) appended.
func (a *AI) RunTurn(history []InputMessage) ([]InputMessage, error) {
	tools := []Tool{}
	for _, tool := range a.tools {
		tools = append(tools, NewTool(tool.Name(), tool.Description(), tool.InputSchema()))
	}

	serverTools := []ServerTool{}
	for _, st := range a.serverTools {
		serverTools = append(serverTools, ServerTool{Name: st.Kind()})
	}

	messages := history
	turnCount := 0

	// Loop until we get a response without tool calls
	for {
		if turnCount >= a.maxTurns {
			return nil, fmt.Errorf("max turns (%d) reached", a.maxTurns)
		}
		turnCount++

		compacted, err := a.maybeCompact(&messages)
		if err != nil {
			return nil, err
		}

		if compacted {
			a.lastInputTokens = 0
		}

		request := CreateMessageRequest{
			Messages:    messages,
			Tools:       tools,
			ServerTools: serverTools,
			ToolChoice:  NewAutoToolChoice(),
			System:      a.systemPrompt,
		}

		response, err := a.agent.CreateMessage(request)
		if err != nil {
			return nil, err
		}

		a.lastInputTokens = response.Usage.InputTokens

		if !a.quiet {
			fmt.Printf("Usage: %d input tokens, %d output tokens\n", response.Usage.InputTokens, response.Usage.OutputTokens)
		}

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
			case ContentTypeGrounding:
				// Server-side tool activity is informational only: never
				// added to the assistant content we replay to the model
				// and never triggers a tool-result follow-up.
				a.audit.LogEvent(audit.Event{
					Type:      audit.GroundingEvent,
					Grounding: block.Grounding,
				})

				continue
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
				if !a.hideThinking && !a.quiet {
					color.New(color.FgHiBlue, color.Italic).Print("Thinking: ")
					// Render markdown
					out, err := glamour.Render(block.Text, "dark")
					if err != nil {
						// Fallback to plain text if rendering fails
						fmt.Println(block.Text)
					} else {
						fmt.Print(out)
					}
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
				filePath := filepath.Join(cwd, fileName)

				err = os.WriteFile(filePath, imageBytes, 0o600)
				if err != nil {
					utils.NewExitError().WithMessage("failed to write image to file").WithReason(err).Done()
				}

				if !a.quiet {
					color.New(color.FgBlue, color.Bold).Print("CLI:\n")
					fmt.Printf("\tImage saved to %s\n", fileName)
				}

				// Update the assistant content block with the file path
				for i, c := range assistantContent {
					if c.Type == ContentTypeImage && c.Source != nil && c.Source.Data == block.Blob.Data {
						assistantContent[i].FilePath = filePath
					}
				}

				continue
			}

			if block.Type == ContentTypeText {
				if !a.quiet {
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

				continue
			}

			if block.Type == ContentTypeGrounding && block.Grounding != nil {
				if !a.hideGrounding {
					printGroundingSummary(block.Grounding)
				}

				continue
			}
		}

		if !hasToolCalls {
			return messages, nil
		}

		var allToolResults []ContentBlock
		for _, block := range response.Content {
			if block.Type != ContentTypeToolUse {
				continue
			}

			toolResults, err := a.processTools(block)
			if err != nil {
				return nil, err
			}

			allToolResults = append(allToolResults, toolResults...)
		}

		if len(allToolResults) > 0 {
			// Add tool results to messages
			messages = append(messages, InputMessage{
				Role:    RoleUser,
				Content: allToolResults,
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

			if tool.Name() == "plan" {
				if snapshot := GlobalPlan.Snapshot(); snapshot != nil {
					a.audit.LogEvent(audit.Event{
						Type: audit.PlanEvent,
						Plan: snapshot,
					})
				}
			}

			if tool.Name() == "todo" {
				a.audit.LogEvent(audit.Event{
					Type: audit.TodoEvent,
					Todo: GlobalTodo.GetTasks(),
				})
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

// printGroundingSummary renders a short, human-readable digest of what the
// provider-side tools did to produce the answer.
func printGroundingSummary(g *GroundingMetadata) {
	if g == nil {
		return
	}

	hasContent := len(g.WebSearchQueries) > 0 || len(g.Sources) > 0 || len(g.RetrievedURLs) > 0
	if !hasContent {
		return
	}

	header := color.New(color.FgMagenta, color.Bold)
	body := color.New(color.FgMagenta)

	header.Println("Grounding:")

	if len(g.WebSearchQueries) > 0 {
		body.Printf("  Search queries: %s\n", strings.Join(g.WebSearchQueries, ", "))
	}

	if len(g.Sources) > 0 {
		body.Println("  Sources:")
		for i, src := range g.Sources {
			label := src.Title
			if label == "" {
				label = src.URI
			}
			body.Printf("    %d. %s (%s)\n", i+1, label, src.URI)
		}
	}

	if len(g.RetrievedURLs) > 0 {
		body.Println("  Retrieved URLs:")
		for _, u := range g.RetrievedURLs {
			if u.Status != "" {
				body.Printf("    - %s [%s]\n", u.URL, u.Status)
			} else {
				body.Printf("    - %s\n", u.URL)
			}
		}
	}
}

type functionCallAudit struct {
	Name  string `json:"name"`
	Input any    `json:"input"`
}

type functionResponseAudit struct {
	Name     string `json:"name"`
	Response any    `json:"response"`
}

type routeSelectionAudit struct {
	Route      string  `json:"route"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
	LatencyMs  int64   `json:"latency_ms"`
}

type handoffAudit struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Summary      string `json:"summary"`
	SystemPrompt string `json:"system_prompt,omitempty"`
}
