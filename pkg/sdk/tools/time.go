package tools

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
