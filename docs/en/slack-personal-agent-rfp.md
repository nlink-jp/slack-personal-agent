# RFP: slack-personal-agent (spa)

> Generated: 2026-04-22
> Status: Draft

## 1. Problem Statement

In Slack workspaces, information flows continuously across multiple channels. Users cannot monitor all channels at all times, leading to missed critical information and difficulty searching past discussions. The existing Slack bot (sai) operates as a workspace-wide shared agent with fundamental limitations: (1) no channel isolation in RAG (global scope retrieval), (2) no temporal awareness causing relative time queries to fail, and (3) inability to maintain a personal perspective.

slack-personal-agent (spa) is a personal knowledge agent that automatically monitors channels across **multiple Slack workspaces** from an **individual user's perspective**, accumulates information, and provides time-aware, channel-scoped RAG queries. It enforces a 3-tier information isolation model (channel → workspace cross-channel → cross-workspace) to prevent information leakage, supports knowledge integration only with explicit user permission at each tier, and offers proxy responses through an approval gate. The target user is any team member who needs to track information across multiple Slack workspaces and channels daily.

## 2. Functional Specification

### GUI Interface

Desktop GUI application built with Wails v2 + React.

- Channel status dashboard (monitored channels list, update status, accumulation metrics)
- Time-aware knowledge query interface
- Proxy response review/approve/reject UI
- Cross-channel permission management UI
- Internal knowledge base registration and management UI

### Input / Output

- **Input**: Message data from Slack Conversations API (JSON), file attachments, user-registered internal knowledge
- **Output**: Digest display in GUI, RAG responses, proxy posts (Slack posts via User Token)

### Configuration

- TOML config file (`~/Library/Application Support/slack-personal-agent/config.toml`)
- Environment variable overrides (`SPA_*` prefix)
- Configuration items:
  - Workspace definitions (multiple): name, Slack User Token (`xoxp-`)
  - Monitored channels (auto-detection + manual exclusion per workspace)
  - Polling interval (default + per-channel overrides)
  - LLM backend selection (local / vertex_ai)
  - LLM endpoint and model settings
  - Knowledge scope permission settings (3-tier isolation model):
    - Level 1: Per-channel (default, no configuration needed)
    - Level 2: Workspace-internal cross-channel permission groups
    - Level 3: Cross-workspace permission flags
  - Proxy response timeout
  - Lifecycle thresholds (Hot→Warm, Warm→Cold durations)

### Credential Management

Slack User Tokens and LLM API keys are stored in **macOS Keychain**. Config files never contain tokens.

- **Storage**: `github.com/zalando/go-keyring` (proven in scli)
  - Supports macOS Keychain / Linux libsecret / Windows Credential Manager
- **Key structure**: Service=`slack-personal-agent`, Account=`workspace:<name>` / `llm:<backend>`
- **config.toml stores only workspace names and metadata** (no tokens)
- **Test fallback**: Environment variable `SPA_TOKEN_<WORKSPACE>` injection (development/testing only)
- **Testability**: `Store` interface enables Keychain mocking (follows scli pattern)

```toml
# config.toml — tokens stored in Keychain, never here
[[workspace]]
name = "company-a"

[[workspace]]
name = "company-b"
```

Initial setup: GUI workspace addition → token input → stored in Keychain → only name recorded in config

### External Dependencies

- **Slack API** — Conversations API (User Token `xoxp-`, retrieved from Keychain)
- **LLM** — Local LLM (OpenAI-compatible API) or Vertex AI Gemini
- **DuckDB** — Local database with VSS extension
- **macOS Notifications** — Proxy response approval request notifications
- **macOS Keychain** — Secure credential storage

### Core Feature Details

#### Channel Monitoring and API Queue Control

A per-workspace queue abstraction layer manages all Slack API calls centrally.

- **Per-workspace queues**: Rate limits are independent per workspace, so queues are managed per workspace
- Normal mode: all monitored channels polled at uniform intervals
- Response mode: dynamically assign priority queue to the active channel
- Auto-adjust within Tier 3 rate limit (~50 req/min per workspace)
- Calculate consumption rate from channel count and polling interval to prevent limit breaches

#### Information Accumulation and 3-Tier Lifecycle

Adopts the 3-tier memory model proven in shell-agent / sai.

| Tier | State | Content |
|------|-------|---------|
| Hot | Recent raw data | Original text preserved with full timestamps |
| Warm | Summarized | LLM-generated summary with time range |
| Cold | Archive | Long-term storage, read-only |

All records carry absolute timestamps. Current time is injected into context, enabling the LLM to resolve relative time expressions ("yesterday", "last week", etc.) following the shell-agent pattern.

#### Embedding Abstraction

Text vectorization (embedding generation) is designed as an abstraction layer **completely independent** from the LLM backend.

```
Embedder interface
├── BuiltinEmbedder   ← Default. Built-in model (all-MiniLM-L6-v2 etc.)
├── LocalEmbedder     ← OpenAI-compatible /v1/embeddings API (LM Studio etc.)
└── VertexAIEmbedder  ← Vertex AI text-embedding-005
```

**Design rationale:**

- Switching LLM backends does **not trigger re-indexing** (embeddings are independent)
- Default is built-in model for **zero-config, offline operation**
- Can be swapped for higher-quality models when needed
- DB records the `ModelID` used at index time; detects mismatch on startup → prompts re-index

```toml
[embedding]
backend = "builtin"  # "builtin" | "local" | "vertex_ai"
```

#### 3-Tier Knowledge Isolation Model and Channel-Scoped RAG

The fundamental unit in the system is the **channel**. Knowledge scope is managed in three tiers:

| Level | Scope | Default | Activation |
|-------|-------|---------|------------|
| Level 1 | Within channel only | **Enabled** | Always enabled (fundamental unit) |
| Level 2 | Cross-channel within same workspace | Disabled | User defines permitted channel groups |
| Level 3 | Cross-workspace | Disabled | Only user-explicitly-permitted knowledge |

- DuckDB VSS vector search with mandatory `workspace_id` + `channel_id` filter
- Level 1 (default): answer using only knowledge from the channel where the query originated
- Level 2: cross-channel search within user-permitted channel groups in the same workspace
- Level 3: only knowledge explicitly marked as cross-workspace shareable by the user
- Information from non-permitted scopes must never be included
- Internal knowledge base entries can be assigned to Level 2 or Level 3 scopes

#### Proxy Response (MITL)

1. Triggered when the system detects a response opportunity during channel monitoring
2. Generates draft response, displays via macOS notification + GUI
3. User approves → posts via User Token (appears as the user)
4. Timeout or rejection → discarded
5. Dynamically increases polling priority for the response channel

#### Internal Knowledge Base

- Users manually register and manage knowledge beyond Slack
- Supports text and document ingestion
- Included as a RAG knowledge source alongside Slack messages

#### File Attachment Analysis

- Ingests content from files shared in channels
- Downloads via `files:read` scope → extracts content → integrates into knowledge base

## 3. Design Decisions

| Decision | Rationale |
|----------|-----------|
| Go + Wails v2 + React | Proven GUI + LLM + DuckDB combination in shell-agent. Keeps architecture simple |
| User Token (`xoxp-`) | Personal agent operates under user's own permissions. Channel visibility enforced by Slack API |
| Polling (not Events API) | Socket Mode unavailable for User Tokens. HTTP webhooks unsuitable for local apps. Polling is the only realistic option |
| LLM Backend interface | Follows data-agent pattern. `Backend` interface switches between local LLM and Vertex AI via config. Chat/summarization only |
| Embedding abstraction (independent of LLM) | Embeddings decoupled from LLM backend. Prevents re-indexing on backend switch. Default is built-in model (zero-config); swappable when needed. ModelID mismatch detection |
| DuckDB VSS | Proven in lite-rag / gem-rag / shell-agent. Channel ID-filtered vector search is naturally implementable |
| MITL proxy response | Posting via User Token appears as the user — high risk for automatic posting. Follows shell-agent's approval gate pattern |
| Per-workspace API queues | Slack API rate limits are independent per workspace. Queues managed per workspace with priority control balancing response quality and API consumption |
| 3-tier knowledge isolation | Channel (fundamental unit) → workspace cross-channel → cross-workspace. Default is channel-scoped; progressively opened with explicit permission |
| Multi-workspace support | Users participate in multiple Slack workspaces. Each workspace has independent User Token and API queue management |
| Keychain-first credentials | GUI app with no headless operation. Tokens stored in macOS Keychain, never in config.toml. Follows scli's `go-keyring` pattern |
| 3-tier lifecycle | Memory model progressively refined through sai → shell-agent. Time awareness based on shell-agent's implementation |

### Relationship with Existing Tools

| Tool | Relationship |
|------|-------------|
| sai (lab-series) | Prototype for memory model and RAG. This project resolves sai's lack of channel isolation and time awareness |
| shell-agent (util-series) | Foundation for time-aware memory, GUI stack, MITL, and LLM integration |
| data-agent (util-series) | Source of the LLM Backend abstraction pattern |
| stail (chatops-series) | Socket Mode real-time monitoring expertise (this project adopts polling instead) |
| gem-rag / lite-rag | DuckDB VSS RAG implementation reference |

### Out of Scope

- DM monitoring
- Multi-user support (single-user personal tool)
- Automatic integration with external information sources beyond Slack (internal knowledge base is manual registration only)

## 4. Development Plan

### Phase 1: Core Foundation

- Slack User Token authentication and `conversations.list` / `conversations.history` polling
- Global API queue (rate limit management, priority control)
- DuckDB schema design (messages, embeddings, channel metadata)
- 3-tier lifecycle (Hot/Warm/Cold) implementation
- Channel-scoped RAG (DuckDB VSS + channel_id filter)
- LLM Backend interface (local LLM + Vertex AI)
- Tests: lifecycle transitions, RAG isolation, queue control

**Reviewable**: Core operation of API queue + accumulation + RAG verifiable via CLI

### Phase 2: GUI + Query

- Wails v2 + React GUI foundation
- Channel status dashboard
- Time-aware query interface
- Cross-channel permission management UI
- Settings screen

**Reviewable**: Full information query flow verifiable in GUI

### Phase 3: Proxy Response + Knowledge Base

- Response intent detection logic
- MITL flow (macOS notification + GUI approve/reject)
- Proxy posting (User Token `chat:write`)
- Dynamic polling priority adjustment on response
- Internal knowledge base (manual registration, management, RAG integration)

**Reviewable**: End-to-end proxy response flow verifiable

### Phase 4: Attachments + Polish

- File attachment download and content extraction
- File content knowledge integration
- Documentation (README.md / README.ja.md / CHANGELOG.md)
- E2E testing
- Release

## 5. Required API Scopes / Permissions

### Slack User Token (`xoxp-`) Scopes

| Scope | Purpose |
|-------|---------|
| `channels:history` | Retrieve public channel messages |
| `channels:read` | List public channels and metadata |
| `groups:history` | Retrieve private channel messages |
| `groups:read` | List private channels |
| `chat:write` | Post proxy responses |
| `files:read` | Download file attachments |
| `users:read` | Resolve user names |

### LLM (Vertex AI)

- Google Cloud Application Default Credentials (ADC)
- Vertex AI API enabled
- `aiplatform.endpoints.predict` permission

### LLM (Local)

- No external permissions required (local endpoint such as LM Studio)

## 6. Series Placement

**Series:** chatops-series

**Reason:** The primary function is Slack workspace information collection and automation, which aligns with the chatops-series domain (Slack ChatOps automation tools). It is positioned within the same Slack ecosystem as swrite / scat / stail / slack-router.

## 7. External Platform Constraints

### Slack API

| Constraint | Impact | Mitigation |
|-----------|--------|------------|
| Tier 3 rate limit (~50 req/min per workspace) | May reach limit with many channels × multiple workspaces | Per-workspace queue manages consumption rate |
| `conversations.history` limit 1000 msgs/call | Initial bulk fetch of messages | Cursor pagination support |
| User Token OAuth flow | Requires browser authentication for initial setup | Provide setup wizard |
| Token expiration | User Tokens are long-lived but may expire | Error handling and re-authentication flow |
| File download | URL requires token parameter | `files:read` scope + Authorization header |

### macOS

| Constraint | Impact | Mitigation |
|-----------|--------|------------|
| Notification permission | macOS permission required for MITL notifications | Request permission on first launch |
| Background execution | Polling continuity | Design as macOS service |

---

## Discussion Log

### sai Limitation Analysis

Evaluated existing sai (lab-series) and identified fundamental issues:

1. **No channel isolation**: RAG search scoped to entire workspace; `retriever.retrieve()` has no channel filter. Questions in #private-engineering could retrieve #general content
2. **No temporal awareness**: Messages have timestamps but current time is not injected into LLM context, causing relative time queries ("yesterday's discussion") to fail
3. **Global bot**: Shared agent using Bot Token serving the entire workspace; cannot maintain per-user perspective, permissions, or knowledge scope

→ Decided to design as a **completely new project** rather than evolving sai.

### Technical Inheritance from shell-agent

shell-agent had already incorporated sai's memory concept and added time awareness:

- Current time injected into system prompt, Hot messages prefixed with `[HH:MM:SS]`
- 3-tier lifecycle (Hot/Warm/Cold) + Pinned memory
- LLM resolves relative time by comparing current time with message timestamps

→ This time awareness pattern serves as the foundation for this project.

### Authentication Model Decision

- Bot Token is suited for shared agents; User Token is appropriate for personal agents
- User Token ensures channel visibility is enforced by the Slack API
- Socket Mode unavailable for User Token → polling adopted

### Proxy Response Safety Design

- Posting via User Token appears as the user, making automatic posting high-risk
- Follows shell-agent's MITL pattern: system detects response intent → macOS notification + GUI → user approval/timeout

### LLM Abstraction Strategy

- Adopts data-agent's `Backend` interface pattern
- Switches via `[llm] backend = "local" | "vertex_ai"` in config.toml
- Whether to send Slack messages to cloud is left to the user's judgment

### Scope Adjustments

Re-evaluated items initially marked as out of scope:

- **Internal knowledge base**: Added as a feature for manually registering non-Slack knowledge
- **File attachment analysis**: Desirable if feasible; placed in Phase 4

### Multi-Workspace Support

Initially assumed a single workspace, but updated to reflect the reality that users participate in multiple Slack workspaces:

- Each workspace requires its own User Token
- API queues managed independently per workspace (rate limits are per-workspace)
- GUI provides an integrated view across workspaces

### 3-Tier Knowledge Isolation Model

Established channels as the system's fundamental unit, with global knowledge structured in two additional layers:

- **Level 1 (default)**: Channel-local knowledge only — safest, no configuration needed
- **Level 2**: Workspace-internal cross-channel — shared within user-permitted channel groups
- **Level 3**: Cross-workspace — only knowledge explicitly permitted by the user can be used across workspaces

This design minimizes information leakage risk while allowing progressive expansion of knowledge integration scope as needed.

### Embedding Decoupling from LLM Backend

Initially, embedding generation was part of the LLM backend (local / vertex_ai), but the following issue was identified:

- **Switching LLM backends changes the embedding model, requiring full vector re-indexing**
- This is unacceptable from a user experience standpoint (hours of rebuilding on each switch)

→ Embeddings are fully decoupled from the LLM backend as an independent `Embedder` interface:

- Default is a built-in model (all-MiniLM-L6-v2 etc.) for zero-config, offline operation
- Can be swapped to local (OpenAI-compatible API) or Vertex AI when needed
- DB records `ModelID`; on model change, explicitly prompts for re-index (prevents silent inconsistency)
