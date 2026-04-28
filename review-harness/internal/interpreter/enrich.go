package interpreter

import (
	"regexp"
	"strings"

	"github.com/shiron-dev/ai-baton/internal/schema"
)

var (
	backtickRe = regexp.MustCompile("`([^`]+)`")
	// Matches exported Go identifiers (Foo, pkg.Bar) or method chains (pkg.Func).
	goSymbolRe = regexp.MustCompile(`\b([A-Z][A-Za-z0-9]+(?:\.[A-Z][A-Za-z0-9]*)*|[a-z][a-z0-9]*(?:\.[A-Z][A-Za-z0-9]+)+)\b`)
)

// Enrich populates Symbol and CodeContextSummary for each finding in-place.
// patches maps file path → raw diff patch text (PRFile.Patch).
func Enrich(findings []schema.Finding, patches map[string]string) []schema.Finding {
	for i := range findings {
		f := &findings[i]
		if f.Symbol == "" {
			f.Symbol = extractSymbol(f.Title + " " + f.Body)
		}
		if f.CodeContextSummary == "" {
			if patch, ok := patches[f.FilePath]; ok {
				f.CodeContextSummary = patchContext(patch)
			}
		}
	}
	return findings
}

func extractSymbol(text string) string {
	if m := backtickRe.FindStringSubmatch(text); len(m) > 1 {
		s := strings.TrimSpace(m[1])
		// Prefer single-token symbols (no spaces inside backticks).
		if !strings.Contains(s, " ") {
			return s
		}
	}
	if m := goSymbolRe.FindString(text); m != "" {
		return m
	}
	return ""
}

// patchContext trims a raw diff patch to a brief context summary (≤400 chars).
func patchContext(patch string) string {
	lines := strings.SplitN(patch, "\n", 25)
	out := strings.Join(lines, "\n")
	if len(out) > 400 {
		out = out[:397] + "..."
	}
	return strings.TrimSpace(out)
}
