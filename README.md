# slack-personal-agent (spa)

Personal knowledge agent that monitors multiple Slack workspaces, accumulates
channel information with time-aware memory, and provides channel-scoped RAG
queries with strict 3-tier information isolation.

## Features

- **Multi-workspace monitoring** — Poll multiple Slack workspaces via User Token
- **3-tier knowledge isolation** — Channel-local (default) → workspace cross-channel → cross-workspace
- **Time-aware memory** — Hot/Warm/Cold lifecycle with timestamp-based relative time queries
- **Channel-scoped RAG** — DuckDB vector search with mandatory workspace/channel filters
- **MITL proxy response** — Draft responses with approval gate, signature for sender identification
- **Internal knowledge base** — Register and manage non-Slack knowledge with scope control
- **Dual LLM backend** — Local LLM (OpenAI-compatible) or Vertex AI Gemini
- **Independent embedding** — Decoupled from LLM backend; no re-indexing on backend switch
- **Secure credentials** — macOS Keychain via go-keyring; tokens never in config files

## Installation

### Prerequisites

- Go 1.23+
- [Wails v2](https://wails.io/)
- Node.js 18+

### Build

```bash
make build
```

The app bundle is created at `dist/slack-personal-agent.app`.

### Development

```bash
make dev
```

### Test

```bash
make test
```

## Architecture

```
app.go (orchestrator)
 ├── internal/config      — TOML config, env overrides, validation
 ├── internal/keychain    — macOS Keychain credential storage
 ├── internal/slack       — Slack API client, priority queue, polling
 ├── internal/memory      — DuckDB message store, Hot/Warm/Cold lifecycle
 ├── internal/llm         — Chat/summarize backend (local / Vertex AI)
 ├── internal/embedding   — Text vectorization (independent of LLM)
 ├── internal/rag         — 3-tier scoped vector similarity search
 ├── internal/mitl        — Proxy response approval workflow
 └── internal/knowledge   — Internal knowledge base
```

See [Architecture Document](docs/en/architecture.md) for detailed design decisions.

## Configuration

Config file: `~/Library/Application Support/slack-personal-agent/config.toml`

```toml
# Workspaces — tokens stored in macOS Keychain, not here
[[workspace]]
name = "company-a"

[[workspace]]
name = "company-b"

# LLM backend (chat/summarization)
[llm]
backend = "local"  # or "vertex_ai"

[local_llm]
endpoint = "http://localhost:1234/v1"
model = "google/gemma-4-26b-a4b"

[vertex_ai]
project = "PROJECT_ID"
region = "us-central1"
model = "gemini-2.5-flash"

# Embedding (independent of LLM)
[embedding]
backend = "builtin"  # "builtin" | "local" | "vertex_ai"

# Polling
[polling]
interval_sec = 120
priority_boost_sec = 15
max_rate_per_min = 45

# Memory lifecycle
[memory]
hot_to_warm_min = 1440   # 24 hours
warm_to_cold_min = 10080 # 7 days

# MITL proxy response
[response]
timeout_sec = 120
signature = "— via spa (slack-personal-agent)"
```

### Slack User Token Scopes

| Scope | Purpose |
|-------|---------|
| `channels:history` | Read public channel messages |
| `channels:read` | List public channels |
| `groups:history` | Read private channel messages |
| `groups:read` | List private channels |
| `chat:write` | Post proxy responses |
| `files:read` | Download file attachments |
| `users:read` | Resolve user names |

## Knowledge Isolation Model

| Level | Scope | Default | Activation |
|-------|-------|---------|------------|
| Level 1 | Within channel only | Enabled | Always (fundamental unit) |
| Level 2 | Cross-channel within workspace | Disabled | User-defined channel groups |
| Level 3 | Cross-workspace | Disabled | User-explicit permission |

## Documentation

- [Architecture (EN)](docs/en/architecture.md) / [(JA)](docs/ja/architecture.ja.md)
- [RFP (EN)](docs/en/slack-personal-agent-rfp.md) / [(JA)](docs/ja/slack-personal-agent-rfp.ja.md)

## License

MIT License. See [LICENSE](LICENSE) for details.
