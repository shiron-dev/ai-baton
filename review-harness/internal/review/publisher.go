package review

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
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
	patches map[string]string,
) (*githubadapter.PostReviewResult, error) {
	inlines := make([]*githubadapter.InlineComment, 0, len(findings))
	for i := range findings {
		if !normalizeInlineLocation(&findings[i], patches[findings[i].FilePath]) {
			slog.Warn("skipping inline comment for non-diff line", "file", findings[i].FilePath, "start_line", findings[i].StartLine, "end_line", findings[i].EndLine)
			continue
		}
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

func normalizeInlineLocation(f *schema.Finding, patch string) bool {
	if patch == "" || f.StartLine <= 0 {
		return false
	}
	commentable := commentableNewLines(patch)
	if len(commentable) == 0 {
		return false
	}

	line := f.EndLine
	if line <= 0 {
		line = f.StartLine
	}
	if !commentable[line] {
		if commentable[f.StartLine] {
			line = f.StartLine
		} else {
			return false
		}
	}
	f.EndLine = line
	if f.StartLine > line || (f.StartLine != line && !commentable[f.StartLine]) {
		f.StartLine = line
	}
	return true
}

var hunkHeaderRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

func commentableNewLines(patch string) map[int]bool {
	lines := make(map[int]bool)
	newLine := 0
	for _, line := range strings.Split(patch, "\n") {
		if m := hunkHeaderRe.FindStringSubmatch(line); len(m) == 2 {
			n, err := strconv.Atoi(m[1])
			if err == nil {
				newLine = n
			}
			continue
		}
		if newLine == 0 {
			continue
		}
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			lines[newLine] = true
			newLine++
		case strings.HasPrefix(line, " "):
			lines[newLine] = true
			newLine++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			// Deleted lines do not advance the new-file line counter.
		case line == "":
			lines[newLine] = true
			newLine++
		}
	}
	return lines
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
