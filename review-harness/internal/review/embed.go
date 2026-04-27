package review

import (
	"context"
	"fmt"
	"log/slog"
)

// EmbedAll generates and stores embeddings for any memories that lack one.
// Returns the number of embeddings successfully created.
func (p *Pipeline) EmbedAll(ctx context.Context) (int, error) {
	if p.embedder == nil {
		return 0, fmt.Errorf("no embedder configured: set OPENAI_API_KEY")
	}

	batch, err := p.store.ListWithoutEmbedding(ctx, 500)
	if err != nil {
		return 0, fmt.Errorf("list unembed: %w", err)
	}
	if len(batch) == 0 {
		return 0, nil
	}

	count := 0
	for _, m := range batch {
		text := m.CanonicalClaim
		if text == "" {
			text = m.RawComment
		}
		vec, err := p.embedder.Embed(ctx, text)
		if err != nil {
			slog.Warn("embed failed", "id", m.ID, "err", err)
			continue
		}
		if len(vec) == 0 {
			continue
		}
		if err := p.store.SaveEmbedding(ctx, m.ID, vec); err != nil {
			slog.Warn("save embedding failed", "id", m.ID, "err", err)
			continue
		}
		count++
	}
	return count, nil
}
