package tools

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowseURLToolSmoke(t *testing.T) {
	if os.Getenv("AGENTS_BROWSER_TEST") != "1" {
		t.Skip("set AGENTS_BROWSER_TEST=1 to run browser smoke test")
	}

	testURL := os.Getenv("AGENTS_BROWSER_TEST_URL")
	if testURL == "" {
		testURL = "https://example.com"
	}
	wait := os.Getenv("AGENTS_BROWSER_TEST_WAIT")
	if wait == "" {
		wait = "load"
	}
	timeoutSeconds := 30
	if rawTimeout := os.Getenv("AGENTS_BROWSER_TEST_TIMEOUT"); rawTimeout != "" {
		parsed, err := strconv.Atoi(rawTimeout)
		require.NoError(t, err)
		timeoutSeconds = parsed
	}
	settleMillis := 1000
	if rawSettle := os.Getenv("AGENTS_BROWSER_TEST_SETTLE_MS"); rawSettle != "" {
		parsed, err := strconv.Atoi(rawSettle)
		require.NoError(t, err)
		settleMillis = parsed
	}
	headless := false
	if rawHeadless := os.Getenv("AGENTS_BROWSER_TEST_HEADLESS"); rawHeadless != "" {
		parsed, err := strconv.ParseBool(rawHeadless)
		require.NoError(t, err)
		headless = parsed
	}

	tool := &BrowseURLTool{}
	tool.Init(map[string]string{"require_confirmation": "false"}, nil)

	start := time.Now()
	result, err := tool.Call(map[string]interface{}{
		"url":                 testURL,
		"wait":                wait,
		"timeout_seconds":     timeoutSeconds,
		"settle_milliseconds": settleMillis,
		"headless":            headless,
	})
	t.Logf("browse_url rendered %s with wait=%s timeout=%ds settle=%dms headless=%t in %s", testURL, wait, timeoutSeconds, settleMillis, headless, time.Since(start).Round(time.Millisecond))
	require.NoError(t, err)

	text, ok := result.(string)
	require.True(t, ok)
	assert.Contains(t, text, testURL)
}
