// Package finisher — real security scanners. Where the original Security
// gate relied on regex credential detection, this layer adds three
// industry-standard tools that catch the things regex never will:
//
//   - **semgrep**  : SAST patterns for OWASP top-10 (injection, deserialization,
//                    unsafe crypto, hardcoded auth bypass, …). 1000+ rules
//                    out-of-the-box via `--config=auto`.
//   - **govulncheck**: Go-specific CVE scanner that walks the module graph
//                    AND the call graph — only flags vulnerabilities the
//                    code actually reaches, so false positives are low.
//   - **npm audit** : Reads `package-lock.json` and surfaces upstream
//                    advisories. Cheap to run when the lockfile already
//                    exists.
//
// Every scanner runs inside the workspace via env.Runtime.Exec so the host
// is never exposed. If the binary isn't installed (exit 127 / "command not
// found") we degrade silently — the gate stays useful without forcing a
// specific toolchain on every workspace image.
package finisher

import (
	"context"
	"encoding/json"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/runtime"
)

// runSecurityScanners is invoked by SecurityGate.Check after the in-memory
// regex pass. It batches all three scanners; each one is best-effort and
// contributes its findings independently.
func runSecurityScanners(ctx context.Context, env *GateEnv) []domain.Issue {
	if !env.HasRuntime() {
		return nil
	}
	var issues []domain.Issue
	issues = append(issues, runSemgrep(ctx, env)...)
	issues = append(issues, runGovulncheck(ctx, env)...)
	issues = append(issues, runNpmAudit(ctx, env)...)
	return issues
}

// runSemgrep executes `semgrep --config=auto --json --quiet` against the
// workspace root. Returns one Issue per finding above INFO severity. A
// missing binary is silent; a non-zero exit with usable JSON still parses.
func runSemgrep(ctx context.Context, env *GateEnv) []domain.Issue {
	cmd := "command -v semgrep >/dev/null 2>&1 && semgrep --config=auto --json --quiet --error --exclude=node_modules --exclude=.git --timeout=60 . 2>/dev/null"
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 120,
	})
	if err != nil || res.TimedOut {
		return nil
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return nil
	}
	type semgrepOut struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			Extra struct {
				Severity string `json:"severity"`
				Message  string `json:"message"`
				Metadata struct {
					Owasp []string `json:"owasp,omitempty"`
				} `json:"metadata"`
			} `json:"extra"`
		} `json:"results"`
	}
	var doc semgrepOut
	if err := json.Unmarshal([]byte(res.Stdout), &doc); err != nil {
		return nil
	}
	out := make([]domain.Issue, 0, len(doc.Results))
	for _, r := range doc.Results {
		sev := mapSemgrepSeverity(r.Extra.Severity)
		msg := r.Extra.Message
		if r.CheckID != "" {
			msg = r.CheckID + ": " + msg
		}
		if len(r.Extra.Metadata.Owasp) > 0 {
			msg += " (" + strings.Join(r.Extra.Metadata.Owasp, ", ") + ")"
		}
		path := r.Path
		if r.Start.Line > 0 {
			path += ":" + itoaPositive(r.Start.Line)
		}
		out = append(out, domain.Issue{
			Gate:     domain.GateSecurity,
			Severity: sev,
			Message:  msg,
			Path:     path,
			Hint:     "semgrep rule — review and remediate per the rule docs",
		})
	}
	return out
}

func mapSemgrepSeverity(s string) domain.Severity {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "ERROR":
		return domain.SeverityCritical
	case "WARNING":
		return domain.SeverityError
	case "INFO":
		return domain.SeverityWarning
	}
	return domain.SeverityWarning
}

// runGovulncheck executes `govulncheck -json ./...` when the workspace has
// a go.mod. Each Vulnerability with at least one CallSite becomes an Issue.
// We surface only call-graph-reachable findings to avoid drowning the gate
// in transitive-dependency noise the user can't act on.
func runGovulncheck(ctx context.Context, env *GateEnv) []domain.Issue {
	if !workspaceHasFile(ctx, env, "go.mod") {
		return nil
	}
	cmd := "command -v govulncheck >/dev/null 2>&1 && govulncheck -json ./... 2>/dev/null"
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 180,
	})
	if err != nil || res.TimedOut || strings.TrimSpace(res.Stdout) == "" {
		return nil
	}
	// govulncheck emits a stream of JSON objects (one per line). We parse
	// each one and gather Finding entries that include a Trace (call path
	// from the user's code).
	var out []domain.Issue
	for _, line := range strings.Split(res.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var doc struct {
			Finding *struct {
				OSV          string `json:"osv"`
				FixedVersion string `json:"fixed_version,omitempty"`
				Trace        []struct {
					Function string `json:"function,omitempty"`
					Module   string `json:"module,omitempty"`
					Package  string `json:"package,omitempty"`
				} `json:"trace,omitempty"`
			} `json:"finding,omitempty"`
		}
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			continue
		}
		if doc.Finding == nil || len(doc.Finding.Trace) == 0 {
			continue
		}
		t := doc.Finding.Trace[0]
		msg := doc.Finding.OSV
		if t.Module != "" {
			msg += " in " + t.Module
		}
		if doc.Finding.FixedVersion != "" {
			msg += " (fixed in " + doc.Finding.FixedVersion + ")"
		}
		out = append(out, domain.Issue{
			Gate:     domain.GateSecurity,
			Severity: domain.SeverityCritical,
			Message:  msg,
			Path:     t.Package,
			Hint:     "upgrade the affected module — govulncheck only reports call-graph-reachable CVEs",
		})
	}
	return out
}

// runNpmAudit executes `npm audit --json --omit=dev` when the workspace has
// a package-lock.json. Surfaces vulnerabilities at "high" or "critical"
// only — moderates and lows are noisy and rarely actionable in an MVP.
func runNpmAudit(ctx context.Context, env *GateEnv) []domain.Issue {
	if !workspaceHasFile(ctx, env, "package-lock.json") {
		return nil
	}
	cmd := "command -v npm >/dev/null 2>&1 && npm audit --json --omit=dev 2>/dev/null || true"
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 120,
	})
	if err != nil || strings.TrimSpace(res.Stdout) == "" {
		return nil
	}
	var doc struct {
		Vulnerabilities map[string]struct {
			Severity string   `json:"severity"`
			Via      []any    `json:"via"`
			FixAvail any      `json:"fixAvailable,omitempty"`
			Range    string   `json:"range,omitempty"`
			Effects  []string `json:"effects,omitempty"`
		} `json:"vulnerabilities"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &doc); err != nil {
		return nil
	}
	out := make([]domain.Issue, 0, len(doc.Vulnerabilities))
	for name, v := range doc.Vulnerabilities {
		sev := strings.ToLower(v.Severity)
		if sev != "high" && sev != "critical" {
			continue
		}
		mapped := domain.SeverityError
		if sev == "critical" {
			mapped = domain.SeverityCritical
		}
		msg := "npm advisory: " + name
		if v.Range != "" {
			msg += " " + v.Range
		}
		out = append(out, domain.Issue{
			Gate:     domain.GateSecurity,
			Severity: mapped,
			Message:  msg,
			Path:     "package-lock.json",
			Hint:     "run `npm audit fix` (or pin a patched range) — high+ severity",
		})
	}
	return out
}

// workspaceHasFile is the runtime equivalent of an os.Stat — a one-off
// `test -f <path>` exec. Returns false on any error so callers degrade
// safely when the workspace is unavailable.
func workspaceHasFile(ctx context.Context, env *GateEnv, path string) bool {
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: "test -f " + shellQuote(path), TimeoutSeconds: 10,
	})
	if err != nil {
		return false
	}
	return res.ExitCode == 0
}
