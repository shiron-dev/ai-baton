package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/shiron-dev/ai-baton/internal/githubadapter"
	"github.com/shiron-dev/ai-baton/internal/schema"
)

// Publisher posts review findings to GitHub.
type Publisher struct {
	gh     *githubadapter.Client
	dryRun bool
}

// NewPublisher creates a Publisher.
func NewPublisher(gh *githubadapter.Client, dryRun bool) *Publisher {
	return &Publisher{gh: gh, dryRun: dryRun}
}

// Publish posts the given findings as a GitHub PR review.
func (p *Publisher) Publish(
	ctx context.Context,
	prNumber int,
	commitID string,
	findings []schema.Finding,
) (*githubadapter.PostReviewResult, error) {
	inlines := make([]*githubadapter.InlineComment, 0, len(findings))
	for i := range findings {
		inlines = append(inlines, &githubadapter.InlineComment{
			Finding:  &findings[i],
			CommitID: commitID,
		})
	}

	summary := buildSummary(findings)
	result, err := p.gh.PostReview(ctx, prNumber, commitID, inlines, summary, p.dryRun)
	if err != nil {
		return nil, fmt.Errorf("post review: %w", err)
	}
	return result, nil
}

func buildSummary(findings []schema.Finding) string {
	if len(findings) == 0 {
		return "AI Review: No issues found."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "## AI Review Summary\n\n%d issue(s) found.\n\n", len(findings))
	for _, f := range findings {
		icon := severityIcon(f.Severity)
		fmt.Fprintf(&sb, "- %s **%s** (`%s`:%d)\n", icon, f.Title, f.FilePath, f.StartLine)
	}
	return sb.String()
}

func severityIcon(severity string) string {
	switch severity {
	case "error":
		return ":red_circle:"
	case "warning":
		return ":yellow_circle:"
	default:
		return ":blue_circle:"
	}
}
