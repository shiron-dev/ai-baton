package agent

import "testing"

func TestJSONParserWrappedFindings(t *testing.T) {
	raw := `{"findings":[{"file_path":"main.go","start_line":1,"end_line":1,"title":"t","body":"b","severity":"info","confidence":0.8}]}`
	findings, err := (&JSONParser{}).Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("len(findings) = %d, want 1", len(findings))
	}
	if findings[0].FilePath != "main.go" {
		t.Fatalf("FilePath = %q, want main.go", findings[0].FilePath)
	}
}
