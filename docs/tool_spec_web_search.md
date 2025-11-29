# Tool Specification: `web_search`

## Overview
This tool allows the agent to perform Google searches using the Google Custom Search JSON API. It is essential for retrieving up-to-date information, finding documentation, debugging errors, and general knowledge retrieval.

## Tool Declaration (JSON Schema)

```json
{
  "name": "web_search",
  "description": "Performs a Google search using the Google Custom Search JSON API. Returns a list of results containing titles, links, and snippets.",
  "parameters": {
    "type": "OBJECT",
    "properties": {
      "query": {
        "description": "The search query string.",
        "type": "STRING"
      },
      "count": {
        "description": "The number of results to return. Defaults to 5. Maximum is usually 10 per request.",
        "type": "INTEGER"
      },
      "start_index": {
        "description": "The index of the first result to return (1-based). Useful for pagination.",
        "type": "INTEGER"
      },
      "file_type": {
        "description": "Restrict results to a specific file extension (e.g., 'pdf', 'md').",
        "type": "STRING"
      }
    },
    "required": ["query"]
  }
}
```

## Implementation Details

### Backend Requirements
The backend executing this tool requires the following configuration:

1.  **Google Custom Search JSON API**: Enabled in the Google Cloud Console.
2.  **API Key**: Stored in environment variable `GOOGLE_API_KEY`.
3.  **Search Engine ID (CX)**: Stored in environment variable `GOOGLE_CSE_ID`. The Custom Search Engine should be configured to search the entire web or a broad set of technical sites.

### Request Format
The tool will make a `GET` request to:
`https://www.googleapis.com/customsearch/v1`

**Query Parameters:**
- `key`: `GOOGLE_API_KEY`
- `cx`: `GOOGLE_CSE_ID`
- `q`: `query` (from arguments)
- `num`: `count` (default 5, max 10)
- `start`: `start_index` (default 1)
- `fileType`: `file_type` (optional)

### Response Format
The raw API response should be parsed to return a concise list of objects to the LLM.

**Example Internal Output:**
```json
[
  {
    "title": "Go Programming Language",
    "link": "https://go.dev/",
    "snippet": "Go is an open source programming language that makes it easy to build simple, reliable, and efficient software."
  },
  {
    "title": "Documentation - The Go Programming Language",
    "link": "https://go.dev/doc/",
    "snippet": "Documentation. Go is an open source project developed by a team at Google and many contributors from the open source community."
  }
]
```

## Usage Guidelines
- **Idempotency**: This is a read-only operation.
- **Error Handling**: 
  - If the API key is invalid, return a clear error message.
  - If no results are found, return an empty list or a message stating "No results found for query: [query]".
