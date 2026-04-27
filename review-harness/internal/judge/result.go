package judge

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shiron-dev/ai-baton/internal/schema"
)

type judgeResponse struct {
	SameUnderlyingIssue  bool    `json:"same_underlying_issue"`
	AppliesToCurrentDiff bool    `json:"applies_to_current_diff"`
	PriorOutcome         string  `json:"prior_outcome"`
	RecommendedAction    string  `json:"recommended_action"`
	Reason               string  `json:"reason"`
	Confidence           float64 `json:"confidence"`
}

func parseJudgeResponse(raw string) (*judgeResponse, error) {
	text := strings.TrimSpace(raw)

	// Try direct parse
	var resp judgeResponse
	if err := json.Unmarshal([]byte(text), &resp); err == nil {
		return &resp, nil
	}

	// Strip markdown fences
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(text[start:end+1]), &resp); err == nil {
			return &resp, nil
		}
	}
	return nil, fmt.Errorf("parse judge response: %s", raw[:min(len(raw), 200)])
}

func toJudgeResult(findingID, memoryID string, r *judgeResponse) *schema.JudgeResult {
	return &schema.JudgeResult{
		FindingID:            findingID,
		MemoryID:             memoryID,
		SameUnderlyingIssue:  r.SameUnderlyingIssue,
		AppliesToCurrentDiff: r.AppliesToCurrentDiff,
		PriorOutcome:         schema.Outcome(r.PriorOutcome),
		RecommendedAction:    schema.RecommendedAction(r.RecommendedAction),
		Reason:               r.Reason,
		Confidence:           r.Confidence,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
