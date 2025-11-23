package tools

import (
	"fmt"
)

// CalculatorTool performs basic arithmetic operations
type CalculatorTool struct{}

func (c *CalculatorTool) Name() string {
	return "calculator"
}

func (c *CalculatorTool) Description() string {
	return "Performs basic arithmetic operations (add, subtract, multiply, divide) on two numbers"
}

func (c *CalculatorTool) Init(config map[string]string) {
}

func (c *CalculatorTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "The operation to perform",
				"enum":        []string{"add", "subtract", "multiply", "divide"},
			},
			"a": map[string]interface{}{
				"type":        "number",
				"description": "The first number",
			},
			"b": map[string]interface{}{
				"type":        "number",
				"description": "The second number",
			},
		},
		"required": []string{"operation", "a", "b"},
	}
}

func (c *CalculatorTool) Call(input map[string]interface{}) (interface{}, error) {
	operation, ok := input["operation"].(string)
	if !ok {
		return "", fmt.Errorf("operation must be a string")
	}

	a, ok := input["a"].(float64)
	if !ok {
		return "", fmt.Errorf("a must be a number")
	}

	b, ok := input["b"].(float64)
	if !ok {
		return "", fmt.Errorf("b must be a number")
	}

	var result float64
	switch operation {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b == 0 {
			return "", fmt.Errorf("cannot divide by zero")
		}
		result = a / b
	default:
		return "", fmt.Errorf("unknown operation: %s", operation)
	}

	return map[string]interface{}{
		"result": result,
	}, nil
}
