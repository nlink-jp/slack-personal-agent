# Setup Guide: slack-personal-agent

This guide walks you through the complete setup process, from Slack App
creation to your first query.

## Prerequisites

- macOS (Apple Silicon or Intel)
- Go 1.23+
- [Wails v2](https://wails.io/docs/gettingstarted/installation)
- Node.js 18+
- A Slack workspace where you have permission to install apps

## Step 1: Create a Slack App

1. Go to [https://api.slack.com/apps](https://api.slack.com/apps)
2. Click **Create New App** > **From scratch**
3. Enter:
   - App Name: `slack-personal-agent` (or any name you prefer)
   - Workspace: Select the workspace you want to monitor
4. Click **Create App**

> **Note:** You need to create a separate Slack App for each workspace you
> want to monitor. The app itself is not shared — it exists to grant your
> personal User Token the necessary scopes.

## Step 2: Configure OAuth Scopes

1. In the app settings, go to **OAuth & Permissions**
2. Scroll to **User Token Scopes** (not Bot Token Scopes)
3. Add the following scopes:

| Scope | Purpose |
|-------|---------|
| `channels:history` | Read public channel messages |
| `channels:read` | List public channels and metadata |
| `groups:history` | Read private channel messages |
| `groups:read` | List private channels |
| `chat:write` | Post proxy responses (as yourself) |
| `files:read` | Download file attachments |
| `users:read` | Resolve user display names |

> **Important:** These are **User Token Scopes**, not Bot Token Scopes.
> spa operates as you, not as a bot.

## Step 3: Install the App and Get Your User Token

1. Still in **OAuth & Permissions**, scroll up and click **Install to Workspace**
2. Review the permissions and click **Allow**
3. After installation, you'll see a **User OAuth Token** starting with `xoxp-`
4. Copy this token — you'll need it in Step 5

> **Security:** Never share this token. It has your personal Slack permissions.
> spa stores it in macOS Keychain, never in config files.

## Step 4: Build the Application

```bash
cd chatops-series/slack-personal-agent

# Install frontend dependencies
cd frontend && npm install && cd ..

# Build the app
make build
```

The app bundle is created at `dist/slack-personal-agent.app`.

## Step 5: Configure Workspaces

Create the config file:

```bash
mkdir -p ~/Library/Application\ Support/slack-personal-agent
```

Edit `~/Library/Application Support/slack-personal-agent/config.toml`:

```toml
# Workspace definitions — tokens are stored in Keychain, not here
[[workspace]]
name = "my-company"

# Add more workspaces if needed
# [[workspace]]
# name = "other-workspace"

# LLM backend for chat/summarization
[llm]
backend = "local"  # or "vertex_ai"

[local_llm]
endpoint = "http://localhost:1234/v1"
model = "google/gemma-4-26b-a4b"

# Embedding (independent of LLM — defaults to builtin)
[embedding]
backend = "builtin"

# Polling
[polling]
interval_sec = 120        # Check channels every 2 minutes
priority_boost_sec = 15   # 15 seconds when actively responding
max_rate_per_min = 45     # Stay under Slack's 50/min limit

# Memory lifecycle
[memory]
hot_to_warm_min = 1440    # Summarize after 24 hours
warm_to_cold_min = 10080  # Archive after 7 days

# Proxy response
[response]
timeout_sec = 120
signature = "— via spa (slack-personal-agent)"
```

## Step 6: Store Your Token in Keychain

Launch the app and use the GUI to add your token, or use the macOS
`security` command:

```bash
# Store via command line (alternative to GUI)
security add-generic-password \
  -s "slack-personal-agent" \
  -a "workspace:my-company" \
  -w "xoxp-your-token-here"
```

> The workspace name in the `-a` flag must match the `name` in config.toml,
> prefixed with `workspace:`.

## Step 7: Launch and Start Monitoring

1. Open `dist/slack-personal-agent.app`
2. On the **Dashboard** tab, your workspace should show "Token set"
3. Click **Start** to begin polling
4. The app will discover your channels and start collecting messages

## Step 8: Verify

- **Dashboard:** Workspace shows "Polling" badge, memory stats increase
- **Query tab:** Enter a workspace ID, channel ID, and a question
- **Knowledge tab:** Add a test entry and verify it appears

## Multiple Workspaces

To monitor multiple workspaces:

1. Create a Slack App in each workspace (Step 1-3)
2. Add each workspace to config.toml:
   ```toml
   [[workspace]]
   name = "company-a"

   [[workspace]]
   name = "company-b"
   ```
3. Store each token in Keychain:
   ```bash
   security add-generic-password -s "slack-personal-agent" -a "workspace:company-a" -w "xoxp-..."
   security add-generic-password -s "slack-personal-agent" -a "workspace:company-b" -w "xoxp-..."
   ```

## Cross-Channel Scope Groups

To allow knowledge sharing between specific channels (Level 2/3):

```toml
# Channels in this group can see each other's knowledge
[[scope_group]]
name = "security-team"

[[scope_group.member]]
workspace = "company-a"
channel = "C01SECURITY"

[[scope_group.member]]
workspace = "company-a"
channel = "C02INCIDENTS"

# Cross-workspace group
[[scope_group]]
name = "infra-cross"

[[scope_group.member]]
workspace = "company-a"
channel = "C03INFRA"

[[scope_group.member]]
workspace = "company-b"
channel = "C04INFRA"
```

## LLM Backend Options

### Local LLM (default)

Requires [LM Studio](https://lmstudio.ai/) or compatible server:

```toml
[llm]
backend = "local"

[local_llm]
endpoint = "http://localhost:1234/v1"
model = "google/gemma-4-26b-a4b"
```

### Vertex AI (Gemini)

Requires Google Cloud credentials ([ADC setup](https://cloud.google.com/docs/authentication/application-default-credentials)):

```toml
[llm]
backend = "vertex_ai"

[vertex_ai]
project = "your-project-id"
region = "us-central1"
model = "gemini-2.5-flash"
```

## Troubleshooting

### "No token" for workspace

- Verify the keychain entry exists: `security find-generic-password -s "slack-personal-agent" -a "workspace:my-company"`
- Ensure the workspace name matches exactly between config.toml and keychain

### Rate limit errors

- Reduce `max_rate_per_min` in config.toml
- Reduce the number of monitored channels
- Increase `interval_sec`

### Embedding model download

On first run with `backend = "builtin"`, the app downloads the
all-MiniLM-L6-v2 model (~90MB). This requires internet access. Subsequent
runs use the cached model.

### Build errors

- Ensure Wails v2 is installed: `wails doctor`
- DuckDB requires the `no_duckdb_arrow` build tag (handled by Makefile)
- Frontend must be built first: `cd frontend && npm install && npm run build`
