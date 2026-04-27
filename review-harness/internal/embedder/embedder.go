package embedder

import "context"

// Embedder converts text to a dense vector representation.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
