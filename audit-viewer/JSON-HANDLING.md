# JSON Response Handling - Enhanced

## What's New

The improved audit viewer now has **specialized handling for JSON responses** with:

### 1. **Automatic JSON Detection**
The parser tries to parse responses as JSON first. If successful, it's displayed as a JSON response.

### 2. **Syntax Highlighting**
JSON responses get color-coded highlighting:
- **Keys**: Light blue (`#9cdcfe`)
- **String values**: Orange (`#ce9178`)
- **Numbers**: Light green (`#b5cea8`)
- **Booleans/null**: Blue (`#569cd6`)

### 3. **Metadata Badge**
Shows whether the JSON is an Object or Array, plus size info:
```
┌─────────────────────────────────────────────┐
│ JSON Object    5 keys • 1.2 KB             │
└─────────────────────────────────────────────┘
```

### 4. **Smart Formatting**
- Pretty-printed with 2-space indentation
- Line numbers preserved
- Copy button for entire JSON

## Examples

### Example 1: JSON Object Response
```json
{
  "status": "success",
  "data": {
    "id": 123,
    "name": "test"
  },
  "timestamp": 1234567890
}
```

**Display:**
```
┌────────────────────────────────────────┐
│ JSON Object    3 keys • 98 B      [📋] │
├────────────────────────────────────────┤
│ {                                      │
│   "status": "success",                 │
│   "data": {                            │
│     "id": 123,                         │
│     "name": "test"                     │
│   },                                   │
│   "timestamp": 1234567890              │
│ }                                      │
└────────────────────────────────────────┘
```

### Example 2: JSON Array Response
```json
[
  {"name": "file1.txt", "size": 1024},
  {"name": "file2.txt", "size": 2048}
]
```

**Display:**
```
┌────────────────────────────────────────┐
│ JSON Array     2 items • 86 B     [📋] │
├────────────────────────────────────────┤
│ [                                      │
│   {                                    │
│     "name": "file1.txt",               │
│     "size": 1024                       │
│   },                                   │
│   {                                    │
│     "name": "file2.txt",               │
│     "size": 2048                       │
│   }                                    │
│ ]                                      │
└────────────────────────────────────────┘
```

## Response Type Priority

The parser checks responses in this order:

1. **JSON** - Try `JSON.parse()` first
2. **Structured XML** - Check for `<tag>content</tag>` patterns
3. **Plain Text** - Fallback to markdown rendering

## Implementation Details

### Parser Function
```typescript
function parseToolResponse(response: string): ParsedToolResponse {
  // Try JSON first
  try {
    const parsed = JSON.parse(response);
    return {
      type: "json",
      raw: JSON.stringify(parsed, null, 2), // Pretty-print
    };
  } catch {
    // Not JSON, try XML...
  }
  
  // XML parsing...
  // Text fallback...
}
```

### Display Component
```typescript
if (parsed.type === "json") {
  const jsonObj = JSON.parse(parsed.raw);
  const isArray = Array.isArray(jsonObj);
  const itemCount = isArray ? jsonObj.length : Object.keys(jsonObj).length;
  
  return (
    <div className="json-response">
      <div className="response-header">
        <span className="json-badge">
          JSON {isArray ? 'Array' : 'Object'}
        </span>
        <span className="response-meta-info">
          {itemCount} {isArray ? 'items' : 'keys'} • {formatBytes(size)}
        </span>
      </div>
      {/* Syntax-highlighted JSON */}
    </div>
  );
}
```

### Syntax Highlighting
Uses simple regex-based highlighting:
```typescript
function highlightJSON(json: string): string {
  return json
    .replace(/(".*?")\s*:/g, '<span style="color: #9cdcfe">$1</span>:')  // Keys
    .replace(/:\s*(".*?")/g, ': <span style="color: #ce9178">$1</span>') // Strings
    .replace(/:\s*(-?\d+\.?\d*)/g, ': <span style="color: #b5cea8">$1</span>') // Numbers
    .replace(/:\s*(true|false)/g, ': <span style="color: #569cd6">$1</span>') // Booleans
    .replace(/:\s*(null)/g, ': <span style="color: #569cd6">$1</span>'); // Null
}
```

## Benefits

1. **Instant Recognition** - JSON badge makes it obvious
2. **Better Readability** - Colors help distinguish types
3. **Size Awareness** - Know if response is large before expanding
4. **Structure Preview** - See array vs object at a glance
5. **Easy Copy** - One-click copy of entire JSON

## Testing

To test JSON handling, look for tools that return JSON responses, such as:
- Database query tools
- API call tools
- Configuration readers
- List/search operations

Example tool that returns JSON:
```go
func (t *MyTool) Call(input map[string]interface{}) (interface{}, error) {
    result := map[string]interface{}{
        "status": "success",
        "data": []string{"item1", "item2"},
        "count": 2,
    }
    return result, nil
}
```

The SDK will automatically JSON-serialize this and the viewer will:
1. Parse it back to JSON
2. Show "JSON Object" badge
3. Display "3 keys • XX B"
4. Syntax highlight the output

## Future Enhancements

- **JSON Tree View** - Collapsible object/array nodes
- **Value Copying** - Copy individual values
- **JSON Path** - Show path to clicked property
- **Search** - Filter JSON by key/value
- **Format Toggle** - Switch between compact/pretty
