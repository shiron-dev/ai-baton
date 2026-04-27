package githubadapter

import (
	"context"
	"fmt"

	"github.com/google/go-github/v62/github"
	"github.com/shiron-dev/ai-baton/internal/schema"
)

// ExistingReviewComment is a GitHub PR review comment already posted.
type ExistingReviewComment struct {
	ID       int64
	Path     string
	Line     int
	Body     string
	User     string
	IsBot    bool
	Replies  []ExistingReviewComment
}

// ListReviewComments returns all review comments on the PR.
func (c *Client) ListReviewComments(ctx context.Context, prNumber int) ([]*ExistingReviewComment, error) {
	opts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var all []*github.PullRequestComment
	for {
		comments, resp, err := c.gh.PullRequests.ListComments(ctx, c.owner, c.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("list review comments PR %d: %w", prNumber, err)
		}
		all = append(all, comments...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	result := make([]*ExistingReviewComment, 0, len(all))
	for _, c := range all {
		user := c.GetUser()
		isBot := user != nil && user.GetType() == "Bot"
		result = append(result, &ExistingReviewComment{
			ID:    c.GetID(),
			Path:  c.GetPath(),
			Line:  c.GetLine(),
			Body:  c.GetBody(),
			User:  user.GetLogin(),
			IsBot: isBot,
		})
	}
	return result, nil
}

// ListIssueComments returns top-level (non-review) comments on the PR as issue comments.
// These are used to detect human replies to bot summary comments.
func (c *Client) ListIssueComments(ctx context.Context, prNumber int) ([]schema.HumanReply, error) {
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var replies []schema.HumanReply
	for {
		comments, resp, err := c.gh.Issues.ListComments(ctx, c.owner, c.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("list issue comments PR %d: %w", prNumber, err)
		}
		for _, ic := range comments {
			user := ic.GetUser()
			if user != nil && user.GetType() == "Bot" {
				continue
			}
			replies = append(replies, schema.HumanReply{
				ID:        fmt.Sprintf("%d", ic.GetID()),
				Author:    user.GetLogin(),
				Body:      ic.GetBody(),
				CreatedAt: ic.GetCreatedAt().Time,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return replies, nil
}
