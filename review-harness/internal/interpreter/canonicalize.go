package interpreter

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/shiron-dev/ai-baton/internal/schema"
)

// Canonicalizer generates canonical_claim strings from Findings using an LLM.
type Canonicalizer struct {
	client *anthropic.Client
	model  string
}

// NewCanonicalizer creates a Canonicalizer backed by the Anthropic API.
func NewCanonicalizer(apiKey, model string) *Canonicalizer {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	return &Canonicalizer{client: &client, model: model}
}

// Canonicalize sets Finding.CanonicalClaim for each finding in-place.
// The claim is a language-neutral, concise summary of the core assertion,
// independent of the reviewer's phrasing.
func (c *Canonicalizer) Canonicalize(ctx context.Context, findings []schema.Finding) ([]schema.Finding, error) {
	for i := range findings {
		claim, err := c.generateClaim(ctx, &findings[i])
		if err != nil {
			return nil, fmt.Errorf("canonicalize finding %s: %w", findings[i].ID, err)
		}
		findings[i].CanonicalClaim = claim
	}
	return findings, nil
}

func (c *Canonicalizer) generateClaim(ctx context.Context, f *schema.Finding) (string, error) {
	prompt := fmt.Sprintf(
		`Summarize the core assertion of this code review comment in one concise Japanese sentence.
Do NOT include the file name, line number, or reviewer's phrasing — focus on the logical claim.

Title: %s
Body: %s

Output only the single sentence, nothing else.`, f.Title, f.Body)

	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 200,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(msg.Content[0].Text), nil
}

// ExtractCanonicalClaim returns the canonical claim for a finding.
// Falls back to Title when CanonicalClaim is empty.
func ExtractCanonicalClaim(f *schema.Finding) string {
	if f.CanonicalClaim != "" {
		return f.CanonicalClaim
	}
	return f.Title
}
