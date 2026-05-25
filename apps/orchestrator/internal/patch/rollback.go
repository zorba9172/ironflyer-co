// Package patch — post-rollback gate verification.
//
// A rollback restores the project tree to a prior snapshot. That tree
// is supposed to be "green" — every gate already passed at snapshot
// time — but in practice the workspace can drift (a dependency was
// removed, a secret rotated, a flaky external API came back) and the
// rolled-back tree no longer passes gates. We surface that explicitly
// so the operator sees "rollback succeeded, but Tests gate is now red".
//
// The Engine carries an optional GateRunner (wired by the host so the
// patch package stays free of the finisher dependency); when set, we
// re-run Lint, Tests, and Security after each rollback and stuff the
// verdicts into RollbackResult.
package patch

import (
	"context"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/metrics"
)

// RollbackResult is the structured response Engine.RollbackWithVerify
// returns. Snapshot is the snapshot that was restored; GateVerdicts is
// the per-gate result of the post-rollback re-run (empty when no
// gateRunner is wired). Ok is false when any verified gate failed.
type RollbackResult struct {
	Ok           bool               `json:"ok"`
	Snapshot     Snapshot           `json:"snapshot"`
	GateVerdicts []domain.GateState `json:"gateVerdicts,omitempty"`
}

// rollbackVerificationGates is the canonical set of gates the engine
// re-runs after a rollback. Spec / UX / Arch / Budget / Deploy are
// intentionally skipped — they're either pre-code or end-of-pipeline
// gates whose verdict isn't meaningfully changed by a code revert.
var rollbackVerificationGates = []domain.GateName{
	domain.GateLint,
	domain.GateTest,
	domain.GateSecurity,
}

// RollbackWithVerify is the rollback path the GraphQL resolver calls.
// It performs the snapshot restore via the existing Rollback method,
// then — if a GateRunner is wired — re-runs the verification gates
// and surfaces their verdicts in the response. The base counter
// `ironflyer_patch_apply_outcome_total{outcome=rolled_back}` is bumped
// regardless; the per-gate counter is the GateRunner's responsibility.
func (e *Engine) RollbackWithVerify(ctx context.Context, projectID, snapID string) (RollbackResult, error) {
	snap, err := e.Rollback(projectID, snapID)
	if err != nil {
		return RollbackResult{}, err
	}
	metrics.ObservePatchApplyOutcome("rolled_back")
	res := RollbackResult{Ok: true, Snapshot: snap}
	if e.gateRunner == nil {
		return res, nil
	}
	for _, g := range rollbackVerificationGates {
		gs, runErr := e.gateRunner.RunGate(ctx, projectID, string(g))
		if runErr != nil {
			// Don't fail the rollback — surface the runner error as a
			// synthetic Failed verdict so the UI shows "we tried to
			// verify and couldn't" instead of silent success.
			res.Ok = false
			res.GateVerdicts = append(res.GateVerdicts, domain.GateState{
				Name:   g,
				Status: domain.GateStatusFailed,
				Issues: []domain.Issue{{
					Gate:     g,
					Severity: domain.SeverityWarning,
					Message:  "post-rollback gate verification failed to run: " + runErr.Error(),
				}},
			})
			continue
		}
		if gs.Status != domain.GateStatusPassed {
			res.Ok = false
		}
		res.GateVerdicts = append(res.GateVerdicts, gs)
	}
	return res, nil
}
