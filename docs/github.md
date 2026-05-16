# GitHub PR Review Agent Tools

This document outlines the plan for building tools that enable an AI agent to perform GitHub Pull Request reviews. The agent needs the ability to read PR context, analyze code locally, and write feedback.

## 1. GitHub Context Tools (Read)
To review a PR, the agent must understand the PR's intent, the changes made, and any ongoing discussions.

### `github_pr_details`
Fetches PR metadata to provide high-level context.
*   **Inputs:**
    *   `owner` (string): The repository owner (e.g., "octocat").
    *   `repo` (string): The repository name (e.g., "Hello-World").
    *   `pr_number` (integer): The pull request number.
*   **Outputs:** JSON object containing `title`, `body` (description), `state`, `author`, `base_branch`, `head_branch`, and `created_at`.
*   **Mechanism:** Uses GitHub REST API `GET /repos/{owner}/{repo}/pulls/{pull_number}`.

### `github_pr_diff`
Fetches the list of changed files and the actual diff/patch to tell the agent exactly what code was modified.
*   **Inputs:**
    *   `owner` (string): The repository owner.
    *   `repo` (string): The repository name.
    *   `pr_number` (integer): The pull request number.
*   **Outputs:** A raw diff string or a structured list of files with their additions, deletions, and patch content.
*   **Mechanism:** Uses GitHub REST API `GET /repos/{owner}/{repo}/pulls/{pull_number}` with the `Accept: application/vnd.github.v3.diff` header.

### `github_pr_comments`
Fetches existing review comments and general PR discussion threads to ensure the agent understands ongoing discussions and avoids repeating feedback.
*   **Inputs:**
    *   `owner` (string): The repository owner.
    *   `repo` (string): The repository name.
    *   `pr_number` (integer): The pull request number.
*   **Outputs:** JSON array of comments, including `author`, `body`, `created_at`, and (for review comments) `path` and `line`.
*   **Mechanism:** Uses GitHub REST API `GET /repos/{owner}/{repo}/issues/{issue_number}/comments` (general comments) and `GET /repos/{owner}/{repo}/pulls/{pull_number}/comments` (inline code comments).

## 2. Local Git Tools (Pull/Analyze)
While the agent could use a generic bash tool, dedicated git tools provide a more reliable and structured way to pull down code for local analysis.

### `git_checkout_pr`
Fetches the PR branch and checks it out locally. Once checked out, the agent can use existing tools (`view_file`, `list_dir`, `bash`) to inspect the code, run tests, or execute linters.
*   **Inputs:**
    *   `pr_number` (integer): The pull request number to check out.
*   **Outputs:** Success/failure message and the name of the checked-out branch.
*   **Mechanism:** Executes shell commands:
    1. `git fetch origin pull/{pr_number}/head:pr-{pr_number}`
    2. `git checkout pr-{pr_number}`

## 3. GitHub Review Tools (Write)
After analyzing the code, the agent needs a way to post its findings back to GitHub.

### `github_pr_review`
Submits a formal PR review. Supports adding line-specific comments on the diff and providing a general summary.
*   **Inputs:**
    *   `owner` (string): The repository owner.
    *   `repo` (string): The repository name.
    *   `pr_number` (integer): The pull request number.
    *   `event` (string): The review action (`APPROVE`, `REQUEST_CHANGES`, or `COMMENT`).
    *   `body` (string): The main summary body of the review.
    *   `comments` (array of objects): Optional inline comments containing `path`, `line`, and `body`.
*   **Outputs:** URL of the created review and success confirmation.
*   **Mechanism:** Uses GitHub REST API `POST /repos/{owner}/{repo}/pulls/{pull_number}/reviews`.

## Implementation Plan
*   **Dependencies**: Use the `github.com/google/go-github` library for clean GitHub API interactions.
*   **Authentication & Wiring**: 
    *   Add `GitHubToken string \`yaml:"github_token"\`` to the `Config` struct in `pkg/config/config.go`.
    *   Add a `GetGitHubToken() string` method to `ConfigFactory` that checks the `GITHUB_TOKEN` environment variable first, falling back to the YAML config.
    *   The new tools will implement the `Init(config map[string]string, c *config.ConfigFactory)` interface method to retrieve the token via `c.GetGitHubToken()` and initialize the `go-github` client.
*   **Milestones**:
    1. Implement `github_pr_details` to establish basic API connectivity.
    2. Implement `github_pr_comments` and `github_pr_diff` for full read context.
    3. Implement `git_checkout_pr` wrapping the `git` CLI for local analysis.
    4. Implement `github_pr_review` to close the loop with automated feedback.
