package tools

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

// WebSearchTool is a tool for searching the web using DuckDuckGo
type WebSearchTool struct{}

type WebSearchInput struct {
	SearchQuery string `json:"search_query"`
	NumResults  int    `json:"num_results"`
}

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func (w *WebSearchTool) Name() string {
	return "web-search"
}

func (w *WebSearchTool) Description() string {
	return "Searches the web using DuckDuckGo and returns relevant results. Use this to find current information, documentation, or answers to questions that require up-to-date web content."
}

func (w *WebSearchTool) Init(_ map[string]string, c *config.ConfigFactory) {
}

func (w *WebSearchTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"search_query": map[string]interface{}{
				"type":        "string",
				"description": "The search query to look up on the web.",
			},
			"num_results": map[string]interface{}{
				"type":        "integer",
				"description": "Number of results to return (default: 5, max: 10).",
				"default":     5,
			},
		},
		"required": []interface{}{"search_query"},
	}
}

func (w *WebSearchTool) Call(input map[string]interface{}) (interface{}, error) {
	var in WebSearchInput
	if err := mapstruct(input, &in); err != nil {
		return "", err
	}

	if in.SearchQuery == "" {
		return "", sdk.NewAIError("search_query is required")
	}

	numResults := in.NumResults
	if numResults <= 0 {
		numResults = 5
	}
	if numResults > 10 {
		numResults = 10
	}

	results, err := searchDuckDuckGo(in.SearchQuery, numResults)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "No results found for the query.", nil
	}

	return formatResults(results), nil
}

func searchDuckDuckGo(query string, numResults int) ([]SearchResult, error) {
	// Use DuckDuckGo HTML search
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	var htmlContent string
	err := utils.NewHTTPBuilder(searchURL).
		New().
		Into(&htmlContent).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search results: %w", err)
	}

	fmt.Println(htmlContent)

	return parseSearchResults(htmlContent, numResults), nil
}

func parseSearchResults(html string, maxResults int) []SearchResult {
	var results []SearchResult

	// Find all result blocks - DuckDuckGo HTML uses class="result"
	resultPattern := regexp.MustCompile(`(?s)<div[^>]*class="[^"]*result[^"]*"[^>]*>.*?</div>\s*</div>`)
	resultBlocks := resultPattern.FindAllString(html, -1)

	// Pattern to extract link and title from result__a class
	linkPattern := regexp.MustCompile(`<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]*)"[^>]*>([^<]*)</a>`)

	// Pattern to extract snippet from result__snippet class
	snippetPattern := regexp.MustCompile(`(?s)<a[^>]*class="[^"]*result__snippet[^"]*"[^>]*>([^<]*(?:<[^>]*>[^<]*)*)</a>`)

	for _, block := range resultBlocks {
		if len(results) >= maxResults {
			break
		}

		linkMatch := linkPattern.FindStringSubmatch(block)
		if linkMatch == nil || len(linkMatch) < 3 {
			continue
		}

		rawURL := linkMatch[1]
		title := strings.TrimSpace(linkMatch[2])

		// DuckDuckGo wraps URLs in a redirect, extract the actual URL
		actualURL := extractActualURL(rawURL)
		if actualURL == "" {
			continue
		}

		// Skip DuckDuckGo internal links
		if strings.Contains(actualURL, "duckduckgo.com") {
			continue
		}

		snippet := ""
		snippetMatch := snippetPattern.FindStringSubmatch(block)
		if snippetMatch != nil && len(snippetMatch) >= 2 {
			snippet = cleanHTML(snippetMatch[1])
		}

		if title != "" && actualURL != "" {
			results = append(results, SearchResult{
				Title:   title,
				URL:     actualURL,
				Snippet: snippet,
			})
		}
	}

	return results
}

func extractActualURL(ddgURL string) string {
	// DuckDuckGo uses //duckduckgo.com/l/?uddg=<encoded_url>&... format
	if strings.Contains(ddgURL, "uddg=") {
		parsed, err := url.Parse(ddgURL)
		if err != nil {
			return ""
		}
		uddg := parsed.Query().Get("uddg")
		if uddg != "" {
			decoded, err := url.QueryUnescape(uddg)
			if err != nil {
				return uddg
			}
			return decoded
		}
	}

	// If it's already a direct URL
	if strings.HasPrefix(ddgURL, "http://") || strings.HasPrefix(ddgURL, "https://") {
		return ddgURL
	}

	return ""
}

func cleanHTML(s string) string {
	// Remove HTML tags
	tagPattern := regexp.MustCompile(`<[^>]*>`)
	s = tagPattern.ReplaceAllString(s, "")

	// Decode common HTML entities
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")

	// Clean up whitespace
	s = strings.TrimSpace(s)
	spacePattern := regexp.MustCompile(`\s+`)
	s = spacePattern.ReplaceAllString(s, " ")

	return s
}

func formatResults(results []SearchResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Found %d results:\n\n", len(results)))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("## %d. %s\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("**URL:** %s\n", r.URL))
		if r.Snippet != "" {
			sb.WriteString(fmt.Sprintf("**Snippet:** %s\n", r.Snippet))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
