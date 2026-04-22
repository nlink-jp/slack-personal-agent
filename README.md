# slack-personal-agent (spa)

Personal knowledge agent that monitors multiple Slack workspaces, accumulates
channel information with time awareness, and provides channel-scoped RAG queries
with strict information isolation.

## Features

- **Multi-workspace monitoring** — Poll multiple Slack workspaces via User Token
- **3-tier knowledge isolation** — Channel-local (default) → workspace cross-channel → cross-workspace
- **Time-aware memory** — 3-tier lifecycle (Hot/Warm/Cold) with timestamp-based relative time queries
- **Channel-scoped RAG** — DuckDB VSS with mandatory workspace/channel filters
- **MITL proxy response** — Draft responses with macOS notification + GUI approval gate
- **Internal knowledge base** — Register and manage non-Slack knowledge
- **Dual LLM backend** — Local LLM (OpenAI-compatible) or Vertex AI Gemini
- **Secure credential storage** — macOS Keychain via go-keyring (tokens never in config files)

## Installation

### Prerequisites

- Go 1.23+
- [Wails v2](https://wails.io/)
- Node.js 18+
- Slack User Token (`xoxp-`) with required scopes

### Build

```bash
make build
```

The app bundle is created at `dist/slack-personal-agent.app`.

### Development

```bash
make dev
```

## Configuration

Config file: `~/Library/Application Support/slack-personal-agent/config.toml`

```toml
# Workspaces — tokens are stored in macOS Keychain, not here
[[workspace]]
name = "company-a"

[[workspace]]
name = "company-b"

# LLM backend
[llm]
backend = "local"  # or "vertex_ai"

[local_llm]
endpoint = "http://localhost:1234/v1"
model = "google/gemma-4-26b-a4b"

[vertex_ai]
project = "PROJECT_ID"
region = "us-central1"
model = "gemini-2.5-flash"
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

- [RFP (EN)](docs/en/slack-personal-agent-rfp.md)
- [RFP (JA)](docs/ja/slack-personal-agent-rfp.ja.md)

## License

MIT License. See [LICENSE](LICENSE) for details.
