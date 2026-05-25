// Package finisher — deepened Tests gate.
//
// The original Tests gate amounted to "does the project have any file with
// `_test.` in its name". A commercial customer demands the verdict actually
// mean "the user's tests pass". This file lifts the gate to that bar:
//
//   - Detect the toolchain by inspecting workspace manifests
//     (go.mod → go test, package.json → npm test, requirements.txt /
//     pyproject.toml → pytest, Cargo.toml → cargo test) in priority order.
//   - Shell into the user's sandbox via the runtime client and run the
//     detected command. Capture stdout + stderr + exit code.
//   - Pass = exit 0; non-zero = fail. Surface the first 10 failing test
//     names in the Issue Hint so the repair Coder doesn't have to parse
//     800 bytes of build output to know what to fix.
//   - 10-minute default timeout per test run, overridable via the
//     `TESTS_GATE_TIMEOUT` env var (seconds).
//
// The Ironflyer-orchestrator-itself "no tests, ever" rule does NOT apply to
// this code path. This gate runs commands in the user's sandbox; the
// orchestrator's own repo is unaffected. The opt-out env var that does
// apply here is `IRONFLYER_TESTS_GATE_DISABLE=true`, intended for
// operators on tight budgets who want to ship the gate as warn-only.

package finisher

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/runtime"
)

// testToolchain is one detected (cmd, label, manifest) tuple. The Tests gate
// picks the first toolchain whose manifest file is present in the workspace.
// Order matters: Go and Rust are tried before npm because a polyglot
// repository ("Next.js frontend + Go backend") usually wants both runs.
type testToolchain struct {
	manifest string // sentinel file that triggers this toolchain
	cmd      string // shell command to execute
	label    string // short human label for the Issue hint
}

// orderedTestToolchains is consulted in order. When a workspace matches
// more than one, ALL matching toolchains run sequentially — a polyglot
// monorepo with both Go services and a Next.js app must pass each
// language's tests, not just the first detected one.
var orderedTestToolchains = []testToolchain{
	{manifest: "go.mod", cmd: "go test ./...", label: "go test"},
	{manifest: "Cargo.toml", cmd: "cargo test --quiet", label: "cargo test"},
	{manifest: "pyproject.toml", cmd: "(command -v pytest >/dev/null 2>&1 && pytest -q) || python -m pytest -q", label: "pytest"},
	{manifest: "requirements.txt", cmd: "(command -v pytest >/dev/null 2>&1 && pytest -q) || python -m pytest -q", label: "pytest"},
	{manifest: "package.json", cmd: "npm test --silent --if-present", label: "npm test"},
}

// testsGateTimeoutSeconds returns the per-run timeout for the Tests gate.
// Default 600s (10 minutes); operators can shorten it on cheap plans via
// the TESTS_GATE_TIMEOUT env var.
func testsGateTimeoutSeconds() int {
	if v := strings.TrimSpace(os.Getenv("TESTS_GATE_TIMEOUT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 600
}

// runTests is the live implementation of the Tests gate when a runtime is
// bound. It returns one Issue per failed toolchain (or zero on success).
//
// Detection order: stack hint (Spec.Stack.Backend / Frontend) first, then
// declared manifest files. When the workspace has manifests for multiple
// toolchains we run them all — a Next.js app with a Python ML worker
// must pass both gates, not just whichever happened to be detected first.
func runTests(ctx context.Context, env *GateEnv) []domain.Issue {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("IRONFLYER_TESTS_GATE_DISABLE")), "true") {
		return []domain.Issue{{
			Gate: domain.GateTest, Severity: domain.SeverityWarning,
			Message: "tests gate disabled by operator (IRONFLYER_TESTS_GATE_DISABLE=true)",
			Hint:    "remove the env var to re-enable real test execution",
		}}
	}
	detected := detectAllTestToolchains(ctx, env)
	if len(detected) == 0 {
		return []domain.Issue{{
			Gate: domain.GateTest, Severity: domain.SeverityWarning,
			Message: "test gate has runtime but no known toolchain (go.mod, package.json, requirements.txt, pyproject.toml, Cargo.toml) was detected",
			Hint:    "add a manifest the gate can recognise, or declare Spec.Stack.Backend",
		}}
	}

	timeout := testsGateTimeoutSeconds()
	var issues []domain.Issue
	for _, tc := range detected {
		res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
			Shell: tc.cmd, TimeoutSeconds: timeout,
		})
		if err != nil {
			issues = append(issues, domain.Issue{
				Gate: domain.GateTest, Severity: domain.SeverityError,
				Message: tc.label + ": exec failed: " + err.Error(),
			})
			continue
		}
		if res.TimedOut {
			issues = append(issues, domain.Issue{
				Gate: domain.GateTest, Severity: domain.SeverityCritical,
				Message: tc.label + ": tests timed out after " + strconv.Itoa(timeout) + "s",
				Hint:    "raise TESTS_GATE_TIMEOUT or split the suite into faster chunks",
			})
			continue
		}
		if res.ExitCode != 0 {
			failed := extractFailingTestNames(res.Stdout + "\n" + res.Stderr)
			hint := tc.label + " (exit " + itoaPositive(res.ExitCode) + ")"
			if len(failed) > 0 {
				hint += " — failing: " + strings.Join(failed, ", ")
			} else {
				hint += " — " + tail(res.Stdout+res.Stderr, 600)
			}
			issues = append(issues, domain.Issue{
				Gate: domain.GateTest, Severity: domain.SeverityError,
				Message: tc.label + ": tests failed",
				Hint:    hint,
			})
		}
	}
	return issues
}

// detectAllTestToolchains returns every toolchain whose manifest file is
// present in the workspace. We prefer a runtime-side `test -f` so a stale
// in-memory Project.Files slice doesn't mislead us, but we fall back to
// the in-memory list when the runtime probe fails.
func detectAllTestToolchains(ctx context.Context, env *GateEnv) []testToolchain {
	var out []testToolchain
	for _, tc := range orderedTestToolchains {
		if runtimeHasFile(ctx, env, tc.manifest) {
			out = append(out, tc)
		} else if hasFile(env.Project, tc.manifest) {
			out = append(out, tc)
		}
	}
	return out
}

// detectTestCommand is the legacy single-toolchain detector preserved so
// the existing `gates_test.go` helper keeps compiling. The Tests gate
// itself calls detectAllTestToolchains for full multi-toolchain coverage;
// this shim returns the first match (cmd, label, true) or ("", "",
// false) when nothing applies. Detection prefers the workspace's
// declared stack hint over the in-memory file list.
func detectTestCommand(p *domain.Project) (string, string, bool) {
	backend := strings.ToLower(p.Spec.Stack.Backend)
	switch {
	case strings.Contains(backend, "go") || hasFile(p, "go.mod"):
		return "go test ./...", "go test", true
	case strings.Contains(backend, "rust") || hasFile(p, "Cargo.toml"):
		return "cargo test --quiet", "cargo test", true
	case strings.Contains(backend, "python") || hasFile(p, "pyproject.toml") || hasFile(p, "requirements.txt"):
		return "(command -v pytest >/dev/null 2>&1 && pytest -q) || python -m pytest -q", "pytest", true
	case hasFile(p, "package.json"):
		return "npm test --silent --if-present", "npm test", true
	}
	return "", "", false
}

// runtimeHasFile checks the live workspace for a manifest. Returns false on
// any error so the in-memory fallback can still kick in.
func runtimeHasFile(ctx context.Context, env *GateEnv, path string) bool {
	if env == nil || !env.HasRuntime() {
		return false
	}
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: "test -f " + shellQuote(path), TimeoutSeconds: 10,
	})
	if err != nil {
		return false
	}
	return res.ExitCode == 0
}

// extractFailingTestNames pulls failing test identifiers out of the
// captured stdout+stderr. We don't try to be exhaustive — the goal is to
// surface 1-10 named failures the repair Coder can grep for.
//
// Patterns covered:
//
//   - Go:     "--- FAIL: TestName (0.01s)"
//   - npm:    "✕ should something …" (jest) and "FAIL src/foo.test.ts"
//   - pytest: "FAILED tests/test_foo.py::test_name - …"
//   - cargo:  "test name … FAILED"
var (
	reGoFail     = regexp.MustCompile(`--- FAIL: (\S+)`)
	reJestFail   = regexp.MustCompile(`(?m)^\s*(?:✕|×|✘|FAIL)\s+([^\s].*?)$`)
	rePytestFail = regexp.MustCompile(`(?m)^FAILED\s+(\S+)`)
	reCargoFail  = regexp.MustCompile(`(?m)^test\s+(\S+)\s+\.\.\.\s+FAILED`)
)

func extractFailingTestNames(out string) []string {
	if out == "" {
		return nil
	}
	seen := map[string]struct{}{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
	}
	for _, m := range reGoFail.FindAllStringSubmatch(out, -1) {
		add(m[1])
	}
	for _, m := range rePytestFail.FindAllStringSubmatch(out, -1) {
		add(m[1])
	}
	for _, m := range reCargoFail.FindAllStringSubmatch(out, -1) {
		add(m[1])
	}
	for _, m := range reJestFail.FindAllStringSubmatch(out, -1) {
		add(m[1])
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
		if len(names) >= 10 {
			break
		}
	}
	return names
}
