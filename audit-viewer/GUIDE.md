# Audit Viewer Setup and Usage Guide

## Overview

The Audit Viewer is a Bun/React-based web application that provides an intuitive interface for reviewing audit logs stored in the `audit/` folder. It features markdown rendering, syntax highlighting, and a clean dark theme.

## Prerequisites

- **Bun**: JavaScript runtime and bundler
  ```bash
  curl -fsSL https://bun.sh/install | bash
  ```

## Installation

1. Navigate to the audit-viewer directory:
   ```bash
   cd audit-viewer
   ```

2. Install dependencies:
   ```bash
   bun install
   ```

## Starting the Server

### Option 1: Using the startup script (recommended)
```bash
./start-audit-viewer.sh
```

### Option 2: Manual start
```bash
cd audit-viewer
bun run dev
```

The server will start on **http://localhost:3000**

## Features

### 📋 Audit List Sidebar
- All audit files are listed chronologically (newest first)
- Shows truncated hash and timestamp
- Click any audit to view its contents

### 📝 Message Type Rendering

The viewer intelligently renders different message types:

1. **System Initialization** (Purple)
   - Displays session ID
   - Renders system prompt as markdown

2. **User Messages** (Blue)
   - Initial user requests
   - All text rendered as markdown

3. **Assistant Messages** (Green)
   - AI responses
   - Full markdown support with syntax highlighting

4. **Function Calls** (Orange)
   - Tool invocations
   - JSON input formatted and syntax-highlighted

5. **Function Responses** (Orange)
   - Tool outputs
   - Auto-detects JSON vs text
   - JSON: syntax-highlighted
   - Text: rendered as markdown

### 🎨 Markdown Features

All text content supports full GitHub Flavored Markdown:

- **Headers** (H1, H2, H3)
- **Bold** and *italic* text
- `Inline code`
- Code blocks with syntax highlighting
- Lists (ordered and unordered)
- Blockquotes
- Tables
- Links
- And more...

### 🌙 Dark Theme

- Optimized for extended viewing sessions
- High contrast for readability
- Color-coded message types
- Syntax-highlighted code blocks

## Architecture

### Server (server.ts)
- Bun-based HTTP server
- Serves both static assets and API endpoints
- On-the-fly TypeScript transpilation for React app

### API Endpoints

#### `GET /api/audits`
Lists all audit files with metadata:
```json
[
  {
    "filename": "hash_timestamp.json",
    "hash": "08d7027b",
    "fullHash": "08d7027befc8b48d9fa443def23d570e5fc577f2fe309944190c64664617a77a",
    "timestamp": 1764763918,
    "date": "2025-12-03T12:11:58.000Z"
  }
]
```

#### `GET /api/audit/:filename`
Returns parsed JSONL content as array of message objects:
```json
[
  {
    "id": "session-id",
    "system_prompt": "...",
    "type": "initial_message",
    "content": "..."
  }
]
```

### Frontend (app.tsx)
- React 18 with TypeScript
- react-markdown for content rendering
- remark-gfm for GitHub Flavored Markdown
- Responsive layout with sidebar navigation

## File Format

Audit files are expected in **JSON Lines (JSONL)** format:
- Each line is a valid JSON object
- No commas between lines
- Common message types:
  - Session initialization (contains `id` and `system_prompt`)
  - `initial_message` (user input)
  - `assistant_message` (AI response)
  - `function_call` (tool invocation)
  - `function_response` (tool output)

## Development

### Project Structure
```
audit-viewer/
├── server.ts          # Bun server with API and static serving
├── app.tsx            # React application
├── index.html         # HTML template with embedded CSS
├── package.json       # Dependencies
├── tsconfig.json      # TypeScript configuration
└── README.md          # Documentation
```

### Adding Features

**Server-side changes:**
- Edit `server.ts` to add new API endpoints
- Restart the dev server to see changes

**Client-side changes:**
- Edit `app.tsx` for React components
- Changes are transpiled on-the-fly by Bun

**Styling:**
- All styles are in `index.html`
- No build step required

## Troubleshooting

### Server won't start
- Ensure Bun is installed: `bun --version`
- Check port 3000 is available: `lsof -i :3000`
- Verify dependencies: `bun install`

### Audits not showing
- Verify audit files exist in `../audit/` directory
- Check audit files are valid JSON Lines format
- Check browser console for errors

### Markdown not rendering
- Ensure react-markdown is installed
- Check browser console for import errors
- Verify content is valid markdown

## Performance

- Lightweight single-server architecture
- Fast TypeScript transpilation with Bun
- No build step for development
- Handles large audit files efficiently
- On-demand loading of audit content

## Security Note

⚠️ **This application is designed for LOCAL USE ONLY**
- No authentication
- Direct file system access
- Exposes audit content via HTTP
- Do not expose to public networks

## Future Enhancements

Possible improvements:
- Search functionality across audits
- Filter by message type
- Export audit as formatted document
- Diff view between audits
- Collapsible message sections
- Theme customization
- Keyboard navigation

## License

Same as parent project.
