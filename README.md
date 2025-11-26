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

## Usage

### Managing Agents

#### Add an Agent
Create a new agent with a name, system prompt, and model. You can optionally attach tools.

```bash
agents agent add \
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

#### System Prompt Templating
System prompts support dynamic content via Go templates (e.g., adding current time or file listings).
See [Templating System Prompts](docs/templating_system_prompts.md) for a full guide.

#### List Agents
View all configured agents.

```bash
agents agent list
```

### Interacting with Agents

#### Engage (Chat)
Chat with an agent directly from the terminal.

**Interactive Mode:**
```bash
agents agent engage --name coder
```
*(Note: Prompt can be passed as an argument or via stdin)*

**Single Prompt:**
```bash
agents agent engage "Write a hello world in Go" --name coder
```

**Piped Input:**
```bash
cat main.go | agents agent engage "Explain this code" --name coder -
```

### Retrieval-Augmented Generation (RAG)

The CLI supports RAG to let agents answer questions based on your local files. This uses **DuckDB** for storage and **Gemini** for embeddings.

#### 1. Embed Documents
Index a folder of documents.

```bash
agents rag embed --folder ./my-docs
```

*   Required Env/Config: `GEMINI_API_KEY` (used for embeddings).
*   Data is stored in `agents.db` (local DuckDB file) by default.

#### 2. Search (Manual)
Test the search functionality manually.

```bash
agents rag search "How do I configure the agent?"
```

#### 3. Using RAG in Agents
To enable an agent to use your embedded data, you must configure the `rag` tool.

1.  **Create the agent**:
    ```bash
    agents agent add \
      --name "researcher" \
      --system-prompt "You are a helpful assistant. Use the rag tool to find information." \
      --model "claude-sonnet-4-5-20250929" \
      --tools "rag"
    ```

2.  **Configure the tool**:
    The CLI `add` command doesn't yet support tool specific configuration. You must edit `~/.agents/config.yaml` to point the agent to your database.

    Open the config file and update the `rag` tool entry for your agent:

    ```yaml
    agents:
      researcher:
        # ... other fields ...
        tools:
          - name: rag
            config:
              db_path: "agents.db"      # Path to the file created in step 1
              embedding_dim: "2048"     # Default dimension
    ```

3.  **Engage**:
    ```bash
    agents agent engage "What is in the documentation?" --name researcher
    ```

## Advanced Configuration

You can override the default config file location:

```bash
agents agent list --config ./my-config.yaml
```

Or use the environment variable `agents_CONFIG`.

```bash
export agents_CONFIG=./custom-config.yaml
```
