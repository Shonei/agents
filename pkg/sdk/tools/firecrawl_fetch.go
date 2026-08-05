package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
)

const defaultFirecrawlBaseURL = "https://api.firecrawl.dev/v2"

type FirecrawlFetchTool struct {
	apiKey  string
	baseURL string
}

func (f *FirecrawlFetchTool) Name() string {
	return "firecrawl_fetch"
}

func (f *FirecrawlFetchTool) Description() string {
	return "Fetches a single URL through Firecrawl's hosted scrape API and returns LLM-ready markdown with source metadata. Use this as a hosted alternative to browse_url when you want clean markdown without local browser automation. This tool only fetches one URL; it does not crawl a site."
}

func (f *FirecrawlFetchTool) Init(config map[string]string, configFactory *config.ConfigFactory) {
	f.baseURL = defaultFirecrawlBaseURL
	if val, ok := config["base_url"]; ok && val != "" {
		f.baseURL = strings.TrimRight(val, "/")
	}
	if val, ok := config["api_key"]; ok && val != "" {
		f.apiKey = val
	} else if configFactory != nil {
		f.apiKey = configFactory.GetFirecrawlAPIKey()
	}
}

func (f *FirecrawlFetchTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The HTTP(S) URL to scrape. Bare hosts are normalized to https://.",
				"example":     "https://docs.example.com/api",
			},
			"timeout_seconds": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum time for the Firecrawl request. Defaults to 60 seconds.",
				"example":     60,
			},
			"wait_milliseconds": map[string]interface{}{
				"type":        "integer",
				"description": "Extra wait time Firecrawl should apply before capturing content. Defaults to 1000 milliseconds.",
				"example":     1000,
			},
			"only_main_content": map[string]interface{}{
				"type":        "boolean",
				"description": "Ask Firecrawl to return only the main page content. Defaults to true.",
				"example":     true,
			},
			"only_clean_content": map[string]interface{}{
				"type":        "boolean",
				"description": "Ask Firecrawl to run its beta clean-content pass over markdown. Defaults to false.",
				"example":     false,
			},
			"include_html": map[string]interface{}{
				"type":        "boolean",
				"description": "Also request cleaned HTML in addition to markdown. Defaults to false.",
				"example":     false,
			},
		},
		"required": []interface{}{"url"},
	}
}

type FirecrawlFetchToolInput struct {
	URL              string `json:"url"`
	TimeoutSeconds   int    `json:"timeout_seconds"`
	WaitMilliseconds *int   `json:"wait_milliseconds"`
	OnlyMainContent  *bool  `json:"only_main_content"`
	OnlyCleanContent *bool  `json:"only_clean_content"`
	IncludeHTML      bool   `json:"include_html"`
}

type firecrawlScrapeRequest struct {
	URL              string   `json:"url"`
	Formats          []string `json:"formats"`
	OnlyMainContent  bool     `json:"onlyMainContent"`
	OnlyCleanContent bool     `json:"onlyCleanContent,omitempty"`
	WaitFor          int      `json:"waitFor,omitempty"`
	Timeout          int      `json:"timeout,omitempty"`
}

type firecrawlScrapeResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Markdown string                 `json:"markdown"`
		HTML     string                 `json:"html"`
		Metadata map[string]interface{} `json:"metadata"`
	} `json:"data"`
	Code  string `json:"code"`
	Error string `json:"error"`
}

func (f *FirecrawlFetchTool) Call(input map[string]interface{}) (interface{}, error) {
	var in FirecrawlFetchToolInput
	if err := mapstruct(input, &in); err != nil {
		return "", err
	}
	if in.URL == "" {
		return "", sdk.NewAIError("url is required")
	}
	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		in.URL = "https://" + in.URL
	}
	if in.TimeoutSeconds <= 0 {
		in.TimeoutSeconds = 60
	}
	waitMilliseconds := 1000
	if in.WaitMilliseconds != nil {
		waitMilliseconds = *in.WaitMilliseconds
	}
	if waitMilliseconds < 0 {
		return "", sdk.NewAIError("wait_milliseconds must be >= 0")
	}
	onlyMainContent := true
	if in.OnlyMainContent != nil {
		onlyMainContent = *in.OnlyMainContent
	}
	onlyCleanContent := false
	if in.OnlyCleanContent != nil {
		onlyCleanContent = *in.OnlyCleanContent
	}

	formats := []string{"markdown"}
	if in.IncludeHTML {
		formats = append(formats, "html")
	}

	reqBody := firecrawlScrapeRequest{
		URL:              in.URL,
		Formats:          formats,
		OnlyMainContent:  onlyMainContent,
		OnlyCleanContent: onlyCleanContent,
		WaitFor:          waitMilliseconds,
		Timeout:          in.TimeoutSeconds * 1000,
	}
	rawReq, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Firecrawl request: %w", err)
	}

	client := &http.Client{Timeout: time.Duration(in.TimeoutSeconds) * time.Second}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(f.baseURL, "/")+"/scrape", bytes.NewReader(rawReq))
	if err != nil {
		return "", fmt.Errorf("failed to create Firecrawl request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if f.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+f.apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Firecrawl: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	rawResp, err := io.ReadAll(io.LimitReader(resp.Body, browseURLMaxLen+4096))
	if err != nil {
		return "", fmt.Errorf("failed to read Firecrawl response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("firecrawl returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(rawResp)))
	}

	var scrapeResp firecrawlScrapeResponse
	if err := json.Unmarshal(rawResp, &scrapeResp); err != nil {
		return "", fmt.Errorf("failed to parse Firecrawl response: %w", err)
	}
	if !scrapeResp.Success {
		if scrapeResp.Error == "" {
			scrapeResp.Error = "unknown Firecrawl error"
		}

		return "", fmt.Errorf("firecrawl scrape failed: %s", scrapeResp.Error)
	}

	output := formatFirecrawlOutput(in.URL, scrapeResp, in.IncludeHTML)
	if len(output) > browseURLMaxLen {
		output = output[:browseURLMaxLen] + "\n\n... (content truncated)"
	}

	return output, nil
}

func formatFirecrawlOutput(requestedURL string, resp firecrawlScrapeResponse, includeHTML bool) string {
	metadata := resp.Data.Metadata
	title := metadataValue(metadata, "title")
	sourceURL := metadataValue(metadata, "sourceURL")
	finalURL := metadataValue(metadata, "url")
	if sourceURL == "" {
		sourceURL = requestedURL
	}
	if finalURL == "" {
		finalURL = sourceURL
	}

	var sb strings.Builder
	if title != "" {
		sb.WriteString("Title: " + title + "\n")
	}
	sb.WriteString("Source URL: " + sourceURL + "\n")
	if finalURL != sourceURL {
		sb.WriteString("Final URL: " + finalURL + "\n")
	}
	sb.WriteString("Fetched by: firecrawl_fetch\n\n")
	if resp.Data.Markdown != "" {
		sb.WriteString(resp.Data.Markdown)
	} else {
		sb.WriteString("(Firecrawl returned no markdown content)")
	}
	if includeHTML && resp.Data.HTML != "" {
		sb.WriteString("\n\n## HTML\n\n")
		sb.WriteString(resp.Data.HTML)
	}

	return sb.String()
}

func metadataValue(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []interface{}:
		if len(typed) == 0 {
			return ""
		}
		if first, ok := typed[0].(string); ok {
			return first
		}
	}

	return fmt.Sprint(value)
}
