package wireup

import (
	"context"

	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/execution"
	"ironflyer/apps/orchestrator/internal/profitguard"
	"ironflyer/apps/orchestrator/internal/profitguardbridge"
)

// ArtifactStoreHookAdapter satisfies patch.ArtifactStoreHook. The
// hook is consulted before persisting a patch whose total payload
// exceeds the configured threshold (1 MiB by default).
type ArtifactStoreHookAdapter struct {
	Guard      profitguard.Guard
	Exec       execution.Service
	BridgeDeps profitguardbridge.BridgeDeps
	Logger     zerolog.Logger
}

// BeforeArtifactStore snapshots the live execution and asks the
// guard. A Stop/KillBranch/PauseForBudget verdict short-circuits the
// patch write. nil guard or empty execution is permissive — the
// economic stop only matters when there's an executable economy.
func (a ArtifactStoreHookAdapter) BeforeArtifactStore(ctx context.Context, executionID string, sizeBytes int64) (string, string, error) {
	if a.Guard == nil || a.Exec == nil || executionID == "" {
		return "continue", "no_execution_context", nil
	}
	in, err := profitguardbridge.SnapshotFor(ctx, a.Exec, executionID, a.BridgeDeps, profitguardbridge.BridgeFlags{}, nil, &a.Logger)
	if err != nil {
		return "continue", "snapshot_unavailable", nil
	}
	dec, err := a.Guard.Decide(ctx, profitguard.BeforeArtifactStore, in)
	if err != nil {
		return "continue", "guard_error", nil
	}
	_ = a.Guard.Record(ctx, executionID, profitguard.BeforeArtifactStore, dec, in)
	return string(dec.Action), dec.Reason, nil
}

// LongVerificationHookAdapter satisfies finisher.LongVerificationHook.
// Called before the finisher kicks off a verification step it expects
// to run beyond the long-verify duration threshold.
type LongVerificationHookAdapter struct {
	Guard      profitguard.Guard
	Exec       execution.Service
	BridgeDeps profitguardbridge.BridgeDeps
	Logger     zerolog.Logger
}

// BeforeLongVerification consults the guard at the BeforeLongVerify
// enforcement point. estimatedSec is advisory — the guard's policy
// uses the live spent/budget shape from the execution row.
func (a LongVerificationHookAdapter) BeforeLongVerification(ctx context.Context, executionID string, estimatedSec float64) (string, string, error) {
	if a.Guard == nil || a.Exec == nil || executionID == "" {
		return "continue", "no_execution_context", nil
	}
	in, err := profitguardbridge.SnapshotFor(ctx, a.Exec, executionID, a.BridgeDeps, profitguardbridge.BridgeFlags{}, nil, &a.Logger)
	if err != nil {
		return "continue", "snapshot_unavailable", nil
	}
	dec, err := a.Guard.Decide(ctx, profitguard.BeforeLongVerification, in)
	if err != nil {
		return "continue", "guard_error", nil
	}
	_ = a.Guard.Record(ctx, executionID, profitguard.BeforeLongVerification, dec, in)
	return string(dec.Action), dec.Reason, nil
}
