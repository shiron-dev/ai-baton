package agent

import (
	"time"

	"github.com/shiron-dev/ai-baton/internal/config"
)

// NewClaudeAgent creates an Agent backed by the Claude Code CLI.
// Uses stdin mode: the args should end with "-" so the prompt is piped via stdin.
func NewClaudeAgent(def config.AgentDef) Agent {
	timeout := def.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	args := def.Args
	// Ensure prompt is read from stdin (-p -).
	if len(args) == 0 || args[len(args)-1] != "-" {
		args = append(args, "-")
	}
	return NewShellAgent("claude", def.Command, args, timeout, &JSONParser{})
}
