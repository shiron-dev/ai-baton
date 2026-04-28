AI Review Memory Harness 設計案

1. 概要

本プロジェクトは、Codex、Claude Code、CrowdCode などのCLIベースAIエージェントを用いて、GitHub Pull Requestに対するAIコードレビューを行うOSSである。

単なるAIレビュー実行ツールではなく、過去のレビューコメント、人間の返信、修正結果、false positiveの履歴を蓄積し、次回以降のレビューに反映する Review Memory Layer を中心機能とする。

AIエージェントは差し替え可能な外部レビュー生成器として扱い、本OSSは以下を担当する。

- PR差分の取得
- コード文脈の構築
- 過去レビューコメントの検索
- 類似コメントの判定
- 過去の人間反応の解釈
- AIレビュー結果の補正
- GitHubへの投稿制御
- レビュー履歴の保存

⸻

2. 基本方針

2.1 AIエージェントは交換可能にする

CodexやCrowdCodeに強く依存しない。
各AIエージェントはCLIとして呼び出し、共通インターフェースで扱う。

Codex CLI
Claude Code CLI
CrowdCode CLI
Gemini CLI
local LLM CLI

これらはすべて Agent Adapter 経由で呼び出す。

AIエージェントには「レビュー候補の生成」を任せる。
一方で、以下の判断はGo側で行う。

- どのコメントを投稿するか
- 過去コメントと重複しているか
- false positiveの可能性が高いか
- プロジェクト固有ルールに反していないか
- GitHubにどう投稿するか

⸻

3. 全体アーキテクチャ

GitHub Pull Request
  ↓
GitHub Actions
  ↓
review-memory-harness
  ├─ GitHub Adapter
  │   ├─ PR差分取得
  │   ├─ 既存コメント取得
  │   ├─ 人間の返信取得
  │   └─ review comment投稿
  │
  ├─ Code Context Builder
  │   ├─ git diff解析
  │   ├─ tree-sitter解析
  │   ├─ LSP連携
  │   ├─ symbol抽出
  │   ├─ 関連テスト探索
  │   └─ 影響範囲推定
  │
  ├─ Agent Harness
  │   ├─ Codex Adapter
  │   ├─ Claude/CrowdCode Adapter
  │   └─ Custom CLI Adapter
  │
  ├─ Review Memory
  │   ├─ 過去コメント保存
  │   ├─ 人間返信保存
  │   ├─ outcome保存
  │   ├─ 全文検索
  │   ├─ ベクトル検索
  │   └─ judge結果保存
  │
  ├─ Retrieval Pipeline
  │   ├─ canonical claim生成
  │   ├─ vector search
  │   ├─ full-text search
  │   ├─ symbol/location再スコア
  │   └─ 類似コメント候補取得
  │
  ├─ Review Judge
  │   ├─ 同種コメント判定
  │   ├─ 今回の差分への適用可否判定
  │   ├─ false positive抑制
  │   └─ コメント補正
  │
  └─ Review Publisher
      ├─ inline comment
      ├─ summary comment
      └─ annotations

⸻

4. Review Memoryの考え方

本プロジェクトでは、過去のAIレビューコメントを単なるログとして保存しない。
それらを「判例」として扱う。

保存するべき情報は、AIが何を言ったかだけではなく、その指摘が人間にどう扱われたかである。

4.1 保存対象

- AIが投稿したコメント
- コメントの正規化された主張
- 対象ファイル
- 対象symbol
- 対象diff
- そのときのコード文脈
- 人間の返信
- 修正されたか
- 却下されたか
- false positiveだったか
- 議論になったか
- 最終的な解釈

4.2 コメントの3層構造

過去コメントは、最低でも以下の3層で扱う。

raw_comment:
  AIが実際に投稿した本文
canonical_claim:
  コメントの本質的な主張を正規化したもの
outcome_summary:
  人間の反応や最終判断を要約したもの

例:

raw_comment:
  "This may panic when user is nil."
canonical_claim:
  "user が nil の場合に panic する可能性がある"
outcome_summary:
  "rejected: この関数は認証middleware後にしか呼ばれないため、userはnilにならない"

検索やRAGで主に使うのは canonical_claim と outcome_summary である。
raw_comment だけを使うと、AIの言い回しに引っ張られるため避ける。

⸻

5. 検索方式

5.1 ラベル主導にはしない

レビューコメントに対してラベルを付けることは有用だが、主役にはしない。

ラベルは粗すぎるため、同じ performance でも以下のように内容がまったく異なる。

- N+1 query
- goroutine leak
- 不要なJSON marshal
- DB index不足
- 巨大sliceコピー

そのため、ラベルは補助情報として扱う。

- 集計
- severity調整
- UI表示
- policy適用
- 検索時の軽い補助

5.2 ハイブリッド検索を採用する

過去コメント検索は、以下の組み合わせで行う。

- ベクトル検索
- 全文検索
- symbol一致
- file path一致
- diff context一致
- 軽いラベル一致

ベクトル検索は言い換えに強い。
全文検索は関数名、型名、API名、エラー文字列、プロジェクト固有用語に強い。

どちらか片方ではなく、両方を使う。

5.3 スコアリング例

score =
  0.45 * vector_similarity
+ 0.25 * full_text_score
+ 0.15 * code_location_score
+ 0.10 * symbol_score
+ 0.05 * label_score

このスコアは固定ではなく、将来的には設定可能にする。

⸻

6. Review Judge

検索結果だけで「同じコメント」とは判断しない。
検索はあくまで候補抽出であり、その後に Judge を行う。

6.1 Judgeの役割

Judgeは以下を判定する。

- 過去コメントと今回コメントは同じ論点か
- 過去コメントの前提は今回にも当てはまるか
- 過去の人間返信は今回にも適用できるか
- 投稿すべきか
- 抑制すべきか
- コメントを弱めるべきか
- 過去の修正パターンを追記すべきか

6.2 Judge出力例

{
  "same_underlying_issue": true,
  "applies_to_current_diff": false,
  "prior_outcome": "false_positive",
  "recommended_action": "do_not_suppress",
  "reason": "過去の指摘はREST handlerのvalidation不足に関するものだが、今回の変更はinternal batch処理であり入力経路が異なる",
  "confidence": 0.72
}

6.3 Judge結果も保存する

Judgeの結果自体もReview Memoryに保存する。

これにより、

- AとBは似ているが適用条件が違う
- この種類の指摘はこのプロジェクトではfalse positiveになりやすい
- この指摘は特定レイヤーでは有効だが、別レイヤーでは無効

といった知識が蓄積される。

⸻

7. レビュー実行フロー

1. GitHub ActionsでPRイベントを受ける
2. review-memory-harnessを起動する
3. GitHub APIからPR差分を取得する
4. 変更ファイルを解析する
5. LSP / tree-sitter / ripgrepでコード文脈を構築する
6. AIエージェントにレビュー候補を生成させる
7. AI出力をFindingとして正規化する
8. 各Findingからcanonical_claimを生成する
9. Review Memoryから類似コメントを検索する
10. 上位候補に対してJudgeを行う
11. 投稿・抑制・補正を決定する
12. GitHubにreview commentを投稿する
13. 投稿内容と判断結果をMemoryに保存する
14. 後続実行で人間の返信や修正結果を同期する

⸻

8. データモデル

8.1 Finding

type Finding struct {
    ID             string
    FilePath       string
    StartLine      int
    EndLine        int
    Title          string
    Body           string
    SuggestedPatch string
    Severity       string
    Confidence     float64
    Labels         []string
    Evidence       []Evidence
}

8.2 ReviewMemory

type ReviewMemory struct {
    ID                 string
    Repo               string
    PRNumber           int
    Agent              string
    FilePath           string
    Symbol             string
    RawComment         string
    CanonicalClaim     string
    CodeContextSummary string
    Outcome            string
    OutcomeSummary     string
    HumanReplies       []HumanReply
    EmbeddingID        string
    CreatedAt          time.Time
}

8.3 Outcome

type Outcome string
const (
    OutcomeAccepted      Outcome = "accepted"
    OutcomeRejected      Outcome = "rejected"
    OutcomeIgnored       Outcome = "ignored"
    OutcomeDiscussed     Outcome = "discussed"
    OutcomeFalsePositive Outcome = "false_positive"
    OutcomeDuplicate     Outcome = "duplicate"
)

8.4 MemoryHit

type MemoryHit struct {
    MemoryID       string
    CommentText    string
    CanonicalClaim string
    Outcome        string
    OutcomeSummary string
    Similarity     float64
    Reason         string
}

8.5 JudgeResult

type JudgeResult struct {
    FindingID              string
    MemoryID               string
    SameUnderlyingIssue    bool
    AppliesToCurrentDiff   bool
    PriorOutcome           string
    RecommendedAction      string
    Reason                 string
    Confidence             float64
}

⸻

9. DB設計

MVPではSQLiteを標準にする。

SQLite
+ FTS5
+ sqlite-vec / sqlite-vss

将来的にはPostgreSQL + pgvectorにも対応できるようにする。

9.1 テーブル例

review_comments
  id
  repo
  pr_number
  agent
  file_path
  symbol
  raw_comment
  canonical_claim
  code_context_summary
  outcome
  outcome_summary
  created_at
human_replies
  id
  review_comment_id
  author
  body
  created_at
comment_embeddings
  review_comment_id
  embedding
judge_results
  id
  finding_id
  memory_id
  same_underlying_issue
  applies_to_current_diff
  recommended_action
  reason
  confidence
  created_at

⸻

10. Goパッケージ構成

cmd/review-harness/
  main.go
internal/config/
  config.go
internal/git/
  diff.go
  checkout.go
internal/github/
  client.go
  pr.go
  comments.go
  review.go
internal/agent/
  agent.go
  codex.go
  claude.go
  shell.go
  parser.go
internal/context/
  builder.go
  changed_files.go
  related_symbols.go
  related_tests.go
internal/lsp/
  client.go
  gopls.go
  tsserver.go
internal/index/
  indexer.go
  symbols.go
  chunks.go
  embeddings.go
internal/memory/
  store.go
  sqlite.go
  vector.go
  fts.go
  retrieval.go
internal/judge/
  judge.go
  prompt.go
  result.go
internal/interpreter/
  normalize.go
  canonicalize.go
  dedupe.go
  enrich.go
internal/review/
  pipeline.go
  policy.go
  publisher.go
internal/prompt/
  templates.go
internal/schema/
  finding.go
  memory.go

⸻

11. Agent Adapter設計

type Agent interface {
    Name() string
    RunReview(ctx context.Context, req ReviewRequest) (*AgentReviewResult, error)
}
type ReviewRequest struct {
    RepoPath      string
    Diff          string
    CodeContext   CodeContext
    MemoryContext []MemoryHit
    Prompt        string
}
type AgentReviewResult struct {
    RawOutput string
    Findings  []Finding
    Metadata  map[string]string
}

各AgentはCLI呼び出しとして実装する。

type ShellAgent struct {
    NameValue string
    Command   string
    Args      []string
    Timeout   time.Duration
}

⸻

12. 設定ファイル

version: 1
agent:
  default: codex
  agents:
    codex:
      command: codex
      args: ["exec"]
      timeout: 600s
    claude:
      command: claude
      args: ["-p"]
      timeout: 600s
review:
  max_comments: 20
  min_confidence: 0.65
  post_inline: true
  post_summary: true
context:
  max_tokens: 50000
  include_related_tests: true
  include_callers: true
  include_docs: true
memory:
  backend: sqlite
  path: .review-harness/memory.sqlite
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
labels:
  lightweight: true
  enabled:
    - bug
    - security
    - performance
    - maintainability
    - test-missing
    - project-convention

⸻

13. GitHub Actions利用例

name: AI Review
on:
  pull_request:
    types: [opened, synchronize, reopened]
permissions:
  contents: read
  pull-requests: write
  issues: write
jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
      - name: Install review harness
        run: go install github.com/example/review-harness/cmd/review-harness@latest
      - name: Run AI Review
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: |
          review-harness review \
            --provider github \
            --pr ${{ github.event.pull_request.number }} \
            --agent codex \
            --config .review-harness.yaml

⸻

14. MVPスコープ

最初からLSPや高度なindexを作り込みすぎない。

MVPでは以下に絞る。

- GitHub PR diff取得
- Codex/Claude/CrowdCode CLI呼び出し
- JSON Finding出力
- SQLite保存
- FTS5による全文検索
- embeddingによる類似検索
- 類似コメント候補の取得
- 簡易Judge
- false positive履歴による抑制
- inline review comment投稿

LSP連携はv1以降でよい。

⸻

15. 段階的ロードマップ

v0: 最小レビュー実行

- GitHub Actionsで動作
- diff取得
- AI CLI呼び出し
- JSONパース
- review comment投稿

v0.1: Memory保存

- 投稿コメント保存
- PR番号保存
- file path / line保存
- 人間返信の同期

v0.2: Hybrid Retrieval

- SQLite FTS5
- embedding検索
- canonical_claim生成
- 類似コメント候補表示

v0.3: Judge導入

- 同種コメント判定
- false positive抑制
- acceptedコメントの修正パターン活用

v1: Code Intelligence強化

- tree-sitter
- LSP
- symbol index
- related tests
- call graph

v2: SaaS風運用対応

- PostgreSQL
- pgvector
- Web UI
- org横断memory
- repoごとのpolicy
- dashboard

⸻

16. この設計の中心思想

このOSSの本質は、AIエージェントそのものを作ることではない。

Codex / Claude / CrowdCode:
  レビュー候補を生成するエンジン
review-memory-harness:
  プロジェクト固有のレビュー文脈を蓄積し、
  過去の人間反応を解釈し、
  AIレビューを投稿可能な品質に補正する制御層

つまり主役は、AIエージェントではなく レビュー記憶と解釈のレイヤー である。

これにより、エージェントを将来変更しても、過去レビューから得た知識は失われない。

Codex → Claude Code → CrowdCode → Gemini CLI → local LLM

と変わっても、Review Memoryは継続して利用できる。
