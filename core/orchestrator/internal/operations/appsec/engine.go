package appsec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

type Engine struct {
	scanners []Scanner
	policy   Policy
	now      func() time.Time
}

func NewEngine(scanners ...Scanner) *Engine {
	return &Engine{
		scanners: scanners,
		policy:   DefaultPolicy(),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func DefaultEngine() *Engine {
	return NewEngine(DefaultScanners()...)
}

func DefaultScanners() []Scanner {
	return []Scanner{
		NativeSecretScanner{},
		DependencyHealthScanner{},
		OSVScanner{},
		OSVScannerCLI{},
		RuntimeSecretScanner{},
		TrufflehogScanner{},
		GitleaksScanner{},
		SemgrepScanner{},
		TrivyScanner{},
		GovulncheckScanner{},
		GoModuleDeprecationScanner{},
		NpmAuditScanner{},
		SyftSBOMScanner{},
		ScanCodeLicenseScanner{},
		ConfigScanner{},
	}
}

func (e *Engine) WithPolicy(policy Policy) *Engine {
	if e == nil {
		return e
	}
	e.policy = policy
	return e
}

func (e *Engine) Scan(ctx context.Context, target Target) Result {
	if e == nil {
		e = DefaultEngine()
	}
	started := e.now()
	cfg := ResolveConfig(target)
	inv := BuildInventory(target)
	var findings []Finding
	var scannerErrors []ScannerError
	for _, scanner := range e.scanners {
		if scanner == nil || !scanner.Supports(inv) {
			continue
		}
		if cfg.ScannerDisabled(scanner.ID()) {
			continue
		}
		got, err := scanner.Scan(ctx, target, inv)
		if err != nil {
			scannerErrors = append(scannerErrors, ScannerError{
				Tool:       scanner.ID(),
				Message:    err.Error(),
				DetectedAt: e.now(),
			})
			continue
		}
		for _, f := range got {
			f.Tool = firstNonEmpty(f.Tool, scanner.ID())
			if f.DetectedAt.IsZero() {
				f.DetectedAt = e.now()
			}
			findings = append(findings, f)
		}
	}
	findings = cfg.Apply(findings, e.now())
	findings = dedupeAndCap(ensureFindingIDs(target, findings), e.policy.MaxFindingsPerRun)
	graph := BuildRiskGraph(target, inv, findings)
	return Result{
		Inventory:     inv,
		Config:        cfg,
		Findings:      findings,
		ScannerErrors: scannerErrors,
		Graph:         graph,
		Verdict:       e.policy.Evaluate(findings, scannerErrors...),
		StartedAt:     started,
		EndedAt:       e.now(),
	}
}

func ensureFindingIDs(target Target, findings []Finding) []Finding {
	for i := range findings {
		if findings[i].ID != "" {
			continue
		}
		sum := sha256.Sum256([]byte(strings.Join([]string{
			target.ProjectID,
			findings[i].Tool,
			findings[i].RuleID,
			findings[i].Path,
			itoaPositive(findings[i].Line),
			findings[i].Package,
		}, "\x00")))
		findings[i].ID = hex.EncodeToString(sum[:8])
	}
	return findings
}

func dedupeAndCap(findings []Finding, capN int) []Finding {
	if capN <= 0 {
		capN = DefaultPolicy().MaxFindingsPerRun
	}
	seen := map[string]bool{}
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		key := f.ID
		if key == "" {
			key = strings.Join([]string{f.Tool, f.RuleID, f.Path, itoaPositive(f.Line), f.Package}, "\x00")
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return severityRank(out[i].Severity) > severityRank(out[j].Severity)
	})
	if len(out) > capN {
		out = out[:capN]
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
