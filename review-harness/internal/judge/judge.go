package judge

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/shiron-dev/ai-baton/internal/schema"
)

// Judge evaluates whether a Finding should be posted based on past memory.
type Judge interface {
	Evaluate(ctx context.Context, finding *schema.Finding, hits []*schema.MemoryHit) (*schema.JudgeResult, error)
}

// LLMJudge uses an LLM to evaluate findings against past memory.
type LLMJudge struct {
	client *anthropic.Client
	model  string
}

// NewLLMJudge creates a Judge backed by the Anthropic Claude API.
func NewLLMJudge(apiKey, model string) *LLMJudge {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	return &LLMJudge{client: &client, model: model}
}

// Evaluate sends the finding + memory hits to the LLM and parses the judgment.
func (j *LLMJudge) Evaluate(ctx context.Context, finding *schema.Finding, hits []*schema.MemoryHit) (*schema.JudgeResult, error) {
	prompt := buildJudgePrompt(finding, hits)

	msg, err := j.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(j.model),
		MaxTokens: 512,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("judge LLM call: %w", err)
	}

	raw := strings.TrimSpace(msg.Content[0].Text)
	resp, err := parseJudgeResponse(raw)
	if err != nil {
		return nil, fmt.Errorf("judge parse: %w", err)
	}

	memoryID := ""
	if len(hits) > 0 {
		memoryID = hits[0].MemoryID
	}
	return toJudgeResult(finding.ID, memoryID, resp), nil
}

// NoopJudge always recommends posting (used when judge is disabled).
type NoopJudge struct{}

func (n *NoopJudge) Evaluate(_ context.Context, f *schema.Finding, _ []*schema.MemoryHit) (*schema.JudgeResult, error) {
	return &schema.JudgeResult{
		FindingID:            f.ID,
		AppliesToCurrentDiff: true,
		RecommendedAction:    schema.ActionPost,
		Confidence:           1.0,
	}, nil
}
