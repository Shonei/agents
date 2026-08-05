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
  - Function calls / responses (local tools: `bash`, `write_to_file`, `view_file`, …)
  - **Web tools / grounding** — provider-executed tools such as `google_search`,
    `url_context`, `web_search`, and `web_fetch` do **not** appear as function
    calls. They are logged as `grounding` events and rendered as “Web tools”
    blocks showing which tool ran, search queries, retrieved URLs, and sources.
  - Plan / todo snapshots, router handoffs, compaction summaries
