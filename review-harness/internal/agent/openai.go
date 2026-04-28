package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/shiron-dev/ai-baton/internal/config"
	"github.com/shiron-dev/ai-baton/internal/schema"
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

	chunks := splitDiffByFile(req.Diff)

	var allFindings []schema.Finding
	var rawParts []string

	for _, chunk := range chunks {
		chunkReq := req
		chunkReq.Diff = chunk
		result, err := a.runChunk(ctx, chunkReq)
		if err != nil {
			return nil, err
		}
		allFindings = append(allFindings, result.Findings...)
		rawParts = append(rawParts, result.RawOutput)
	}

	return &AgentReviewResult{
		RawOutput: strings.Join(rawParts, "\n"),
		Findings:  allFindings,
		Metadata:  map[string]string{"model": a.model},
	}, nil
}

func (a *OpenAIAgent) runChunk(ctx context.Context, req ReviewRequest) (*AgentReviewResult, error) {
	prompt := buildPrompt(req)
	slog.Debug("agent prompt", "agent", "openai", "model", a.model, "prompt", prompt)

	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: `You are an expert code reviewer. Output only a JSON object in this exact shape: {"findings":[...]}. Do not use markdown fences or prose. If there are no issues, output {"findings":[]}.`,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		MaxCompletionTokens: 4096,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("openai chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai chat completion returned no choices")
	}

	raw := resp.Choices[0].Message.Content
	slog.Debug("agent raw output", "agent", "openai", "model", a.model,
		"output", raw,
		"prompt_tokens", resp.Usage.PromptTokens,
		"completion_tokens", resp.Usage.CompletionTokens,
		"finish_reason", resp.Choices[0].FinishReason,
	)
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
	}, nil
}

// splitDiffByFile splits a unified diff into per-file chunks.
// Each chunk starts with the "diff --git" header for one file.
func splitDiffByFile(diff string) []string {
	var chunks []string
	var current strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") && current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	if len(chunks) == 0 {
		return []string{diff}
	}
	return chunks
}
