# Agent Commands

The `agent` command group allows you to manage and interact with AI agents defined in your configuration.

## Commands

### `add`

Adds a new agent to your configuration.

**Usage:**
```bash
agents agent add --name <name> --system-prompt <system-prompt> --model <model> [--tools <tool1,tool2...>]
```

**Flags:**
- `--name` (required): Unique name for the agent.
- `--system-prompt` (required): The system instructions for the agent.
- `--model` (required): The AI model to use.
- `--tools` (optional): Comma-separated list of tools available to the agent.

**Example:**
```bash
agents agent add \
  --name coder \
  --system-prompt "You are an expert Go developer." \
  --model claude-sonnet-4-5-20250929 \
  --tools bash,write_to_file,view_file
```

### `engage`

Engages (chats with) a configured agent.

**Usage:**
```bash
agents agent engage [prompt] --name <name>
```

**Flags:**
- `--name` (required): The name of the agent to engage.

**Input Methods:**
1. **Command Line Argument:**
   ```bash
   agents agent engage "Write a hello world program in Go" --name coder
   ```

2. **Standard Input (Stdin):**
   Use `-` as the prompt argument to read from stdin.
   ```bash
   echo "Explain quantum computing" | agents agent engage - --name researcher
   ```

### `list`

Lists all agents currently defined in your configuration file.

**Usage:**
```bash
agents agent list
```

## Supported Models

The following models are currently supported:
- `claude-sonnet-4-5-20250929`
- `gemini-3-pro-preview`

*(Note: Model names are retrieved dynamically and may change.)*

## Available Tools

The following tools can be assigned to agents:
- `calculator`
- `bash`
- `write_to_file`
- `view_file`
- `list_dir`

*(Note: Tool names correspond to the implementations in `cmd/agent/sdk/tools`.)*
