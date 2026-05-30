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
	cmd := "if command -v trufflehog >/dev/null 2>&1; then " +
		"tmp=${TMPDIR:-/tmp}/ironflyer-trufflehog-exclude.txt; " +
		"printf 'node_modules\\n.git\\n' > \"$tmp\"; " +
		"trufflehog filesystem --json --no-update --exclude-paths \"$tmp\" .; fi"
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 180,
	})
	if err := scannerTransportError(s.ID(), res, err); err != nil {
		return nil, err
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return nil, scannerExitError(s.ID(), res)
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
			return nil, scannerParseError(s.ID(), err)
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
	if len(out) == 0 {
		return nil, scannerExitError(s.ID(), res)
	}
	return out, nil
}

type GitleaksScanner struct{}

func (GitleaksScanner) ID() string { return "gitleaks" }

func (GitleaksScanner) Supports(inv Inventory) bool { return inv.RuntimeEnabled }

func (s GitleaksScanner) Scan(ctx context.Context, target Target, _ Inventory) ([]Finding, error) {
	cmd := "if command -v gitleaks >/dev/null 2>&1; then " +
		"gitleaks detect --no-git --no-banner --redact " +
		"--report-format json --report-path /dev/stdout .; fi"
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 120,
	})
	if err := scannerTransportError(s.ID(), res, err); err != nil {
		return nil, err
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return nil, scannerExitError(s.ID(), res)
	}
	var findings []struct {
		Description string `json:"Description"`
		File        string `json:"File"`
		StartLine   int    `json:"StartLine"`
		RuleID      string `json:"RuleID"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &findings); err != nil {
		return nil, scannerParseError(s.ID(), err)
	}
	if len(findings) == 0 {
		return nil, scannerExitError(s.ID(), res)
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
	cmd := "if command -v semgrep >/dev/null 2>&1; then semgrep --config=auto --json --quiet --error --exclude=node_modules --exclude=.git --timeout=60 .; fi"
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 120,
	})
	if err := scannerTransportError(s.ID(), res, err); err != nil {
		return nil, err
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return nil, scannerExitError(s.ID(), res)
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
		return nil, scannerParseError(s.ID(), err)
	}
	if len(doc.Results) == 0 {
		return nil, scannerExitError(s.ID(), res)
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

type OSVScannerCLI struct{}

func (OSVScannerCLI) ID() string { return "osv-scanner" }

func (OSVScannerCLI) Supports(inv Inventory) bool {
	return inv.RuntimeEnabled && (inv.HasGoMod || inv.HasPackageLock || inv.HasPNPMLock || inv.HasYarnLock || inv.HasPythonDeps)
}

func (s OSVScannerCLI) Scan(ctx context.Context, target Target, _ Inventory) ([]Finding, error) {
	cmd := "if command -v osv-scanner >/dev/null 2>&1; then " +
		"osv-scanner --format json --recursive --skip-git .; fi"
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 180,
	})
	if err := scannerTransportError(s.ID(), res, err); err != nil {
		return nil, err
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return nil, scannerExitError(s.ID(), res)
	}
	if !json.Valid([]byte(res.Stdout)) {
		return nil, scannerParseError(s.ID(), json.Unmarshal([]byte(res.Stdout), &struct{}{}))
	}
	out := parseOSVScannerOutput(s.ID(), res.Stdout)
	if len(out) == 0 {
		return nil, scannerExitError(s.ID(), res)
	}
	return out, nil
}

func parseOSVScannerOutput(toolID, stdout string) []Finding {
	var doc struct {
		Results []struct {
			Source struct {
				Path string `json:"path"`
			} `json:"source"`
			Packages []struct {
				Package struct {
					Name      string `json:"name"`
					Version   string `json:"version"`
					Ecosystem string `json:"ecosystem"`
				} `json:"package"`
				Vulnerabilities []struct {
					ID       string `json:"id"`
					Summary  string `json:"summary"`
					Details  string `json:"details"`
					Severity []struct {
						Type  string `json:"type"`
						Score string `json:"score"`
					} `json:"severity"`
					DatabaseSpecific map[string]any `json:"database_specific"`
				} `json:"vulnerabilities"`
			} `json:"packages"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		return nil
	}
	var out []Finding
	for _, result := range doc.Results {
		for _, pkg := range result.Packages {
			for _, vuln := range pkg.Vulnerabilities {
				id := strings.TrimSpace(vuln.ID)
				if id == "" {
					continue
				}
				out = append(out, Finding{
					Tool:        toolID,
					Category:    CategoryDeps,
					Severity:    osvSeverity(vuln.DatabaseSpecific, vuln.Severity),
					RuleID:      "osv-scanner-" + strings.ToLower(id),
					Path:        cleanPath(result.Source.Path),
					Package:     pkg.Package.Name + "@" + pkg.Package.Version,
					Summary:     firstNonEmpty(vuln.Summary, firstSentence(vuln.Details), id+" affects "+pkg.Package.Name),
					Remediation: "upgrade or replace the affected package and re-run osv-scanner",
					Metadata: map[string]string{
						"ecosystem":  pkg.Package.Ecosystem,
						"dependency": pkg.Package.Name,
						"version":    pkg.Package.Version,
						"osv_id":     id,
					},
				})
			}
		}
	}
	return out
}

type TrivyScanner struct{}

func (TrivyScanner) ID() string { return "trivy" }

func (TrivyScanner) Supports(inv Inventory) bool { return inv.RuntimeEnabled }

func (s TrivyScanner) Scan(ctx context.Context, target Target, _ Inventory) ([]Finding, error) {
	cmd := "if command -v trivy >/dev/null 2>&1; then " +
		"trivy fs --format json --quiet --scanners vuln,secret,misconfig " +
		"--skip-dirs node_modules --skip-dirs .git .; fi"
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 240,
	})
	if err := scannerTransportError(s.ID(), res, err); err != nil {
		return nil, err
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return nil, scannerExitError(s.ID(), res)
	}
	if !json.Valid([]byte(res.Stdout)) {
		return nil, scannerParseError(s.ID(), json.Unmarshal([]byte(res.Stdout), &struct{}{}))
	}
	out := parseTrivyOutput(s.ID(), res.Stdout)
	if len(out) == 0 {
		return nil, scannerExitError(s.ID(), res)
	}
	return out, nil
}

func parseTrivyOutput(toolID, stdout string) []Finding {
	var doc struct {
		Results []struct {
			Target          string `json:"Target"`
			Vulnerabilities []struct {
				VulnerabilityID  string `json:"VulnerabilityID"`
				PkgName          string `json:"PkgName"`
				InstalledVersion string `json:"InstalledVersion"`
				FixedVersion     string `json:"FixedVersion"`
				Severity         string `json:"Severity"`
				Title            string `json:"Title"`
			} `json:"Vulnerabilities"`
			Misconfigurations []struct {
				ID         string `json:"ID"`
				Severity   string `json:"Severity"`
				Title      string `json:"Title"`
				Message    string `json:"Message"`
				Resolution string `json:"Resolution"`
				PrimaryURL string `json:"PrimaryURL"`
			} `json:"Misconfigurations"`
			Secrets []struct {
				RuleID    string `json:"RuleID"`
				Category  string `json:"Category"`
				Severity  string `json:"Severity"`
				Title     string `json:"Title"`
				StartLine int    `json:"StartLine"`
			} `json:"Secrets"`
		} `json:"Results"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		return nil
	}
	var out []Finding
	for _, r := range doc.Results {
		path := cleanPath(r.Target)
		for _, v := range r.Vulnerabilities {
			pkg := v.PkgName
			if v.InstalledVersion != "" {
				pkg += "@" + v.InstalledVersion
			}
			remediation := "upgrade or replace the affected dependency"
			if strings.TrimSpace(v.FixedVersion) != "" {
				remediation = "upgrade to " + v.FixedVersion
			}
			out = append(out, Finding{
				Tool:        toolID,
				Category:    CategoryDeps,
				Severity:    mapScannerSeverity(v.Severity),
				RuleID:      "trivy-" + strings.ToLower(v.VulnerabilityID),
				Path:        path,
				Package:     pkg,
				Summary:     firstNonEmpty(v.Title, v.VulnerabilityID+" affects "+v.PkgName),
				Remediation: remediation,
			})
		}
		for _, m := range r.Misconfigurations {
			out = append(out, Finding{
				Tool:        toolID,
				Category:    CategoryConfig,
				Severity:    mapScannerSeverity(m.Severity),
				RuleID:      "trivy-" + slug(m.ID),
				Path:        path,
				Summary:     firstNonEmpty(m.Title, m.Message, m.ID),
				Remediation: firstNonEmpty(m.Resolution, m.PrimaryURL, "fix the misconfiguration and re-run Trivy"),
			})
		}
		for _, sec := range r.Secrets {
			out = append(out, Finding{
				Tool:        toolID,
				Category:    CategorySecrets,
				Severity:    maxAppSecSeverity(mapScannerSeverity(sec.Severity), SeverityHigh),
				RuleID:      "trivy-secret-" + slug(sec.RuleID),
				Path:        path,
				Line:        sec.StartLine,
				Summary:     firstNonEmpty(sec.Title, sec.Category, sec.RuleID),
				Remediation: "remove the secret, rotate it upstream, and re-run Trivy",
			})
		}
	}
	return out
}

type SyftSBOMScanner struct{}

func (SyftSBOMScanner) ID() string { return "syft" }

func (SyftSBOMScanner) Supports(inv Inventory) bool { return inv.RuntimeEnabled }

func (s SyftSBOMScanner) Scan(ctx context.Context, target Target, _ Inventory) ([]Finding, error) {
	cmd := "if command -v syft >/dev/null 2>&1; then syft dir:. -o cyclonedx-json --quiet; fi"
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 180,
	})
	if err := scannerTransportError(s.ID(), res, err); err != nil {
		return nil, err
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return nil, scannerExitError(s.ID(), res)
	}
	var doc struct {
		Components []json.RawMessage `json:"components"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &doc); err != nil {
		return nil, scannerParseError(s.ID(), err)
	}
	return []Finding{{
		Tool:     s.ID(),
		Category: CategoryInventory,
		Severity: SeverityInfo,
		RuleID:   "sbom-syft-cyclonedx",
		Path:     ".",
		Summary:  "Syft generated CycloneDX SBOM with " + itoaPositive(len(doc.Components)) + " components",
		Metadata: map[string]string{"components": itoaPositive(len(doc.Components)), "format": "CycloneDX JSON"},
	}}, nil
}

type ScanCodeLicenseScanner struct{}

func (ScanCodeLicenseScanner) ID() string { return "scancode" }

func (ScanCodeLicenseScanner) Supports(inv Inventory) bool { return inv.RuntimeEnabled }

func (s ScanCodeLicenseScanner) Scan(ctx context.Context, target Target, _ Inventory) ([]Finding, error) {
	cmd := "if command -v scancode >/dev/null 2>&1; then " +
		"scancode --license --json-pp - --ignore node_modules --ignore .git .; fi"
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 300,
	})
	if err := scannerTransportError(s.ID(), res, err); err != nil {
		return nil, err
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return nil, scannerExitError(s.ID(), res)
	}
	if !json.Valid([]byte(res.Stdout)) {
		return nil, scannerParseError(s.ID(), json.Unmarshal([]byte(res.Stdout), &struct{}{}))
	}
	out := parseScanCodeOutput(s.ID(), res.Stdout)
	if len(out) == 0 {
		return nil, scannerExitError(s.ID(), res)
	}
	return out, nil
}

func parseScanCodeOutput(toolID, stdout string) []Finding {
	var doc struct {
		Files []struct {
			Path     string `json:"path"`
			Licenses []struct {
				SPDXLicenseKey string  `json:"spdx_license_key"`
				Key            string  `json:"key"`
				Score          float64 `json:"score"`
			} `json:"licenses"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []Finding
	for _, file := range doc.Files {
		for _, lic := range file.Licenses {
			key := strings.ToUpper(firstNonEmpty(lic.SPDXLicenseKey, lic.Key))
			if key == "" || key == "UNKNOWN" {
				continue
			}
			sev, blocked := licenseSeverity(key)
			if !blocked {
				continue
			}
			dedupe := key + "\x00" + cleanPath(file.Path)
			if seen[dedupe] {
				continue
			}
			seen[dedupe] = true
			out = append(out, Finding{
				Tool:        toolID,
				Category:    CategoryPolicy,
				Severity:    sev,
				RuleID:      "license-" + slug(key),
				Path:        cleanPath(file.Path),
				Summary:     "license policy review required: " + key,
				Remediation: "replace the dependency or add an explicit tenant-approved license waiver",
				Metadata:    map[string]string{"license": key},
			})
		}
	}
	return out
}

func licenseSeverity(key string) (Severity, bool) {
	switch {
	case strings.Contains(key, "AGPL"):
		return SeverityHigh, true
	case strings.Contains(key, "GPL"):
		return SeverityHigh, true
	case strings.Contains(key, "LGPL"), strings.Contains(key, "MPL"), strings.Contains(key, "EPL"), strings.Contains(key, "CDDL"):
		return SeverityMedium, true
	default:
		return SeverityInfo, false
	}
}

func mapScannerSeverity(s string) Severity {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "CRITICAL":
		return SeverityCritical
	case "HIGH", "ERROR":
		return SeverityHigh
	case "MEDIUM", "MODERATE", "WARNING":
		return SeverityMedium
	case "LOW", "INFO", "INFORMATIONAL":
		return SeverityLow
	default:
		return SeverityMedium
	}
}

func maxAppSecSeverity(a, b Severity) Severity {
	if severityRank(a) >= severityRank(b) {
		return a
	}
	return b
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
		exists, err := workspaceFileExists(ctx, target, modPath)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		cwd := serviceRoot(modPath)
		cmd := "if command -v govulncheck >/dev/null 2>&1; then govulncheck -json ./...; fi"
		res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
			Shell: cmd, Cwd: cwd, TimeoutSeconds: 180,
		})
		if err := scannerTransportError(s.ID(), res, err); err != nil {
			return nil, err
		}
		if strings.TrimSpace(res.Stdout) == "" {
			if err := scannerExitError(s.ID(), res); err != nil {
				return nil, err
			}
			continue
		}
		got := parseGovulncheckOutput(s.ID(), cwd, res.Stdout)
		if len(got) == 0 {
			if err := scannerExitError(s.ID(), res); err != nil {
				return nil, err
			}
		}
		out = append(out, got...)
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
		var msgBuf strings.Builder
		msgBuf.Grow(len(doc.Finding.OSV) + len(t.Module) + len(doc.Finding.FixedVersion) + 16)
		msgBuf.WriteString(doc.Finding.OSV)
		if t.Module != "" {
			msgBuf.WriteString(" in ")
			msgBuf.WriteString(t.Module)
		}
		if doc.Finding.FixedVersion != "" {
			msgBuf.WriteString(" (fixed in ")
			msgBuf.WriteString(doc.Finding.FixedVersion)
			msgBuf.WriteByte(')')
		}
		msg := msgBuf.String()
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
		exists, err := workspaceFileExists(ctx, target, modPath)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		cwd := serviceRoot(modPath)
		cmd := "if command -v go >/dev/null 2>&1; then go list -m -u -json all; fi"
		res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
			Shell: cmd, Cwd: cwd, TimeoutSeconds: 120,
		})
		if err := scannerTransportError(s.ID(), res, err); err != nil {
			return nil, err
		}
		if strings.TrimSpace(res.Stdout) == "" {
			if err := scannerExitError(s.ID(), res); err != nil {
				return nil, err
			}
			continue
		}
		got := parseGoListDeprecations(s.ID(), modPath, res.Stdout)
		if len(got) == 0 {
			if err := scannerExitError(s.ID(), res); err != nil {
				return nil, err
			}
		}
		out = append(out, got...)
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
		exists, err := workspaceFileExists(ctx, target, lockPath)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		cwd := serviceRoot(lockPath)
		cmd := "if command -v npm >/dev/null 2>&1; then npm audit --json --omit=dev; fi"
		res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
			Shell: cmd, Cwd: cwd, TimeoutSeconds: 120,
		})
		if err := scannerTransportError(s.ID(), res, err); err != nil {
			return nil, err
		}
		if strings.TrimSpace(res.Stdout) == "" {
			if err := scannerExitError(s.ID(), res); err != nil {
				return nil, err
			}
			continue
		}
		if !json.Valid([]byte(res.Stdout)) {
			return nil, scannerParseError(s.ID(), json.Unmarshal([]byte(res.Stdout), &struct{}{}))
		}
		got := parseNpmAuditOutput(s.ID(), lockPath, res.Stdout)
		if len(got) == 0 {
			if err := scannerExitError(s.ID(), res); err != nil {
				return nil, err
			}
		}
		out = append(out, got...)
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
		var msgBuf strings.Builder
		msgBuf.Grow(len("npm advisory: ") + len(name) + 1 + len(v.Range))
		msgBuf.WriteString("npm advisory: ")
		msgBuf.WriteString(name)
		if v.Range != "" {
			msgBuf.WriteByte(' ')
			msgBuf.WriteString(v.Range)
		}
		msg := msgBuf.String()
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
	ok, _ := workspaceFileExists(ctx, target, path)
	return ok
}

func workspaceFileExists(ctx context.Context, target Target, path string) (bool, error) {
	if !target.HasRuntime() {
		return false, nil
	}
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: "test -f " + shellQuote(path), TimeoutSeconds: 10,
	})
	if err := scannerTransportError("workspace-file-probe", res, err); err != nil {
		return false, err
	}
	if res.ExitCode == 0 {
		return true, nil
	}
	if res.ExitCode == 1 {
		return false, nil
	}
	return false, scannerExitError("workspace-file-probe", res)
}
