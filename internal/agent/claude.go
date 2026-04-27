package agent

import (
	"time"

	"github.com/shiron-dev/ai-baton/internal/config"
)

// NewClaudeAgent creates an Agent backed by the Claude Code CLI.
func NewClaudeAgent(def config.AgentDef) Agent {
	timeout := def.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	return NewShellAgent("claude", def.Command, def.Args, timeout, &JSONParser{})
}
