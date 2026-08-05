package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirecrawlFetchTool(t *testing.T) {
	var gotRequest firecrawlScrapeRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/scrape", r.URL.Path)
		assert.Equal(t, "Bearer fc-test", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotRequest))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"success": true,
			"data": {
				"markdown": "# Docs\n\nRendered content",
				"html": "<main><h1>Docs</h1></main>",
				"metadata": {
					"title": "Example Docs",
					"sourceURL": "https://example.com/docs",
					"url": "https://example.com/docs?redirected=true"
				}
			}
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	tool := &FirecrawlFetchTool{}
	tool.Init(map[string]string{
		"api_key":  "fc-test",
		"base_url": server.URL,
	}, nil)

	result, err := tool.Call(map[string]interface{}{
		"url":                "https://example.com/docs",
		"timeout_seconds":    5,
		"wait_milliseconds":  123,
		"only_clean_content": true,
		"include_html":       true,
	})
	require.NoError(t, err)

	assert.Equal(t, "https://example.com/docs", gotRequest.URL)
	assert.Equal(t, []string{"markdown", "html"}, gotRequest.Formats)
	assert.True(t, gotRequest.OnlyMainContent)
	assert.True(t, gotRequest.OnlyCleanContent)
	assert.Equal(t, 123, gotRequest.WaitFor)
	assert.Equal(t, 5000, gotRequest.Timeout)

	text, ok := result.(string)
	require.True(t, ok)
	assert.Contains(t, text, "Title: Example Docs")
	assert.Contains(t, text, "Source URL: https://example.com/docs")
	assert.Contains(t, text, "Final URL: https://example.com/docs?redirected=true")
	assert.Contains(t, text, "# Docs")
	assert.Contains(t, text, "## HTML")
	assert.Contains(t, text, "<main><h1>Docs</h1></main>")
}

func TestFirecrawlFetchToolNamesIncludesTool(t *testing.T) {
	assert.Contains(t, ToolNames(), "firecrawl_fetch")
}
