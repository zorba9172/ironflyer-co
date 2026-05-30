package finisher

import "testing"

func TestParseJSONSummaryCoverageUsesTotalWhenPresent(t *testing.T) {
	out := `noise before report
{
  "total": {"lines": {"total": 10, "covered": 8, "pct": 80}},
  "/workspace/app/src/main.ts": {"lines": {"total": 4, "covered": 2, "pct": 50}},
  "/workspace/app/src/lib.ts": {"lines": {"total": 6, "covered": 6, "pct": 100}}
}`

	overall, files, ok := parseJSONSummaryCoverage(out)
	if !ok {
		t.Fatal("expected json-summary coverage to parse")
	}
	if overall != 80 {
		t.Fatalf("overall = %v, want 80", overall)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
	if files[0].Path != "src/main.ts" && files[1].Path != "src/main.ts" {
		t.Fatalf("expected src/main.ts to be shortened in %+v", files)
	}
}

func TestParseJSONSummaryCoverageComputesOverallWithoutTotal(t *testing.T) {
	out := `{
  "/workspace/app/src/a.ts": {"lines": {"total": 5, "covered": 5, "pct": 100}},
  "/workspace/app/src/b.ts": {"lines": {"total": 5, "covered": 2, "pct": 40}}
}`

	overall, files, ok := parseJSONSummaryCoverage(out)
	if !ok {
		t.Fatal("expected coverage without total to parse from files")
	}
	if overall != 70 {
		t.Fatalf("overall = %v, want 70", overall)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
}

func TestParseGoCoverage(t *testing.T) {
	out := `github.com/acme/app/main.go:12:	main		100.0%
github.com/acme/app/main.go:42:	handler		0.0%
github.com/acme/app/internal/usecase.go:9:	Run		75.0%
total:						(statements)		66.7%`

	overall, files, ok := parseGoCoverage(out)
	if !ok {
		t.Fatal("expected go coverage to parse")
	}
	if overall != 66.7 {
		t.Fatalf("overall = %v, want 66.7", overall)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
	if files[0].Path != "github.com/acme/app/main.go" || files[0].LinePct != 50 || files[0].Uncovered != 1 {
		t.Fatalf("unexpected first file coverage: %+v", files[0])
	}
}

func TestExtractJSONObjectIgnoresBracesInsideStrings(t *testing.T) {
	raw := `before {"message":"brace } inside string","nested":{"ok":true}} after {"ignored":true}`
	got := extractJSONObject(raw)
	want := `{"message":"brace } inside string","nested":{"ok":true}}`
	if got != want {
		t.Fatalf("extractJSONObject = %q, want %q", got, want)
	}
}
