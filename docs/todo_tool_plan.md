# Todo List Tool Implementation Plan

## Overview
A "todo list" tool is a common pattern for AI agents. It acts as an external memory or scratchpad, allowing the agent to plan, break down complex tasks, and maintain focus over long horizons. It helps the agent keep track of what needs to be done, what is currently in progress, and what has been completed.

## Proposed Approach

1.  **Create Tool File**: Create `pkg/sdk/tools/todo.go`.
2.  **Define Tool Struct**: Implement the `sdk.AITool` interface for a `TodoTool` struct.
3.  **State Management**: The `TodoTool` will need to maintain state (the list of tasks) in memory during the agent's execution.
4.  **Define Schema**: The tool will accept a JSON schema with the following parameters:
    *   `action` (string, required): The operation to perform (`add`, `update`, `list`, `remove`).
    *   `id` (string, optional): The ID of the task (required for `update` and `remove`).
    *   `task` (string, optional): The description of the task (required for `add`, optional for `update`).
    *   `status` (string, optional): The status of the task (`pending`, `in_progress`, `completed`, `blocked`). Default for `add` is `pending`.
5.  **Implement Logic**:
    *   `add`: Generate a unique ID (or use a simple counter), store the task, and return the ID.
    *   `update`: Find the task by ID, update its description and/or status.
    *   `list`: Return a formatted string of all tasks and their statuses.
    *   `remove`: Delete the task by ID.
6.  **Register Tool**: Add `&TodoTool{}` to the `availableTools` slice in `pkg/sdk/tools/config.go`.

## Risks / Open Questions
*   **Persistence**: Should the todo list persist across different agent sessions? For now, in-memory per agent session seems sufficient and aligns with typical scratchpad usage. If persistence is needed, it could be backed by a file or the existing memory tool.
*   **Concurrency**: If the agent runs tools concurrently, the in-memory state will need a mutex to prevent race conditions.

## Next Steps
1. Review this plan.
2. Implement `pkg/sdk/tools/todo.go`.
3. Update `pkg/sdk/tools/config.go`.
4. Test the tool.
