# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.1.0] - Unreleased

### Added

- Project scaffold (Wails v2 + React + TypeScript)
- `internal/config` — TOML configuration with env overrides, workspace/LLM/embedding/polling settings
- `internal/keychain` — macOS Keychain credential storage via go-keyring (Store interface + MockStore)
- `internal/slack` — Slack Web API client, priority queue with rate limiting, polling scheduler
- `internal/memory` — DuckDB message store with 3-tier lifecycle (Hot/Warm/Cold)
- `internal/llm` — LLM backend interface with local (OpenAI-compatible) and Vertex AI implementations
- `internal/embedding` — Embedder interface independent of LLM backend (local/Vertex AI/mock; builtin planned)
- `internal/rag` — Channel-scoped vector similarity search with 3-tier knowledge isolation (L1/L2/L3)
- `internal/mitl` — Man-in-the-loop proxy response workflow (proposal → approve/reject/timeout)
- `internal/knowledge` — Internal knowledge base with workspace/global scope
- Wails app integration with dashboard UI (workspace status, memory stats, query interface)
- Proxy response signature for sender identification
- RFP documentation (EN/JA)
- Architecture documentation (EN/JA)
