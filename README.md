# Agents CLI

`agents` is a command-line tool for managing and interacting with AI agents locally. It supports creating specialized agents with specific system prompts, models, and tools, as well as Retrieval-Augmented Generation (RAG) capabilities using local files.

Agents can be backed by **Gemini** directly or by any model available through **OpenRouter**.

## Installation

To install the CLI, make sure you have [Go](https://go.dev/) installed (see `go.mod` for the required version; 1.26+ at time of writing).

```bash
git clone https://github.com/Shonei/agents.git
cd agents
go install .
```

Ensure your `$GOPATH/bin` is in your system `PATH`.

## Configuration

By default, the configuration is stored in `~/agents/config.yaml`. The CLI will automatically create this file if it doesn't exist.

### API Keys

Which key you need depends on the models your agents use (see [Models and Providers](#models-and-providers)).

**Gemini** — required for native Gemini models, and for all embedding-based features (`rag`, `memory`) regardless of which provider your agent uses:

1. **Environment Variable** (Recommended):
   ```bash
   export GEMINI_API_KEY="your-gemini-key"
   ```

2. **Config File**:
   ```yaml
   gemini_api_key: "your-gemini-key"
   ```

**OpenRouter** — required for any model routed through OpenRouter:

1. **Environment Variable** (Recommended):
   ```bash
   export OPENROUTER_API_KEY="your-openrouter-key"
   ```

2. **Config File**:
   ```yaml
   openrouter_api_key: "your-openrouter-key"
   ```

**GitHub** — required for the GitHub PR tools. Use a Personal Access Token with `repo` scope:

1. **Environment Variable** (Recommended):
   ```bash
   export GITHUB_TOKEN="your-github-token"
   ```

2. **Config File**:
   ```yaml
   github_token: "your-github-token"
   ```

**Firecrawl** — optional, used by the `firecrawl_fetch` hosted scraping tool:

1. **Environment Variable** (Recommended):
   ```bash
   export FIRECRAWL_API_KEY="your-firecrawl-key"
   ```

2. **Config File**:
   ```yaml
   firecrawl_api_key: "your-firecrawl-key"
   ```

Environment variables always take precedence over values in the config file.

### Global Options

These live at the top level of `config.yaml`:

```yaml
hide_thinking: true    # suppress thinking blocks in terminal output
hide_grounding: true   # suppress the Grounding: summary from server-side tools
db_path: /absolute/path/to/agents.db   # required for rag, memory, and database audit logging
```

`hide_thinking` and `hide_grounding` can also be set per-invocation with the `--hide-thinking` and `--hide-grounding` flags.

### Audit Logging

The CLI supports audit logging to help you understand and debug agent conversations. By default, this is disabled. To enable it, add the `audit` section to your `~/agents/config.yaml`.

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

Audit sessions capture the system prompt, user and assistant messages, tool calls and results, grounding metadata, conversation compaction summaries, and router route selections / handoffs.

### Audit Viewer

The project includes a web-based **Audit Viewer** for reviewing conversation logs:

```bash
./start-audit-viewer.sh
```

Then open http://localhost:3000 in your browser. See [audit-viewer/README.md](audit-viewer/README.md) for detailed documentation.

## Models and Providers

The provider is chosen from the **model string**, not from a separate config field:

| Model string | Provider | Example |
| --- | --- | --- |
| Contains a `/` | OpenRouter | `anthropic/claude-sonnet-4.5`, `openai/gpt-5` |
| Contains `gemini` | Gemini (native) | `gemini-3.1-pro-preview` |
| Anything else | Unsupported — the CLI exits with an error | |

Native Gemini model IDs known to `agents add`:

*   `gemini-3.6-flash`
*   `gemini-3.5-flash`
*   `gemini-3.5-flash-lite`
*   `gemini-3.1-pro-preview`
*   `gemini-3.1-flash-lite`
*   `gemini-3.1-flash-image-preview`

Any other model must be written into the YAML directly. For OpenRouter, use the full namespaced ID from their model list.

**Provider differences worth knowing:**

*   Image output (`response_modalities`) is Gemini-only.
*   Embeddings are Gemini-only, so the `rag` and `memory` tools require a Gemini API key even when the agent itself runs on OpenRouter.
*   Server-side tools differ per provider — see [Provider-executed tools](#provider-executed-server-side-tools).
*   Prompt caching is applied automatically on OpenRouter.

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

`agents add` covers the common fields. The remaining options are YAML-only:

```yaml
agents:
  coder:
    name: coder
    description: "Writes and edits Go code."   # tool description when used as a sub-agent
    model: gemini-3.1-pro-preview
    system_prompt: "You are an expert Go developer."
    thinking_enabled: true       # request thinking/reasoning blocks
    max_tokens: 8192             # output token cap
    temperature: 0.2
    max_context_tokens: 200000   # input-token budget that triggers compaction; 0 disables it
    max_context_turns: 2         # recent turns preserved verbatim when compacting
    response_modalities: [TEXT]  # Gemini only; use [IMAGE, TEXT] for image output
    tools:
      - name: view_file
      - name: bash
        config:
          require_confirmation: "false"
```

#### Conversation Compaction

When an agent's reported input tokens exceed `max_context_tokens`, the older part of the
conversation is replaced with a generated summary. The rebuilt history is:

1. the original user request, **pinned** and never evicted;
2. the summary, injected as an **assistant** message — it is the agent's own memory of its
   own work, and injecting it as a user turn makes the agent behave as though freshly
   instructed;
3. the most recent `max_context_turns` boundaries, verbatim.

Cuts normally land on a safe boundary (a plain user message, or the end of a complete
tool-call round) so a tool call is never separated from its result. When no such boundary
exists — most often because a single enormous tool result sits at the end of the history,
where nothing follows it to keep — the whole body is evicted instead, which is safe because
it leaves no tail to orphan. A rebuild that would not actually shrink the history is
discarded, and compaction is then skipped for the rest of that turn.

Across repeated passes the summary is *rolled*: the previous note is fed back to the
summarizer to be merged rather than re-summarized, so its facts degrade once instead of once
per pass. Compaction failure is never fatal — `max_context_tokens` is a self-imposed budget,
not the model's hard limit, so a failed pass warns and continues.

Each compaction is printed to the terminal and recorded in the audit log. Leaving
`max_context_tokens` unset disables the behaviour entirely.

Note that `max_context_turns` counts *boundaries*, not user turns: a completed tool-call
round counts as one. In a tool-heavy conversation a value of `2` therefore retains roughly
two tool rounds, not two exchanges. Use `agents context compact --dry-run` to see exactly
what a given value would evict before trusting it.

**Available Tools:**
*   `fetch_url`: Fetch content from a URL.
*   `browse_url`: Render a URL in Chromium (chromedp) and return cleaned markdown/text; useful when docs need JavaScript or reject simple HTTP fetches.
*   `firecrawl_fetch`: Fetch a single URL through Firecrawl's hosted scrape API and return markdown; useful as a hosted alternative to `browse_url`.
*   `ingest_api_spec`: Fetch or read an OpenAPI/Swagger or Postman Collection and return a structured summary of servers, auth schemes, and operations (prefer this over HTML scraping when a vendor publishes a spec).
*   `time`: Get the current date and time.
*   `write_to_file`: Create or overwrite files.
*   `delete_file`: Delete a single file. Refuses to delete directories and paths outside its allowed root.
*   `view_file`: Read file contents.
*   `list_dir`: List directory contents.
*   `bash`: Execute shell commands. By default, prompts the user for confirmation before execution.
*   `str_replace_editor`: Safely edits existing files using precise string replacement or insertion.
*   `rag`: Search information in your local code base. Requires `db_path` and a Gemini API key.
*   `memory`: Long-term memory storage (store/retrieve). Requires `db_path` and a Gemini API key.
*   `todo`: A task list shared across all agents in a run.
*   `plan`: A planning tool to create and manage a global plan shared across agents.
*   `github_pr_details`: Fetch PR metadata (title, body, state, author).
*   `github_pr_diff`: Fetch the raw diff/patch of a PR.
*   `github_pr_comments`: Fetch existing review and issue comments on a PR.
*   `git_checkout_pr`: Fetch and checkout a PR branch locally.
*   `github_pr_review`: Submit a formal PR review with inline comments.

`todo` and `plan` state is shared across every agent in a single `engage` run, including
sub-agents and router routes, so one agent can pick up where another left off. See
[GitHub tools](docs/github.md) for the PR tooling.

To inspect what the model actually sees for any tool:

```bash
agents tools details bash
```

#### Tool Configuration

Some tools support custom configuration via the `config` block in the YAML file.

**Confirmation (`bash`, `write_to_file`, `delete_file`, `browse_url`)**
By default, `bash`, `write_to_file`, `delete_file`, and `browse_url` prompt before running. Disable per agent with `require_confirmation: "false"` — useful for trusted bulk-write or docs-scraper agents.

```yaml
      - name: bash
        config:
          require_confirmation: "false"
      - name: write_to_file
        config:
          require_confirmation: "false"
      - name: delete_file
        config:
          require_confirmation: "false"
          allowed_root: "./systems"
      - name: browse_url
        config:
          require_confirmation: "false"
```

> **⚠️ SECURITY WARNING:** Disabling confirmation for `bash` allows arbitrary shell commands without oversight. Disabling it for `write_to_file` lets the agent create/overwrite files freely (still subject to the tool's `force` flag for overwrites). Disabling it for `delete_file` lets the agent remove files within its `allowed_root` without per-file approval (directories and paths outside the root are still refused). Disabling it for `browse_url` lets the agent navigate to web pages in a local browser without per-URL approval. Only do this for trusted agents with strict system prompts. Future versions of this project plan to introduce OS-level sandboxing (e.g., chroot jails or containers) to mitigate this risk.

`delete_file` defaults `allowed_root` to the current working directory. Narrow it per agent when possible, for example `allowed_root: "./systems"` for documentation-maintenance agents.

`browse_url` launches Chromium through chromedp. It is read-only in v1: it navigates, waits for full page load by default (or `domcontentloaded`/`networkidle` when requested), waits an additional 1000ms by default for client-side JavaScript to render (`settle_milliseconds`), extracts rendered HTML, and converts the main content to markdown. It runs visibly by default so the user can watch what the agent is viewing; pass `headless:true` to hide the browser window. The browser session is reused for the life of the running process. It does not click, fill forms, log in, save cookies, or replace `fetch_url` for simple static pages.

Common `browse_url` CLI examples:

```bash
# Visible browser, full page load, 1000ms JS settle (defaults)
agents tools execute browse_url 'url:https://example.com/docs'

# Hide the browser window
agents tools execute browse_url 'url:https://example.com/docs' 'headless:true'

# Give a React/SPA page more time after load before extraction
agents tools execute browse_url 'url:https://example.com/app-docs' 'settle_milliseconds:2500'

# Wait for a specific element before extracting
agents tools execute browse_url 'url:https://example.com/docs' 'wait_for_selector:main'

# Use a lighter wait when full load is blocked by analytics or long-polling
agents tools execute browse_url 'url:https://example.com/docs' 'wait:domcontentloaded'
```

`firecrawl_fetch` is a hosted single-URL fetch alternative to `browse_url`. It does not crawl a site; the agent should still decide which URLs to fetch.

```bash
# Fetch one URL and return Firecrawl markdown
agents tools execute firecrawl_fetch 'url:https://example.com/docs'

# Give Firecrawl more time before capturing a React/SPA page
agents tools execute firecrawl_fetch 'url:https://example.com/docs' 'wait_milliseconds:2500'

# Also request cleaned HTML
agents tools execute firecrawl_fetch 'url:https://example.com/docs' 'include_html:true'
```

#### Provider-executed (server-side) tools

These tools run inside the model provider, so the SDK never invokes them locally. After each turn that used them, a short `Grounding:` summary is printed (search queries, sources, retrieved URLs) and the same data is appended to the audit log as a `grounding` event.

| Tool | Provider | Description |
| --- | --- | --- |
| `google_search` | Gemini | Grounding with Google Search. The model decides when to issue queries. |
| `url_context` | Gemini | The model fetches URLs it finds in the prompt and grounds the answer on their contents. |
| `web_search` | OpenRouter | Provider-side web search. |
| `web_fetch` | OpenRouter | Provider-side URL fetching. |

A server-side tool listed for a provider that doesn't support it is ignored rather than erroring. See [web search tool spec](docs/tool_spec_web_search.md) for details.

#### Sub-agents (agent-as-tool)

Any agent name can be listed in another agent's `tools:` block. It is then exposed to the
parent as a callable tool that runs the sub-agent to completion and returns its final
message. Sub-agents share the parent's audit session, plan, and todo state.

```yaml
agents:
  researcher:
    model: gemini-3.5-flash
    description: "Reads files and answers questions about the codebase."
    system_prompt: "You investigate and report. You never edit files."
    tools: [view_file, list_dir, rag]

  lead:
    model: gemini-3.1-pro-preview
    system_prompt: "You implement changes, delegating investigation to researcher."
    tools: [str_replace_editor, bash, researcher]
```

See [Composite agents](docs/composite_agents_plan.md) for the design details.

#### System Prompt Templating
System prompts support dynamic content via Go templates (e.g., adding current time or file listings).
See [Templating System Prompts](docs/templating_system_prompts.md) for a full guide.

To review the fully rendered prompt for an agent:

```bash
agents prompt coder --pretty
```

#### Router Agents
A **router agent** combines several specialized sub-agents under one name and
automatically dispatches each user turn to the most appropriate one based on a
cheap per-turn classifier. Use this to give the user a single chat that moves
between, e.g., a `planner` and a `builder` persona without having to switch
agents by hand.

Router agents must be authored directly in YAML (the `agents add` shortcut
does not yet cover them).

**Example Configuration:**
```yaml
agents:
  planner:
    model: gemini-3.1-pro-preview
    system_prompt: "You are the PLAN agent. Help the user scope and design changes."
    tools: [view_file, list_dir, todo]

  builder:
    model: gemini-3.1-pro-preview
    system_prompt: "You are the BUILD agent. Implement the plan the user has approved."
    tools: [view_file, list_dir, str_replace_editor, write_to_file, bash]

  dev:
    kind: router
    classifier:
      model: gemini-3.1-pro-preview
      default_route: planner
      confidence_threshold: 0.7
    routes:
      - agent: planner
        when: "User is exploring, scoping, or asking 'how should we...'."
      - agent: builder
        when: "User has approved a plan and wants code changes implemented."
```

Routers require at least two routes, cannot nest inside one another, and their
`default_route` must be one of the configured routes — the config is validated on load.
See [Router Agents](docs/router_agents.md) for the full configuration reference and runtime semantics.

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

Submit an empty message to end an interactive session.

#### Inspecting Compaction and Handoff

`agents context` replays a recorded conversation through either context transform and shows
what the agent would see afterwards. It reads from the audit log, so it needs
`audit.type: database` and `db_path` configured.

```bash
agents context list                                    # recorded conversations, newest first
agents context compact <session> --agent coder --dry-run
agents context handoff <session> --agent planner --from planner
```

`--dry-run` resolves the cut point and summarizer input **without calling the model**, so
exploring is free; without it, one real summarizer call is made so you can read the summary
itself. `--show-prompt` prints the driving system prompt, `--show-input` the serialized
transcript the summarizer receives, and `--keep-turns` overrides the cut aggressiveness.

The two commands exist side by side because the transforms solve different problems:
`compact` continues the **same** agent with its system prompt intact, so its summary is a
first-person memory containing no directives; `handoff` briefs a **different** agent that
shares none of the history, so its summary is a briefing that must re-establish the goal and
may end in a next action. Running both on one session is the quickest way to see the
difference.

Replay is a reconstruction, not a recording: the audit log stores a tool call's name and
input but not its `tool_use` ID, and does not store thinking blocks at all. Roles, turn
boundaries and block structure — everything the transforms key off — are faithful, but the
commands print the caveats on every run.

#### Debugging Tools

Inspect and run tools without going through an agent:

```bash
agents tools details view_file            # description + input schema
agents tools execute view_file path:./README.md
```

Parameters are `key:value` pairs. Use dot notation for nested values:

```bash
agents tools execute view_file path:./main.go view_range.0:1 view_range.1:10
```

#### Image Generation

An experimental one-shot image generation command using Gemini's image model. It reads a
prompt from stdin and writes the resulting image into the current directory:

```bash
agents image-gen
```

### Example: PR Review Agent
You can configure an agent to act as an automated PR reviewer using the GitHub tools. See the example configuration in `examples/agents.yaml`.

```bash
export GITHUB_TOKEN="your-github-token"
export AGENTS_CONFIG="./examples/agents.yaml"
agents engage pr-reviewer --prompt "Can you review PR #123 in the owner/repo repository?"
```

### Retrieval-Augmented Generation (RAG)

> **Note:** The RAG feature is currently experimental.

The CLI supports RAG to let agents answer questions based on your local files. This uses **DuckDB** for storage and **Gemini** for embeddings, so `db_path` and a Gemini API key are both required.

#### Quick Start

1. **Index your documents:**
   ```bash
   agents rag index --dir ./my-docs
   ```

   `--dir` respects `.gitignore`. Use `--file` for a single file (the two are mutually
   exclusive), and `--strategy` to pick a chunking strategy (`none`, `summary`, or `go`).

#### Available Commands

- `agents rag index` - Index a directory into the RAG store
- `agents rag list-stores` - List all available RAG stores
- `agents rag search` - Search a RAG store for relevant documents
- `agents rag summary` - Test the summary strategy on a single file
- `agents rag delete-store` - Delete a RAG store

For detailed documentation on RAG commands, see [RAG Commands Reference](docs/rag_commands.md).


## Advanced Configuration

You can override the default config file location:

```bash
agents list --config ./my-config.yaml
```

Or use the environment variable `AGENTS_CONFIG`.

```bash
export AGENTS_CONFIG=./custom-config.yaml
```

Output format for commands that print structured data can be set with `--output` (`table`, `yaml`, or `json`).

## Development

```bash
make build      # build all binaries into bin/
make test       # go test ./...
make lint       # golangci-lint run
make lint-fix   # golangci-lint run --fix
```

See [AGENTS.md](AGENTS.md) for architecture notes and coding standards.
