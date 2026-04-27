package review

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/shiron-dev/ai-baton/internal/agent"
	"github.com/shiron-dev/ai-baton/internal/config"
	"github.com/shiron-dev/ai-baton/internal/embedder"
	"github.com/shiron-dev/ai-baton/internal/githubadapter"
	"github.com/shiron-dev/ai-baton/internal/interpreter"
	"github.com/shiron-dev/ai-baton/internal/judge"
	"github.com/shiron-dev/ai-baton/internal/memory"
	"github.com/shiron-dev/ai-baton/internal/schema"
	"github.com/shiron-dev/ai-baton/internal/storage"
)

// Pipeline orchestrates the full review flow (plan.md §7).
type Pipeline struct {
	cfg           *config.Config
	gh            *githubadapter.Client
	reviewAgent   agent.Agent
	store         memory.Store
	retriever     *memory.Retriever
	canonicalizer *interpreter.Canonicalizer
	embedder      embedder.Embedder
	judger        judge.Judge
	filter        *Filter
	publisher     *Publisher
	backend       storage.Backend
	repo          string
	dryRun        bool
}

// PipelineOptions configures the Pipeline.
type PipelineOptions struct {
	Cfg          *config.Config
	Repo         string
	GithubToken  string
	OpenAIKey    string
	AnthropicKey string
	DryRun       bool
	// Storage overrides: if non-empty these override cfg.Storage.*
	StorageBackend string
	S3Bucket       string
	S3Key          string
	S3Region       string
}

// NewPipeline constructs a Pipeline wiring all components together.
func NewPipeline(ctx context.Context, opts PipelineOptions) (*Pipeline, error) {
	cfg := opts.Cfg

	// Resolve storage config (flags override YAML).
	storageCfg := cfg.Storage
	if opts.StorageBackend != "" {
		storageCfg.Backend = opts.StorageBackend
	}
	if opts.S3Bucket != "" {
		storageCfg.S3Bucket = opts.S3Bucket
	}
	if opts.S3Key != "" {
		storageCfg.S3Key = opts.S3Key
	}
	if opts.S3Region != "" {
		storageCfg.S3Region = opts.S3Region
	}

	// Build storage backend and fetch DB before opening SQLite.
	backend, err := buildBackend(ctx, storageCfg)
	if err != nil {
		return nil, fmt.Errorf("storage backend: %w", err)
	}
	if err := backend.Fetch(ctx, cfg.Memory.Path); err != nil {
		slog.Warn("storage fetch failed, continuing with empty DB", "err", err)
	}

	gh, err := githubadapter.NewClient(ctx, opts.GithubToken, opts.Repo)
	if err != nil {
		return nil, fmt.Errorf("github client: %w", err)
	}

	store, err := memory.NewSQLiteStore(cfg.Memory.Path)
	if err != nil {
		return nil, fmt.Errorf("memory store: %w", err)
	}

	weights := memory.RetrievalWeights{
		Vector:       cfg.Memory.Retrieval.VectorWeight,
		FullText:     cfg.Memory.Retrieval.FullTextWeight,
		CodeLocation: cfg.Memory.Retrieval.CodeLocationWeight,
		Symbol:       cfg.Memory.Retrieval.SymbolWeight,
		Label:        cfg.Memory.Retrieval.LabelWeight,
	}
	retriever := memory.NewRetriever(store, weights)

	var reviewAgent agent.Agent
	agentDef, ok := cfg.Agent.Agents[cfg.Agent.Default]
	if !ok {
		return nil, fmt.Errorf("agent %q not configured", cfg.Agent.Default)
	}
	switch cfg.Agent.Default {
	case "codex":
		reviewAgent = agent.NewCodexAgent(agentDef)
	case "claude":
		reviewAgent = agent.NewClaudeAgent(agentDef)
	default:
		reviewAgent = agent.NewShellAgent(cfg.Agent.Default, agentDef.Command, agentDef.Args, agentDef.Timeout, &agent.JSONParser{})
	}

	var canonicalizer *interpreter.Canonicalizer
	if opts.AnthropicKey != "" {
		canonicalizer = interpreter.NewCanonicalizer(opts.AnthropicKey, cfg.Judge.Model)
	}

	var emb embedder.Embedder
	if opts.OpenAIKey != "" {
		emb = embedder.NewOpenAI(opts.OpenAIKey, cfg.Memory.Embedding.Model)
	}

	var judger judge.Judge
	if cfg.Judge.Enabled && opts.AnthropicKey != "" {
		judger = judge.NewLLMJudge(opts.AnthropicKey, cfg.Judge.Model)
	} else {
		judger = &judge.NoopJudge{}
	}

	filter := NewFilter(PolicyConfig{
		MaxComments:           cfg.Review.MaxComments,
		MinConfidence:         cfg.Review.MinConfidence,
		SuppressFalsePositive: cfg.Judge.SuppressFalsePositive,
	})
	publisher := NewPublisher(gh, opts.DryRun)

	return &Pipeline{
		cfg:           cfg,
		gh:            gh,
		reviewAgent:   reviewAgent,
		store:         store,
		retriever:     retriever,
		canonicalizer: canonicalizer,
		embedder:      emb,
		judger:        judger,
		filter:        filter,
		publisher:     publisher,
		backend:       backend,
		repo:          opts.Repo,
		dryRun:        opts.DryRun,
	}, nil
}

// Close releases resources and flushes the memory DB to the storage backend.
func (p *Pipeline) Close() error {
	storeErr := p.store.Close()
	if !p.dryRun {
		if err := p.backend.Flush(context.Background(), p.cfg.Memory.Path); err != nil {
			slog.Warn("storage flush failed", "err", err)
		}
	}
	return storeErr
}

// RunResult summarises what a review run did.
type RunResult struct {
	PRNumber   int
	Posted     int
	Suppressed int
	DryRun     bool
}

// Run executes the full review pipeline for a given PR (plan.md §7 steps 1-13).
func (p *Pipeline) Run(ctx context.Context, prNumber int) (*RunResult, error) {
	slog.Info("starting review", "pr", prNumber, "repo", p.repo)

	// Step 3: fetch PR diff
	diff, err := p.gh.GetPRDiff(ctx, prNumber)
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}

	// Build patch map for Enrich.
	patches := make(map[string]string, len(diff.Files))
	for _, f := range diff.Files {
		patches[f.Filename] = f.Patch
	}

	// Fetch a few recent memories to give the agent awareness of past issues.
	memoryCtx := p.buildMemoryContext(ctx)

	// Step 6: run AI agent
	agentReq := agent.ReviewRequest{
		Diff:          diff.RawDiff,
		MemoryContext: memoryCtx,
		Prompt:        "",
	}
	agentResult, err := p.reviewAgent.RunReview(ctx, agentReq)
	if err != nil {
		return nil, fmt.Errorf("agent review: %w", err)
	}
	slog.Info("agent findings", "count", len(agentResult.Findings))

	// Step 7: normalize, deduplicate, and enrich
	findings := interpreter.Normalize(agentResult.Findings, p.cfg.Review.MinConfidence)
	findings = interpreter.Dedupe(findings)
	findings = interpreter.Enrich(findings, patches)

	// Step 8: generate canonical claims
	if p.canonicalizer != nil {
		findings, err = p.canonicalizer.Canonicalize(ctx, findings)
		if err != nil {
			slog.Warn("canonicalize failed", "err", err)
		}
	}

	// Steps 9-10: retrieve memory and judge
	judgeResults := make(map[string]*schema.JudgeResult)
	for i := range findings {
		f := &findings[i]
		claim := interpreter.ExtractCanonicalClaim(f)

		var queryVec []float32
		if p.embedder != nil {
			queryVec, _ = p.embedder.Embed(ctx, claim)
		}
		hits, err := p.retriever.Search(ctx, memory.RetrievalQuery{
			Text:     claim,
			Vector:   queryVec,
			FilePath: f.FilePath,
			Symbol:   f.Symbol,
			Limit:    p.cfg.Judge.MaxCandidates,
		})
		if err != nil {
			slog.Warn("retrieval failed", "finding", f.ID, "err", err)
			hits = nil
		}

		jr, err := p.judger.Evaluate(ctx, f, hits)
		if err != nil {
			slog.Warn("judge failed", "finding", f.ID, "err", err)
			continue
		}
		judgeResults[f.ID] = jr
		// Only persist when a concrete memory match was used (memory_id required by FK).
		if jr.MemoryID != "" {
			if err := p.store.SaveJudgeResult(ctx, jr); err != nil {
				slog.Warn("save judge result", "err", err)
			}
		}
	}

	// Step 11: apply policy
	totalBefore := len(findings)
	allowed := p.filter.Apply(findings, judgeResults)
	suppressed := totalBefore - len(allowed)
	slog.Info("policy applied", "allowed", len(allowed), "suppressed", suppressed)

	// Step 12: post to GitHub
	result, err := p.publisher.Publish(ctx, prNumber, diff.HeadSHA, allowed)
	if err != nil {
		return nil, fmt.Errorf("publish: %w", err)
	}

	// Step 13: save memories
	if !p.dryRun {
		for _, f := range allowed {
			m := findingToMemory(f, p.repo, prNumber, p.reviewAgent.Name())
			id, err := p.store.Save(ctx, m)
			if err != nil {
				slog.Warn("save memory", "err", err)
				continue
			}
			if p.embedder != nil {
				claim := interpreter.ExtractCanonicalClaim(&f)
				vec, err := p.embedder.Embed(ctx, claim)
				if err == nil && len(vec) > 0 {
					_ = p.store.SaveEmbedding(ctx, id, vec)
				}
			}
		}
	}

	slog.Info("review complete", "pr", prNumber, "posted", result.Posted, "suppressed", suppressed)
	return &RunResult{
		PRNumber:   prNumber,
		Posted:     result.Posted,
		Suppressed: suppressed,
		DryRun:     p.dryRun,
	}, nil
}

// Sync fetches human replies for previously posted comments and updates outcomes.
func (p *Pipeline) Sync(ctx context.Context, prNumber int) error {
	slog.Info("syncing human replies", "pr", prNumber)
	replies, err := p.gh.ListIssueComments(ctx, prNumber)
	if err != nil {
		return fmt.Errorf("list issue comments: %w", err)
	}
	memories, err := p.store.ListByRepo(ctx, p.repo, 500)
	if err != nil {
		return fmt.Errorf("list memories: %w", err)
	}
	prMemories := filterByPR(memories, prNumber)
	for _, reply := range replies {
		for _, m := range prMemories {
			_ = p.store.AddHumanReply(ctx, m.ID, reply)
		}
	}
	return nil
}

// buildMemoryContext fetches recent repo memories to provide context to the agent.
func (p *Pipeline) buildMemoryContext(ctx context.Context) []schema.MemoryHit {
	memories, err := p.store.ListByRepo(ctx, p.repo, 5)
	if err != nil || len(memories) == 0 {
		return nil
	}
	hits := make([]schema.MemoryHit, 0, len(memories))
	for _, m := range memories {
		claim := m.CanonicalClaim
		if claim == "" {
			claim = m.RawComment
		}
		hits = append(hits, schema.MemoryHit{
			MemoryID:       m.ID,
			CommentText:    m.RawComment,
			CanonicalClaim: claim,
			Outcome:        m.Outcome,
			OutcomeSummary: m.OutcomeSummary,
		})
	}
	return hits
}

func filterByPR(memories []*schema.ReviewMemory, prNumber int) []*schema.ReviewMemory {
	var result []*schema.ReviewMemory
	for _, m := range memories {
		if m.PRNumber == prNumber {
			result = append(result, m)
		}
	}
	return result
}

func findingToMemory(f schema.Finding, repo string, prNumber int, agentName string) *schema.ReviewMemory {
	return &schema.ReviewMemory{
		ID:                 uuid.New().String(),
		Repo:               repo,
		PRNumber:           prNumber,
		Agent:              agentName,
		FilePath:           f.FilePath,
		Symbol:             f.Symbol,
		RawComment:         f.Body,
		CanonicalClaim:     interpreter.ExtractCanonicalClaim(&f),
		CodeContextSummary: f.CodeContextSummary,
		Outcome:            schema.OutcomePending,
		CreatedAt:          time.Now().UTC(),
	}
}

func buildBackend(ctx context.Context, cfg config.StorageConfig) (storage.Backend, error) {
	switch cfg.Backend {
	case "s3":
		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("storage.s3_bucket is required when storage.backend=s3")
		}
		return storage.NewS3Backend(ctx, storage.S3Config{
			Bucket: cfg.S3Bucket,
			Key:    cfg.S3Key,
			Region: cfg.S3Region,
		})
	default:
		return storage.LocalBackend{}, nil
	}
}
