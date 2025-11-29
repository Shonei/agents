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

You need to provide API keys for the models you intend to use. You can set them in two ways:

1. **Environment Variables** (Recommended):
   ```bash
   export ANTHROPIC_API_KEY="your-claude-key"
   export GEMINI_API_KEY="your-gemini-key"
   ```

2. **Config File**:
   Add them directly to `~/.agents/config.yaml`:
   ```yaml
   claude_api_key: "your-claude-key"
   gemini_api_key: "your-gemini-key"
   ```

### Audit Logging

The CLI supports audit logging to help you understand and debug agent conversations. By default, this is disabled. To enable it, add the `audit` section to your `~/.agents/config.yaml`:

```yaml
audit:
  enabled: true
  type: file
  path: /absolute/path/to/audit/dir
```

**Note:** The directory specified in `path` must exist. Use an absolute path to ensure logs are always written to the same location regardless of your current working directory.

## Usage

### Managing Agents

#### Add an Agent
Create a new agent with a name, system prompt, and model. You can optionally attach tools.

```bash
agents add \
  --name "coder" \
  --system-prompt "You are an expert Go developer." \
  --model "claude-sonnet-4-5-20250929" \
  --tools "bash,write_to_file,view_file"
```

**Supported Models:**
*   `claude-sonnet-4-5-20250929`
*   `gemini-3-pro-preview`

**Available Tools:**
*   `calculator`: Basic arithmetic.
*   `bash`: Execute shell commands.
*   `write_to_file`: Create or overwrite files.
*   `view_file`: Read file contents.
*   `list_dir`: List directory contents.
*   `fetch_url`: Fetch content from a URL.
*   `ask_user`: Ask the user a question.
*   `time`: Get the current date and time.
*   `rag`: Search information in your local code base.
*   `memory`: Long-term memory storage (store/retrieve).

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
