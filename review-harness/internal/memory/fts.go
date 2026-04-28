package memory

import "strings"

// ftsEscape builds a safe FTS5 MATCH expression from plain text.
// Each whitespace-delimited token is quoted and joined with AND,
// which handles multilingual text and code identifiers better than phrase search.
func ftsEscape(q string) string {
	tokens := strings.Fields(q)
	if len(tokens) == 0 {
		return `""`
	}
	terms := make([]string, 0, len(tokens))
	for _, t := range tokens {
		clean := sanitizeFTSToken(t)
		if clean != "" {
			terms = append(terms, `"`+clean+`"`)
		}
	}
	if len(terms) == 0 {
		return `""`
	}
	// FTS5 AND is implicit when terms are space-separated, but explicit AND is clearer.
	return strings.Join(terms, " AND ")
}

func sanitizeFTSToken(t string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '"', '*', '^', '(', ')', '-', '+', '{', '}':
			return -1
		}
		return r
	}, t)
}
