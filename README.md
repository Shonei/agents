# Agents CLI

`agents` is a command-line tool for managing and interacting with AI agents locally. It supports creating specialized agents with specific system prompts, models, and tools, as well as Retrieval-Augmented Generation (RAG) capabilities using local files.

## Installation

To install the CLI, make sure you have [Go](https://go.dev/) installed (version 1.25+ recommended).

```bash
git clone https://github.com/Shonei/agents.git
cd agents
go install .
```

Ensure your `$GOPATH/bin` is in your system `PATH`.

## Configuration

By default, the configuration is stored in `~/.agents/config.yaml`. The CLI will automatically create this file if it doesn't exist.

### API Keys

You need to provide a Gemini API key. You can set it in two ways:

1. **Environment Variable** (Recommended):
   ```bash
   export GEMINI_API_KEY="your-gemini-key"
   ```

2. **Config File**:
   Add it directly to `~/.agents/config.yaml`:
   ```yaml
   gemini_api_key: "your-gemini-key"
   ```

If you plan to use the GitHub PR Review tools, you will also need a GitHub Personal Access Token (with `repo` scope):

1. **Environment Variable** (Recommended):
   ```bash
   export GITHUB_TOKEN="your-github-token"
   ```

2. **Config File**:
   ```yaml
   github_token: "your-github-token"
   ```

### Audit Logging

The CLI supports audit logging to help you understand and debug agent conversations. By default, this is disabled. To enable it, add the `audit` section to your `~/.agents/config.yaml`.

You can choose to store logs in a **file** or a **database**.

#### File Logging (Default)

```yaml
audit:
  enabled: true
  type: file
  path: /absolute/path/to/audit/dir
```

#### Database Logging

To log to the internal DuckDB database:

```yaml
audit:
  enabled: true
  type: database

# Ensure you have the database path configured
db_path: /absolute/path/to/agents.db
```

**Note:** The directory specified in `path` (for file logging) or `db_path` (for database logging) must exist. Use absolute paths.

### Audit Viewer

The project includes a web-based **Audit Viewer** for reviewing conversation logs:

```bash
./start-audit-viewer.sh
```

Then open http://localhost:3000 in your browser. See [audit-viewer/GUIDE.md](audit-viewer/GUIDE.md) for detailed documentation.

## Usage

### Managing Agents

#### Add an Agent
Create a new agent with a name, system prompt, and model. You can optionally attach tools.

```bash
agents add \
  --name "coder" \
  --system-prompt "You are an expert Go developer." \
  --model "gemini-3.1-pro-preview" \
  --tools "bash,write_to_file,view_file"
```

**Supported Models:**
*   `gemini-3.1-pro-preview`
*   `gemini-3.1-flash-lite`
*   `gemini-3.1-flash-image-preview`

**Available Tools:**
*   `fetch_url`: Fetch content from a URL.
*   `time`: Get the current date and time.
*   `write_to_file`: Create or overwrite files.
*   `view_file`: Read file contents.
*   `list_dir`: List directory contents.
*   `bash`: Execute shell commands.
*   `str_replace_editor`: Safely edits existing files using precise string replacement or insertion.
*   `rag`: Search information in your local code base.
*   `memory`: Long-term memory storage (store/retrieve).
*   `github_pr_details`: Fetch PR metadata (title, body, state, author).
*   `github_pr_diff`: Fetch the raw diff/patch of a PR.
*   `github_pr_comments`: Fetch existing review and issue comments on a PR.
*   `git_checkout_pr`: Fetch and checkout a PR branch locally.
*   `github_pr_review`: Submit a formal PR review with inline comments.

**Provider-executed (server-side) tools:**

These tools run inside the model provider, so the SDK never invokes them locally. After each turn that used them, a short `Grounding:` summary is printed (search queries, sources, retrieved URLs) and the same data is appended to the audit log as a `grounding` event.

*   `google_search`: Gemini's grounding-with-Google-Search. Model decides when to issue queries.
*   `url_context`: Gemini's URL-context tool. Model fetches URLs it finds in the prompt and grounds the answer on their contents.

#### System Prompt Templating
System prompts support dynamic content via Go templates (e.g., adding current time or file listings).
See [Templating System Prompts](docs/templating_system_prompts.md) for a full guide.

#### List Agents
View all configured agents.

```bash
agents list
```

### Interacting with Agents

#### Engage (Chat)
Chat with an agent directly from the terminal.

**Interactive Mode:**
```bash
agents engage coder
```

**Single Prompt:**
```bash
agents engage coder --prompt "Write a hello world in Go"
```

**Piped Input:**
```bash
echo "Write a hello world in Go" | agents engage coder
```

### Example: PR Review Agent
You can configure an agent to act as an automated PR reviewer using the GitHub tools. See the example configuration in `examples/pr-reviewer-config.yaml`.

```bash
export GITHUB_TOKEN="your-github-token"
export AGENTS_CONFIG="./examples/pr-reviewer-config.yaml"
agents engage pr-reviewer --prompt "Can you review PR #123 in the owner/repo repository?"
```

### Retrieval-Augmented Generation (RAG)

The CLI supports RAG to let agents answer questions based on your local files. This uses **DuckDB** for storage and **Gemini** for embeddings.

#### Quick Start

1. **Index your documents:**
   ```bash
   agents rag index --dir ./my-docs
   ```

#### Available Commands

- `agents rag index` - Index a directory into the RAG store
- `agents rag list-stores` - List all available RAG stores
- `agents rag search` - Search a RAG store for relevant documents
- `agents rag delete-store` - Delete a RAG store

For detailed documentation on RAG commands, see [RAG Commands Reference](docs/rag_commands.md).


## Advanced Configuration

You can override the default config file location:

```bash
agents list --config ./my-config.yaml
```

Or use the environment variable `agents_CONFIG`.

```bash
export agents_CONFIG=./custom-config.yaml
```
