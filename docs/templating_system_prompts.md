# Templating System Prompts

The `agents` CLI supports dynamic system prompts using Go's [text/template](https://pkg.go.dev/text/template) engine. This allows you to inject dynamic information into your agent's system prompt at runtime.

## Available Helper Functions

The following helper functions are available within your system prompt templates:

### `{{ .Cwd }}`
Returns the current working directory (absolute path).

**Example:**
```
You are a helpful assistant working in {{ .Cwd }}.
```

### `{{ .Now }}`
Returns the current time in RFC3339 format.

**Example:**
```
Current time: {{ .Now }}
```

### `{{ .OSInfo }}`
Returns the operating system and architecture information.

**Example:**
```
You are running on {{ .OSInfo }}.
```

### `{{ .Shell }}`
Returns the current shell environment variable (e.g., `/bin/zsh`, `/bin/bash`).

**Example:**
```
Assume the user is using {{ .Shell }}.
```

### `{{ .DirList <depth> }}`
Returns a listing of files and directories in the current working directory up to the specified depth. The output is formatted in XML-like tags.

**Example:**
```
Here is the file structure:
{{ .DirList 1 }}
```

## Example Usage

When adding an agent via the CLI, you can use these templates directly in the system prompt string.

```bash
agents add \
  --name "context-aware" \
  --system-prompt "You are an AI assistant. Current time: {{ .Now }}. Working in: {{ .Cwd }}." \
  --model "claude-sonnet-4-5-20250929"
```

Or when editing the `config.yaml` manually:

```yaml
agents:
  context-aware:
    name: context-aware
    model: claude-sonnet-4-5-20250929
    system_prompt: |
      You are an expert developer.
      
      Environment Context:
      - OS: {{ .OSInfo }}
      - Shell: {{ .Shell }}
      
      File Context:
      {{ .DirList 1 }}
```
