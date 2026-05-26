package wowloop

import (
	"context"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

// DefaultBuilder is the pure-go implementation of Builder. It is the
// canonical wow-loop assembler — every backend (memory, postgres,
// future) reuses it; only the injected Source adapters change.
//
// Every source is required. Missing sources turn into ErrNotConfigured
// so the resolver can return a typed error instead of a nil pointer
// panic.
type DefaultBuilder struct {
	Execution ExecutionSource
	Ledger    LedgerSource
	Gates     GateSource
	Patches   PatchSource
	Repairs   RepairSource
	Deploys   DeploySource

	// Runtime, when set, lets the builder surface a live workspace
	// preview URL while the execution is still running — the studio
	// iframe gets a real dev-server view instead of waiting for a
	// deploy step to succeed. Nil is fine: the bundle falls back to
	// the DeploySource preview URL exactly as it did pre-A56.
	Runtime RuntimeSource

	// Now is the time source the bundle stamps GeneratedAt with.
	// Defaults to time.Now when nil.
	Now func() time.Time
}

// NewDefaultBuilder constructs a DefaultBuilder. Callers may also
// build the struct literally to inject mocks.
func NewDefaultBuilder(exec ExecutionSource, led LedgerSource, gates GateSource, patches PatchSource, repairs RepairSource, deploys DeploySource) *DefaultBuilder {
	return &DefaultBuilder{
		Execution: exec,
		Ledger:    led,
		Gates:     gates,
		Patches:   patches,
		Repairs:   repairs,
		Deploys:   deploys,
		Now:       time.Now,
	}
}

// Build implements Builder. It fans out reads to every source
// sequentially (the fan-out is bounded by N sources, all in-process
// for the memory backend and one DB pool for the Postgres backend —
// goroutine fan-out is not worth the complexity at this scale).
func (b *DefaultBuilder) Build(ctx context.Context, executionID string) (SupportBundle, error) {
	if b == nil {
		return SupportBundle{}, ErrNotConfigured
	}
	if executionID == "" {
		return SupportBundle{}, ErrExecutionRequired
	}
	if b.Execution == nil || b.Ledger == nil || b.Gates == nil || b.Patches == nil || b.Repairs == nil || b.Deploys == nil {
		return SupportBundle{}, ErrNotConfigured
	}

	exec, err := b.Execution.GetExecution(ctx, executionID)
	if err != nil {
		return SupportBundle{}, err
	}

	ledger, err := b.Ledger.LedgerFor(ctx, executionID, exec.TenantID)
	if err != nil {
		return SupportBundle{}, err
	}

	gateRows, err := b.Gates.GatesFor(ctx, executionID)
	if err != nil {
		return SupportBundle{}, err
	}
	findings, err := b.Gates.SecurityFindingsFor(ctx, executionID)
	if err != nil {
		return SupportBundle{}, err
	}

	patches, err := b.Patches.PatchesFor(ctx, executionID)
	if err != nil {
		return SupportBundle{}, err
	}

	repairs, err := b.Repairs.RepairsFor(ctx, executionID)
	if err != nil {
		return SupportBundle{}, err
	}

	deploy, err := b.Deploys.DeployFor(ctx, executionID)
	if err != nil {
		return SupportBundle{}, err
	}

	gateReport := buildGateReport(gateRows, repairs)
	securityReport := buildSecurityReport(findings)
	costReport := buildCostReport(exec)
	changedFiles, patchCount := buildPatchSummary(patches)

	// Prefer the live workspace preview URL while the execution is
	// still in flight — the studio iframe should show the running
	// dev server, not the empty preview waiting for the deploy step
	// to mint a URL. Once the execution reaches a terminal state we
	// fall back to the deploy URL (which is the publishable artifact
	// the user wants to share).
	previewURL := deploy.PreviewURL
	if b.Runtime != nil && exec.WorkspaceID != "" && !isTerminalStatus(exec.Status) {
		if live, err := b.Runtime.PreviewURL(ctx, exec.WorkspaceID); err == nil && live != "" {
			previewURL = live
		}
	}

	nbInput := nextActionInput{
		Status:            exec.Status,
		ExecutionID:       exec.ID,
		PreviewURL:        previewURL,
		HasPreview:        previewURL != "",
		HasProduction:     deploy.ProductionURL != "",
		HasSecurityIssue:  len(findings) > 0,
		SecurityBlocks:    securityReport.BlockedDeploy,
		WalletBalanceUSD:  ledger.BalanceUSD,
		WalletHoldsActive: ledger.HoldsActive,
		PatchCount:        patchCount,
	}

	now := time.Now
	if b.Now != nil {
		now = b.Now
	}

	return SupportBundle{
		ExecutionID:    exec.ID,
		TenantID:       exec.TenantID,
		Status:         exec.Status,
		PreviewURL:     previewURL,
		ProductionURL:  deploy.ProductionURL,
		ChangedFiles:   changedFiles,
		PatchCount:     patchCount,
		GateReport:     gateReport,
		SecurityReport: securityReport,
		CostReport:     costReport,
		NextBestAction: decideNextAction(nbInput),
		GeneratedAt:    now(),
	}, nil
}

// buildGateReport merges raw gate verdicts with repair attempts. A
// gate that originally failed but had a succeeded repair attempt is
// surfaced as "repaired" — the user sees the load (IssuesCount stays
// non-zero) but understands the finisher handled it.
func buildGateReport(gates []GateSnapshot, repairs []RepairSnapshot) GateReport {
	// Index repair successes by gate name. A single succeeded repair
	// flips the gate's reported status; the bundle does not try to
	// model multiple repair rounds because the dashboard surfaces the
	// raw history separately.
	repaired := map[string]bool{}
	for _, r := range repairs {
		if r.Succeeded {
			repaired[r.GateName] = true
		}
	}

	out := GateReport{Stages: make([]GateStage, 0, len(gates))}
	if len(gates) == 0 {
		out.CompletionScore = 0
		return out
	}
	passOrRepaired := 0
	for _, g := range gates {
		status := g.Status
		if status == "fail" && repaired[g.Name] {
			status = "repaired"
		}
		if status == "pass" || status == "repaired" {
			passOrRepaired++
		}
		out.Stages = append(out.Stages, GateStage{
			Name:        g.Name,
			Status:      status,
			IssuesCount: g.IssuesCount,
		})
	}
	out.CompletionScore = float64(passOrRepaired) / float64(len(gates))
	return out
}

// buildSecurityReport converts the source-side findings into the
// bundle's view and derives PassRate + BlockedDeploy.
func buildSecurityReport(findings []SecurityFindingSnapshot) SecurityReport {
	out := SecurityReport{Findings: make([]SecurityFinding, 0, len(findings))}
	blocking := 0
	for _, f := range findings {
		if f.BlocksDeploy {
			blocking++
			out.BlockedDeploy = true
		}
		out.Findings = append(out.Findings, SecurityFinding{
			Severity: f.Severity,
			RuleID:   f.RuleID,
			Path:     f.Path,
			Line:     f.Line,
			Summary:  f.Summary,
		})
	}
	if len(findings) == 0 {
		out.PassRate = 1.0
		return out
	}
	out.PassRate = 1.0 - float64(blocking)/float64(len(findings))
	return out
}

// buildCostReport pulls money fields off the execution snapshot.
// When GrossMarginPct is unset on the source we recompute from
// revenue - sum(costs).
func buildCostReport(exec ExecutionSnapshot) CostReport {
	rep := CostReport{
		RevenueUSD:        exec.RevenueUSD,
		ProviderCostUSD:   exec.ProviderCostUSD,
		SandboxCostUSD:    exec.SandboxCostUSD,
		StorageCostUSD:    exec.StorageCostUSD,
		DeploymentCostUSD: exec.DeploymentCostUSD,
		GrossMarginPct:    exec.GrossMarginPct,
	}
	if exec.GrossMarginPct.IsZero() && exec.RevenueUSD.Sign() > 0 {
		totalCost := exec.ProviderCostUSD.
			Add(exec.SandboxCostUSD).
			Add(exec.StorageCostUSD).
			Add(exec.DeploymentCostUSD)
		rep.GrossMarginPct = exec.RevenueUSD.
			Sub(totalCost).
			Div(exec.RevenueUSD).
			Mul(decimal.NewFromInt(100))
	}
	return rep
}

// buildPatchSummary dedupes + sorts the union of patch paths and
// returns the count of patches.
func buildPatchSummary(patches []PatchSnapshot) ([]string, int) {
	seen := map[string]bool{}
	out := make([]string, 0, len(patches))
	for _, p := range patches {
		if p.Path == "" || seen[p.Path] {
			continue
		}
		seen[p.Path] = true
		out = append(out, p.Path)
	}
	sort.Strings(out)
	return out, len(patches)
}

// isTerminalStatus mirrors the execution FSM's terminal set without
// importing the execution package (wowloop is consciously decoupled).
// Status strings match execution.Status constants.
func isTerminalStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "stopped", "killed", "refunded":
		return true
	default:
		return false
	}
}

// Compile-time check.
var _ Builder = (*DefaultBuilder)(nil)
