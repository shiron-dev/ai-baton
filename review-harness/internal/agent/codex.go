package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/shiron-dev/ai-baton/internal/config"
)

// CodexAgent implements Agent using the Codex CLI (codex exec).
type CodexAgent struct {
	command string
	timeout time.Duration
	model   string
	parser  Parser
}

// NewCodexAgent creates an Agent backed by the Codex CLI.
func NewCodexAgent(def config.AgentDef) Agent {
	timeout := def.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	return &CodexAgent{
		command: def.Command,
		timeout: timeout,
		parser:  &JSONParser{},
	}
}

func (a *CodexAgent) Name() string { return "codex" }

func (a *CodexAgent) RunReview(ctx context.Context, req ReviewRequest) (*AgentReviewResult, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	// Write the agent's final response to a temp file for clean extraction.
	outFile, err := os.CreateTemp("", "codex-out-*.txt")
	if err != nil {
		return nil, fmt.Errorf("codex: create temp output file: %w", err)
	}
	outPath := outFile.Name()
	outFile.Close()
	defer os.Remove(outPath)

	args := []string{
		"exec", "-",
		"--dangerously-bypass-approvals-and-sandbox",
		"-s", "read-only",
		"--ephemeral",
		"--output-last-message", outPath,
	}
	if req.RepoPath != "" {
		args = append(args, "-C", req.RepoPath)
	}
	if a.model != "" {
		args = append(args, "-m", a.model)
	}

	prompt := buildPrompt(req)
	cmd := exec.CommandContext(ctx, a.command, args...)
	cmd.Stdin = strings.NewReader(prompt)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("codex exec: %w\nstderr: %s", err, stderr.String())
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("codex: read output file: %w", err)
	}

	findings, err := a.parser.Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("codex parse output: %w", err)
	}

	return &AgentReviewResult{
		RawOutput: string(raw),
		Findings:  findings,
		Metadata:  map[string]string{"stderr": stderr.String()},
	}, nil
}
