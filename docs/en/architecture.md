# Architecture: slack-personal-agent (spa)

## Overview

slack-personal-agent is a macOS desktop application that monitors multiple Slack
workspaces from an individual user's perspective, accumulates channel messages
with time-aware memory, and provides channel-scoped RAG queries with strict
3-tier knowledge isolation.

```
┌─ React Frontend ──────────────────────────────────┐
│ Dashboard │ Query UI │ MITL Approval │ Knowledge   │
└────────────────────┬──────────────────────────────┘
                     │ Wails v2 Bindings
┌────────────────────┴──────────────────────────────┐
│ app.go (orchestrator)                              │
│  ├── config       ← TOML + env overrides          │
│  ├── keychain     ← macOS Keychain (go-keyring)   │
│  ├── slack        ← API client + priority queue   │
│  ├── memory       ← DuckDB + Hot/Warm/Cold        │
│  ├── llm          ← Chat backend (local/Vertex)   │
│  ├── embedding    ← Independent vectorization     │
│  ├── rag          ← 3-tier scoped search          │
│  ├── mitl         ← Proxy response approval       │
│  └── knowledge    ← Internal knowledge base       │
└───────────────────────────────────────────────────┘
         │                    │
    Slack API            DuckDB (local)
  (User Token)           spa.db
```

## Package Architecture

### Dependency Graph

```
app.go
 ├── config          (no internal deps)
 ├── keychain        (no internal deps)
 ├── slack           (no internal deps)
 ├── memory          (no internal deps)
 ├── llm             (config)
 ├── embedding       (config)
 ├── rag             (embedding)
 ├── mitl            (no internal deps)
 └── knowledge       (no internal deps)
```

Each package is designed to be testable in isolation. The only package with
cross-package dependencies is `rag` (depends on `embedding.Embedder` interface).

### Package Responsibilities

| Package | Responsibility | Key Types |
|---------|---------------|-----------|
| `config` | TOML config loading, env overrides, validation | `Config`, `WorkspaceConfig`, `EmbeddingConfig` |
| `keychain` | Secure credential storage via OS keychain | `Store` interface, `OSStore`, `MockStore` |
| `slack` | Slack Web API client, rate-limited queue, polling | `Client`, `Queue`, `Scheduler`, `WorkspacePoller` |
| `memory` | Message storage with lifecycle management | `Store`, `Record`, `Tier`, `Lifecycle` |
| `llm` | Chat/summarization LLM interface | `Backend` interface, `LocalBackend`, `VertexAIBackend` |
| `embedding` | Text vectorization (independent of LLM) | `Embedder` interface, `LocalEmbedder`, `VertexAIEmbedder` |
| `rag` | Channel-scoped vector similarity search | `Retriever`, `SearchScope`, `SearchResult` |
| `mitl` | Man-in-the-loop proxy response workflow | `Manager`, `Proposal`, `State` |
| `knowledge` | Internal knowledge base (non-Slack) | `Store`, `Entry`, `Scope` |

## Data Flow

### Message Collection

```
Slack API ──poll──→ WorkspacePoller ──→ handleMessages()
                                          ├── memory.Store.InsertRecord() → DuckDB records table
                                          └── rag.Retriever.Index()       → DuckDB embeddings table
```

### Query

```
User ──question──→ App.Query()
                     ├── embedding.Embedder.Embed(question) → query vector
                     └── rag.Retriever.Search(vector, scope) → DuckDB list_cosine_similarity
                            └── scope filter: workspace_id + channel_id
```

### MITL Proxy Response

```
System detects response opportunity
  → mitl.Manager.CreateProposal() → StatePending
    → OnProposal callback (macOS notification)
      → User reviews in GUI
        ├── Approve → PostProxyMessage(text + signature) → Slack
        │              └── BoostChannel(polling priority)
        ├── Edit+Approve → modified text → PostProxyMessage
        ├── Reject → StateRejected (discarded)
        └── Timeout → StateExpired (discarded)
```

### Memory Lifecycle

```
                    24h                     7d
  Hot ─────────────────→ Warm ─────────────────→ Cold
  (raw messages)         (LLM summaries)         (archive)

  Compaction: Hot records grouped by (workspace, channel)
              → LLM summarizes → Warm record created
              → Hot records deleted
```

## Key Design Decisions

### Why User Token, not Bot Token

**Decision:** Use Slack User Token (`xoxp-`) for all API operations.

**Alternatives considered:**
- Bot Token (`xoxb-`): Would create a shared workspace agent, not a personal one.
  Channel visibility would need to be managed manually rather than inheriting from
  the user's Slack permissions.
- Events API: Requires an HTTP endpoint, which is impractical for a desktop app.
- Socket Mode: Not available for User Tokens.

**Consequence:** Polling is the only viable approach. This limits real-time
responsiveness but simplifies deployment (no inbound connections needed).

### Why Polling with Priority Queue

**Decision:** Fixed-interval polling with dynamic priority boosting.

**Why not fixed polling only:** When the system detects a response opportunity
(MITL), the relevant channel needs faster refresh to maintain conversational
coherence. A flat polling interval would miss follow-up context.

**Rate limit management:** Slack Tier 3 allows ~50 requests/minute per workspace.
A per-workspace queue with sliding-window rate tracking ensures compliance
without manual tuning.

### Why Embedding is Independent of LLM

**Decision:** The `embedding` package is completely separate from the `llm` package.

**Problem:** If embeddings were generated by the LLM backend (local or Vertex AI),
switching backends would invalidate all stored vectors, requiring full re-indexing
(potentially hours of computation).

**Solution:** Embedding has its own backend selection (`builtin`, `local`, `vertex_ai`)
in config. The `ModelID()` method tracks which model generated each index.
On startup, `CheckModelConsistency()` detects mismatches and warns the user.

**Default:** Built-in model (planned: all-MiniLM-L6-v2 via Hugot) for zero-config
offline operation.

### Why 3-Tier Knowledge Isolation

**Decision:** Channel is the fundamental unit. Knowledge scope expands only
with explicit user permission.

```
Level 1: Channel-local (default, always on)
Level 2: Cross-channel within workspace (user-permitted groups)
Level 3: Cross-workspace (user-explicit permission)
```

**Problem (from sai):** The predecessor project (sai) had global RAG search
with no channel filtering. A question in `#private-engineering` could retrieve
messages from `#general`, creating information leakage.

**Enforcement:** The `rag.SearchScope` struct requires explicit workspace/channel
IDs. There is no "search everything" API. The scope filter is applied at the
SQL level (`WHERE workspace_id = ? AND channel_id = ?`), not post-retrieval.

### Why Keychain-First Credentials

**Decision:** All tokens stored in macOS Keychain. Config files never contain secrets.

**Problem:** Most nlink-jp tools store tokens in plaintext config files (JSON/TOML).
This is a known security gap, especially for User Tokens which have broader
permissions than Bot Tokens.

**Implementation:** `go-keyring` library (proven in scli) provides cross-platform
keychain access. Environment variable fallback exists only for testing.
The GUI provides token setup flow: input → Keychain → config records name only.

### Why Proxy Response Signature

**Decision:** All MITL proxy responses include a configurable signature line.

**Problem:** User Token posts appear as the user. Without identification, there
is no way to distinguish human-authored messages from system-generated ones.
This creates accountability and trust issues.

**Default:** `— via spa (slack-personal-agent)` appended to every proxy post.

## Storage Schema

All data is stored in a single DuckDB file (`spa.db`).

### Tables

| Table | Purpose |
|-------|---------|
| `records` | Slack messages with lifecycle tier (hot/warm/cold) |
| `channels` | Channel metadata and polling state |
| `embeddings` | Vector embeddings with workspace/channel scope |
| `embedding_meta` | Embedding model tracking (ModelID) |
| `knowledge` | Internal knowledge base entries |

### Index Strategy

DuckDB's ART index has known limitations with UPDATE operations on primary keys.
Records use UUID identifiers without primary key constraints. Indexes are
created on query-critical columns: `(workspace_id, channel_id)`, `tier`,
`created_at`, `(scope, workspace_id)`.

Vector search uses `list_cosine_similarity()` (native DuckDB function) rather
than the VSS extension. This avoids extension dependency issues while providing
adequate performance for the expected data volume (tens of thousands of records
per workspace). If performance becomes an issue, migration to VSS HNSW index
is straightforward.

## Security Model

1. **Credentials:** macOS Keychain only. No plaintext tokens in config.
2. **Channel isolation:** Enforced at SQL query level, not application logic.
3. **MITL gate:** All Slack posts require explicit user approval.
4. **Sender identification:** System posts carry signature for attribution.
5. **Embedding model tracking:** Prevents silent retrieval degradation on model change.

## Technology Choices

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Language | Go | Consistent with nlink-jp ecosystem; shell-agent/data-agent precedent |
| GUI | Wails v2 + React | Proven in shell-agent and data-agent |
| Database | DuckDB | Vector search + analytics; proven in lite-rag/gem-rag/shell-agent |
| Keychain | go-keyring | Cross-platform; proven in scli |
| LLM (chat) | OpenAI-compatible / Vertex AI | data-agent Backend interface pattern |
| LLM (embed) | Independent Embedder | Prevents re-index on LLM backend switch |
