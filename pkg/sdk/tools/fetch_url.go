package tools

import (
	"fmt"
	"os"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/Shonei/agents/pkg/config"

	"github.com/Shonei/agents/pkg/utils"
)

// FetchURLTool fetches the content of a URL
type FetchURLTool struct{}

func (f *FetchURLTool) Name() string {
	return "fetch_url"
}

func (f *FetchURLTool) Description() string {
	return "Fetches the content of a URL and returns it as text. Useful for reading web pages or documentation."
}

func (f *FetchURLTool) Init(config map[string]string, _ *config.ConfigFactory) {
}

func (f *FetchURLTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to fetch",
				"example":     "https://example.com",
			},
		},
		"required": []interface{}{"url"},
	}
}

func (f *FetchURLTool) Call(input map[string]interface{}) (interface{}, error) {
	url, ok := input["url"].(string)
	if !ok {
		return "", fmt.Errorf("url must be a string")
	}

	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	// Basic validation to ensure protocol is present
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	var content string
	err := utils.NewHTTPBuilder(url).New().Into(&content).Do()
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}

	converter := md.NewConverter("", true, nil)
	if markdown, err := converter.ConvertString(content); err == nil {
		content = markdown
	}

	// Simple truncation to avoid blowing up context window too much with huge pages
	// 100kb limit roughly
	const maxLen = 100000
	if len(content) > maxLen {
		content = content[:maxLen] + "\n... (content truncated)"
	}

	if os.Getenv("DEBUG") != "" {
		fmt.Println(content)
	}

	return content, nil
}
