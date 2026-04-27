package githubadapter

import (
	"context"
	"fmt"

	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client.
type Client struct {
	gh    *github.Client
	owner string
	repo  string
}

// NewClient creates an authenticated GitHub client for the given repo slug (owner/repo).
func NewClient(ctx context.Context, token, repoSlug string) (*Client, error) {
	owner, repo, ok := splitSlug(repoSlug)
	if !ok {
		return nil, fmt.Errorf("invalid repo slug %q, expected owner/repo", repoSlug)
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return &Client{
		gh:    github.NewClient(tc),
		owner: owner,
		repo:  repo,
	}, nil
}

func splitSlug(slug string) (owner, repo string, ok bool) {
	for i, c := range slug {
		if c == '/' {
			return slug[:i], slug[i+1:], true
		}
	}
	return "", "", false
}
