package memory

import (
	"context"

	"github.com/shiron-dev/ai-baton/internal/schema"
)

// Store is the interface for persisting and retrieving review memories.
type Store interface {
	// Save persists a new ReviewMemory and returns its assigned ID.
	Save(ctx context.Context, m *schema.ReviewMemory) (string, error)

	// UpdateOutcome updates the outcome and summary of an existing memory.
	UpdateOutcome(ctx context.Context, id string, outcome schema.Outcome, summary string) error

	// AddHumanReply appends a human reply to an existing memory.
	AddHumanReply(ctx context.Context, memoryID string, reply schema.HumanReply) error

	// GetByID returns a ReviewMemory by its ID.
	GetByID(ctx context.Context, id string) (*schema.ReviewMemory, error)

	// ListByRepo returns all memories for a given repo, ordered by created_at desc.
	ListByRepo(ctx context.Context, repo string, limit int) ([]*schema.ReviewMemory, error)

	// SaveEmbedding stores the embedding vector for a memory.
	SaveEmbedding(ctx context.Context, memoryID string, vector []float32) error

	// SearchFTS performs full-text search and returns matching memory IDs with scores.
	SearchFTS(ctx context.Context, query string, limit int) ([]FTSResult, error)

	// SearchVector performs cosine-similarity search and returns top-k matches.
	SearchVector(ctx context.Context, query []float32, limit int) ([]VectorResult, error)

	// SaveJudgeResult persists a JudgeResult.
	SaveJudgeResult(ctx context.Context, r *schema.JudgeResult) error

	// Close releases all resources.
	Close() error
}

// FTSResult is a full-text search match.
type FTSResult struct {
	MemoryID string
	Score    float64
}

// VectorResult is a vector similarity search match.
type VectorResult struct {
	MemoryID   string
	Similarity float64
}
