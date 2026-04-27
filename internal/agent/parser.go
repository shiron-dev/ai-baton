package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shiron-dev/ai-baton/internal/schema"
)

// Parser extracts Findings from raw agent output.
type Parser interface {
	Parse(raw string) ([]schema.Finding, error)
}

// JSONParser extracts a JSON array of findings from agent output.
// It tries strict parse first, then strips markdown code fences.
type JSONParser struct{}

func (p *JSONParser) Parse(raw string) ([]schema.Finding, error) {
	text := strings.TrimSpace(raw)

	// Try direct parse
	if findings, err := parseJSON(text); err == nil {
		return findings, nil
	}

	// Strip markdown code fences
	text = stripCodeFence(text)
	if findings, err := parseJSON(text); err == nil {
		return findings, nil
	}

	// Find the first '[' ... ']' block
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start >= 0 && end > start {
		if findings, err := parseJSON(text[start : end+1]); err == nil {
			return findings, nil
		}
	}

	return nil, fmt.Errorf("could not parse findings from agent output (len=%d)", len(raw))
}

func parseJSON(s string) ([]schema.Finding, error) {
	var findings []schema.Finding
	if err := json.Unmarshal([]byte(s), &findings); err != nil {
		return nil, err
	}
	return findings, nil
}

func stripCodeFence(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if !inFence || trimmed != "" {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
