package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ShellAgent implements Agent by invoking an external CLI.
type ShellAgent struct {
	name    string
	command string
	args    []string
	timeout time.Duration
	parser  Parser
}

// NewShellAgent creates a ShellAgent with the given CLI command.
func NewShellAgent(name, command string, args []string, timeout time.Duration, parser Parser) *ShellAgent {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	return &ShellAgent{
		name:    name,
		command: command,
		args:    args,
		timeout: timeout,
		parser:  parser,
	}
}

func (a *ShellAgent) Name() string { return a.name }

func (a *ShellAgent) RunReview(ctx context.Context, req ReviewRequest) (*AgentReviewResult, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	prompt := buildPrompt(req)

	// If the last configured arg is "-", pass prompt via stdin; otherwise append as positional arg.
	var args []string
	stdinMode := len(a.args) > 0 && a.args[len(a.args)-1] == "-"
	if stdinMode {
		args = a.args
	} else {
		args = append(a.args, prompt) //nolint:gocritic
	}

	cmd := exec.CommandContext(ctx, a.command, args...)
	if req.RepoPath != "" {
		cmd.Dir = req.RepoPath
	}
	if stdinMode {
		cmd.Stdin = strings.NewReader(prompt)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("agent %s: %w\nstderr: %s", a.name, err, stderr.String())
	}

	raw := stdout.String()
	findings, err := a.parser.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("agent %s parse output: %w", a.name, err)
	}

	return &AgentReviewResult{
		RawOutput: raw,
		Findings:  findings,
		Metadata:  map[string]string{"stderr": stderr.String()},
	}, nil
}

func buildPrompt(req ReviewRequest) string {
	var sb strings.Builder
	sb.WriteString(reviewSystemPrompt)
	sb.WriteString("\n\n## Diff\n\n```diff\n")
	sb.WriteString(req.Diff)
	sb.WriteString("\n```\n")

	if req.CodeContext != "" {
		sb.WriteString("\n## Code Context\n\n")
		sb.WriteString(req.CodeContext)
		sb.WriteString("\n")
	}

	if len(req.MemoryContext) > 0 {
		sb.WriteString("\n## Past Review Memory (for your awareness)\n\n")
		for _, hit := range req.MemoryContext {
			sb.WriteString("- ")
			sb.WriteString(hit.CanonicalClaim)
			if hit.OutcomeSummary != "" {
				sb.WriteString(" [outcome: ")
				sb.WriteString(hit.OutcomeSummary)
				sb.WriteString("]")
			}
			sb.WriteString("\n")
		}
	}

	if req.Prompt != "" {
		sb.WriteString("\n## Additional Instructions\n\n")
		sb.WriteString(req.Prompt)
		sb.WriteString("\n")
	}

	return sb.String()
}

const reviewSystemPrompt = `You are an expert code reviewer. Analyze the diff below and output a JSON array of findings.

Each finding must be a JSON object with these fields:
- "file_path": string (the file being reviewed)
- "start_line": int (first line of the issue, in the new file)
- "end_line": int (last line of the issue, in the new file)
- "title": string (short title, ≤80 chars)
- "body": string (detailed explanation of the issue)
- "severity": string ("error" | "warning" | "info")
- "confidence": float (0.0–1.0)
- "labels": array of strings (e.g. ["bug", "security"])
- "suggested_patch": string (optional corrected code)

Output ONLY a JSON array. No markdown fences, no explanation outside the JSON.
Example: [{"file_path":"main.go","start_line":10,"end_line":12,"title":"nil dereference","body":"...","severity":"error","confidence":0.9,"labels":["bug"]}]`
