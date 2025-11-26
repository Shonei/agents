# Configuring RAG (Retrieval-Augmented Generation)

This guide explains how to configure an agent to use the `rag` tool, enabling it to search your local documents.

## Prerequisites

Ensure your Gemini API key is set, as it is required for generating embeddings.

```bash
export GEMINI_API_KEY="your-gemini-key"
```

## Step 1: Embed Your Documents

First, you need to create the database by embedding your documents. The default database file is `agents.db` and the default embedding dimension is `2048`.

```bash
# Embed a folder of documents
agents rag embed --folder ./my-docs
```

This will create `agents.db` in your current directory (or update it if it exists).

## Step 2: Create a RAG-Capable Agent

Use the CLI to create an agent with the `rag` tool enabled.

```bash
agents add \
  --name "researcher" \
  --system-prompt "You are a helpful assistant with access to a knowledge base. Use the rag tool to find information." \
  --model "claude-sonnet-4-5-20250929" \
  --tools "rag"
```

## Step 3: Configure the RAG Tool

The CLI `add` command does not currently support passing tool-specific configuration arguments. You must manually edit the configuration file to tell the agent where the database is located.

1.  Open your config file (usually `~/.agents/config.yaml`).
2.  Locate the `researcher` agent entry.
3.  Update the `rag` tool section to include `db_path` and `embedding_dim`.

**Before:**
```yaml
agents:
  researcher:
    name: researcher
    system_prompt: ...
    model: claude-sonnet-4-5-20250929
    tools:
      - name: rag
```

**After:**
```yaml
agents:
  researcher:
    name: researcher
    system_prompt: ...
    model: claude-sonnet-4-5-20250929
    tools:
      - name: rag
        config:
          db_path: "agents.db"      # Path to the file created in Step 1
          embedding_dim: "2048"     # Must match the dimension used in Step 1 (default is 2048)
```

*Note: You do not need to add `gemini_api_key` here; the CLI automatically injects it from your environment variables.*

## Step 4: Engage the Agent

Now you can chat with your agent, and it will be able to query the database.

```bash
agents engage researcher --prompt "What does the documentation say about configuration?"
```
