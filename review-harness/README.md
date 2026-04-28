# review-memory-harness

GitHub Pull Request に対して AI コードレビューを実行し、過去のレビュー結果（承認・却下・false positive）を SQLite に蓄積して次回以降の判定に活用するツールです。

- **エージェントは差し替え可能** — Claude Code CLI（デフォルト）、Codex CLI、または任意の CLI ツールを使用できます
- **ストレージは選択可能** — Google Cloud Storage（デフォルト）、S3、またはローカルファイルシステム
- **Reusable GitHub Action** として外部リポジトリから呼び出せます

---

## クイックスタート

### パターン 1: Cloud Storage + Claude Code API key

```yaml
# .github/workflows/ai-review.yml
name: AI Review
on:
  pull_request:
    types: [opened, synchronize, reopened]

permissions:
  contents: read
  pull-requests: write
  id-token: write   # Workload Identity Federation で GCP に認証する場合

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Authenticate to Google Cloud
        uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: ${{ vars.GCP_WORKLOAD_IDENTITY_PROVIDER }}
          service_account: ${{ vars.GCP_SERVICE_ACCOUNT_EMAIL }}

      - uses: shiron-dev/ai-baton/review-harness@main
        with:
          pr-number: ${{ github.event.pull_request.number }}
          agent: claude
          storage-backend: cloudstorage
          cloudstorage-bucket: ${{ vars.REVIEW_HARNESS_CLOUDSTORAGE_BUCKET }}
          anthropic-api-key: ${{ secrets.ANTHROPIC_API_KEY }}
```

**必要な Variables**

| Variable | 用途 |
| --- | --- |
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | GitHub Actions OIDC 用の Workload Identity Provider |
| `GCP_SERVICE_ACCOUNT_EMAIL` | Cloud Storage へ読み書きするサービスアカウント |
| `REVIEW_HARNESS_CLOUDSTORAGE_BUCKET` | メモリ DB を保存する Cloud Storage バケット名 |

**必要な Secrets**

| Secret | 用途 |
| --- | --- |
| `ANTHROPIC_API_KEY` | Claude Code / Judge / canonical claim 生成に使用 |

### パターン 2: S3 + codex exec

```yaml
# .github/workflows/ai-review.yml
name: AI Review
on:
  pull_request:
    types: [opened, synchronize, reopened]

permissions:
  contents: read
  pull-requests: write
  id-token: write   # OIDC で AWS に認証する場合

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_ROLE_ARN }}
          aws-region: us-east-1

      - name: Install Codex CLI
        run: npm install -g @openai/codex

      - uses: shiron-dev/ai-baton/review-harness@main
        with:
          pr-number: ${{ github.event.pull_request.number }}
          agent: codex
          agent-model: gpt-5.1-codex-mini
          storage-backend: s3
          s3-bucket: ${{ secrets.REVIEW_HARNESS_S3_BUCKET }}
          openai-api-key: ${{ secrets.OPENAI_API_KEY }}
```

**必要なシークレット**

| シークレット | 用途 |
| --- | --- |
| `AWS_ROLE_ARN` | S3 への読み書き権限を持つ IAM ロール（OIDC） |
| `REVIEW_HARNESS_S3_BUCKET` | メモリ DB を保存する S3 バケット名 |
| `OPENAI_API_KEY` | codex exec と embedding に使用 |

---

## Action インプット一覧

| インプット | デフォルト | 説明 |
| --- | --- | --- |
| `command` | `review` | 実行するコマンド。`review` または `sync` |
| `pr-number` | — | レビューする PR 番号（**必須**） |
| `github-token` | `github.token` | pull-requests: write 権限を持つ GitHub トークン |
| `agent` | `claude` | 使用するエージェント名（後述） |
| `agent-model` | — | 対応するエージェントに渡すモデル名（例: `gpt-5-mini`） |
| `storage-backend` | `cloudstorage` | `cloudstorage` / `s3` / `local` |
| `cloudstorage-bucket` | — | Cloud Storage バケット名（`storage-backend=cloudstorage` の場合に必須） |
| `cloudstorage-object` | `review-harness/memory.sqlite` | Cloud Storage オブジェクト名 |
| `s3-bucket` | — | S3 バケット名（`storage-backend=s3` の場合に必須） |
| `s3-key` | `review-harness/memory.sqlite` | S3 オブジェクトキー |
| `aws-region` | `us-east-1` | AWS リージョン |
| `anthropic-api-key` | — | Claude Code・Judge・canonical claim 生成に使用（省略時は Claude/Judge/canonicalization を使用不可） |
| `openai-api-key` | — | `openai` agent とベクトル検索の embedding に使用（`agent=openai` では必須） |
| `config-file` | `.review-harness.yaml` | 設定ファイルのパス（ワークスペース相対） |
| `dry-run` | `false` | `true` にすると GitHub への投稿をスキップ |
| `go-version` | `1.24` | ビルドに使用する Go バージョン |

---

## エージェントの設定

エージェントは `.review-harness.yaml` で自由に定義できます。`agent` インプットで使用するエージェントを選択します。

### 組み込みエージェント

| 名前 | コマンド | 動作 |
| --- | --- | --- |
| `claude`（デフォルト） | `claude -p` | `ANTHROPIC_API_KEY` を使い、stdin でプロンプトを渡して JSON を取得 |
| `openai` | OpenAI API | `OPENAI_API_KEY` を使い、OpenAI API から JSON を取得 |
| `codex` | `codex exec` | stdin でプロンプトを渡し `--output-last-message` で JSON を取得 |

### カスタムエージェントの追加

`.review-harness.yaml` に `agent.agents` を追加します。エージェントは **stdout に JSON 配列を出力** する CLI であれば何でも使えます。

```yaml
# .review-harness.yaml
agent:
  default: my-agent
  agents:
    my-agent:
      command: my-review-cli
      args: ["--output-format", "json", "-"]   # "-" で stdin 読み取り
      timeout: 300s
    gemini:
      command: gemini
      args: ["--json", "-"]
      timeout: 600s
```

エージェントが出力すべき JSON フォーマット：

```json
[
  {
    "file_path": "main.go",
    "start_line": 42,
    "end_line": 45,
    "title": "nil dereference",
    "body": "user が nil の場合に panic します。",
    "severity": "error",
    "confidence": 0.9,
    "labels": ["bug"],
    "suggested_patch": "if user == nil {\n    return nil\n}"
  }
]
```

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `file_path` | string | 対象ファイル |
| `start_line` / `end_line` | int | 問題の行範囲（新ファイル基準） |
| `title` | string | 短いタイトル（80 文字以内） |
| `body` | string | 詳細な説明 |
| `severity` | string | `"error"` / `"warning"` / `"info"` |
| `confidence` | float | 確信度 0.0–1.0 |
| `labels` | []string | 任意のラベル |
| `suggested_patch` | string | コード修正案（省略可） |

---

## ストレージの設定

### Google Cloud Storage（デフォルト）

レビュー開始前に Cloud Storage から SQLite DB をダウンロードし、終了後にアップロードします。Google 認証は Application Default Credentials（Workload Identity Federation / `GOOGLE_APPLICATION_CREDENTIALS` / gcloud など）に対応します。

```yaml
storage:
  backend: cloudstorage
  cloudstorage_bucket: my-bucket          # action インプットでも上書き可
  cloudstorage_object: review-harness/memory.sqlite
```

Action インプットが `.review-harness.yaml` の設定より優先されます。

Terraform は [`../infra/cloudstorage`](../infra/cloudstorage) にあります。

### S3

レビュー開始前に S3 から SQLite DB をダウンロードし、終了後にアップロードします。AWS 認証は標準のクレデンシャルチェーン（OIDC / 環境変数 / インスタンスプロファイル）に対応します。

```yaml
storage:
  backend: s3
  s3_bucket: my-bucket          # action インプットでも上書き可
  s3_key: review-harness/memory.sqlite
  s3_region: ap-northeast-1
```

Action インプットが `.review-harness.yaml` の設定より優先されます。

### ローカル（エフェメラル）

```yaml
# .github/workflows/ai-review.yml
- uses: shiron-dev/ai-baton/review-harness@main
  with:
    pr-number: ${{ github.event.pull_request.number }}
    storage-backend: local
```

ランナーが終了するとデータは失われます。メモリ機能を使わずシンプルにレビューだけ行いたい場合に使用します。

---

## 設定ファイルリファレンス

```yaml
# .review-harness.yaml
version: 1

agent:
  default: claude
  agents:
    codex:
      command: codex
      args: []
      model: gpt-5.1-codex-mini
      timeout: 600s
    openai:
      command: openai
      args: []
      model: gpt-5-mini
      timeout: 600s
    claude:
      command: claude
      args: ["-p", "--dangerously-skip-permissions", "-"]
      timeout: 600s

review:
  max_comments: 20        # 1 回のレビューで投稿する最大コメント数
  min_confidence: 0.65    # この値未満の finding は無視
  post_inline: true
  post_summary: true

storage:
  backend: cloudstorage
  cloudstorage_bucket: "" # 必須（action インプットでも渡せる）
  cloudstorage_object: review-harness/memory.sqlite
  s3_bucket: ""
  s3_key: review-harness/memory.sqlite
  s3_region: us-east-1

memory:
  path: .review-harness/memory.sqlite   # ランナー上の一時パス
  retrieval:
    vector_weight: 0.45
    full_text_weight: 0.25
    code_location_weight: 0.15
    symbol_weight: 0.10
    label_weight: 0.05
  embedding:
    provider: openai
    model: text-embedding-3-small

judge:
  enabled: true
  max_candidates: 10
  suppress_false_positive: true
  provider: anthropic
  model: claude-haiku-4-5-20251001

labels:
  lightweight: true
  enabled:
    - bug
    - security
    - performance
    - maintainability
    - test-missing
    - project-convention
```

---

## human reply の同期

PR に人間がコメントを返した場合、`sync` で過去の判定結果を更新できます。

```yaml
  sync:
    runs-on: ubuntu-latest
    if: github.event.action == 'synchronize'
    steps:
      - uses: actions/checkout@v4
      - uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: ${{ vars.GCP_WORKLOAD_IDENTITY_PROVIDER }}
          service_account: ${{ vars.GCP_SERVICE_ACCOUNT_EMAIL }}
      - name: Sync human replies
        run: |
          cd review-harness
          go run ./cmd/review-harness sync \
            --repo "${{ github.repository }}" \
            --pr "${{ github.event.pull_request.number }}" \
            --storage cloudstorage \
            --cloudstorage-bucket "${{ vars.REVIEW_HARNESS_CLOUDSTORAGE_BUCKET }}"
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## ローカル実行

```bash
cd review-harness
go build -o review-harness-bin ./cmd/review-harness

# Cloud Storage + Claude Code API key
GITHUB_TOKEN=ghp_xxx \
ANTHROPIC_API_KEY=sk-ant-xxx \
OPENAI_API_KEY=sk-xxx \
  ./review-harness-bin review \
    --repo owner/repo \
    --pr 42 \
    --agent claude \
    --storage cloudstorage \
    --cloudstorage-bucket my-bucket

# S3 + codex exec
GITHUB_TOKEN=ghp_xxx \
OPENAI_API_KEY=sk-xxx \
  ./review-harness-bin review \
    --repo owner/repo \
    --pr 42 \
    --agent codex \
    --agent-model gpt-5.1-codex-mini \
    --storage s3 \
    --s3-bucket my-bucket
```

---

## アーキテクチャ

```text
GitHub Pull Request
  ↓
review-harness
  ├─ GitHub Adapter     … PR diff 取得 / review comment 投稿
  ├─ Agent Harness      … 任意の CLI エージェントを呼び出して Finding を生成
  ├─ Interpreter        … Finding の正規化・canonical claim 生成・重複排除
  ├─ Review Memory      … SQLite + FTS5 + コサイン類似度によるハイブリッド検索
  ├─ Review Judge       … 過去の false positive・却下実績に基づくコメント抑制
  ├─ Policy Filter      … max_comments / min_confidence によるフィルタリング
  ├─ Review Publisher   … inline comment + summary を GitHub に投稿
  └─ Storage Backend    … Cloud Storage / S3 / local への SQLite DB の fetch / flush
```

レビューメモリの 3 層構造：

| 層 | 内容 |
| --- | --- |
| `raw_comment` | エージェントが生成した元のコメント |
| `canonical_claim` | コメントの本質的な主張を正規化したもの |
| `outcome_summary` | 人間の反応・最終判断の要約 |

ハイブリッド検索スコアリング（デフォルト）：

```text
score = 0.45 × ベクトル類似度
      + 0.25 × 全文検索スコア (FTS5)
      + 0.15 × ファイルパス一致
      + 0.10 × シンボル一致
      + 0.05 × ラベル一致
```

---

## RAG とコードインデックスの仕組み

### RAG（Retrieval-Augmented Generation）

review-harness の RAG は以下の流れで動作します。

#### 1. Finding の canonical claim 化

エージェントが生成した `Finding.Body`（生コメント）を Claude API に渡し、言語・表現に依存しない **canonical claim**（正規化された主張）を生成します。

```
"This may panic when user is nil."
  ↓ Claude API (interpreter/canonicalize.go)
"user が nil の場合に panic する可能性がある"
```

同じ問題を別の言い回しで指摘しても、canonical claim レベルで一致を検出できます。

#### 2. Embedding の生成と保存

canonical claim を OpenAI `text-embedding-3-small` でベクトル化し、SQLite の `comment_embeddings` テーブルに JSON BLOB として保存します（`internal/memory/sqlite.go`）。

#### 3. ハイブリッド検索（`internal/memory/retrieval.go`）

新しい Finding が来るたびに過去メモリをハイブリッド検索します。

```
FTS5 全文検索（BM25）
  … 関数名・型名・エラー文字列など固有名詞に強い

コサイン類似度（Go で計算）
  … 言い換えや翻訳に強い

ファイルパス / シンボル一致ボーナス
  … 同じ場所の過去指摘を優先
```

スコアを合算して上位 N 件（`judge.max_candidates`）を Review Judge に渡します。

#### 4. Review Judge による判定

Judge は「過去コメントが今回の diff にも適用されるか」を Claude API で判定します（`internal/judge/judge.go`）。

```json
{
  "same_underlying_issue": true,
  "applies_to_current_diff": false,
  "prior_outcome": "false_positive",
  "recommended_action": "suppress",
  "reason": "過去の指摘は REST handler の validation 不足だが、今回は internal batch 処理"
}
```

`recommended_action: suppress` の場合はそのコメントを投稿しません。

---

### コードインデックス（現状と予定）

#### 現状（MVP）

現在のコードインデックスは **diff テキストのみ** です。エージェントには `git diff` の生テキストを渡しており、シンボル解析・呼び出しグラフ・関連テスト探索は行っていません。

```
PR diff (raw text)
  → エージェントへのプロンプトに埋め込む (internal/agent/shell.go)
```

#### v1 予定

`plan.md` §15 に記載されている v1 スコープで以下を追加予定：

| 機能 | 実装予定箇所 | 効果 |
| --- | --- | --- |
| **tree-sitter 解析** | `internal/index/symbols.go` | 変更された関数・型・変数を特定 |
| **LSP 連携**（gopls など） | `internal/lsp/` | 定義元・参照元のシンボルを取得 |
| **関連テスト探索** | `internal/context/related_tests.go` | 対象関数のテストファイルをコンテキストに追加 |
| **呼び出しグラフ** | `internal/index/chunks.go` | 変更の影響範囲を推定 |

これらが揃うと、エージェントへのプロンプトに「この関数を呼んでいる箇所」「対応するテスト」「型定義」を含められるようになり、より精度の高い Finding が生成されます。

---

## ライセンス

MIT
