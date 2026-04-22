# アーキテクチャ: slack-personal-agent (spa)

## 概要

slack-personal-agent は、個人ユーザーの視点から複数の Slack ワークスペースを監視し、
時間認識付きメモリでチャネルメッセージを蓄積、3層知識分離モデルによる
チャネルスコープ RAG クエリを提供する macOS デスクトップアプリケーションである。

```
┌─ React フロントエンド ────────────────────────────┐
│ ダッシュボード │ クエリUI │ MITL承認 │ 知識ベース   │
└────────────────────┬──────────────────────────────┘
                     │ Wails v2 バインディング
┌────────────────────┴──────────────────────────────┐
│ app.go (オーケストレータ)                           │
│  ├── config       ← TOML + 環境変数オーバーライド  │
│  ├── keychain     ← macOS Keychain (go-keyring)   │
│  ├── slack        ← API クライアント + 優先度キュー │
│  ├── memory       ← DuckDB + Hot/Warm/Cold        │
│  ├── llm          ← チャット (local/Vertex AI)    │
│  ├── embedding    ← 独立ベクトル化                  │
│  ├── rag          ← 3層スコープ付き検索             │
│  ├── mitl         ← 代理応答承認ゲート              │
│  └── knowledge    ← 内部知識ベース                  │
└───────────────────────────────────────────────────┘
         │                    │
    Slack API            DuckDB (ローカル)
  (User Token)           spa.db
```

## パッケージアーキテクチャ

### 依存関係グラフ

```
app.go
 ├── config          (内部依存なし)
 ├── keychain        (内部依存なし)
 ├── slack           (内部依存なし)
 ├── memory          (内部依存なし)
 ├── llm             (config)
 ├── embedding       (config)
 ├── rag             (embedding)
 ├── mitl            (内部依存なし)
 └── knowledge       (内部依存なし)
```

各パッケージは独立してテスト可能に設計されている。パッケージ間依存を持つのは
`rag`（`embedding.Embedder` インターフェースに依存）のみ。

### パッケージ責務

| パッケージ | 責務 | 主要な型 |
|-----------|------|---------|
| `config` | TOML 設定読み込み、環境変数、バリデーション | `Config`, `WorkspaceConfig`, `EmbeddingConfig` |
| `keychain` | OS キーチェーンによるセキュアなクレデンシャル保存 | `Store` interface, `OSStore`, `MockStore` |
| `slack` | Slack Web API、レート制限付きキュー、ポーリング | `Client`, `Queue`, `Scheduler`, `WorkspacePoller` |
| `memory` | メッセージ保存とライフサイクル管理 | `Store`, `Record`, `Tier`, `Lifecycle` |
| `llm` | チャット/要約用 LLM インターフェース | `Backend` interface, `LocalBackend`, `VertexAIBackend` |
| `embedding` | テキストベクトル化（LLM から独立） | `Embedder` interface, `LocalEmbedder`, `VertexAIEmbedder` |
| `rag` | チャネルスコープ付きベクトル類似度検索 | `Retriever`, `SearchScope`, `SearchResult` |
| `mitl` | 代理応答の承認ワークフロー | `Manager`, `Proposal`, `State` |
| `knowledge` | 内部知識ベース（Slack 外） | `Store`, `Entry`, `Scope` |

## データフロー

### メッセージ収集

```
Slack API ──poll──→ WorkspacePoller ──→ handleMessages()
                                          ├── memory.Store.InsertRecord() → DuckDB records テーブル
                                          └── rag.Retriever.Index()       → DuckDB embeddings テーブル
```

### クエリ

```
ユーザー ──質問──→ App.Query()
                    ├── embedding.Embedder.Embed(question) → クエリベクトル
                    └── rag.Retriever.Search(vector, scope) → DuckDB list_cosine_similarity
                           └── スコープフィルタ: workspace_id + channel_id
```

### MITL 代理応答

```
システムが応答機会を検知
  → mitl.Manager.CreateProposal() → StatePending
    → OnProposal コールバック（macOS 通知）
      → ユーザーが GUI で確認
        ├── 承認 → PostProxyMessage(text + シグニチャ) → Slack
        │            └── BoostChannel(ポーリング優先度)
        ├── 編集+承認 → 修正テキスト → PostProxyMessage
        ├── 拒否 → StateRejected（破棄）
        └── タイムアウト → StateExpired（破棄）
```

### メモリライフサイクル

```
                    24時間                    7日
  Hot ─────────────────→ Warm ─────────────────→ Cold
  (生メッセージ)          (LLM 要約)              (アーカイブ)

  コンパクション: Hot レコードを (workspace, channel) でグループ化
                → LLM が要約 → Warm レコード作成
                → Hot レコード削除
```

## 主要な設計判断

### なぜ User Token であり Bot Token ではないのか

**判断:** すべての API 操作に Slack User Token（`xoxp-`）を使用する。

**検討した代替案:**
- Bot Token（`xoxb-`）: 共有ワークスペースエージェントになり、パーソナルではなくなる。
  チャネル可視性をユーザーの Slack 権限から自動継承できず、手動管理が必要。
- Events API: HTTP エンドポイントが必要で、デスクトップアプリには不適。
- Socket Mode: User Token では利用不可。

**帰結:** ポーリングが唯一の現実的アプローチ。リアルタイム性は制限されるが、
デプロイが単純（受信接続不要）。

### なぜ優先度付きポーリングか

**判断:** 一定間隔ポーリング + 動的優先度ブースト。

**固定ポーリングのみではない理由:** MITL で応答機会を検知した際、該当チャネルの
リフレッシュを高速化しないと会話の文脈を追えない。フラットな間隔ではフォロー
アップの文脈を見逃す。

**レート制限管理:** Slack Tier 3 はワークスペースごとに約 50 req/min。
ワークスペース単位のキューとスライディングウィンドウ追跡で、手動チューニング
なしにコンプライアンスを確保。

### なぜエンベディングは LLM から独立しているのか

**判断:** `embedding` パッケージは `llm` パッケージから完全に分離。

**問題:** エンベディングが LLM バックエンド（local / Vertex AI）で生成される場合、
バックエンド切替時にすべてのベクトルが無効化され、全再 index が必要になる
（数時間のコンピューティング）。

**解決策:** エンベディングは独自のバックエンド選択（`builtin`, `local`, `vertex_ai`）を
config に持つ。`ModelID()` メソッドが各インデックスの生成モデルを追跡。
起動時に `CheckModelConsistency()` が不一致を検知しユーザーに警告。

**デフォルト:** 内蔵モデル（予定: Hugot による all-MiniLM-L6-v2）で
ゼロ設定・オフライン動作。

### なぜ 3 層知識分離か

**判断:** チャネルを基本単位とし、ユーザーの明示的許可でのみスコープを拡大。

```
Level 1: チャネル内（デフォルト、常に有効）
Level 2: ワークスペース内クロスチャネル（ユーザー許可グループ）
Level 3: クロスワークスペース（ユーザー明示許可）
```

**問題（sai からの教訓）:** 前身プロジェクト sai は、チャネルフィルタなしの
グローバル RAG 検索を持っていた。`#private-engineering` での質問に
`#general` のメッセージが混入し、情報漏えいを引き起こす構造だった。

**強制方法:** `rag.SearchScope` 構造体がワークスペース/チャネル ID を必須とする。
「すべてを検索」する API は存在しない。スコープフィルタは SQL レベル
（`WHERE workspace_id = ? AND channel_id = ?`）で適用され、検索後のフィルタリングではない。

### なぜ Keychain ファーストか

**判断:** すべてのトークンを macOS Keychain に保存。設定ファイルに秘密値を含めない。

**問題:** nlink-jp の多くのツールはトークンを平文の設定ファイル（JSON/TOML）に
保存している。User Token は Bot Token より広い権限を持つため、
このセキュリティギャップは特に深刻。

**実装:** `go-keyring` ライブラリ（scli で実績あり）でクロスプラットフォーム
キーチェーンアクセス。環境変数フォールバックはテスト用のみ。
GUI がトークンセットアップフローを提供: 入力 → Keychain → config には名前のみ記録。

### なぜ代理応答にシグニチャが必要か

**判断:** すべての MITL 代理応答に設定可能なシグニチャ行を含める。

**問題:** User Token による投稿はユーザー本人として表示される。
識別子なしでは、人間が書いたメッセージとシステム生成のメッセージを区別できない。
これはアカウンタビリティと信頼の問題を生じる。

**デフォルト:** `— via spa (slack-personal-agent)` がすべての代理投稿に付与される。

## ストレージスキーマ

すべてのデータは単一の DuckDB ファイル（`spa.db`）に保存される。

### テーブル

| テーブル | 目的 |
|---------|------|
| `records` | Slack メッセージ（ライフサイクル tier: hot/warm/cold） |
| `channels` | チャネルメタデータとポーリング状態 |
| `embeddings` | ベクトルエンベディング（workspace/channel スコープ付き） |
| `embedding_meta` | エンベディングモデル追跡（ModelID） |
| `knowledge` | 内部知識ベースエントリ |

### インデックス戦略

DuckDB の ART インデックスは PRIMARY KEY に対する UPDATE 操作に既知の制限がある。
レコードは UUID 識別子を使用し、主キー制約は設けない。
インデックスはクエリ頻度の高いカラムに作成: `(workspace_id, channel_id)`, `tier`,
`created_at`, `(scope, workspace_id)`。

ベクトル検索は VSS 拡張ではなく `list_cosine_similarity()`（DuckDB ネイティブ関数）
を使用。拡張の依存問題を回避しつつ、想定データ量（ワークスペースあたり数万レコード）
で十分な性能を提供。性能が問題になった場合、VSS HNSW インデックスへの移行は容易。

## セキュリティモデル

1. **クレデンシャル:** macOS Keychain のみ。設定に平文トークンを含めない。
2. **チャネル分離:** アプリケーションロジックではなく SQL クエリレベルで強制。
3. **MITL ゲート:** すべての Slack 投稿にユーザーの明示的承認が必要。
4. **送信者識別:** システム投稿はシグニチャで帰属を明示。
5. **エンベディングモデル追跡:** モデル変更時の暗黙的な検索精度低下を防止。

## 技術選択

| コンポーネント | 選択 | 根拠 |
|--------------|------|------|
| 言語 | Go | nlink-jp エコシステムとの一貫性; shell-agent/data-agent の実績 |
| GUI | Wails v2 + React | shell-agent、data-agent で実証済み |
| データベース | DuckDB | ベクトル検索 + 分析; lite-rag/gem-rag/shell-agent で実績あり |
| キーチェーン | go-keyring | クロスプラットフォーム; scli で実績あり |
| LLM (チャット) | OpenAI 互換 / Vertex AI | data-agent の Backend インターフェースパターン |
| LLM (エンベディング) | 独立 Embedder | LLM バックエンド切替時の再 index を防止 |
