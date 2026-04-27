package schema

// Evidence is a supporting piece of data backing a Finding.
type Evidence struct {
	Kind    string `json:"kind"`    // "code_snippet", "doc_ref", "test_ref"
	Content string `json:"content"`
	Source  string `json:"source"`
}

// Finding is a single review comment candidate produced by an AI agent.
type Finding struct {
	ID             string     `json:"id"`
	FilePath       string     `json:"file_path"`
	StartLine      int        `json:"start_line"`
	EndLine        int        `json:"end_line"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	SuggestedPatch string     `json:"suggested_patch,omitempty"`
	Severity       string     `json:"severity"` // "error", "warning", "info"
	Confidence     float64    `json:"confidence"`
	Labels         []string   `json:"labels,omitempty"`
	Evidence       []Evidence `json:"evidence,omitempty"`
}
