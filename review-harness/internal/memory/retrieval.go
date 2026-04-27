package memory

import (
	"context"
	"sort"

	"github.com/shiron-dev/ai-baton/internal/schema"
)

// RetrievalWeights controls the hybrid search scoring.
type RetrievalWeights struct {
	Vector       float64
	FullText     float64
	CodeLocation float64
	Symbol       float64
	Label        float64
}

// DefaultWeights matches the design spec (plan.md §5.3).
var DefaultWeights = RetrievalWeights{
	Vector:       0.45,
	FullText:     0.25,
	CodeLocation: 0.15,
	Symbol:       0.10,
	Label:        0.05,
}

// RetrievalQuery describes a search request.
type RetrievalQuery struct {
	Text      string
	Vector    []float32
	FilePath  string
	Symbol    string
	Labels    []string
	Limit     int
}

// Retriever performs hybrid search over the Store.
type Retriever struct {
	store   Store
	weights RetrievalWeights
}

// NewRetriever creates a Retriever with the given weights.
func NewRetriever(store Store, weights RetrievalWeights) *Retriever {
	return &Retriever{store: store, weights: weights}
}

// Search executes a hybrid retrieval and returns ranked MemoryHits.
func (r *Retriever) Search(ctx context.Context, q RetrievalQuery) ([]*schema.MemoryHit, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}

	scores := make(map[string]float64)

	if q.Text != "" {
		ftsResults, err := r.store.SearchFTS(ctx, ftsEscape(q.Text), limit*2)
		if err == nil {
			maxScore := 1.0
			if len(ftsResults) > 0 {
				maxScore = ftsResults[0].Score
				if maxScore == 0 {
					maxScore = 1
				}
			}
			for _, res := range ftsResults {
				scores[res.MemoryID] += r.weights.FullText * (res.Score / maxScore)
			}
		}
	}

	if len(q.Vector) > 0 {
		vecResults, err := r.store.SearchVector(ctx, q.Vector, limit*2)
		if err == nil {
			for _, res := range vecResults {
				scores[res.MemoryID] += r.weights.Vector * res.Similarity
			}
		}
	}

	// Collect and rank
	type scored struct {
		id    string
		score float64
	}
	var ranked []scored
	for id, s := range scores {
		ranked = append(ranked, scored{id, s})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	if limit > len(ranked) {
		limit = len(ranked)
	}
	ranked = ranked[:limit]

	hits := make([]*schema.MemoryHit, 0, len(ranked))
	for _, s := range ranked {
		m, err := r.store.GetByID(ctx, s.id)
		if err != nil {
			continue
		}
		hit := &schema.MemoryHit{
			MemoryID:       m.ID,
			CommentText:    m.RawComment,
			CanonicalClaim: m.CanonicalClaim,
			Outcome:        m.Outcome,
			OutcomeSummary: m.OutcomeSummary,
			Similarity:     s.score,
		}
		// Apply code location bonus
		if q.FilePath != "" && q.FilePath == m.FilePath {
			hit.Similarity += r.weights.CodeLocation
		}
		// Apply symbol bonus
		if q.Symbol != "" && q.Symbol == m.Symbol {
			hit.Similarity += r.weights.Symbol
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

// ftsEscape wraps a plain query string for FTS5 to avoid syntax errors.
func ftsEscape(q string) string {
	// Wrap in double quotes for phrase search; strip any internal quotes.
	out := make([]byte, 0, len(q)+2)
	out = append(out, '"')
	for i := 0; i < len(q); i++ {
		if q[i] != '"' {
			out = append(out, q[i])
		}
	}
	out = append(out, '"')
	return string(out)
}
