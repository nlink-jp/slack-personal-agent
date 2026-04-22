# CLAUDE.md — slack-personal-agent

**Organization rules (mandatory): https://github.com/nlink-jp/.github/blob/main/CONVENTIONS.md**

## Overview

Personal Slack knowledge agent. Monitors multiple workspaces via User Token
polling, accumulates channel information with time-aware 3-tier memory, and
provides channel-scoped RAG with strict 3-tier knowledge isolation.
Go + Wails v2 + React GUI.

## Build

- Always `make build` (outputs to `dist/slack-personal-agent.app`)
- Development: `make dev`
- Tests: `make test`
- Build tag: `no_duckdb_arrow` is required for DuckDB + Wails compatibility

## Architecture

- **main.go** — Entry point, Wails app initialization
- **app.go** — App struct, Wails bindings
- **internal/slack/** — Slack API client, polling, API queue
- **internal/memory/** — Hot/Warm/Cold lifecycle, time-aware records
- **internal/rag/** — DuckDB VSS, channel-scoped retrieval
- **internal/llm/** — LLM backend interface (local + Vertex AI)
- **internal/keychain/** — Credential storage (go-keyring)
- **internal/config/** — TOML config management
- **frontend/src/** — React frontend

## Key Design Decisions

- **User Token** — Personal agent, not a shared bot. Channel visibility enforced by Slack API.
- **Polling** — Socket Mode unavailable for User Tokens. Only realistic option.
- **Per-workspace API queue** — Rate limits are per-workspace. Priority control on response.
- **3-tier knowledge isolation** — Channel (L1) → WS cross-channel (L2) → cross-WS (L3). Default is L1.
- **Keychain-first** — Tokens in macOS Keychain via go-keyring. Never in config.toml.
- **MITL proxy response** — All Slack posts require user approval. No automatic posting.
- **Time awareness** — Current time in system prompt + timestamps on all records (shell-agent pattern).
- **LLM backend interface** — data-agent pattern. Config-driven local/vertex_ai switching.

## Series

chatops-series (umbrella: nlink-jp/chatops-series)
