// Wow-loop wireup — V22 Wave-3 (Agent 34 / Agent 38 close-out).
//
// The wowloop.DefaultBuilder requires SIX source adapters, all
// non-nil. The execution + ledger adapters have always been live;
// Agent 38 closes the wired-degraded gap by replacing the gate /
// patch / repair / deploy stubs with real adapters that read from
// the canonical V22 services.
//
// Backwards compatibility: BuildWowLoop keeps its (execSvc,
// ledgerSvc, walletSvc, log) signature. Patch / repair / deploy
// sources arrive via optional WithPatchEngine / WithRepairGenome /
// WithDeployService variadic options so the integration agent
// (cmd/orchestrator/main.go) can opt in without breaking the
// existing call site. Calls that do NOT pass options keep degrading
// to the stub adapters — equivalent to the pre-Agent-38 behaviour.
//
// The gate source is always live: it reads gate.verdict.v1 +
// siblings out of the execution_events feed via the new
// execution.Service.GateEventsByExecution surface. The same path
// powers SecurityFindings via LatestSecurityFindings — that surface
// already shipped earlier in wave-3.
package wireup

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/operations/deploy"
	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/business/ledger"
	"ironflyer/core/orchestrator/internal/operations/patch"
	"ironflyer/core/orchestrator/internal/ai/repair"
	"ironflyer/core/orchestrator/internal/business/wallet"
	"ironflyer/core/orchestrator/internal/business/wowloop"
)

// WowLoopOption tunes the wow-loop builder wiring without breaking
// the BuildWowLoop signature. Each option is purely additive and
// nil-safe; passing nil services leaves the matching adapter on its
// stub fallback.
type WowLoopOption func(*wowLoopOpts)

type wowLoopOpts struct {
	patchEngine   *patch.Engine
	repairGenome  repair.Genome
	deployService deploy.Service
	runtimeSource wowloop.RuntimeSource
}

// WithPatchEngine swaps the stub PatchSource for an adapter that
// reads from the patch engine. Today the engine still falls back to
// execution_events for the per-execution view; see wowPatchAdapter
// for the wiring rationale.
func WithPatchEngine(e *patch.Engine) WowLoopOption {
	return func(o *wowLoopOpts) { o.patchEngine = e }
}

// WithRepairGenome swaps the stub RepairSource for an adapter that
// reads from the repair genome.
func WithRepairGenome(g repair.Genome) WowLoopOption {
	return func(o *wowLoopOpts) { o.repairGenome = g }
}

// WithDeployService swaps the stub DeploySource for an adapter that
// reads the most recent deploy for the execution.
func WithDeployService(s deploy.Service) WowLoopOption {
	return func(o *wowLoopOpts) { o.deployService = s }
}

// WithRuntimeSource wires the live-preview adapter so the bundle
// returns the running workspace dev-server URL while the execution is
// still in flight. Without this option the builder falls back to the
// deploy preview URL (which is empty until publish completes).
//
// nil-safe: passing nil leaves the builder's Runtime field unset and
// the bundle uses the deploy URL only.
func WithRuntimeSource(rs wowloop.RuntimeSource) WowLoopOption {
	return func(o *wowLoopOpts) { o.runtimeSource = rs }
}

// BuildWowLoop returns a fully-wired DefaultBuilder. Returns nil when
// the execution source is missing — the resolver layer degrades to
// gqlNotConfigured rather than panic.
//
// The (execSvc, ledgerSvc, walletSvc, log) signature is FROZEN; new
// wiring lands through the variadic opts so the integration agent
// can adopt it without coordinated edits to main.go.
func BuildWowLoop(
	execSvc execution.Service,
	ledgerSvc ledger.Service,
	walletSvc wallet.Service,
	log zerolog.Logger,
	opts ...WowLoopOption,
) wowloop.Builder {
	if execSvc == nil {
		log.Warn().Msg("wowloop: execution service unwired; builder disabled")
		return nil
	}
	o := wowLoopOpts{}
	for _, fn := range opts {
		if fn != nil {
			fn(&o)
		}
	}

	gateSrc := wowloop.GateSource(&wowGateAdapter{exec: execSvc, log: log})

	var patchSrc wowloop.PatchSource = &wowPatchAdapter{exec: execSvc, engine: o.patchEngine, log: log}
	if execSvc == nil && o.patchEngine == nil {
		patchSrc = stubPatchSource{}
	}

	var repairSrc wowloop.RepairSource = &wowRepairAdapter{exec: execSvc, genome: o.repairGenome, log: log}
	if execSvc == nil && o.repairGenome == nil {
		repairSrc = stubRepairSource{}
	}

	var deploySrc wowloop.DeploySource = stubDeploySource{}
	if o.deployService != nil {
		deploySrc = &wowDeployAdapter{svc: o.deployService, log: log}
	}

	b := wowloop.NewDefaultBuilder(
		&wowExecutionAdapter{exec: execSvc},
		&wowLedgerAdapter{ledger: ledgerSvc, wallet: walletSvc, log: log},
		gateSrc,
		patchSrc,
		repairSrc,
		deploySrc,
	)
	if o.runtimeSource != nil {
		b.Runtime = o.runtimeSource
	}
	return b
}

// ---------- ExecutionSource ----------------------------------------

type wowExecutionAdapter struct {
	exec execution.Service
}

func (a *wowExecutionAdapter) GetExecution(ctx context.Context, id string) (wowloop.ExecutionSnapshot, error) {
	e, err := a.exec.Get(ctx, id)
	if err != nil {
		return wowloop.ExecutionSnapshot{}, err
	}
	snap := wowloop.ExecutionSnapshot{
		ID:                e.ID,
		TenantID:          e.TenantID,
		Status:            string(e.Status),
		RevenueUSD:        e.RevenueUSD,
		ProviderCostUSD:   e.ProviderCostUSD,
		SandboxCostUSD:    e.SandboxCostUSD,
		StorageCostUSD:    e.StorageCostUSD,
		DeploymentCostUSD: e.DeploymentCostUSD,
	}
	if e.GrossMarginPct != nil {
		snap.GrossMarginPct = *e.GrossMarginPct
	}
	if e.EndedAt != nil {
		snap.EndedAt = *e.EndedAt
	}
	// A63 — prefer the real workspace_id stamped by the finisher engine
	// the moment a sandbox was resolved/allocated for this execution
	// (see finisher/engine.go: e.executionService.SetWorkspaceID after
	// FindWorkspaceForProject). Falls back to ProjectID for backward
	// compat with legacy executions written before migration 00040 +
	// the engine wireup landed, AND for executions whose run aborted
	// before the workspace allocation site fired. The RuntimeSource
	// adapter accepts both: a real workspaceID hits PreviewURL
	// directly; a projectID still resolves through
	// FindWorkspaceForProject as the pre-A63 path did.
	if e.WorkspaceID != "" {
		snap.WorkspaceID = e.WorkspaceID
	} else {
		snap.WorkspaceID = e.ProjectID
	}
	return snap, nil
}

// ---------- LedgerSource -------------------------------------------

type wowLedgerAdapter struct {
	ledger ledger.Service
	wallet wallet.Service
	log    zerolog.Logger
}

func (a *wowLedgerAdapter) LedgerFor(ctx context.Context, executionID, tenantID string) (wowloop.LedgerSnapshot, error) {
	snap := wowloop.LedgerSnapshot{}

	// Pull tenant balance from wallet — single source of truth.
	if a.wallet != nil && tenantID != "" {
		if w, err := a.wallet.Get(ctx, tenantID); err == nil {
			snap.BalanceUSD = w.BalanceUSD
			snap.HoldsActive = w.HoldUSD.Sign() > 0
		}
	}
	// Walk the per-execution trail to find the most recent
	// CreditRelease so the next-action picker can spot "released hold
	// but balance is low".
	if a.ledger != nil && executionID != "" {
		if execID, err := uuid.Parse(executionID); err == nil {
			entries, err := a.ledger.ListByExecution(ctx, execID)
			if err == nil {
				for _, e := range entries {
					if e.EntryType == ledger.EntryCreditRelease {
						if e.CreatedAt.After(snap.LastReleaseAt) {
							snap.LastReleaseAt = e.CreatedAt
						}
					}
				}
			}
		}
	}
	return snap, nil
}

// ---------- GateSource ---------------------------------------------
//
// Reads gate verdicts out of execution_events via the canonical
// execution.Service.GateEventsByExecution surface. Empty result is a
// valid pass-through ("no gate verdicts recorded yet") rather than
// an error — the wow-loop builder renders the gate panel with zero
// stages and CompletionScore=0 in that case.
//
// SecurityFindings reuses LatestSecurityFindings (the wave-3 surface
// already shipped) and projects the raw payload map onto the
// wow-loop SecurityFindingSnapshot shape.

type wowGateAdapter struct {
	exec execution.Service
	log  zerolog.Logger
}

func (a *wowGateAdapter) GatesFor(ctx context.Context, executionID string) ([]wowloop.GateSnapshot, error) {
	if a == nil || a.exec == nil || executionID == "" {
		return nil, nil
	}
	events, err := a.exec.GateEventsByExecution(ctx, executionID)
	if err != nil {
		a.log.Warn().Err(err).Str("execution_id", executionID).Msg("wowloop: gate events query failed")
		// Tolerate the error — the wow-loop is a read-only dashboard;
		// surfacing the failure as an empty panel keeps the rest of
		// the bundle renderable.
		return nil, nil
	}
	out := make([]wowloop.GateSnapshot, 0, len(events))
	for _, e := range events {
		out = append(out, wowloop.GateSnapshot{
			Name:        e.Gate,
			Status:      e.Status,
			IssuesCount: e.IssuesCount,
		})
	}
	return out, nil
}

func (a *wowGateAdapter) SecurityFindingsFor(ctx context.Context, executionID string) ([]wowloop.SecurityFindingSnapshot, error) {
	if a == nil || a.exec == nil || executionID == "" {
		return nil, nil
	}
	raws, err := a.exec.LatestSecurityFindings(ctx, executionID)
	if err != nil {
		a.log.Warn().Err(err).Str("execution_id", executionID).Msg("wowloop: security findings query failed")
		return nil, nil
	}
	out := make([]wowloop.SecurityFindingSnapshot, 0, len(raws))
	for _, raw := range raws {
		out = append(out, securityFindingFromPayload(raw))
	}
	return out, nil
}

// securityFindingFromPayload projects a raw execution-event payload
// onto the wow-loop SecurityFindingSnapshot shape. Missing fields
// degrade to zero values; we never error on a malformed entry —
// that's the FindingSource adapter's contract from wave-3.
func securityFindingFromPayload(raw map[string]any) wowloop.SecurityFindingSnapshot {
	out := wowloop.SecurityFindingSnapshot{}
	if v, ok := raw["severity"].(string); ok {
		out.Severity = v
	}
	if v, ok := raw["rule_id"].(string); ok {
		out.RuleID = v
	} else if v, ok := raw["rule"].(string); ok {
		out.RuleID = v
	}
	if v, ok := raw["path"].(string); ok {
		out.Path = v
	}
	if v, ok := raw["line"].(float64); ok {
		out.Line = int(v)
	}
	if v, ok := raw["summary"].(string); ok {
		out.Summary = v
	} else if v, ok := raw["message"].(string); ok {
		out.Summary = v
	}
	if v, ok := raw["blocks_deploy"].(bool); ok {
		out.BlocksDeploy = v
	}
	return out
}

// ---------- PatchSource --------------------------------------------
//
// Reads applied patches from execution_events via the canonical
// execution.Service.PatchAppliedEventsByExecution surface. The patch
// engine is held by reference for future use (e.g. enriching the
// summary with the Patch.Title) but the per-execution index lives on
// the executions feed — the engine itself does not currently track
// which executionID applied which patch.

type wowPatchAdapter struct {
	exec   execution.Service
	engine *patch.Engine // reserved for future enrichment; may be nil
	log    zerolog.Logger
}

func (a *wowPatchAdapter) PatchesFor(ctx context.Context, executionID string) ([]wowloop.PatchSnapshot, error) {
	if a == nil || a.exec == nil || executionID == "" {
		return nil, nil
	}
	events, err := a.exec.PatchAppliedEventsByExecution(ctx, executionID)
	if err != nil {
		a.log.Warn().Err(err).Str("execution_id", executionID).Msg("wowloop: patch events query failed")
		return nil, nil
	}
	out := make([]wowloop.PatchSnapshot, 0, len(events))
	for _, e := range events {
		if len(e.AffectedPaths) == 0 {
			// Still surface the patch — the builder dedupes by path
			// and counts patches separately, so a path-less entry
			// contributes to PatchCount without inflating
			// ChangedFiles.
			out = append(out, wowloop.PatchSnapshot{
				ID:        e.PatchID,
				AppliedAt: e.AppliedAt,
			})
			continue
		}
		for _, p := range e.AffectedPaths {
			out = append(out, wowloop.PatchSnapshot{
				ID:        e.PatchID,
				Path:      p,
				AppliedAt: e.AppliedAt,
			})
		}
	}
	return out, nil
}

// ---------- RepairSource -------------------------------------------
//
// Reads recovery attempts from execution_events via the canonical
// execution.Service.RecoveryAttemptsByExecution surface. The repair
// genome is held by reference so a future genome that learns to
// index attempts by executionID can take over without a wireup
// change.

type wowRepairAdapter struct {
	exec   execution.Service
	genome repair.Genome // reserved; may be nil today
	log    zerolog.Logger
}

func (a *wowRepairAdapter) RepairsFor(ctx context.Context, executionID string) ([]wowloop.RepairSnapshot, error) {
	if a == nil || a.exec == nil || executionID == "" {
		return nil, nil
	}
	attempts, err := a.exec.RecoveryAttemptsByExecution(ctx, executionID)
	if err != nil {
		a.log.Warn().Err(err).Str("execution_id", executionID).Msg("wowloop: recovery events query failed")
		return nil, nil
	}
	out := make([]wowloop.RepairSnapshot, 0, len(attempts))
	for _, at := range attempts {
		gate := at.Gate
		if gate == "" {
			// Older recipe events may only carry the failure
			// signature; surface a placeholder so the builder still
			// counts the attempt (even if it can't pin it to a
			// specific gate).
			gate = fmt.Sprintf("signature:%s", short(at.FailureSignature))
		}
		out = append(out, wowloop.RepairSnapshot{
			GateName:  gate,
			Succeeded: at.Applied && at.Success,
		})
	}
	return out, nil
}

func short(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

// ---------- DeploySource -------------------------------------------
//
// Reads the most recent deploy for the execution via the canonical
// deploy.Service.GetByExecution surface. Empty result (no deploy
// yet) renders the bundle's preview/production URLs as empty
// strings — the next-action picker uses that as the trigger to
// suggest "deploy preview" / "promote to production".

type wowDeployAdapter struct {
	svc deploy.Service
	log zerolog.Logger
}

func (a *wowDeployAdapter) DeployFor(ctx context.Context, executionID string) (wowloop.DeploySnapshot, error) {
	if a == nil || a.svc == nil || executionID == "" {
		return wowloop.DeploySnapshot{}, nil
	}
	d, ok, err := a.svc.GetByExecution(ctx, executionID)
	if err != nil {
		a.log.Warn().Err(err).Str("execution_id", executionID).Msg("wowloop: deploy lookup failed")
		return wowloop.DeploySnapshot{}, nil
	}
	if !ok {
		return wowloop.DeploySnapshot{}, nil
	}
	return wowloop.DeploySnapshot{
		PreviewURL:    d.PreviewURL,
		ProductionURL: d.ProductionURL,
	}, nil
}

// ---------- Stub sources -------------------------------------------
//
// Retained as the fallback path when BuildWowLoop is called without
// the matching WithPatchEngine / WithRepairGenome /
// WithDeployService option AND the execSvc parameter is also nil —
// effectively only the test-injection cases. Empty results keep the
// bundle's "no data yet" rendering intact.

type stubPatchSource struct{}

func (stubPatchSource) PatchesFor(_ context.Context, _ string) ([]wowloop.PatchSnapshot, error) {
	return nil, nil
}

type stubRepairSource struct{}

func (stubRepairSource) RepairsFor(_ context.Context, _ string) ([]wowloop.RepairSnapshot, error) {
	return nil, nil
}

type stubDeploySource struct{}

func (stubDeploySource) DeployFor(_ context.Context, _ string) (wowloop.DeploySnapshot, error) {
	return wowloop.DeploySnapshot{}, nil
}
