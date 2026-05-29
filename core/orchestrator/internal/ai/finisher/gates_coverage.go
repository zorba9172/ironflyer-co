// Package finisher — test-coverage gate.
//
// CoverageGate is the engine half of the product's "test coverage" capability:
// a user building an app on Ironflyer can toggle coverage on
// (Project.Settings.CoverageEnabled), and the finisher then runs that project's
// own suite with coverage instrumentation inside the sandbox, parses the
// report, and surfaces which files are NOT closed (uncovered). The normalized
// report is stored as the "coverage" artifact for the studio Coverage tab; the
// per-file findings flow out as Issues so they also appear in the gate verdict
// stream and drive the repair loop.
//
// As with gates_tests.go, the orchestrator's own "no tests, ever" rule does
// NOT apply here — this measures the USER's generated project in their sandbox,
// never this repository.

package finisher

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/runtime"
)

// CoverageFile is one file's coverage line in the normalized report.
type CoverageFile struct {
	Path string `json:"path"`
	// LinePct is the percentage of the file's lines/statements covered (0..100).
	LinePct float64 `json:"linePct"`
	// Uncovered is the count of uncovered lines/statements in the file.
	Uncovered int `json:"uncovered"`
}

// CoverageReport is the normalized coverage document persisted as the
// domain.ArtifactCoverage artifact and surfaced to the studio.
type CoverageReport struct {
	Enabled     bool           `json:"enabled"`
	OverallPct  float64        `json:"overallPct"`
	MinPct      float64        `json:"minPct"`
	Tool        string         `json:"tool"`
	Files       []CoverageFile `json:"files"`
	GeneratedAt time.Time      `json:"generatedAt"`
}

// coverageToolchain pairs a manifest sentinel with the command that produces a
// machine-readable coverage report and the parser that normalizes it. `cmd`
// always ends by emitting the report content to stdout (Go prints the func
// table directly; the others `cat` their JSON report) so a single Exec capture
// is enough to parse.
type coverageToolchain struct {
	manifest string
	cmd      string
	label    string
	parse    func(out string) (overall float64, files []CoverageFile, ok bool)
}

// orderedCoverageToolchains mirrors the Tests gate's detection order. The first
// matching toolchain runs (coverage is heavier than a plain test run, so we do
// not fan out across every language — the project's primary stack is enough).
var orderedCoverageToolchains = []coverageToolchain{
	{
		manifest: "go.mod",
		cmd:      "go test -coverprofile=/tmp/if_cover.out ./... >/dev/null 2>&1; go tool cover -func=/tmp/if_cover.out 2>/dev/null",
		label:    "go test -cover",
		parse:    parseGoCoverage,
	},
	{
		manifest: "package.json",
		cmd:      "(npx --no-install vitest run --coverage --coverage.reporter=json-summary >/dev/null 2>&1 || npm test -- --coverage --coverageReporters=json-summary >/dev/null 2>&1); cat coverage/coverage-summary.json 2>/dev/null",
		label:    "vitest/jest --coverage",
		parse:    parseJSONSummaryCoverage,
	},
	{
		manifest: "pyproject.toml",
		cmd:      "(command -v pytest >/dev/null 2>&1 && pytest --cov --cov-report=json:/tmp/if_cov.json -q >/dev/null 2>&1 || python -m pytest --cov --cov-report=json:/tmp/if_cov.json -q >/dev/null 2>&1); cat /tmp/if_cov.json 2>/dev/null",
		label:    "pytest --cov",
		parse:    parsePyCoverage,
	},
	{
		manifest: "requirements.txt",
		cmd:      "(command -v pytest >/dev/null 2>&1 && pytest --cov --cov-report=json:/tmp/if_cov.json -q >/dev/null 2>&1 || python -m pytest --cov --cov-report=json:/tmp/if_cov.json -q >/dev/null 2>&1); cat /tmp/if_cov.json 2>/dev/null",
		label:    "pytest --cov",
		parse:    parsePyCoverage,
	},
}

// coverageGateTimeoutSeconds is the per-run timeout. Coverage runs the full
// suite plus instrumentation, so it gets the same 10-minute default as Tests,
// overridable via COVERAGE_GATE_TIMEOUT (seconds).
func coverageGateTimeoutSeconds() int {
	if v := strings.TrimSpace(os.Getenv("COVERAGE_GATE_TIMEOUT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 600
}

// CoverageGate runs the user project's coverage when opted in. It is a no-op
// (zero issues, no work) unless Project.Settings.CoverageEnabled is true, so it
// is safe to keep in the default gate set.
type CoverageGate struct{}

func (CoverageGate) Name() domain.GateName    { return domain.GateCoverage }
func (CoverageGate) RepairAgent() agents.Role { return agents.RoleTester }

func (CoverageGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil || !env.Project.Settings.CoverageEnabled {
		return nil // capability off → no-op
	}
	if !env.HasRuntime() {
		// Honest signal: the toggle is on but there is no sandbox to measure in
		// yet. Info severity so it never blocks; the report stays empty.
		_ = env.Project.SetArtifact(domain.ArtifactCoverage, CoverageReport{
			Enabled: true, MinPct: env.Project.Settings.CoverageMinPct, GeneratedAt: time.Now().UTC(),
		})
		return []domain.Issue{{
			Gate: domain.GateCoverage, Severity: domain.SeverityInfo,
			Message: "coverage is enabled but no workspace runtime is attached yet",
			Hint:    "coverage measures the project once it has a sandbox",
		}}
	}
	report, issues := runCoverage(ctx, env)
	if report != nil {
		_ = env.Project.SetArtifact(domain.ArtifactCoverage, report)
	}
	return issues
}

// runCoverage detects the project's toolchain, runs coverage in the sandbox,
// parses the report, persists it, and returns the not-closed findings.
func runCoverage(ctx context.Context, env *GateEnv) (*CoverageReport, []domain.Issue) {
	minPct := env.Project.Settings.CoverageMinPct
	tc, ok := detectCoverageToolchain(ctx, env)
	if !ok {
		return &CoverageReport{Enabled: true, MinPct: minPct, GeneratedAt: time.Now().UTC()},
			[]domain.Issue{{
				Gate: domain.GateCoverage, Severity: domain.SeverityInfo,
				Message: "coverage is enabled but no known test runner (go.mod, package.json, pyproject.toml, requirements.txt) was detected",
				Hint:    "add a test runner the gate can recognise",
			}}
	}

	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: tc.cmd, TimeoutSeconds: coverageGateTimeoutSeconds(),
	})
	if err != nil {
		return nil, []domain.Issue{{
			Gate: domain.GateCoverage, Severity: domain.SeverityWarning,
			Message: tc.label + ": coverage exec failed: " + err.Error(),
		}}
	}
	if res.TimedOut {
		return nil, []domain.Issue{{
			Gate: domain.GateCoverage, Severity: domain.SeverityWarning,
			Message: tc.label + ": coverage timed out after " + strconv.Itoa(coverageGateTimeoutSeconds()) + "s",
			Hint:    "raise COVERAGE_GATE_TIMEOUT or split the suite",
		}}
	}

	overall, files, parsed := tc.parse(res.Stdout)
	if !parsed {
		return &CoverageReport{Enabled: true, MinPct: minPct, Tool: tc.label, GeneratedAt: time.Now().UTC()},
			[]domain.Issue{{
				Gate: domain.GateCoverage, Severity: domain.SeverityWarning,
				Message: tc.label + ": coverage ran but no report could be parsed",
				Hint:    tail(res.Stdout, 400),
			}}
	}

	sort.Slice(files, func(i, j int) bool { return files[i].LinePct < files[j].LinePct })
	report := &CoverageReport{
		Enabled: true, OverallPct: round1(overall), MinPct: minPct,
		Tool: tc.label, Files: files, GeneratedAt: time.Now().UTC(),
	}

	var issues []domain.Issue
	if minPct > 0 && overall+0.05 < minPct {
		issues = append(issues, domain.Issue{
			Gate: domain.GateCoverage, Severity: domain.SeverityWarning,
			Message: "coverage " + pct(overall) + " is below the " + pct(minPct) + " floor",
			Hint:    "raise coverage on the least-covered files below, or lower the floor in project settings",
		})
	}
	// Name what is not closed: the least-covered files, capped so the verdict
	// stays readable. These ride the gates query so the studio surfaces them.
	for _, f := range files {
		if f.LinePct >= 100 {
			continue
		}
		issues = append(issues, domain.Issue{
			Gate: domain.GateCoverage, Severity: domain.SeverityInfo,
			Message: "not closed: " + f.Path + " (" + pct(f.LinePct) + " covered)",
			Path:    f.Path,
		})
		if len(issues) >= 11 { // 1 floor warning + up to ~10 files
			break
		}
	}
	return report, issues
}

// detectCoverageToolchain returns the first toolchain whose manifest is present
// in the workspace (preferring a live runtime probe, then the in-memory list).
func detectCoverageToolchain(ctx context.Context, env *GateEnv) (coverageToolchain, bool) {
	for _, tc := range orderedCoverageToolchains {
		if runtimeHasFile(ctx, env, tc.manifest) || hasFile(env.Project, tc.manifest) {
			return tc, true
		}
	}
	return coverageToolchain{}, false
}

// --- parsers --------------------------------------------------------------

var reGoFuncCover = regexp.MustCompile(`^(\S+?):\d+:\s+\S+\s+([\d.]+)%$`)
var reGoTotalCover = regexp.MustCompile(`(?m)^total:\s+\(statements\)\s+([\d.]+)%`)

// parseGoCoverage reads `go tool cover -func` output. Per-file coverage is the
// mean of that file's function percentages; uncovered counts the file's
// zero-coverage functions. Overall comes from the `total:` line.
func parseGoCoverage(out string) (float64, []CoverageFile, bool) {
	if strings.TrimSpace(out) == "" {
		return 0, nil, false
	}
	type acc struct {
		sum   float64
		n     int
		zeros int
	}
	byFile := map[string]*acc{}
	order := []string{}
	for _, line := range strings.Split(out, "\n") {
		m := reGoFuncCover.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		p, _ := strconv.ParseFloat(m[2], 64)
		a := byFile[m[1]]
		if a == nil {
			a = &acc{}
			byFile[m[1]] = a
			order = append(order, m[1])
		}
		a.sum += p
		a.n++
		if p == 0 {
			a.zeros++
		}
	}
	tm := reGoTotalCover.FindStringSubmatch(out)
	if tm == nil && len(order) == 0 {
		return 0, nil, false
	}
	var overall float64
	if tm != nil {
		overall, _ = strconv.ParseFloat(tm[1], 64)
	}
	files := make([]CoverageFile, 0, len(order))
	for _, path := range order {
		a := byFile[path]
		avg := 0.0
		if a.n > 0 {
			avg = a.sum / float64(a.n)
		}
		files = append(files, CoverageFile{Path: path, LinePct: round1(avg), Uncovered: a.zeros})
	}
	return overall, files, true
}

// jsonSummary is the istanbul / vitest json-summary shape (also emitted by
// jest's json-summary reporter). Per-file keys are absolute paths; "total" is
// the aggregate. We only need the line metric.
type jsonSummaryEntry struct {
	Lines struct {
		Pct     float64 `json:"pct"`
		Total   int     `json:"total"`
		Covered int     `json:"covered"`
	} `json:"lines"`
}

func parseJSONSummaryCoverage(out string) (float64, []CoverageFile, bool) {
	raw := extractJSONObject(out)
	if raw == "" {
		return 0, nil, false
	}
	var m map[string]jsonSummaryEntry
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return 0, nil, false
	}
	total, hasTotal := m["total"]
	files := make([]CoverageFile, 0, len(m))
	totalLines := 0
	coveredLines := 0
	for path, e := range m {
		if path == "total" {
			continue
		}
		totalLines += e.Lines.Total
		coveredLines += e.Lines.Covered
		files = append(files, CoverageFile{
			Path: shortPath(path), LinePct: round1(e.Lines.Pct),
			Uncovered: e.Lines.Total - e.Lines.Covered,
		})
	}
	if !hasTotal && len(files) == 0 {
		return 0, nil, false
	}
	if !hasTotal && totalLines > 0 {
		return float64(coveredLines) / float64(totalLines) * 100, files, true
	}
	return total.Lines.Pct, files, true
}

// pyCoverage is the coverage.py JSON report shape (coverage json / pytest-cov
// --cov-report=json).
type pyCoverage struct {
	Totals struct {
		PercentCovered float64 `json:"percent_covered"`
	} `json:"totals"`
	Files map[string]struct {
		Summary struct {
			PercentCovered float64 `json:"percent_covered"`
			MissingLines   int     `json:"missing_lines"`
		} `json:"summary"`
	} `json:"files"`
}

func parsePyCoverage(out string) (float64, []CoverageFile, bool) {
	raw := extractJSONObject(out)
	if raw == "" {
		return 0, nil, false
	}
	var r pyCoverage
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return 0, nil, false
	}
	if len(r.Files) == 0 {
		return 0, nil, false
	}
	files := make([]CoverageFile, 0, len(r.Files))
	for path, f := range r.Files {
		files = append(files, CoverageFile{
			Path: shortPath(path), LinePct: round1(f.Summary.PercentCovered), Uncovered: f.Summary.MissingLines,
		})
	}
	return r.Totals.PercentCovered, files, true
}

// --- small helpers --------------------------------------------------------

// extractJSONObject returns the first balanced top-level JSON object in s, so a
// parser tolerates leading tool chatter before the `cat`-ed report.
func extractJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case esc:
			esc = false
		case c == '\\' && inStr:
			esc = true
		case c == '"':
			inStr = !inStr
		case inStr:
			// skip
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// shortPath trims a leading absolute workspace prefix so the studio shows
// repo-relative paths. Best-effort: drops everything up to the last "/src/" or
// keeps the trailing path segments when no marker is present.
func shortPath(p string) string {
	p = strings.TrimSpace(p)
	for _, marker := range []string{"/src/", "/app/", "/internal/", "/pkg/"} {
		if i := strings.LastIndex(p, marker); i >= 0 {
			return strings.TrimPrefix(p[i+1:], "")
		}
	}
	if strings.HasPrefix(p, "./") {
		return p[2:]
	}
	return p
}

func round1(f float64) float64 { return float64(int64(f*10+0.5)) / 10 }

func pct(f float64) string { return strconv.FormatFloat(round1(f), 'f', -1, 64) + "%" }
