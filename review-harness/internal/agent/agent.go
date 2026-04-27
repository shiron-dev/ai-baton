package agent

import (
	"context"

	"github.com/shiron-dev/ai-baton/internal/schema"
)

// Agent generates review Finding candidates for a given ReviewRequest.
type Agent interface {
	Name() string
	RunReview(ctx context.Context, req ReviewRequest) (*AgentReviewResult, error)
}

// ReviewRequest is the input to an Agent.
type ReviewRequest struct {
	RepoPath      string
	Diff          string
	CodeContext   string
	MemoryContext []schema.MemoryHit
	Prompt        string
}

// AgentReviewResult is the raw output from an Agent.
type AgentReviewResult struct {
	RawOutput string
	Findings  []schema.Finding
	Metadata  map[string]string
}
