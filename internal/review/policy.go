package review

import (
	"github.com/shiron-dev/ai-baton/internal/schema"
)

// PolicyConfig holds filtering rules.
type PolicyConfig struct {
	MaxComments          int
	MinConfidence        float64
	SuppressFalsePositive bool
}

// Filter applies policy to a slice of findings+judge results and returns
// the subset that should be posted, up to MaxComments.
type Filter struct {
	cfg PolicyConfig
}

// NewFilter creates a Filter with the given policy.
func NewFilter(cfg PolicyConfig) *Filter {
	return &Filter{cfg: cfg}
}

// Apply returns the findings that pass policy, capped at MaxComments.
func (f *Filter) Apply(findings []schema.Finding, judgeResults map[string]*schema.JudgeResult) []schema.Finding {
	var allowed []schema.Finding
	for _, finding := range findings {
		if finding.Confidence < f.cfg.MinConfidence {
			continue
		}
		jr, hasJudge := judgeResults[finding.ID]
		if hasJudge {
			if jr.RecommendedAction == schema.ActionSuppress {
				continue
			}
			if f.cfg.SuppressFalsePositive && jr.PriorOutcome == schema.OutcomeFalsePositive {
				if jr.SameUnderlyingIssue && !jr.AppliesToCurrentDiff {
					continue
				}
			}
		}
		allowed = append(allowed, finding)
		if f.cfg.MaxComments > 0 && len(allowed) >= f.cfg.MaxComments {
			break
		}
	}
	return allowed
}
