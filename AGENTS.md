# AGENTS.md — slack-personal-agent

## Project Summary

slack-personal-agent (spa) is a personal knowledge agent for Slack. It monitors
multiple workspaces via User Token polling, accumulates channel information with
a time-aware 3-tier memory lifecycle, and provides channel-scoped RAG queries
with strict information isolation. Built with Go + Wails v2 + React.

## Build & Test

```bash
make build    # Build macOS app → dist/slack-personal-agent.app
make dev      # Wails dev mode with hot reload
make test     # Run Go tests (requires -tags no_duckdb_arrow)
make clean    # Remove build artifacts
```

**Important:** Always use `make build`, never `go build` directly.
DuckDB requires `no_duckdb_arrow` build tag with Wails.

## Module Path

`github.com/nlink-jp/slack-personal-agent`

## Key Structure

```
slack-personal-agent/
├── main.go              ← Wails entry point
├── app.go               ← App struct, Wails bindings, orchestrator
├── internal/
│   ├── config/          ← TOML config, env overrides, validation
│   │   ├── config.go
│   │   └── config_test.go
│   ├── keychain/        ← macOS Keychain credential storage
│   │   ├── keychain.go  ← Store interface + OSStore
│   │   ├── mock.go      ← MockStore for testing
│   │   └── keychain_test.go
│   ├── slack/           ← Slack Web API client + polling
│   │   ├── client.go    ← API methods (list, history, post)
│   │   ├── queue.go     ← Priority queue, rate limiter, scheduler, poller
│   │   └── queue_test.go
│   ├── memory/          ← DuckDB message store + lifecycle
│   │   ├── record.go    ← Record model, Slack timestamp parser
│   │   ├── store.go     ← DuckDB CRUD, tier transitions
│   │   ├── lifecycle.go ← Hot→Warm→Cold compaction
│   │   ├── record_test.go
│   │   ├── store_test.go
│   │   └── lifecycle_test.go
│   ├── llm/             ← Chat/summarize LLM interface
│   │   ├── backend.go   ← Backend interface + factory
│   │   ├── local.go     ← OpenAI-compatible API
│   │   ├── vertexai.go  ← Vertex AI Gemini
│   │   ├── token.go     ← Token estimation (CJK-aware)
│   │   └── backend_test.go
│   ├── embedding/       ← Text vectorization (LLM-independent)
│   │   ├── embedder.go  ← Embedder interface + factory
│   │   ├── local.go     ← OpenAI-compatible /v1/embeddings
│   │   ├── vertexai.go  ← Vertex AI text-embedding
│   │   ├── mock.go      ← MockEmbedder for testing
│   │   └── embedder_test.go
│   ├── rag/             ← Channel-scoped vector search
│   │   ├── retriever.go ← 3-tier scope filter, DuckDB list_cosine_similarity
│   │   └── retriever_test.go
│   ├── mitl/            ← Proxy response approval
│   │   ├── mitl.go      ← Manager, Proposal lifecycle, timeout
│   │   └── mitl_test.go
│   └── knowledge/       ← Internal knowledge base
│       ├── knowledge.go ← CRUD, scope (workspace/global)
│       └── knowledge_test.go
├── frontend/
│   └── src/
│       ├── App.tsx      ← Dashboard, workspace cards, query UI
│       ├── App.css      ← Dark theme
│       └── main.tsx     ← React entry point
├── docs/
│   ├── en/
│   │   ├── architecture.md
│   │   └── slack-personal-agent-rfp.md
│   └── ja/
│       ├── architecture.ja.md
│       └── slack-personal-agent-rfp.ja.md
├── Makefile
├── wails.json
└── go.mod
```

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `SPA_LLM_BACKEND` | LLM backend (local / vertex_ai) |
| `SPA_LOCAL_ENDPOINT` | Local LLM endpoint |
| `SPA_LOCAL_MODEL` | Local LLM model |
| `SPA_VERTEX_PROJECT` | Vertex AI project |
| `SPA_VERTEX_REGION` | Vertex AI region |
| `SPA_EMBEDDING_BACKEND` | Embedding backend (builtin / local / vertex_ai) |
| `SPA_POLLING_INTERVAL` | Polling interval in seconds |
| `SPA_MAX_RATE_PER_MIN` | Max Slack API calls per minute |
| `SPA_TOKEN_<WORKSPACE>` | Slack User Token override (dev/test only) |

## Gotchas

- **User Token, not Bot Token** — `xoxp-` tokens only. Socket Mode unavailable.
- **DuckDB + Wails** — `no_duckdb_arrow` build tag required.
- **DuckDB ART index** — No PRIMARY KEY on tables with UPDATE operations; use plain indexes.
- **Keychain credentials** — Tokens in macOS Keychain, never in config.toml.
- **3-tier isolation** — RAG queries always require workspace_id + channel_id. No global search.
- **MITL required** — All Slack posts require user approval + signature.
- **Embedding ≠ LLM** — Embedding backend is independent. Switching LLM backend does NOT affect embeddings.
- **ModelID tracking** — Embedding model change requires re-index; system detects mismatch on startup.

## Series

chatops-series (umbrella: nlink-jp/chatops-series)
