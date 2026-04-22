# slack-personal-agent (spa)

複数の Slack ワークスペースを監視し、チャネル情報を時間認識付きで蓄積、
厳格な情報分離モデルによるチャネルスコープ RAG クエリを提供する
パーソナルナレッジエージェント。

## 機能

- **複数ワークスペース監視** — User Token による複数 Slack ワークスペースのポーリング
- **3層知識分離** — チャネル内（デフォルト）→ ワークスペース内クロスチャネル → クロスワークスペース
- **時間認識記憶** — 3層ライフサイクル（Hot/Warm/Cold）、タイムスタンプベースの相対時間クエリ
- **チャネルスコープ RAG** — DuckDB VSS、ワークスペース/チャネルフィルタ必須
- **MITL 代理応答** — 下書き生成 + macOS 通知 + GUI 承認ゲート
- **内部知識ベース** — Slack 外の知識を登録・管理
- **デュアル LLM バックエンド** — ローカル LLM（OpenAI 互換）または Vertex AI Gemini
- **セキュアなクレデンシャル管理** — macOS Keychain（go-keyring）、config にトークンを保存しない

## インストール

### 前提条件

- Go 1.23+
- [Wails v2](https://wails.io/)
- Node.js 18+
- Slack User Token（`xoxp-`）+ 必要なスコープ

### ビルド

```bash
make build
```

アプリバンドルが `dist/slack-personal-agent.app` に作成されます。

### 開発

```bash
make dev
```

## 設定

設定ファイル: `~/Library/Application Support/slack-personal-agent/config.toml`

```toml
# ワークスペース — トークンは macOS Keychain に保存（ここには書かない）
[[workspace]]
name = "company-a"

[[workspace]]
name = "company-b"

# LLM バックエンド
[llm]
backend = "local"  # または "vertex_ai"

[local_llm]
endpoint = "http://localhost:1234/v1"
model = "google/gemma-4-26b-a4b"

[vertex_ai]
project = "PROJECT_ID"
region = "us-central1"
model = "gemini-2.5-flash"
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

- [RFP (EN)](docs/en/slack-personal-agent-rfp.md)
- [RFP (JA)](docs/ja/slack-personal-agent-rfp.ja.md)

## ライセンス

MIT License。詳細は [LICENSE](LICENSE) を参照。
