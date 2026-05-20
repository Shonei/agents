package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const viewFIleTestContent = `package tools

import (
	"fmt"
	"time"
)

// TimeTool returns the current time
type TimeTool struct{}

func (t *TimeTool) Name() string {
	return "time"
}

func (t *TimeTool) Description() string {
	return "Returns the current date and time."
}

func (t *TimeTool) Init(config map[string]string) {
}

func (t *TimeTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Optional format. 'iso' for ISO8601, 'unix' for Unix timestamp. Default is human readable.",
				"enum":        []string{"iso", "unix", "human"},
			},
		},
	}
}

func (t *TimeTool) Call(input map[string]interface{}) (interface{}, error) {
	now := time.Now()
	format, _ := input["format"].(string)

	switch format {
	case "iso":
		return now.Format(time.RFC3339), nil
	case "unix":
		return fmt.Sprintf("%d", now.Unix()), nil
	default:
		return now.Format("Monday, 02 Jan 2006 15:04:05 MST"), nil
	}
}
`

// careful with spaces and tabs when editing. It is hard to see when debugging
const expectOutput = `%s (1.0 KB, 48 lines, modified %s) — lines 5-10 of 48

  ⋮ (4 lines above)
     5|	"time"
     6|)
     7|
     8|// TimeTool returns the current time
     9|type TimeTool struct{}
    10|
  ⋮ (38 lines below)
`

func TestViewFile_readsFile(t *testing.T) {
	viewTool := &ViewFileTool{}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	path := filepath.Join(cwd, "testfiles", "view_file.txt")

	if err := os.WriteFile(path, []byte(viewFIleTestContent), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fileStat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}

	input := map[string]interface{}{
		"path":   path,
		"offset": 5,
		"limit":  6,
	}

	result, err := viewTool.Call(input)
	if err != nil {
		t.Fatalf("failed to view file: %v", err)
	}

	resultWithTime := fmt.Sprintf(expectOutput, path, fileStat.ModTime().Format(time.RFC3339))

	if result != resultWithTime {
		t.Fatalf("expected %s, got %s", resultWithTime, result)
	}
}
