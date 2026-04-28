package interpreter

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/shiron-dev/ai-baton/internal/schema"
)

// Normalize ensures each Finding has a valid ID, sensible defaults, and
// filters out findings below the minimum confidence.
func Normalize(findings []schema.Finding, minConfidence float64) []schema.Finding {
	out := make([]schema.Finding, 0, len(findings))
	for _, f := range findings {
		f = normalizeOne(f)
		if f.Confidence < minConfidence {
			continue
		}
		out = append(out, f)
	}
	return out
}

func normalizeOne(f schema.Finding) schema.Finding {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	f.FilePath = filepath.Clean(strings.TrimSpace(f.FilePath))
	if f.FilePath == "." {
		f.FilePath = ""
	}
	if f.StartLine < 0 {
		f.StartLine = 0
	}
	if f.EndLine < f.StartLine {
		f.EndLine = f.StartLine
	}
	if f.Severity == "" {
		f.Severity = "warning"
	}
	f.Title = strings.TrimSpace(f.Title)
	if len(f.Title) > 200 {
		f.Title = f.Title[:197] + "..."
	}
	f.Body = strings.TrimSpace(f.Body)
	if f.Confidence == 0 {
		f.Confidence = 0.5
	}
	return f
}

// FindingKey returns a stable key that identifies structurally equal findings
// (same file + line range + title prefix).
func FindingKey(f *schema.Finding) string {
	title := f.Title
	if len(title) > 40 {
		title = title[:40]
	}
	return fmt.Sprintf("%s:%d-%d:%s", f.FilePath, f.StartLine, f.EndLine, title)
}
