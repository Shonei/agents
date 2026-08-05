package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBrowsedPagePrefersMainContent(t *testing.T) {
	html := `<!doctype html>
<html>
  <head>
    <title>API Docs</title>
    <meta name="description" content="Rendered docs">
  </head>
  <body>
    <nav>Navigation that should not dominate output</nav>
    <main>
      <h1>List products</h1>
      <p>This endpoint returns products for the authenticated shop.</p>
    </main>
  </body>
</html>`

	page, err := extractBrowsedPage(html, "")
	require.NoError(t, err)

	assert.Equal(t, "API Docs", page.Title)
	assert.Equal(t, "Rendered docs", page.Description)
	assert.Equal(t, "main", page.ExtractionSource)
	assert.False(t, page.FellBackToBody)
	assert.Contains(t, page.Markdown, "List products")
	assert.Contains(t, page.Markdown, "authenticated shop")
	assert.NotContains(t, page.Markdown, "Navigation")
}

func TestExtractBrowsedPageFallsBackToBody(t *testing.T) {
	html := `<!doctype html>
<html>
  <head><title>Fallback</title></head>
  <body>
    <div>
      <h1>Body only docs</h1>
      <p>This page does not expose a main or article element.</p>
    </div>
  </body>
</html>`

	page, err := extractBrowsedPage(html, "")
	require.NoError(t, err)

	assert.Equal(t, "body", page.ExtractionSource)
	assert.True(t, page.FellBackToBody)
	assert.Contains(t, page.Markdown, "Body only docs")
}

func TestExtractBrowsedPageTruncates(t *testing.T) {
	html := `<!doctype html><html><body><main><p>` +
		strings.Repeat("x", browseURLMaxLen+500) +
		`</p></main></body></html>`

	page, err := extractBrowsedPage(html, "")
	require.NoError(t, err)

	assert.True(t, page.Truncated)
	assert.Greater(t, page.OriginalLength, browseURLMaxLen)
	assert.Contains(t, page.Markdown, "... (content truncated)")
}

func TestToolNamesIncludesBrowseURL(t *testing.T) {
	assert.Contains(t, ToolNames(), "browse_url")
}
