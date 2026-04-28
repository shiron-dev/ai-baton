package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/shiron-dev/ai-baton/internal/config"
)

// OpenAIAgent implements Agent using the OpenAI API directly.
type OpenAIAgent struct {
	client  *openai.Client
	timeout time.Duration
	model   string
	parser  Parser
}

// NewOpenAIAgent creates an Agent backed by the OpenAI Chat Completions API.
func NewOpenAIAgent(apiKey string, def config.AgentDef) Agent {
	timeout := def.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	model := def.Model
	if model == "" {
		model = "gpt-4.1-mini"
	}
	return &OpenAIAgent{
		client:  openai.NewClient(apiKey),
		timeout: timeout,
		model:   model,
		parser:  &JSONParser{},
	}
}

func (a *OpenAIAgent) Name() string { return "openai" }

func (a *OpenAIAgent) RunReview(ctx context.Context, req ReviewRequest) (*AgentReviewResult, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	prompt := buildPrompt(req)
	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an expert code reviewer. Output only a JSON array of findings, with no markdown fences or prose.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		MaxCompletionTokens: 4096,
	})
	if err != nil {
		return nil, fmt.Errorf("openai chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai chat completion returned no choices")
	}

	raw := resp.Choices[0].Message.Content
	if raw == "" {
		return nil, fmt.Errorf("openai chat completion returned empty content (model=%s, finish_reason=%s, prompt_tokens=%d, completion_tokens=%d)",
			a.model,
			resp.Choices[0].FinishReason,
			resp.Usage.PromptTokens,
			resp.Usage.CompletionTokens,
		)
	}
	findings, err := a.parser.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("openai parse output: %w", err)
	}

	return &AgentReviewResult{
		RawOutput: raw,
		Findings:  findings,
		Metadata:  map[string]string{"model": a.model},
	}, nil
}
