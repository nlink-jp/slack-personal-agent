# セットアップガイド: slack-personal-agent

Slack App の作成から初回クエリまでの完全な手順を説明します。

## 前提条件

- macOS（Apple Silicon または Intel）
- Go 1.23+
- [Wails v2](https://wails.io/docs/gettingstarted/installation)
- Node.js 18+
- アプリをインストールする権限がある Slack ワークスペース

## Step 1: Slack App を作成

1. [https://api.slack.com/apps](https://api.slack.com/apps) にアクセス
2. **Create New App** > **From scratch** をクリック
3. 以下を入力:
   - App Name: `slack-personal-agent`（任意の名前で可）
   - Workspace: 監視したいワークスペースを選択
4. **Create App** をクリック

> **注意:** 監視したいワークスペースごとに個別の Slack App を作成する必要があります。
> App 自体は共有されません — User Token に必要なスコープを付与するために存在します。

## Step 2: OAuth スコープを設定

1. App 設定画面で **OAuth & Permissions** に移動
2. **User Token Scopes** までスクロール（Bot Token Scopes ではない）
3. 以下のスコープを追加:

| スコープ | 用途 |
|---------|------|
| `channels:history` | パブリックチャネルのメッセージ取得 |
| `channels:read` | パブリックチャネル一覧・メタデータ取得 |
| `groups:history` | プライベートチャネルのメッセージ取得 |
| `groups:read` | プライベートチャネル一覧取得 |
| `chat:write` | 代理応答の投稿（本人として） |
| `files:read` | 添付ファイルのダウンロード |
| `users:read` | ユーザー表示名の解決 |

> **重要:** これらは **User Token Scopes** です（Bot Token Scopes ではありません）。
> spa はボットではなく、あなた自身として動作します。

## Step 3: App をインストールして User Token を取得

1. **OAuth & Permissions** 画面上部の **Install to Workspace** をクリック
2. 権限を確認して **Allow** をクリック
3. インストール後、`xoxp-` で始まる **User OAuth Token** が表示される
4. このトークンをコピー — Step 5 で使用

> **セキュリティ:** このトークンは絶対に共有しないでください。
> あなた個人の Slack 権限を持っています。
> spa は macOS Keychain に保存し、設定ファイルには含めません。

## Step 4: アプリケーションをビルド

```bash
cd chatops-series/slack-personal-agent

# フロントエンド依存のインストール
cd frontend && npm install && cd ..

# ビルド
make build
```

アプリバンドルが `dist/slack-personal-agent.app` に作成されます。

## Step 5: ワークスペースを設定

設定ファイルを作成:

```bash
mkdir -p ~/Library/Application\ Support/slack-personal-agent
```

`~/Library/Application Support/slack-personal-agent/config.toml` を編集:

```toml
# ワークスペース定義 — トークンは Keychain に保存（ここには書かない）
[[workspace]]
name = "my-company"

# 追加のワークスペース
# [[workspace]]
# name = "other-workspace"

# LLM バックエンド（チャット/要約）
[llm]
backend = "local"  # または "vertex_ai"

[local_llm]
endpoint = "http://localhost:1234/v1"
model = "google/gemma-4-26b-a4b"

# エンベディング（LLM から独立 — デフォルトは builtin）
[embedding]
backend = "builtin"

# ポーリング
[polling]
interval_sec = 120        # 2分間隔でチャネルをチェック
priority_boost_sec = 15   # 応答時は15秒間隔に
max_rate_per_min = 45     # Slack の 50/min 制限内に収める

# メモリライフサイクル
[memory]
hot_to_warm_min = 1440    # 24時間後に要約
warm_to_cold_min = 10080  # 7日後にアーカイブ

# 代理応答
[response]
timeout_sec = 120
signature = "— via spa (slack-personal-agent)"
```

## Step 6: トークンを Keychain に保存

アプリの GUI からトークンを追加するか、macOS の `security` コマンドを使用:

```bash
# コマンドラインで保存（GUI の代替）
security add-generic-password \
  -s "slack-personal-agent" \
  -a "workspace:my-company" \
  -w "xoxp-your-token-here"
```

> `-a` フラグのワークスペース名は config.toml の `name` と一致させ、
> `workspace:` プレフィックスを付ける必要があります。

## Step 7: 起動して監視開始

1. `dist/slack-personal-agent.app` を開く
2. **Dashboard** タブでワークスペースに "Token set" と表示されることを確認
3. **Start** をクリックしてポーリングを開始
4. アプリがチャネルを検出し、メッセージ収集を開始

## Step 8: 動作確認

- **Dashboard:** ワークスペースに "Polling" バッジ、メモリ統計が増加
- **Query タブ:** ワークスペース ID、チャネル ID、質問を入力して検索
- **Knowledge タブ:** テスト用エントリを追加して表示を確認

## 複数ワークスペース

複数ワークスペースを監視する場合:

1. 各ワークスペースで Slack App を作成（Step 1-3）
2. config.toml に各ワークスペースを追加:
   ```toml
   [[workspace]]
   name = "company-a"

   [[workspace]]
   name = "company-b"
   ```
3. 各トークンを Keychain に保存:
   ```bash
   security add-generic-password -s "slack-personal-agent" -a "workspace:company-a" -w "xoxp-..."
   security add-generic-password -s "slack-personal-agent" -a "workspace:company-b" -w "xoxp-..."
   ```

## クロスチャネルスコープグループ

特定のチャネル間で知識共有を許可する場合（Level 2/3）:

```toml
# このグループ内のチャネルは互いの知識を参照可能
[[scope_group]]
name = "security-team"

[[scope_group.member]]
workspace = "company-a"
channel = "C01SECURITY"

[[scope_group.member]]
workspace = "company-a"
channel = "C02INCIDENTS"

# ワークスペース横断グループ
[[scope_group]]
name = "infra-cross"

[[scope_group.member]]
workspace = "company-a"
channel = "C03INFRA"

[[scope_group.member]]
workspace = "company-b"
channel = "C04INFRA"
```

## LLM バックエンド選択

### ローカル LLM（デフォルト）

[LM Studio](https://lmstudio.ai/) または互換サーバーが必要:

```toml
[llm]
backend = "local"

[local_llm]
endpoint = "http://localhost:1234/v1"
model = "google/gemma-4-26b-a4b"
```

### Vertex AI（Gemini）

Google Cloud 認証情報が必要（[ADC セットアップ](https://cloud.google.com/docs/authentication/application-default-credentials)）:

```toml
[llm]
backend = "vertex_ai"

[vertex_ai]
project = "your-project-id"
region = "us-central1"
model = "gemini-2.5-flash"
```

## トラブルシューティング

### ワークスペースに "No token" と表示される

- Keychain エントリの存在を確認: `security find-generic-password -s "slack-personal-agent" -a "workspace:my-company"`
- config.toml と Keychain のワークスペース名が完全に一致しているか確認

### レート制限エラー

- config.toml の `max_rate_per_min` を下げる
- 監視チャネル数を減らす
- `interval_sec` を大きくする

### エンベディングモデルのダウンロード

`backend = "builtin"` での初回起動時、all-MiniLM-L6-v2 モデル（約 90MB）を
ダウンロードします。インターネット接続が必要です。以降はキャッシュを使用します。

### ビルドエラー

- Wails v2 のインストール確認: `wails doctor`
- DuckDB は `no_duckdb_arrow` ビルドタグが必要（Makefile で対応済み）
- フロントエンドを先にビルド: `cd frontend && npm install && npm run build`
