# AI Agent Instructions (AGENTS.md)

Welcome to the `agents` repository! This file provides context, architectural guidelines, and development workflows for AI coding assistants (like Cursor, Copilot, or Claude Code) working on this codebase.

## Project Overview

`agents` is a command-line tool for managing and interacting with AI agents locally. It supports creating specialized agents with specific system prompts, models, and tools. It also features Retrieval-Augmented Generation (RAG) capabilities using local files and an Audit Logging system to track agent conversations.

The project consists of two main components:
1. **Go CLI**: The core application, handling agent logic, tool execution, RAG indexing, and CLI interactions.
2. **Audit Viewer**: A lightweight web application to visualize and review conversation logs stored in DuckDB.

## Tech Stack

### Backend (CLI)
- **Language**: Go (1.25+)
- **CLI Framework**: Cobra (`github.com/spf13/cobra`)
- **Database**: DuckDB (`github.com/duckdb/duckdb-go/v2`) used for both RAG vector storage and Audit logging.
- **Markdown Rendering**: Glamour (`github.com/charmbracelet/glamour`)

### Frontend (Audit Viewer)
- **Runtime/Package Manager**: Bun
- **Framework**: React 18 with TypeScript
- **Database Driver**: DuckDB (WASM/Node bindings)

## Repository Structure

- `cmd/`: Cobra CLI commands (e.g., `add.go`, `engage.go`, `rag.go`).
- `pkg/`: Core application logic.
  - `pkg/sdk/`: The core agent SDK, handling interactions with the LLM (Gemini) and tool execution.
  - `pkg/storage/`: Database interactions (DuckDB) for RAG and Audit logs.
  - `pkg/config/`: Configuration parsing and management.
  - `pkg/utils/`: Helper functions.
- `audit-viewer/`: The React/Bun web application for viewing audit logs.
- `examples/`: Example configurations (e.g., PR reviewer agent).
- `docs/`: Additional documentation.

## Development Workflow

### Go CLI
The project uses a `Makefile` for common tasks. Always ensure your code passes these checks before considering a task complete.

- **Build**: `make build` (outputs to `bin/`)
- **Test**: `make test`
- **Lint**: `make lint`
- **Lint & Fix**: `make lint-fix`

### Audit Viewer
To work on the web interface:
```bash
cd audit-viewer
bun install
bun run dev
```
Alternatively, use the root script: `./start-audit-viewer.sh`

## Coding Standards & Guidelines

1. **Strict Linting**: The project uses `golangci-lint` with a strict configuration (`.golangci.yaml`).
   - Formatting is enforced via `gofumpt` and `gci`.
   - Imports must be grouped: standard library, third-party, and local (`github.com/Shonei/agents`).
   - `nlreturn` is enabled: ensure there is a blank line before `return` statements.
   - Always run `make lint-fix` after making changes to Go files.

2. **Error Handling**: Do not swallow errors. Wrap errors with context where appropriate to aid debugging.

3. **Security**: 
   - `gosec` is enabled in the linter.
   - **Never** hardcode API keys (Gemini, GitHub) or tokens in the codebase. Always read from the configuration file or environment variables.

4. **Tool Implementation**:
   - When adding a new local tool, ensure it is registered correctly in the SDK and exposed via the CLI configuration.
   - Differentiate clearly between local tools (executed by the Go binary) and provider-executed tools (like `google_search` or `url_context`).

5. **Database (DuckDB)**:
   - When modifying database schemas (RAG or Audit), ensure backward compatibility or provide clear migration paths if necessary.

## AI Assistant Directives

- **Read Before Writing**: Use your tools to read existing files (`pkg/sdk`, `cmd/`, etc.) to understand the current patterns before implementing new features.
- **Surgical Edits**: Prefer precise string replacements over full file rewrites when modifying existing code.
- **Verify**: After writing Go code, always run `make lint` and `make test` to ensure you haven't broken the build or violated formatting rules.
- **Context**: Remember that this is a "meta" project—an AI agent framework. When writing prompts or system instructions within the code, be mindful of escaping and templating rules (Go templates are used for system prompts).
