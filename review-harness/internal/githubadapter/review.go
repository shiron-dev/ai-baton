package githubadapter

import (
	"context"
	"fmt"

	"github.com/google/go-github/v62/github"
	"github.com/shiron-dev/ai-baton/internal/schema"
)

// InlineComment is a single inline comment to post.
type InlineComment struct {
	Finding  *schema.Finding
	CommitID string
}

// PostReviewResult is the result of posting a review.
type PostReviewResult struct {
	ReviewID int64
	HTMLURL  string
	Posted   int
}

// PostReview creates a GitHub PR review with inline comments and an optional summary.
// If dryRun is true, the review is not actually posted.
func (c *Client) PostReview(
	ctx context.Context,
	prNumber int,
	commitID string,
	inlines []*InlineComment,
	summary string,
	dryRun bool,
) (*PostReviewResult, error) {
	if dryRun {
		return &PostReviewResult{Posted: len(inlines)}, nil
	}

	comments := make([]*github.DraftReviewComment, 0, len(inlines))
	for _, ic := range inlines {
		if ic.Finding.StartLine <= 0 {
			continue
		}
		body := formatInlineBody(ic.Finding)
		line := ic.Finding.EndLine
		if line <= 0 {
			line = ic.Finding.StartLine
		}
		comment := &github.DraftReviewComment{
			Path: github.String(ic.Finding.FilePath),
			Body: github.String(body),
			Line: github.Int(line),
		}
		if ic.Finding.StartLine != ic.Finding.EndLine && ic.Finding.StartLine > 0 {
			comment.StartLine = github.Int(ic.Finding.StartLine)
		}
		comments = append(comments, comment)
	}

	event := "COMMENT"
	req := &github.PullRequestReviewRequest{
		CommitID: github.String(commitID),
		Body:     github.String(summary),
		Event:    github.String(event),
		Comments: comments,
	}

	review, _, err := c.gh.PullRequests.CreateReview(ctx, c.owner, c.repo, prNumber, req)
	if err != nil {
		return nil, fmt.Errorf("create review PR %d: %w", prNumber, err)
	}

	return &PostReviewResult{
		ReviewID: review.GetID(),
		HTMLURL:  review.GetHTMLURL(),
		Posted:   len(comments),
	}, nil
}

func formatInlineBody(f *schema.Finding) string {
	body := f.Body
	if f.SuggestedPatch != "" {
		body += "\n\n```suggestion\n" + f.SuggestedPatch + "\n```"
	}
	return body
}
