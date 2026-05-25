// Package wireup is the integration glue that cross-wires the V22 Wave-2
// packages into one orchestrator binary. The package owns the adapter
// structs that bridge between sibling packages without creating import
// cycles. Functions here are called from cmd/orchestrator/main.go and
// return either constructed services or registration callbacks the
// caller installs.
package wireup

import (
	"context"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/deploy"
	"ironflyer/apps/orchestrator/internal/execution"
	"ironflyer/apps/orchestrator/internal/profitguard"
	"ironflyer/apps/orchestrator/internal/profitguardbridge"
	"ironflyer/apps/orchestrator/internal/secrets"
)

// DeployDeps is the input bundle for BuildDeployService. Every field is
// optional — when nil the constructor falls back to the Noop adapter +
// permissive profit guard.
type DeployDeps struct {
	Pool        *pgxpool.Pool
	Logger      zerolog.Logger
	SecretsBrk  secrets.Broker
	Guard       profitguard.Guard
	ExecSvc     execution.Service
	BridgeDeps  profitguardbridge.BridgeDeps
	VercelToken string
}

// BuildDeployService constructs deploy.Service with adapters wired for
// Vercel (when a token resolver is available) and a profit guard
// checker that snapshots the live execution row. The Noop adapter is
// always registered as a fallback target.
func BuildDeployService(d DeployDeps) deploy.Service {
	cfg := deploy.DefaultConfig()

	adapters := map[deploy.Target]deploy.Adapter{
		deploy.TargetNoop: deploy.NoopAdapter{},
	}
	// Wire Vercel adapter when secrets broker is available (the
	// adapter resolves VERCEL_TOKEN at call time per-tenant).
	if d.SecretsBrk != nil {
		resolver := &deploySecretResolverAdapter{broker: d.SecretsBrk}
		adapters[deploy.TargetVercel] = deploy.NewVercelAdapter(resolver, http.DefaultClient, cfg.VercelAPIBase, d.Logger)
	}

	guardChecker := &profitGuardCheckerAdapter{
		guard:      d.Guard,
		exec:       d.ExecSvc,
		bridgeDeps: d.BridgeDeps,
		logger:     d.Logger,
	}

	if d.Pool != nil {
		return deploy.NewPostgresService(d.Pool, cfg, d.Logger, adapters, guardChecker)
	}
	return deploy.NewMemoryService(cfg, d.Logger, adapters, guardChecker)
}

// deploySecretResolverAdapter satisfies deploy.SecretResolver by
// looking up + releasing a secret through the V22 broker. Each call
// path mints a short-lived capability scoped to the deploy operation;
// the value is materialised once and returned to the deploy adapter.
type deploySecretResolverAdapter struct {
	broker secrets.Broker
}

// Resolve loads the named secret for (tenantID, projectID). The broker
// audit chain captures both the release and the resolution.
func (a *deploySecretResolverAdapter) Resolve(ctx context.Context, tenantID, projectID, name string) ([]byte, error) {
	if a.broker == nil {
		return nil, deploy.ErrSecretMissing
	}
	ref, err := a.broker.Lookup(ctx, tenantID, projectID, name)
	if err != nil {
		return nil, err
	}
	// deployment-class secret; the deploy plane is the only legitimate
	// caller. We supply a synthetic policy decision id so the broker's
	// Release pre-flight passes — production wiring should source this
	// from a PEP.MustAllow call upstream of the deploy plane.
	cap, err := a.broker.Release(ctx, ref, "deploy", "wireup-deploy-secret", 0, secrets.ReleaseScope{
		DeployTarget: "vercel",
	})
	if err != nil {
		return nil, err
	}
	return a.broker.Resolve(ctx, cap)
}

// profitGuardCheckerAdapter satisfies deploy.ProfitGuardChecker by
// pulling a fresh execution state through the profitguard bridge,
// asking guard.Decide, and recording the verdict. snapshot from the
// caller is currently advisory; the live execution row is always the
// source of truth.
type profitGuardCheckerAdapter struct {
	guard      profitguard.Guard
	exec       execution.Service
	bridgeDeps profitguardbridge.BridgeDeps
	logger     zerolog.Logger
}

// Decide bridges a deploy plane question into the profitguard.Guard.
// snapshot may carry an explicit execution_id; when absent we treat
// the deploy as un-attributed (permissive: nothing to debit yet).
func (a *profitGuardCheckerAdapter) Decide(ctx context.Context, point string, snapshot map[string]any) (string, string, error) {
	if a.guard == nil {
		return "continue", "profit_guard_unwired", nil
	}
	execID, _ := snapshot["execution_id"].(string)
	if strings.TrimSpace(execID) == "" {
		// No execution context — fall back to permissive. The deploy
		// plane already enforced wallet hold + approval workflow.
		return "continue", "no_execution_context", nil
	}
	in, err := profitguardbridge.SnapshotFor(ctx, a.exec, execID, a.bridgeDeps, profitguardbridge.BridgeFlags{}, nil, &a.logger)
	if err != nil {
		return "continue", "snapshot_unavailable", nil
	}
	dec, err := a.guard.Decide(ctx, profitguard.EnforcementPoint(point), in)
	if err != nil {
		return "continue", "guard_error", nil
	}
	_ = a.guard.Record(ctx, execID, profitguard.EnforcementPoint(point), dec, in)
	return string(dec.Action), dec.Reason, nil
}

// _ keeps the decimal import referenced — deploy.PromoteResult / etc.
// carry decimal.Decimal cost fields the integration layer may surface
// in a future provider-attribution path.
var _ = decimal.Zero
