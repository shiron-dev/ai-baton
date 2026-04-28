package review

import (
	"testing"

	"github.com/shiron-dev/ai-baton/internal/schema"
)

func TestNormalizeInlineLocation(t *testing.T) {
	patch := `@@ -1,3 +1,4 @@
 package main
+func newThing() {}
 func existing() {}
-func oldThing() {}
`

	tests := []struct {
		name      string
		finding   schema.Finding
		wantOK    bool
		wantStart int
		wantEnd   int
	}{
		{
			name:      "keeps changed line",
			finding:   schema.Finding{FilePath: "main.go", StartLine: 2, EndLine: 2},
			wantOK:    true,
			wantStart: 2,
			wantEnd:   2,
		},
		{
			name:      "falls back to start line when end is outside patch",
			finding:   schema.Finding{FilePath: "main.go", StartLine: 3, EndLine: 99},
			wantOK:    true,
			wantStart: 3,
			wantEnd:   3,
		},
		{
			name:    "rejects line outside patch",
			finding: schema.Finding{FilePath: "main.go", StartLine: 99, EndLine: 99},
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.finding
			ok := normalizeInlineLocation(&got, patch)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.StartLine != tt.wantStart || got.EndLine != tt.wantEnd {
				t.Fatalf("location = %d:%d, want %d:%d", got.StartLine, got.EndLine, tt.wantStart, tt.wantEnd)
			}
		})
	}
}
