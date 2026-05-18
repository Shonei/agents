# RAG Commands Reference

> **Note:** The RAG feature is currently experimental.

The RAG (Retrieval-Augmented Generation) commands allow you to manage local document stores that can be searched by your agents. These commands use DuckDB for storage and Gemini for generating embeddings.

## Prerequisites

All RAG commands require two configurations:

### 1. Gemini API Key

Required for generating embeddings:

```bash
export GEMINI_API_KEY="your-gemini-key"
```

Or add it to your `~/.agents/config.yaml`. It needs to be an absolute path:
```yaml
gemini_api_key: "your-gemini-key"
```

### 2. Database Path

Configure the database path in your `/path/to/your/agents.db`:

```yaml
db_path: "/path/to/your/agents.db"
```

**Important**: The `db_path` configuration is required for all RAG commands. If not set, you'll see an error:
```
DB not initialized, Did you remember to set 'db_path' in your config?
```

### Example Complete Config

```yaml
gemini_api_key: "your-gemini-api-key-here"
db_path: "/path/to/your/agents.db"
```

## Commands Overview

- [`agents rag index`](#index) - Index a directory of files into a RAG store
- [`agents rag list-stores`](#list-stores) - List all available RAG stores
- [`agents rag search`](#search) - Search a RAG store for relevant documents
- [`agents rag delete-store`](#delete-store) - Delete a RAG store and all its documents

---

## `index`

Index a directory of files into a RAG store. Files are embedded using Gemini and stored in a local DuckDB database.

### Usage

```bash
agents rag index [--dir PATH]
```

### Flags

- `--dir` - Path to the directory to index (default: current directory `.`)

### Behavior

- **Store Naming**: The store is automatically named using the absolute path of the directory being indexed
- **Gitignore Support**: Files listed in `.gitignore` are automatically excluded from indexing
- **File Processing**: Each file is read, embedded, and stored with metadata including path, size, and extension
- **Database Location**: Data is stored in the database specified by `db_path` in your config

### Examples

**Index the current directory:**
```bash
agents rag index
```

**Index a specific directory:**
```bash
agents rag index --dir ./my-docs
```

**Index a project's documentation:**
```bash
cd /path/to/my-project
agents rag index --dir ./docs
```

### Output

```
Embedded file /absolute/path/to/file1.md into RAG
Embedded file /absolute/path/to/file2.go into RAG
...
```

---

## `list-stores`

List all RAG stores in the local database, showing the store name and document count.

### Usage

```bash
agents rag list-stores
```

### Aliases

- `agents rag ls`
- `agents rag stores`
- `agents rag list`

### Examples

```bash
agents rag list-stores
```

### Output

The output format depends on your configured output mode (table by default):

```
NAME                              DOCUMENTCOUNT
/Users/you/projects/docs          42
/Users/you/knowledge/articles     15
```

---

## `search`

Search a RAG store for documents relevant to a query. The query is embedded and compared against stored documents using vector similarity.

### Usage

```bash
agents rag search [--store STORE_NAME] [--limit N]
```

### Flags

- `--store` - Name of the RAG store to search (default: absolute path of current working directory)
- `--limit` - Maximum number of results to return (default: `5`)

### Behavior

- **Store Selection**: If no store is specified, searches the store matching the current directory's absolute path
- **Interactive Query**: Prompts you to enter a search query after running the command
- **Similarity Scoring**: Results are ranked by distance (lower distance = more similar)

### Examples

**Search the store for the current directory:**
```bash
agents rag search
# Then enter your query when prompted
```

**Search a specific store with custom limit:**
```bash
agents rag search --store /Users/you/projects/docs --limit 10
```

### Output

```
Search query: how to configure agents
DISTANCE    PATH
0.123      /Users/you/projects/docs/configuration.md
0.245      /Users/you/projects/docs/getting-started.md
0.312      /Users/you/projects/docs/api-reference.md
```

---

## `delete-store`

Delete a RAG store and all its documents from the database.

### Usage

```bash
agents rag delete-store [--store STORE_NAME]
```

### Aliases

- `agents rag ds`
- `agents rag delete`

### Flags

- `--store` - Name of the RAG store to delete (default: absolute path of current working directory)

### Behavior

- **Destructive Operation**: Permanently removes the store and all its documents from the database
- **Store Selection**: If no store is specified, deletes the store matching the current directory's absolute path

### Examples

**Delete the store for the current directory:**
```bash
agents rag delete-store
```

**Delete a specific store:**
```bash
agents rag delete-store --store /Users/you/projects/old-docs
```

### Output

```
Deleted RAG store /Users/you/projects/old-docs
```
---

## Technical Details

### Storage

- **Database**: DuckDB (path configured via `db_path` in your config file)
- **Schema**: Stores documents with content, metadata, embeddings, and store names
- **Vector Dimensions**: 2048 (Gemini embedding default)

### Embedding Model

- **Provider**: Google Gemini
- **Model**: Uses the default Gemini embedding model
- **Dimension**: 2048

### Store Naming Convention

Stores are named using the **absolute path** of the directory being indexed. This means:
- `/Users/you/project/docs` becomes a store named `/Users/you/project/docs`
- When searching without `--store`, the CLI uses the current directory's absolute path
- This automatic naming ensures consistency between indexing and searching

**Note on Reserved Stores**:
- The store name `memory` is reserved for the `memory` tool used by agents. You may see this store listed if you use the memory tool, but you should generally manage it via the agent's tools rather than these CLI commands.

## Troubleshooting

### "DB not initialized, Did you remember to set 'db_path' in your config?"

Add `db_path` to your `~/.agents/config.yaml`:
```yaml
db_path: "agents.db"
```

### "failed to create embedding"

Ensure your `GEMINI_API_KEY` is set correctly and you have network access.

### "failed to search store"

The store name must match exactly. Use `agents rag list-stores` to see available stores, and note they use absolute paths.

### "no results found"

- Verify documents were indexed: `agents rag list-stores`
- Try a different query or increase `--limit`
- Check that you're searching the correct store

## See Also

- [Templating System Prompts](templating_system_prompts.md) - Dynamic system prompt configuration
