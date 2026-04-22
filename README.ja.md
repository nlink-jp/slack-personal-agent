# slack-personal-agent (spa)

複数の Slack ワークスペースを監視し、チャネル情報を時間認識付きで蓄積、
3層知識分離モデルによるチャネルスコープ RAG クエリを提供する
パーソナルナレッジエージェント。

## 機能

- **複数ワークスペース監視** — User Token による複数 Slack ワークスペースのポーリング
- **3層知識分離** — チャネル内（デフォルト）→ ワークスペース内クロスチャネル → クロスワークスペース
- **時間認識記憶** — Hot/Warm/Cold ライフサイクル、タイムスタンプベースの相対時間クエリ
- **チャネルスコープ RAG** — DuckDB ベクトル検索、ワークスペース/チャネルフィルタ必須
- **MITL 代理応答** — 下書き生成 + 承認ゲート、送信者識別シグニチャ
- **内部知識ベース** — Slack 外の知識を登録・管理、スコープ制御付き
- **デュアル LLM バックエンド** — ローカル LLM（OpenAI 互換）または Vertex AI Gemini
- **独立エンベディング** — LLM バックエンドから分離; バックエンド切替時の再 index 不要
- **セキュアなクレデンシャル** — macOS Keychain（go-keyring）; config にトークンを保存しない

## インストール

### 前提条件

- Go 1.23+
- [Wails v2](https://wails.io/)
- Node.js 18+

### ビルド

```bash
make build
```

アプリバンドルが `dist/slack-personal-agent.app` に作成されます。

### 開発

```bash
make dev
```

### テスト

```bash
make test
```

## アーキテクチャ

```
app.go (オーケストレータ)
 ├── internal/config      — TOML 設定、環境変数、バリデーション
 ├── internal/keychain    — macOS Keychain クレデンシャル保存
 ├── internal/slack       — Slack API クライアント、優先度キュー、ポーリング
 ├── internal/memory      — DuckDB メッセージストア、Hot/Warm/Cold ライフサイクル
 ├── internal/llm         — チャット/要約バックエンド (local / Vertex AI)
 ├── internal/embedding   — テキストベクトル化（LLM から独立）
 ├── internal/rag         — 3層スコープ付きベクトル類似度検索
 ├── internal/mitl        — 代理応答承認ワークフロー
 └── internal/knowledge   — 内部知識ベース
```

詳細な設計判断は[アーキテクチャドキュメント](docs/ja/architecture.ja.md)を参照。

## 設定

設定ファイル: `~/Library/Application Support/slack-personal-agent/config.toml`

```toml
# ワークスペース — トークンは macOS Keychain に保存（ここには書かない）
[[workspace]]
name = "company-a"

[[workspace]]
name = "company-b"

# LLM バックエンド（チャット/要約）
[llm]
backend = "local"  # または "vertex_ai"

[local_llm]
endpoint = "http://localhost:1234/v1"
model = "google/gemma-4-26b-a4b"

[vertex_ai]
project = "PROJECT_ID"
region = "us-central1"
model = "gemini-2.5-flash"

# エンベディング（LLM から独立）
[embedding]
backend = "builtin"  # "builtin" | "local" | "vertex_ai"

# ポーリング
[polling]
interval_sec = 120
priority_boost_sec = 15
max_rate_per_min = 45

# メモリライフサイクル
[memory]
hot_to_warm_min = 1440   # 24時間
warm_to_cold_min = 10080 # 7日

# MITL 代理応答
[response]
timeout_sec = 120
signature = "— via spa (slack-personal-agent)"
```

### Slack User Token スコープ

| スコープ | 用途 |
|---------|------|
| `channels:history` | パブリックチャネルのメッセージ取得 |
| `channels:read` | パブリックチャネル一覧取得 |
| `groups:history` | プライベートチャネルのメッセージ取得 |
| `groups:read` | プライベートチャネル一覧取得 |
| `chat:write` | 代理応答の投稿 |
| `files:read` | 添付ファイルのダウンロード |
| `users:read` | ユーザー名解決 |

## 知識分離モデル

| レベル | スコープ | デフォルト | 有効化方法 |
|--------|---------|-----------|-----------|
| Level 1 | チャネル内のみ | 有効 | 常に有効（基本単位） |
| Level 2 | ワークスペース内クロスチャネル | 無効 | ユーザーがチャネルグループを定義 |
| Level 3 | クロスワークスペース | 無効 | ユーザーが明示的に許可 |

## ドキュメント

- **[セットアップガイド (EN)](docs/en/setup.md) / [(JA)](docs/ja/setup.ja.md)** — Slack App 作成、トークン設定、初回起動
- [アーキテクチャ (EN)](docs/en/architecture.md) / [(JA)](docs/ja/architecture.ja.md)
- [RFP (EN)](docs/en/slack-personal-agent-rfp.md) / [(JA)](docs/ja/slack-personal-agent-rfp.ja.md)

## ライセンス

MIT License。詳細は [LICENSE](LICENSE) を参照。
