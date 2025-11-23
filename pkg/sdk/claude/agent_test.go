package claude

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/Shonei/agents/pkg/sdk"
)

const apiKey = ""

func Test_Call_Message(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Anthropic integration test in short mode")
	}

	agent := NewAgent(WithAPIKey(apiKey))
	resp, err := agent.SendMessage("Hello, Claude! How are you today?")
	if err != nil {
		t.Errorf("failed to send message: %v", err)
	}

	t.Log(resp)
	t.Logf("Response: %s", resp.GetTextContent())
}
func Test_call_with_tool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Anthropic integration test in short mode")
	}

	agent := NewAgent(WithAPIKey(apiKey))
	tool := sdk.NewTool("get_weather", "Get the current weather in a given location", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"location": map[string]interface{}{
				"type":        "string",
				"description": "The city and state, e.g. San Francisco, CA",
			},
			"unit": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"celsius", "fahrenheit"},
				"description": "The unit of temperature",
			},
		},
		"required": []string{"location"},
	})
	request := sdk.CreateMessageRequest{
		Model:     ModelClaude45,
		MaxTokens: 1024,
		Messages: []sdk.InputMessage{
			sdk.NewTextMessage(RoleUser, "What's the weather like in San Francisco?. If you can can you check online?"),
		},
		Tools:      []sdk.Tool{tool},
		ToolChoice: sdk.NewAutoToolChoice(),
	}
	resp, err := agent.CreateMessage(request)
	if err != nil {
		t.Errorf("failed to send message: %v", err)
	}

	t.Log(resp.GetToolUses())
}

func Test_stream(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Anthropic integration test in short mode")
	}

	tool := sdk.NewTool("get_weather", "Get the current weather in a given location", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"location": map[string]interface{}{
				"type":        "string",
				"description": "The city and state, e.g. San Francisco, CA",
			},
			"unit": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"celsius", "fahrenheit"},
				"description": "The unit of temperature",
			},
		},
		"required": []string{"location"},
	})

	reqBody := sdk.CreateMessageRequest{
		Model:     ModelClaude45,
		MaxTokens: 1024,
		Messages: []sdk.InputMessage{
			sdk.NewTextMessage(RoleUser, "Can you check the weather for me in San Francisco, CA?"),
		},
		Tools:      []sdk.Tool{tool},
		ToolChoice: sdk.NewAutoToolChoice(),
		Stream:     true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		t.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		t.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", DefaultAPIVersion)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("failed to send message: %v", err)
	}

	t.Log(resp)

	respBOdy, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("failed to read response: %v", err)
	}

	// looks like this
	//
	//        event: message_start
	//        data: {"type":"message_start","message":{"model":"claude-sonnet-4-5-20250929","id":"msg_011u4qLaZN3YNg77b7K1URtc","type":"message","role":"assistant","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":619,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":0},"output_tokens":1,"service_tier":"standard"}}      }
	//
	//        event: content_block_start
	//        data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_01TCx5G1eGL6SmF2RjH7oz83","name":"get_weather","input":{}}         }
	//
	//        event: content_block_delta
	//        data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":""}}
	//
	//        event: content_block_delta
	//        data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"l"}     }
	//
	//        event: content_block_delta
	//        data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"ocation\": "}           }
	//
	//        event: content_block_delta
	//        data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"San Fra"}     }
	//
	//        event: content_block_delta
	//        data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"ncisco, CA\"}"}       }
	//
	//        event: content_block_stop
	//        data: {"type":"content_block_stop","index":0       }
	//
	//        event: message_delta
	//        data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"input_tokens":619,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":56}            }
	//
	//        event: message_stop
	//        data: {"type":"message_stop"       }
	t.Log(string(respBOdy))
}
