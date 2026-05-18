# Composite Agents Plan

Design for two related features that let agents compose:

1. **Prompt classification & routing** — a "router" agent that delegates each
   user turn to a specialized sub-agent based on a cheap classifier.
2. **Agents-as-tools** — any agent can expose another configured agent as a
   callable tool (e.g. a coding agent invoking an image-generation agent).

Status: **draft, under discussion.** Decision points are marked with `Q:` and
need an answer before implementation begins.

---

## Background — what exists today

- One `*sdk.AI` wraps one `sdk.Agent` (LLM client). `AI.Chat()` runs a loop:
  `CreateMessage` → execute any `tool_use` blocks → append `tool_result` →
  repeat until the model stops calling tools → prompt the user for the next
  turn.
- Agents are configured in YAML under `agents:` with `system_prompt`, `model`,
  modalities, and a flat list of `tools:` entries.
- Tools are singletons in `pkg/sdk/tools/config.go`. `engage.go` looks one up
  by name, calls `Init(config, configFactory)`, and registers it on the `AI`.
- A single Gemini agent type sits behind `sdk.Agent`. Tool calls are
  declared per `CreateMessage` request, so the request and the model's
  history must agree on which functions exist.
- `cmd/image-gen.go` is the existing one-shot image flow — useful prior art
  for the image-returning sub-agent path.

---

## Feature 1 — Prompt classification & routing

> **Status: implemented.** User-facing reference lives in
> [router_agents.md](router_agents.md). The text below is the original
> design and is kept for historical context.

### Goal
Combine multiple agents under one name so a single chat session can move
seamlessly between, e.g., a `planner` persona and a `builder` persona without
the user having to switch agents manually.

### Proposed configuration

```yaml
agents:
  planner:
    model: gemini-3.1-pro-preview
    system_prompt: |
      You are the PLAN agent...
    tools: [view_file, list_dir, todo]

  builder:
    model: gemini-3.1-pro-preview
    system_prompt: |
      You are the BUILD agent...
    tools: [str_replace_editor, write_to_file, bash, view_file]

  dev:
    kind: router                          # NEW field; default "agent"
    classifier:
      model: gemini-3.1-pro-preview       # strong model; called rarely
      strategy: sticky                    # sticky | per_turn
      default_route: planner              # initial route + low-confidence fallback
      confidence_threshold: 0.7           # below this → keep current route
    routes:
      - agent: planner
        when: "User is exploring, scoping, or asking 'how should we…'."
      - agent: builder
        when: "User has approved a plan and wants code changes implemented (e.g. 'looks good, let's build it')."
```

### Runtime flow (sticky)
1. `engage dev` sees `kind: router` → builds a `RouterAI` holding one fully
   constructed `*AI` per route, plus an initial `current_route` set to
   `default_route`.
2. On each user turn:
   - Call the classifier with `{routes, current_route, recent history,
     new user message}`, forcing a structured response
     `{route, confidence, reason}` via function-calling.
   - If `confidence < confidence_threshold` **or** the picked route equals
     `current_route` → stay; no handoff.
   - Otherwise → **handoff**:
     1. Generate a handoff summary of the conversation since the last
        handoff (or from the start) using the same summarizer pattern as
        `pkg/sdk/compaction.go`.
     2. Inject a synthetic user message
        `[Handoff from <prev>: <summary>]` into the shared history.
     3. Re-render the new agent's system prompt (so any tool-contributed
        template state — see Feature 3 — reflects the latest world).
     4. Set `current_route` to the new route.
3. The chosen sub-agent runs the turn against the (now-augmented) shared
   history with its own system prompt + tool set.
4. Audit: `RouteSelectionEvent` per classifier call,
   `HandoffEvent` (with `from`, `to`, `summary`) per actual switch.

### Design decisions

- **Q1.1 — Strategy.** ✅ **Sticky.** Re-classify every turn, but only switch
  when the classifier confidently picks a different route. On switch,
  produce a conversation summary and inject it as a handoff message so the
  new agent starts grounded.
- **Q1.2 — Shared history vs separate.** ✅ **Shared history, with a
  handoff summary at switch time, and the summary is the contract.** The
  conversation log is one stream across agents. When the active agent
  changes, the router (a) generates a summary of everything since the
  last handoff, (b) replaces the new agent's view of the prior history
  with a single synthetic user message `[Handoff from <prev>: <summary>]`,
  and (c) appends the user's latest message after it. The full,
  un-truncated history is still retained at the router level for audit
  and for the previous agent if control ever swings back — only the
  *view* presented to the new agent on its first turn is truncated. This
  trivially avoids orphan `tool_use`/`tool_result` blocks because the
  new agent's history starts at a clean user-text boundary.
- **Q1.3 — Classifier prompt.** ✅ **Agreed.** Hard-code the classifier
  system prompt; expose only `routes[].when`, `default_route`,
  `confidence_threshold` in YAML.
- **Q1.4 — Audit.** ✅ **Agreed.** Add `RouteSelectionEvent`
  (`{route, confidence, reason, latency_ms}`) and `HandoffEvent`
  (`{from, to, summary}`).
- **Q1.5 — Classifier model.** ✅ **Use a strong model**
  (e.g. `gemini-3.1-pro-preview`). Sticky routing means the classifier is
  called once per turn but switching happens rarely, so the cost is bounded
  and switch quality matters more than latency. Model is configurable per
  router via `classifier.model`.

### Open questions / risks
- Mid-turn handoff: should the model be able to ask "switch me to builder
  now" via a tool? Probably yes eventually, but out of scope for v1.
- Compaction (`pkg/sdk/compaction.go`) currently lives on `*AI`. With shared
  history we want compaction to happen at the router level so all sub-agents
  benefit. Likely a small lift, but worth confirming during implementation.

---

## Feature 2 — Agents as tools

### Goal
Let an agent invoke another configured agent as a tool. The classic example:
a game-dev agent that calls an image-generation agent to produce sprites.

### Proposed configuration

```yaml
agents:
  image_maker:
    model: gemini-3.1-flash-image-preview
    response_modalities: [IMAGE, TEXT]
    system_prompt: |
      You generate game art assets...

  game_dev:
    model: gemini-3.1-pro-preview
    system_prompt: |
      You are a game developer building a 2D platformer.
    tools:
      - name: write_to_file
      - name: bash
      - name: agent                       # NEW built-in tool kind
        config:
          agent: image_maker              # which agent to wrap
          tool_name: generate_image       # name exposed to the parent model
          description: "Generate a game asset from a text prompt. Returns saved file paths."
          max_turns: "4"                  # safety budget for the inner loop
```

### Runtime mechanics
- `engage.go` special-cases `name: agent`: for each entry, construct a fresh
  `AgentTool` instance (not a shared singleton — each one wraps a distinct
  target agent).
- `AgentTool.Name()` returns `config.tool_name`; `Description()` returns
  `config.description`; `InputSchema()` is `{prompt: string}` (and possibly
  `attachments`).
- `Call(input)` builds a fresh `*AI` for the referenced agent (reusing the
  same `ConfigFactory` for API keys / audit logger), runs a non-interactive
  one-shot loop, and returns the final assistant text as the tool result.
- For sub-agents that emit images: write each `image` block to disk like
  `cmd/image-gen.go` does today; return JSON like
  `{"text": "...", "images": ["./image_abc.png"]}` so the parent can read
  paths.
- **Recursion safety**: pass a depth counter through the SDK; refuse to
  nest deeper than `max_depth` (default 3); refuse self-reference;
  `max_turns` caps the inner tool-loop.
- **Audit**: inner-agent events carry `parent_session_id` and
  `via_tool: <tool_name>` so the audit-viewer can render nesting.

### Design decisions

- **Q2.1 — Inner-agent output on the terminal.**
  - **Quiet**: inner prints suppressed; only parent output shows.
  - **Verbose-indented**: print everything prefixed with
    `[generate_image]›`.
  - **Config flag**, default quiet.
  _Proposal: config flag (`config.verbose: "true"`), default quiet._
- **Q2.2 — Image return format.** Disk-write + path in JSON result. (Don't
  attempt to ship raw image bytes in the function-response payload in v1.)
  _Proposal: agreed._
- **Q2.3 — Sub-agent tool scope.** Sub-agent uses only the tools listed in
  its own YAML entry — no inheritance from parent. _Proposal: agreed._
- **Q2.4 — `RunOnce` refactor.** Extract a non-interactive
  `(*AI).RunOnce(prompt) (string, error)` from `Chat`. Reused by both this
  feature and Feature 1. _Proposal: agreed._
- **Q2.5 — Tool naming.** Require `tool_name`; validate uniqueness within
  an agent's `tools:` list. _Proposal: agreed._
- **Q2.6 — Attachments / structured input.** v1 schema is `{prompt: string}`.
  Add `attachments`/`context_files` in a follow-up if needed.
  _Proposal: agreed._

### Open questions / risks
- Tool registry pattern today (`availableTools` singletons + `findAITool`)
  doesn't tolerate multiple instances of the same tool with different
  configs. We need either a factory pattern, or — minimally — special-case
  `name: agent` in `engage.go` to construct fresh instances. Latter is
  simpler.
- DB-backed audit logger (`pkg/sdk/audit/audit.go`) currently keys events on
  a single `sessionID` field per logger. Nested sub-agent logging may need
  either a new `parent_session_id` column on the events table or a new
  logger wrapper that injects metadata. Confirm during implementation.
- `image-gen.go` writes to `cwd`. For an inner agent invoked deep inside a
  parent's workflow, `cwd`-relative writes are fine but worth documenting.

---

## Feature 3 — Tool-contributed template state

### Goal
Let stateful local tools (e.g. `todo`, a future `plan` tool, `memory`)
expose their current state to the system-prompt template. Then a planner
agent's prompt can include `{{ .Todos }}` or `{{ .PlanFile }}` so the
state of the world is always in front of the model — and on handoff to a
builder agent, the builder's prompt picks up the same state without us
having to invent a transport mechanism for it.

### Why now
- Cross-cuts both other features:
  - In Feature 1 it gives sticky handoffs a durable, structured channel for
    sharing state (planner writes to it, builder reads it). The generated
    handoff summary becomes a recap of *intent*; the tool state carries
    the *artifacts*.
  - In Feature 2 it lets a parent agent see the inner agent's tool state
    (e.g. an `image_maker` sub-agent's last generated assets) in its own
    system prompt next turn.
- It also makes single-agent setups nicer: `coder` already gets
  `.RepoContext`, `.DirList`; adding `.Todos` is a natural extension.

### Proposed shape

A new optional interface, additive to `sdk.AITool`:

```go
// TemplateContributor is implemented by tools that want to expose state
// into the system-prompt template context.
type TemplateContributor interface {
    // TemplateKey is the dotted name the prompt uses, e.g. "Todos".
    TemplateKey() string
    // TemplateValue returns whatever the template should render. May be a
    // string, a slice, a map, or any struct with exported fields.
    TemplateValue() any
}
```

`engage.go` collects every registered tool that implements
`TemplateContributor` and merges their `{key: value}` pairs into the
template data passed to `RenderPrompt`. Built-in `SystemPromptBuilder`
fields keep working unchanged.

Example after wiring `todo`:

```yaml
agents:
  planner:
    model: gemini-3.1-pro-preview
    system_prompt: |
      You are the PLAN agent.
      Working directory: {{ .Cwd }}

      ## Current todos
      {{ range .Todos }}- [{{ .Status }}] {{ .Task }}
      {{ end }}
    tools: [view_file, list_dir, todo]
```

### Runtime details
- `RenderPrompt` is re-invoked **at the start of every agent turn**
  (cheap; template rendering is microseconds). This guarantees tool state
  is current both within a single agent's session and across router
  handoffs.
- `cmd/system-prompt` (the existing prompt-functions listing) is extended
  to enumerate keys contributed by tools as well, so users get
  discoverability for free.

### Design decisions

- **Q3.1 — Interface placement.** ✅ **Separate optional interface.**
  `TemplateContributor` is its own interface, not bolted onto `AITool`.
  Lets non-tool contributors join later and keeps existing tools
  untouched.
- **Q3.2 — Re-render cadence.** ✅ **Re-render every turn** (and on
  handoff). Cheap and keeps tool state always-current. We do **not**
  re-render mid-turn between tool calls — that would mutate the system
  instruction inside a single `CreateMessage` chain.
- **Q3.3 — Key namespacing.** ✅ **Tools own their keys.** `todo` →
  `Todos`, future `plan` → `Plan`, etc. Keys must be valid Go
  identifiers and unique across all registered contributors; duplicates
  fail at config-load time with a clear error.
- **Q3.4 — Persistence.** ✅ **In-memory only for v1.** All contributing
  tools (currently `todo`, future `plan`) keep their state in-process and
  lose it when the CLI exits. Cross-session continuation is out of scope
  until we have a concrete design for it (resume-by-session-id, on-disk
  scratchpad, etc.).
- **Q3.5 — Discovery.** ✅ **Yes.** Extend `cmd/system_prompt.go` (the
  list of available template functions) and the `agents add --help` text
  to also enumerate the keys contributed by tools, so users get
  discoverability for free.

### Resolved scope notes
- **Tool sharing across nested agents.** ✅ Stateful tools are **shared
  per CLI invocation**. A parent agent and any agents it invokes
  (Feature 2) see the same `TodoTool` / `plan` instance — the user's
  mental model is one workspace. Matches today's behavior; no change
  required.
- **`MemoryTool` is not a contributor.** ✅ The vector-DB-backed
  `memory` tool stays out of the template-state pipeline. Semantic
  retrieval needs a query and is async-feeling; it doesn't fit a
  synchronous prompt render. More importantly, `memory` is a
  *user-facing* store ("remember this for me across sessions"), not a
  per-agent scratchpad. The two roles shouldn't be conflated. Only
  fast, synchronous, agent-scratchpad tools (`todo`, future `plan`)
  participate.

---

## Implementation order

1. **Refactor**: extract `(*AI).RunOnce(prompt) (string, error)` from
   `Chat`. Low-risk, needed by Features 1 and 2.
2. **Feature 3 (template state)**: tiny interface + plumbing in
   `engage.go` and `pkg/sdk/system.go`. Unblocks the planner/builder
   pattern even before the router exists. Wire `todo` as the first
   contributor.
3. **Feature 2 (agent-as-tool)**: validates the "fresh `*AI` per call"
   pattern.
4. **Feature 1 (router)**: builds on `RunOnce`, the handoff summarizer
   (reuses `pkg/sdk/compaction.go` helpers), and Feature 3 for durable
   cross-agent state.

Each feature lands behind a separate PR, with examples added to
`examples/agents.yaml`.

---

## Decisions log (fill in as we agree)

| Q     | Decision |
|-------|----------|
| Q1.1  | ✅ Sticky routing — re-classify per turn, only switch on confident new pick; on switch generate a conversation summary. |
| Q1.2  | ✅ Shared history across agents; at handoff, generate a summary, truncate the new agent's view of prior history, and present it `[Handoff from <prev>: <summary>]` + latest user message. Full history retained at router level. |
| Q1.3  | ✅ Hard-coded classifier prompt; YAML exposes `routes[].when`, `default_route`, `confidence_threshold`. |
| Q1.4  | ✅ Add `RouteSelectionEvent` and `HandoffEvent` to audit. |
| Q1.5  | ✅ Use a strong classifier model (default `gemini-3.1-pro-preview`); configurable per router. Sticky routing keeps the call rate low. |
| Q2.1  | _pending_ |
| Q2.2  | _pending_ |
| Q2.3  | _pending_ |
| Q2.4  | _pending_ |
| Q2.5  | _pending_ |
| Q2.6  | _pending_ |
| Q3.1  | ✅ Separate `TemplateContributor` interface, optional, not on `AITool`. |
| Q3.2  | ✅ Re-render the system prompt at the start of every turn (and on router handoff). No mid-turn re-rendering. |
| Q3.3  | ✅ Tools own their template keys; must be valid Go identifiers and globally unique; duplicates fail at config load. |
| Q3.4  | ✅ In-memory only for v1; no cross-session persistence until we design it explicitly. |
| Q3.5  | ✅ Extend `cmd/system_prompt.go` and `agents add --help` to enumerate tool-contributed keys. |
