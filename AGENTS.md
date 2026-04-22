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
├── app.go               ← App struct, Wails bindings
├── internal/            ← Private packages (to be implemented)
│   ├── slack/           ← Slack API client, polling, queue
│   ├── memory/          ← 3-tier lifecycle (Hot/Warm/Cold)
│   ├── rag/             ← DuckDB VSS, channel-scoped retrieval
│   ├── llm/             ← LLM backend interface
│   ├── keychain/        ← Credential storage (go-keyring)
│   └── config/          ← TOML configuration
├── frontend/
│   └── src/             ← React TypeScript frontend
├── docs/
│   ├── en/              ← English documentation
│   └── ja/              ← Japanese documentation
├── Makefile
├── wails.json
└── go.mod
```

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `SPA_TOKEN_<WORKSPACE>` | Slack User Token override (dev/test only) |
| `SPA_LLM_BACKEND` | LLM backend override (local / vertex_ai) |
| `SPA_LOCAL_ENDPOINT` | Local LLM endpoint override |
| `SPA_VERTEX_PROJECT` | Vertex AI project override |

## Gotchas

- **User Token, not Bot Token** — This is a personal agent using `xoxp-` tokens.
  Socket Mode is not available for User Tokens; polling is the only option.
- **DuckDB + Wails** — Must use `no_duckdb_arrow` build tag to avoid Arrow
  dependency issues on macOS.
- **Keychain credentials** — Tokens are stored in macOS Keychain via go-keyring.
  Config files never contain tokens. Test with env var fallback only.
- **3-tier knowledge isolation** — RAG queries must always include workspace_id +
  channel_id filters. Global search is never permitted by default.
- **MITL required** — All Slack posts go through user approval. No automatic posting.

## Series

chatops-series (umbrella: nlink-jp/chatops-series)
