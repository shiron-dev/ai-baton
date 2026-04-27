package judge

import (
	"fmt"
	"strings"

	"github.com/shiron-dev/ai-baton/internal/schema"
)

func buildJudgePrompt(finding *schema.Finding, hits []*schema.MemoryHit) string {
	var sb strings.Builder
	sb.WriteString(`You are a code review judge. Given a new Finding and past similar review comments, decide whether the new Finding should be posted, suppressed, or weakened.

Respond ONLY with a valid JSON object with these fields:
- "same_underlying_issue": bool
- "applies_to_current_diff": bool
- "prior_outcome": string (one of: accepted, rejected, ignored, discussed, false_positive, duplicate, pending, "")
- "recommended_action": string (one of: "post", "suppress", "weaken", "enrich")
- "reason": string (1-2 sentences in Japanese explaining your decision)
- "confidence": float (0.0-1.0)

`)

	sb.WriteString("## New Finding\n\n")
	sb.WriteString(fmt.Sprintf("File: %s (lines %d-%d)\n", finding.FilePath, finding.StartLine, finding.EndLine))
	sb.WriteString(fmt.Sprintf("Title: %s\n", finding.Title))
	sb.WriteString(fmt.Sprintf("Body: %s\n", finding.Body))
	sb.WriteString(fmt.Sprintf("Severity: %s, Confidence: %.2f\n", finding.Severity, finding.Confidence))

	if len(hits) > 0 {
		sb.WriteString("\n## Past Similar Comments\n\n")
		for i, hit := range hits {
			sb.WriteString(fmt.Sprintf("### Past Comment %d (similarity=%.2f)\n", i+1, hit.Similarity))
			sb.WriteString(fmt.Sprintf("Canonical Claim: %s\n", hit.CanonicalClaim))
			sb.WriteString(fmt.Sprintf("Past Outcome: %s\n", string(hit.Outcome)))
			if hit.OutcomeSummary != "" {
				sb.WriteString(fmt.Sprintf("Outcome Summary: %s\n", hit.OutcomeSummary))
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("\n## Past Similar Comments\n\nNone found.\n")
	}

	sb.WriteString("\nOutput only the JSON object:")
	return sb.String()
}
