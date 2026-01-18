package tools

import (
	"fmt"
	"os"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"

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
	// Added User-Agent to mimic a browser to avoid blocking
	err := utils.NewHTTPBuilder(url).
		New().
		WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36").
		Into(&content).
		Do()
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}

	var title, description string

	// Parse HTML with goquery for better cleanup and extraction
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		// If parsing fails, we proceed with raw content to at least try markdown conversion
		fmt.Fprintf(os.Stderr, "failed to parse HTML with goquery: %v\n", err)
	} else {
		// Extract metadata
		title = doc.Find("title").Text()
		description, _ = doc.Find("meta[name='description']").Attr("content")

		// Remove noise elements
		doc.Find("script, style, nav, footer, header, noscript, svg, iframe, link[rel='stylesheet']").Remove()

		cleanedHTML, err := doc.Html()
		if err == nil {
			content = cleanedHTML
		}
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

	// Construct final output with metadata
	var sb strings.Builder
	if title != "" {
		sb.WriteString("Title: " + strings.TrimSpace(title) + "\n")
	}
	if description != "" {
		sb.WriteString("Description: " + strings.TrimSpace(description) + "\n")
	}
	sb.WriteString("URL: " + url + "\n\n")
	sb.WriteString(content)

	finalContent := sb.String()

	if os.Getenv("DEBUG") != "" {
		fmt.Println(finalContent)
	}

	return finalContent, nil
}
