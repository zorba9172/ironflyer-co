package wireup

import (
	"context"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/business/profitguard"
	"ironflyer/core/orchestrator/internal/business/profitguardbridge"
	"ironflyer/core/orchestrator/internal/business/profitguardctx"
	"ironflyer/core/orchestrator/internal/operations/audit"
)

// NewProfitGuardAuditSink returns a profitguard.AuditSink that lands
// one canonical audit row per Decide / Record call via
// audit.RecordProfitGuardDecision. Wired by main.go alongside
// profitguard.NewWithAuditSink so every ProfitGuard verdict reaches
// the hash-chained audit log without coupling the policy package to
// the audit one.
//
// The sink stamps the canonical attrs (est_cost, model, recommended
// provider, decision metadata) so dashboards can index by tenant /
// execution / enforcement point without parsing free-form strings.
func NewProfitGuardAuditSink(store audit.Store) profitguard.AuditSink {
	if store == nil {
		return nil
	}
	return profitguard.AuditSinkFunc(func(ctx context.Context, row profitguard.RecordedDecision) error {
		tenantID := ""
		if row.Metadata != nil {
			if v, ok := row.Metadata["tenant_id"].(string); ok {
				tenantID = v
			}
		}
		attrs := map[string]any{
			"est_cost_usd": row.EstimatedStepCostUSD.String(),
			"spent_usd":    row.SpentUSD.String(),
			"reserved_usd": row.ReservedUSD.String(),
		}
		if row.RecommendedProvider != "" {
			attrs["recommended_provider"] = row.RecommendedProvider
		}
		if row.ExpectedMarginPct != nil {
			attrs["expected_margin_pct"] = *row.ExpectedMarginPct
		}
		if row.RiskScore != nil {
			attrs["risk_score"] = *row.RiskScore
		}
		if row.Metadata != nil {
			// Surface model + provider hints the caller stamped into
			// the decision metadata (BillingGuard / mobile hook /
			// resolver gating). Mirrors the audit spec's mandatory
			// `model` and `verdict_reason` fields.
			if v, ok := row.Metadata["model"].(string); ok && v != "" {
				attrs["model"] = v
			}
			if v, ok := row.Metadata["provider"].(string); ok && v != "" {
				attrs["provider"] = v
			}
			if v, ok := row.Metadata["resolver_action"].(string); ok && v != "" {
				attrs["resolver_action"] = v
			}
			if v, ok := row.Metadata["mobile_kind"].(string); ok && v != "" {
				attrs["mobile_kind"] = v
			}
			if v, ok := row.Metadata["mobile_target"].(string); ok && v != "" {
				attrs["mobile_target"] = v
			}
			if v, ok := row.Metadata["project_id"].(string); ok && v != "" {
				attrs["project_id"] = v
			}
		}
		_, err := audit.RecordProfitGuardDecision(ctx, store, tenantID, row.ExecutionID,
			string(row.EnforcementPoint), string(row.Decision), row.Reason, attrs)
		return err
	})
}

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

// MobileBuildHookAdapter satisfies finisher.MobileBuildHook. The gate
// consults the adapter before kicking platform builds (gradlew, xcodebuild,
// eas build). The adapter resolves a fresh ExecState via the bridge,
// asks Guard.Decide at BeforeMobileBuild, and records the verdict.
//
// Permissive on missing context — without an executionID on ctx or a
// nil Guard / Exec the hook returns "continue" so dev runs stay
// frictionless. Production wiring always carries an executionID and a
// fully-formed Guard.
type MobileBuildHookAdapter struct {
	Guard      profitguard.Guard
	Exec       execution.Service
	BridgeDeps profitguardbridge.BridgeDeps
	Logger     zerolog.Logger
}

// BeforeMobileBuild consults the guard at the BeforeMobileBuild
// enforcement point. kind and target are advisory metadata recorded on
// the audit row; the policy itself uses the live spent/budget shape
// from the execution snapshot.
func (a MobileBuildHookAdapter) BeforeMobileBuild(ctx context.Context, projectID, kind, target string) (string, string, error) {
	if a.Guard == nil || a.Exec == nil {
		return "continue", "no_guard_wired", nil
	}
	execID, ok := profitguardctx.ExecutionID(ctx)
	if !ok || execID == "" {
		return "continue", "no_execution_context", nil
	}
	in, err := profitguardbridge.SnapshotFor(ctx, a.Exec, execID, a.BridgeDeps, profitguardbridge.BridgeFlags{}, nil, &a.Logger)
	if err != nil {
		return "continue", "snapshot_unavailable", nil
	}
	dec, err := a.Guard.Decide(ctx, profitguard.BeforeMobileBuild, in)
	if err != nil {
		return "continue", "guard_error", nil
	}
	if dec.Metadata == nil {
		dec.Metadata = map[string]any{}
	}
	dec.Metadata["mobile_kind"] = kind
	dec.Metadata["mobile_target"] = target
	dec.Metadata["project_id"] = projectID
	_ = a.Guard.Record(ctx, execID, profitguard.BeforeMobileBuild, dec, in)
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
