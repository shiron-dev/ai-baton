package githubadapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v62/github"
)

// PRDiff contains the parsed diff for a pull request.
type PRDiff struct {
	Number    int
	Title     string
	Body      string
	HeadSHA   string
	BaseSHA   string
	RawDiff   string
	Files     []*PRFile
}

// PRFile is one changed file in a PR.
type PRFile struct {
	Filename  string
	Status    string // "added", "removed", "modified", "renamed"
	Patch     string
	Additions int
	Deletions int
}

// GetPRDiff fetches the diff and file list for the given PR number.
func (c *Client) GetPRDiff(ctx context.Context, prNumber int) (*PRDiff, error) {
	pr, _, err := c.gh.PullRequests.Get(ctx, c.owner, c.repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("get PR %d: %w", prNumber, err)
	}

	rawDiff, _, err := c.gh.PullRequests.GetRaw(ctx, c.owner, c.repo, prNumber,
		github.RawOptions{Type: github.Diff})
	if err != nil {
		return nil, fmt.Errorf("get diff PR %d: %w", prNumber, err)
	}

	var allFiles []*PRFile
	opts := &github.ListOptions{PerPage: 100}
	for {
		files, resp, err := c.gh.PullRequests.ListFiles(ctx, c.owner, c.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("list files PR %d: %w", prNumber, err)
		}
		for _, f := range files {
			allFiles = append(allFiles, &PRFile{
				Filename:  f.GetFilename(),
				Status:    f.GetStatus(),
				Patch:     f.GetPatch(),
				Additions: f.GetAdditions(),
				Deletions: f.GetDeletions(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return &PRDiff{
		Number:  prNumber,
		Title:   pr.GetTitle(),
		Body:    pr.GetBody(),
		HeadSHA: pr.GetHead().GetSHA(),
		BaseSHA: pr.GetBase().GetSHA(),
		RawDiff: rawDiff,
		Files:   allFiles,
	}, nil
}

// DiffContext returns the diff text scoped to a specific file path.
func (d *PRDiff) DiffContext(filePath string) string {
	for _, f := range d.Files {
		if f.Filename == filePath {
			return f.Patch
		}
	}
	return ""
}

// ChangedFilePaths returns the sorted list of changed file paths.
func (d *PRDiff) ChangedFilePaths() []string {
	paths := make([]string, 0, len(d.Files))
	for _, f := range d.Files {
		if !strings.HasSuffix(f.Filename, "/") {
			paths = append(paths, f.Filename)
		}
	}
	return paths
}
