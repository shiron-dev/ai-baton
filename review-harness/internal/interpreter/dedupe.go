package interpreter

import "github.com/shiron-dev/ai-baton/internal/schema"

// Dedupe removes duplicate findings from the same agent pass.
// Findings with identical keys (file + line range + title prefix) are collapsed.
func Dedupe(findings []schema.Finding) []schema.Finding {
	seen := make(map[string]struct{}, len(findings))
	out := make([]schema.Finding, 0, len(findings))
	for _, f := range findings {
		key := FindingKey(&f)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, f)
	}
	return out
}
