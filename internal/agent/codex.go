package agent

import (
	"time"

	"github.com/shiron-dev/ai-baton/internal/config"
)

// NewCodexAgent creates an Agent backed by the Codex CLI.
func NewCodexAgent(def config.AgentDef) Agent {
	timeout := def.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	return NewShellAgent("codex", def.Command, def.Args, timeout, &JSONParser{})
}
