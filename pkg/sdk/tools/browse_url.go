package tools

import (
	"fmt"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/fatih/color"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/sdk/tools/browser"
	"github.com/Shonei/agents/pkg/utils"
)

const browseURLMaxLen = 100000

// BrowseURLTool renders a URL in headless Chromium and returns cleaned page
// content. It is a heavier fallback for docs pages that need JavaScript or
// reject simple HTTP fetches.
type BrowseURLTool struct {
	requireConfirmation bool
}

func (b *BrowseURLTool) Name() string {
	return "browse_url"
}

func (b *BrowseURLTool) Description() string {
	return "Renders a URL in Chromium (via chromedp), waits for the page to load and settle, and returns cleaned markdown/text. Use this as a fallback for documentation pages that need JavaScript or reject simple HTTP fetches. The tool is read-only and does not click, submit forms, or log in. It runs visibly by default so the user can see what the agent is viewing; pass headless:true to hide the browser. By default the user is prompted to confirm navigation; set require_confirmation: \"false\" in the tool config to auto-approve."
}

func (b *BrowseURLTool) Init(config map[string]string, _ *config.ConfigFactory) {
	b.requireConfirmation = true
	if val, ok := config["require_confirmation"]; ok && val == "false" {
		b.requireConfirmation = false
	}
}

func (b *BrowseURLTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The HTTP(S) URL to render. Bare hosts are normalized to https://.",
				"example":     "https://developer.ebay.com/api-docs/sell/inventory/overview.html",
			},
			"wait": map[string]interface{}{
				"type":        "string",
				"description": "Wait strategy: domcontentloaded, load, or networkidle. Defaults to load.",
				"enum":        []interface{}{"domcontentloaded", "load", "networkidle"},
				"example":     "load",
			},
			"timeout_seconds": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum time for launch/navigation/extraction. Defaults to 45 seconds.",
				"example":     45,
			},
			"wait_for_selector": map[string]interface{}{
				"type":        "string",
				"description": "Optional CSS selector to wait for before extracting content, e.g. main or article.",
				"example":     "main",
			},
			"settle_milliseconds": map[string]interface{}{
				"type":        "integer",
				"description": "Extra time to wait after the selected load condition and selector wait, giving client-side JavaScript time to render. Defaults to 1000 milliseconds. Set to 0 to disable.",
				"example":     1000,
			},
			"headless": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to run Chromium headlessly. Defaults to false so the user can watch the page load locally.",
				"example":     false,
			},
		},
		"required": []interface{}{"url"},
	}
}

type BrowseURLToolInput struct {
	URL             string `json:"url"`
	Wait            string `json:"wait"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	WaitForSelector string `json:"wait_for_selector"`
	SettleMillis    *int   `json:"settle_milliseconds"`
	Headless        *bool  `json:"headless"`
}

type browsedPageContent struct {
	Title             string
	Description       string
	Markdown          string
	ExtractionSource  string
	Truncated         bool
	FellBackToBody    bool
	OriginalLength    int
	ExtractedSelector string
}

func (b *BrowseURLTool) Call(input map[string]interface{}) (interface{}, error) {
	var in BrowseURLToolInput
	if err := mapstruct(input, &in); err != nil {
		return "", err
	}

	if in.URL == "" {
		return "", sdk.NewAIError("url is required")
	}
	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		in.URL = "https://" + in.URL
	}
	if in.Wait == "" {
		in.Wait = browser.WaitLoad
	}
	if in.TimeoutSeconds <= 0 {
		in.TimeoutSeconds = 45
	}
	settleMillis := 1000
	if in.SettleMillis != nil {
		settleMillis = *in.SettleMillis
	}
	if settleMillis < 0 {
		return "", sdk.NewAIError("settle_milliseconds must be >= 0")
	}
	headless := false
	if in.Headless != nil {
		headless = *in.Headless
	}
	browserMode := "headless"
	if !headless {
		browserMode = "visible"
	}

	if b.requireConfirmation {
		color.New(color.FgYellow, color.Bold).Printf("\nYou are about to render the following URL in a %s browser:\n", browserMode)
		color.Cyan("  %s", in.URL)
		answer, _ := utils.AskUserConfirmation()
		switch answer {
		case utils.ToolExecutionYes:
			// continue
		case utils.ToolExecutionSkip:
			return "<exitcode>1</exitcode><output>Skipped by user</output>", nil
		case utils.ToolExecutionAbort:
			utils.NewExitError().WithMessage("tool execution aborted by user").Done()
		case utils.ToolExecutionUnknown:
			utils.NewExitError().WithMessage("unknown user choice").Done()
		}
	} else {
		color.New(color.FgYellow, color.Bold).Printf("\nRendering URL in %s browser (auto-confirmed):\n", browserMode)
		color.Cyan("  %s", in.URL)
	}

	rendered, err := browser.FetchURL(browser.FetchOptions{
		URL:             in.URL,
		Wait:            in.Wait,
		Timeout:         time.Duration(in.TimeoutSeconds) * time.Second,
		WaitForSelector: in.WaitForSelector,
		Headless:        &headless,
		Settle:          time.Duration(settleMillis) * time.Millisecond,
	})
	if err != nil {
		return "", fmt.Errorf("failed to browse URL: %w", err)
	}

	page, err := extractBrowsedPage(rendered.HTML, rendered.Title)
	if err != nil {
		return "", fmt.Errorf("failed to extract browsed page content: %w", err)
	}

	var sb strings.Builder
	if page.Title != "" {
		sb.WriteString("Title: " + strings.TrimSpace(page.Title) + "\n")
	}
	if page.Description != "" {
		sb.WriteString("Description: " + strings.TrimSpace(page.Description) + "\n")
	}
	sb.WriteString("URL: " + rendered.RequestedURL + "\n")
	if rendered.FinalURL != "" && rendered.FinalURL != rendered.RequestedURL {
		sb.WriteString("Final URL: " + rendered.FinalURL + "\n")
	}
	sb.WriteString("Wait: " + rendered.Wait + "\n")
	sb.WriteString("Extraction source: " + page.ExtractionSource + "\n")
	if page.FellBackToBody {
		sb.WriteString("Note: main/article extraction was unavailable; used body content.\n")
	}
	if page.Truncated {
		fmt.Fprintf(&sb, "Note: content truncated from %d bytes to %d bytes.\n", page.OriginalLength, browseURLMaxLen)
	}
	sb.WriteString("\n")
	sb.WriteString(page.Markdown)

	return sb.String(), nil
}

func extractBrowsedPage(html, fallbackTitle string) (*browsedPageContent, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())
	if title == "" {
		title = fallbackTitle
	}
	description, _ := doc.Find("meta[name='description']").First().Attr("content")

	doc.Find("script, style, nav, footer, header, noscript, svg, iframe, link[rel='stylesheet']").Remove()

	selector, selection := selectMainContent(doc)
	bodyFallback := selector == "body"

	contentHTML, err := selection.Html()
	if err != nil {
		return nil, err
	}

	converter := md.NewConverter("", true, nil)
	markdown, err := converter.ConvertString(contentHTML)
	if err != nil {
		return nil, err
	}

	markdown = strings.TrimSpace(markdown)
	originalLen := len(markdown)
	truncated := false
	if len(markdown) > browseURLMaxLen {
		markdown = markdown[:browseURLMaxLen] + "\n... (content truncated)"
		truncated = true
	}

	return &browsedPageContent{
		Title:             strings.TrimSpace(title),
		Description:       strings.TrimSpace(description),
		Markdown:          markdown,
		ExtractionSource:  selector,
		Truncated:         truncated,
		FellBackToBody:    bodyFallback,
		OriginalLength:    originalLen,
		ExtractedSelector: selector,
	}, nil
}

func selectMainContent(doc *goquery.Document) (string, *goquery.Selection) {
	selectors := []string{
		"main",
		"article",
		"[role='main']",
		".markdown-body",
		".docs-content",
		".documentation",
		"#content",
		"body",
	}

	for _, selector := range selectors {
		selection := doc.Find(selector).First()
		if selection.Length() == 0 {
			continue
		}

		text := strings.TrimSpace(selection.Text())
		if selector == "body" || len(text) >= 20 {
			return selector, selection
		}
	}

	return "body", doc.Find("body").First()
}
