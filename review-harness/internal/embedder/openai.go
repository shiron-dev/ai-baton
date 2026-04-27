package embedder

import (
	"context"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// OpenAIEmbedder embeds text using the OpenAI Embeddings API.
type OpenAIEmbedder struct {
	client *openai.Client
	model  string
}

// NewOpenAI creates an OpenAIEmbedder. model defaults to text-embedding-3-small.
func NewOpenAI(apiKey, model string) *OpenAIEmbedder {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAIEmbedder{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	resp, err := e.client.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: []string{text},
		Model: openai.EmbeddingModel(e.model),
	})
	if err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	return resp.Data[0].Embedding, nil
}
