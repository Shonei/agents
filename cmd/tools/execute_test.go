package tools

import (
	"reflect"
	"testing"
)

func Test_parseParams(t *testing.T) {
	tests := []struct {
		name     string
		params   []string
		expected map[string]interface{}
	}{
		{
			"simple",
			[]string{"key:value"},
			map[string]interface{}{"key": "value"},
		},
		{
			"nested",
			[]string{"key.subkey:value"},
			map[string]interface{}{"key": map[string]interface{}{"subkey": "value"}},
		},
		{
			"multiple",
			[]string{"key1:value1", "key2:value2"},
			map[string]interface{}{"key1": "value1", "key2": "value2"},
		},
		{
			"numbers",
			[]string{"key1:1.2", "key2:123"},
			map[string]interface{}{"key1": 1.2, "key2": float64(123)},
		},
		{
			"booleans",
			[]string{"enabled:true"},
			map[string]interface{}{"enabled": true},
		},
		{
			"quoted strings",
			[]string{`key:"123"`},
			map[string]interface{}{"key": "123"},
		},
		{
			"nested multiple",
			[]string{"key.subkey1:value1", "key.subkey2:value2"},
			map[string]interface{}{"key": map[string]interface{}{"subkey1": "value1", "subkey2": "value2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseParams(tt.params)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
