package appsec

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"ironflyer/core/orchestrator/internal/operations/runtime"
)

type TrufflehogScanner struct{}

func (TrufflehogScanner) ID() string { return "trufflehog" }

func (TrufflehogScanner) Supports(inv Inventory) bool { return inv.RuntimeEnabled }

func (s TrufflehogScanner) Scan(ctx context.Context, target Target, _ Inventory) ([]Finding, error) {
	cmd := "command -v trufflehog >/dev/null 2>&1 && " +
		"tmp=${TMPDIR:-/tmp}/ironflyer-trufflehog-exclude.txt; " +
		"printf 'node_modules\\n.git\\n' > \"$tmp\"; " +
		"trufflehog filesystem --json --no-update --exclude-paths \"$tmp\" . 2>/dev/null || true"
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 180,
	})
	if err != nil || strings.TrimSpace(res.Stdout) == "" {
		return nil, nil
	}
	var out []Finding
	for _, line := range strings.Split(res.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var doc struct {
			DetectorName   string `json:"DetectorName"`
			Verified       bool   `json:"Verified"`
			SourceMetadata struct {
				Data struct {
					Filesystem struct {
						File string `json:"file"`
						Line int    `json:"line,omitempty"`
					} `json:"Filesystem"`
				} `json:"Data"`
			} `json:"SourceMetadata"`
		}
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			continue
		}
		sev := SeverityHigh
		if doc.Verified {
			sev = SeverityCritical
		}
		summary := "trufflehog: " + doc.DetectorName
		if doc.Verified {
			summary += " (verified live credential)"
		}
		out = append(out, Finding{
			Tool:        s.ID(),
			Category:    CategorySecrets,
			Severity:    sev,
			RuleID:      "trufflehog-" + slug(doc.DetectorName),
			Path:        cleanPath(doc.SourceMetadata.Data.Filesystem.File),
			Line:        doc.SourceMetadata.Data.Filesystem.Line,
			Summary:     summary,
			Remediation: "rotate the credential and remove it from history",
			Verified:    doc.Verified,
		})
	}
	return out, nil
}

type GitleaksScanner struct{}

func (GitleaksScanner) ID() string { return "gitleaks" }

func (GitleaksScanner) Supports(inv Inventory) bool { return inv.RuntimeEnabled }

func (s GitleaksScanner) Scan(ctx context.Context, target Target, _ Inventory) ([]Finding, error) {
	cmd := "command -v gitleaks >/dev/null 2>&1 && " +
		"gitleaks detect --no-git --no-banner --redact " +
		"--report-format json --report-path /dev/stdout . 2>/dev/null || true"
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 120,
	})
	if err != nil || strings.TrimSpace(res.Stdout) == "" {
		return nil, nil
	}
	var findings []struct {
		Description string `json:"Description"`
		File        string `json:"File"`
		StartLine   int    `json:"StartLine"`
		RuleID      string `json:"RuleID"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &findings); err != nil {
		return nil, nil
	}
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		out = append(out, Finding{
			Tool:        s.ID(),
			Category:    CategorySecrets,
			Severity:    SeverityCritical,
			RuleID:      "gitleaks-" + slug(f.RuleID),
			Path:        cleanPath(f.File),
			Line:        f.StartLine,
			Summary:     "gitleaks: " + f.RuleID + " - " + f.Description,
			Remediation: "remove the secret, rotate it upstream, and rewrite history",
		})
	}
	return out, nil
}

type SemgrepScanner struct{}

func (SemgrepScanner) ID() string { return "semgrep" }

func (SemgrepScanner) Supports(inv Inventory) bool { return inv.RuntimeEnabled }

func (s SemgrepScanner) Scan(ctx context.Context, target Target, _ Inventory) ([]Finding, error) {
	cmd := "command -v semgrep >/dev/null 2>&1 && semgrep --config=auto --json --quiet --error --exclude=node_modules --exclude=.git --timeout=60 . 2>/dev/null"
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 120,
	})
	if err != nil || res.TimedOut || strings.TrimSpace(res.Stdout) == "" {
		return nil, nil
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
		return nil, nil
	}
	out := make([]Finding, 0, len(doc.Results))
	for _, r := range doc.Results {
		msg := r.Extra.Message
		if len(r.Extra.Metadata.Owasp) > 0 {
			msg += " (" + strings.Join(r.Extra.Metadata.Owasp, ", ") + ")"
		}
		out = append(out, Finding{
			Tool:        s.ID(),
			Category:    CategoryCode,
			Severity:    mapSemgrepSeverity(r.Extra.Severity),
			RuleID:      "semgrep-" + slug(r.CheckID),
			Path:        cleanPath(r.Path),
			Line:        r.Start.Line,
			Summary:     msg,
			Remediation: "review and remediate per the semgrep rule guidance",
		})
	}
	return out, nil
}

func mapSemgrepSeverity(s string) Severity {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "ERROR":
		return SeverityCritical
	case "WARNING":
		return SeverityHigh
	case "INFO":
		return SeverityMedium
	}
	return SeverityMedium
}

type GovulncheckScanner struct{}

func (GovulncheckScanner) ID() string { return "govulncheck" }

func (GovulncheckScanner) Supports(inv Inventory) bool { return inv.RuntimeEnabled }

func (s GovulncheckScanner) Scan(ctx context.Context, target Target, inv Inventory) ([]Finding, error) {
	paths := inv.GoModPaths
	if len(paths) == 0 {
		paths = []string{"go.mod"}
	}
	var out []Finding
	for _, modPath := range paths {
		if !workspaceHasFile(ctx, target, modPath) {
			continue
		}
		cwd := serviceRoot(modPath)
		cmd := "command -v govulncheck >/dev/null 2>&1 && govulncheck -json ./... 2>/dev/null"
		res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
			Shell: cmd, Cwd: cwd, TimeoutSeconds: 180,
		})
		if err != nil || res.TimedOut || strings.TrimSpace(res.Stdout) == "" {
			continue
		}
		out = append(out, parseGovulncheckOutput(s.ID(), cwd, res.Stdout)...)
	}
	return out, nil
}

func parseGovulncheckOutput(toolID, cwd, stdout string) []Finding {
	var out []Finding
	for _, line := range strings.Split(stdout, "\n") {
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
		path := t.Package
		if cwd != "" && cwd != "." && path != "" {
			path = cwd + "/" + path
		}
		out = append(out, Finding{
			Tool:        toolID,
			Category:    CategoryDeps,
			Severity:    SeverityCritical,
			RuleID:      "govulncheck-" + strings.ToLower(doc.Finding.OSV),
			Path:        path,
			Package:     t.Module,
			Summary:     msg,
			Remediation: "upgrade the affected module; govulncheck only reports call-graph-reachable CVEs",
		})
	}
	return out
}

type GoModuleDeprecationScanner struct{}

func (GoModuleDeprecationScanner) ID() string { return "go-list-deprecations" }

func (GoModuleDeprecationScanner) Supports(inv Inventory) bool {
	return inv.RuntimeEnabled && inv.HasGoMod
}

func (s GoModuleDeprecationScanner) Scan(ctx context.Context, target Target, inv Inventory) ([]Finding, error) {
	paths := inv.GoModPaths
	if len(paths) == 0 {
		paths = []string{"go.mod"}
	}
	var out []Finding
	for _, modPath := range paths {
		if !workspaceHasFile(ctx, target, modPath) {
			continue
		}
		cwd := serviceRoot(modPath)
		cmd := "command -v go >/dev/null 2>&1 && go list -m -u -json all 2>/dev/null"
		res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
			Shell: cmd, Cwd: cwd, TimeoutSeconds: 120,
		})
		if err != nil || res.TimedOut || strings.TrimSpace(res.Stdout) == "" {
			continue
		}
		out = append(out, parseGoListDeprecations(s.ID(), modPath, res.Stdout)...)
	}
	return out, nil
}

func parseGoListDeprecations(toolID, modPath, stdout string) []Finding {
	dec := json.NewDecoder(strings.NewReader(stdout))
	var out []Finding
	for {
		var doc struct {
			Path       string `json:"Path"`
			Version    string `json:"Version,omitempty"`
			Main       bool   `json:"Main,omitempty"`
			Deprecated string `json:"Deprecated,omitempty"`
		}
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			break
		}
		if doc.Main || strings.TrimSpace(doc.Deprecated) == "" {
			continue
		}
		pkg := doc.Path
		if doc.Version != "" {
			pkg += "@" + doc.Version
		}
		out = append(out, Finding{
			Tool:        toolID,
			Category:    CategoryDeps,
			Severity:    SeverityMedium,
			RuleID:      "deps-go-deprecated-module",
			Path:        cleanPath(modPath),
			Package:     pkg,
			Summary:     "Go module is deprecated: " + pkg,
			Remediation: strings.TrimSpace(doc.Deprecated),
			Metadata: map[string]string{
				"ecosystem":  "go",
				"dependency": doc.Path,
				"version":    doc.Version,
			},
		})
	}
	return out
}

type NpmAuditScanner struct{}

func (NpmAuditScanner) ID() string { return "npm-audit" }

func (NpmAuditScanner) Supports(inv Inventory) bool { return inv.RuntimeEnabled }

func (s NpmAuditScanner) Scan(ctx context.Context, target Target, inv Inventory) ([]Finding, error) {
	paths := inv.PackageLockPaths
	if len(paths) == 0 {
		paths = []string{"package-lock.json"}
	}
	var out []Finding
	for _, lockPath := range paths {
		if !workspaceHasFile(ctx, target, lockPath) {
			continue
		}
		cwd := serviceRoot(lockPath)
		cmd := "command -v npm >/dev/null 2>&1 && npm audit --json --omit=dev 2>/dev/null || true"
		res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
			Shell: cmd, Cwd: cwd, TimeoutSeconds: 120,
		})
		if err != nil || strings.TrimSpace(res.Stdout) == "" {
			continue
		}
		out = append(out, parseNpmAuditOutput(s.ID(), lockPath, res.Stdout)...)
	}
	return out, nil
}

func parseNpmAuditOutput(toolID, lockPath, stdout string) []Finding {
	var doc struct {
		Vulnerabilities map[string]struct {
			Severity string `json:"severity"`
			Range    string `json:"range,omitempty"`
		} `json:"vulnerabilities"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		return nil
	}
	out := make([]Finding, 0, len(doc.Vulnerabilities))
	for name, v := range doc.Vulnerabilities {
		sev := strings.ToLower(v.Severity)
		if sev != "high" && sev != "critical" {
			continue
		}
		mapped := SeverityHigh
		if sev == "critical" {
			mapped = SeverityCritical
		}
		msg := "npm advisory: " + name
		if v.Range != "" {
			msg += " " + v.Range
		}
		out = append(out, Finding{
			Tool:        toolID,
			Category:    CategoryDeps,
			Severity:    mapped,
			RuleID:      "npm-" + slug(name),
			Path:        cleanPath(lockPath),
			Package:     name,
			Summary:     msg,
			Remediation: "run npm audit fix or pin a patched range",
		})
	}
	return out
}

func workspaceHasFile(ctx context.Context, target Target, path string) bool {
	if !target.HasRuntime() {
		return false
	}
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: "test -f " + shellQuote(path), TimeoutSeconds: 10,
	})
	return err == nil && res.ExitCode == 0
}
