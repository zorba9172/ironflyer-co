package deploy

import (
	"context"
	"fmt"
)

// ProfitGuardChecker is the local seam between the deploy plane and
// the profitguard.Guard surface. We declare it here (rather than
// importing internal/profitguard at every deploy call site) so the
// package stays self-contained AND so the integration agent can
// inject either:
//
//   - a real adapter that calls profitguardbridge.SnapshotFor + Guard.Decide + Guard.Record
//   - a permissive stub during boot before ProfitGuard is wired
//   - a deny-all stub for ops/break-glass scenarios
//
// The expected wiring lives in cmd/orchestrator/main.go; this package
// only consumes the interface.
type ProfitGuardChecker interface {
	// Decide returns the canonical ProfitGuard action + reason for
	// the supplied enforcement point. action values mirror the
	// profitguard.Action wire values: continue, degrade,
	// pause_for_budget, stop, kill_branch, switch_provider,
	// reuse_blueprint, reuse_repair. Reason is a human-readable
	// audit string.
	//
	// snapshot is the pre-built ExecState payload (per
	// profitguardbridge.SnapshotFor) the integration agent passes in.
	Decide(ctx context.Context, point string, snapshot map[string]any) (action, reason string, err error)
}

// GuardDeploy is the BeforeVercelDeploy enforcement helper every
// production deploy call site calls before consuming any provider
// budget. Preview deploys are not guarded — they go straight to the
// adapter; production deploys MUST clear ProfitGuard or the Service
// refuses to open the row.
//
// GuardDeploy enforces the four hard-stop verdicts the V22 plan
// names: Stop, KillBranch, PauseForBudget — and treats any other
// action (Continue, Degrade, SwitchProvider, ReuseBlueprint,
// ReuseRepair) as "go ahead". The reason string is wrapped into
// ErrProfitGuardBlocked so the resolver can surface it verbatim to
// the operator. nil pg is treated as "ProfitGuard not wired" and is
// permissive — the integration agent injects a real checker once the
// guard is up.
//
// snapshot may be nil; the checker is responsible for synthesising a
// safe default when it is.
func GuardDeploy(ctx context.Context, pg ProfitGuardChecker, snapshot map[string]any, environment string) error {
	if pg == nil {
		return nil
	}
	if environment != string(EnvironmentProduction) {
		return nil
	}
	action, reason, err := pg.Decide(ctx, string(pointBeforeVercelDeploy), snapshot)
	if err != nil {
		return fmt.Errorf("%w: profit guard decide: %v", ErrProfitGuardBlocked, err)
	}
	switch action {
	case actionStop, actionKillBranch, actionPauseForBudget:
		return fmt.Errorf("%w: %s: %s", ErrProfitGuardBlocked, action, reason)
	}
	return nil
}

// Local copies of the profitguard wire constants. We deliberately
// don't import internal/profitguard here — the local mirrors keep
// the package cycle-free and let the integration agent satisfy
// ProfitGuardChecker with whatever shape it prefers.
const (
	pointBeforeVercelDeploy = "before_vercel_deploy"

	actionContinue       = "continue"
	actionDegrade        = "degrade"
	actionPauseForBudget = "pause_for_budget"
	actionStop           = "stop"
	actionKillBranch     = "kill_branch"
	actionSwitchProvider = "switch_provider"
	actionReuseBlueprint = "reuse_blueprint"
	actionReuseRepair    = "reuse_repair"
)

// _ keeps the unused wire constants alive — the integration agent
// can reference them when synthesising audit rows without re-typing
// the strings.
var _ = []string{
	actionContinue, actionDegrade, actionSwitchProvider,
	actionReuseBlueprint, actionReuseRepair,
}
