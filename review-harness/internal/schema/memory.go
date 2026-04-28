package schema

import "time"

// Outcome describes how a past review comment was handled by humans.
type Outcome string

const (
	OutcomeAccepted      Outcome = "accepted"
	OutcomeRejected      Outcome = "rejected"
	OutcomeIgnored       Outcome = "ignored"
	OutcomeDiscussed     Outcome = "discussed"
	OutcomeFalsePositive Outcome = "false_positive"
	OutcomeDuplicate     Outcome = "duplicate"
	OutcomePending       Outcome = "pending"
)

// HumanReply is a human response to an AI review comment.
type HumanReply struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// ReviewMemory is a persisted record of one AI review comment and its outcome.
type ReviewMemory struct {
	ID                 string       `json:"id"`
	Repo               string       `json:"repo"`
	PRNumber           int          `json:"pr_number"`
	Agent              string       `json:"agent"`
	FilePath           string       `json:"file_path"`
	Symbol             string       `json:"symbol"`
	RawComment         string       `json:"raw_comment"`
	CanonicalClaim     string       `json:"canonical_claim"`
	CodeContextSummary string       `json:"code_context_summary"`
	Outcome            Outcome      `json:"outcome"`
	OutcomeSummary     string       `json:"outcome_summary"`
	HumanReplies       []HumanReply `json:"human_replies"`
	EmbeddingID        string       `json:"embedding_id"`
	CreatedAt          time.Time    `json:"created_at"`
}

// MemoryHit is a search result from the Review Memory store.
type MemoryHit struct {
	MemoryID       string  `json:"memory_id"`
	CommentText    string  `json:"comment_text"`
	CanonicalClaim string  `json:"canonical_claim"`
	Outcome        Outcome `json:"outcome"`
	OutcomeSummary string  `json:"outcome_summary"`
	Similarity     float64 `json:"similarity"`
	Reason         string  `json:"reason"`
}

// RecommendedAction is what the Judge recommends for a Finding.
type RecommendedAction string

const (
	ActionPost     RecommendedAction = "post"
	ActionSuppress RecommendedAction = "suppress"
	ActionWeaken   RecommendedAction = "weaken"
	ActionEnrich   RecommendedAction = "enrich"
)

// JudgeResult is the output of the Review Judge for one Finding + MemoryHit pair.
type JudgeResult struct {
	FindingID            string            `json:"finding_id"`
	MemoryID             string            `json:"memory_id"`
	SameUnderlyingIssue  bool              `json:"same_underlying_issue"`
	AppliesToCurrentDiff bool              `json:"applies_to_current_diff"`
	PriorOutcome         Outcome           `json:"prior_outcome"`
	RecommendedAction    RecommendedAction `json:"recommended_action"`
	Reason               string            `json:"reason"`
	Confidence           float64           `json:"confidence"`
}
