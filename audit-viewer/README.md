# Audit Viewer

A Bun/React application for reviewing audit logs stored in JSON Lines format.

## Features

- 📋 Browse all audit files in the sidebar
- 🔍 View detailed audit content with proper formatting
- 📝 Markdown rendering for text blocks
- 🎨 Syntax-highlighted JSON for function calls/responses
- 🌙 Dark theme optimized for extended viewing
- ⚡ Fast single-server setup with Bun

## Installation

```bash
cd audit-viewer
bun install
```

## Usage

```bash
bun run dev
```

Then open http://localhost:3000 in your browser.

## Architecture

- **Server**: Bun server serving both the static React app and API endpoints
- **Frontend**: React with TypeScript for the UI
- **Markdown**: react-markdown with GitHub Flavored Markdown support
- **Styling**: Inline CSS with dark theme

## API Endpoints

- `GET /api/audits` - List all audit files
- `GET /api/audit/:filename` - Get specific audit file content

## File Structure

```
audit-viewer/
├── server.ts       # Bun server with API and static file serving
├── app.tsx         # React application
├── index.html      # HTML template with styling
├── package.json    # Dependencies
└── README.md       # This file
```

## Notes

- Audit files are expected to be in JSON Lines (JSONL) format
- Each line in an audit file should be a valid JSON object
- The viewer automatically parses and displays different message types:
  - System prompts
  - User messages
  - Assistant responses
  - Function calls
  - Function responses
