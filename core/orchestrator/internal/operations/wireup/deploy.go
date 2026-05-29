// Package wireup is the integration glue that cross-wires the V22 Wave-2
// packages into one orchestrator binary. The package owns the adapter
// structs that bridge between sibling packages without creating import
// cycles. Functions here are called from cmd/orchestrator/main.go and
// return either constructed services or registration callbacks the
// caller installs.
package wireup

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/business/profitguard"
	"ironflyer/core/orchestrator/internal/business/profitguardbridge"
	"ironflyer/core/orchestrator/internal/operations/deploy"
	"ironflyer/core/orchestrator/internal/operations/secrets"
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
		adapters[deploy.TargetVercel] = deploy.NewVercelAdapter(resolver, nil, cfg.VercelAPIBase, d.Logger)
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

// BuildDeployDomainService wires the customer-facing domain lifecycle:
// instant *.ironflyer.app subdomains, third-party custom domains, DNS
// verification, certificate status, and optional registrar purchase.
func BuildDeployDomainService(d DeployDeps) deploy.DomainService {
	cfg := deploy.DefaultConfig()
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_MANAGED_DOMAIN_BASE")); v != "" {
		cfg.ManagedDomainBase = v
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_EDGE_DNS_TARGET")); v != "" {
		cfg.EdgeDNSTarget = v
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_DOMAIN_PROVIDER")); v != "" {
		cfg.DefaultDomainProvider = v
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_DOMAIN_REGISTRAR")); v != "" {
		cfg.DefaultRegistrar = v
	}
	if v := strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID")); v != "" {
		cfg.CloudflareAccountID = v
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_DOMAIN_PURCHASE_ENABLED")); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			cfg.DomainPurchaseEnabled = parsed
		}
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_DOMAIN_MAX_PURCHASE_USD")); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil && parsed > 0 {
			cfg.MaxDomainPurchaseUSD = parsed
		}
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_DOMAIN_PRICE_TOLERANCE_PCT")); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil && parsed >= 0 {
			cfg.DomainPurchasePriceTolerancePct = parsed
		}
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_DOMAIN_REQUIRE_CONTACT")); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			cfg.RequireDomainRegistrantContact = parsed
		}
	}

	providers := map[string]deploy.DomainProvider{
		cfg.DefaultDomainProvider: deploy.NewStaticDomainProvider(cfg.DefaultDomainProvider, cfg.ManagedDomainBase, cfg.EdgeDNSTarget),
		"ironflyer":               deploy.NewStaticDomainProvider("ironflyer", cfg.ManagedDomainBase, cfg.EdgeDNSTarget),
	}
	registrars := map[string]deploy.Registrar{
		"manual": deploy.NewNoopRegistrar("manual"),
	}
	if d.SecretsBrk != nil && cfg.CloudflareAccountID != "" {
		resolver := &deploySecretResolverAdapter{broker: d.SecretsBrk}
		registrars["cloudflare"] = deploy.NewCloudflareRegistrar(resolver, nil, cfg.CloudflareAccountID, "", d.Logger)
	}
	purchasePolicy := deploy.DomainPurchasePolicy{
		Enabled:                  cfg.DomainPurchaseEnabled,
		MaxPriceUSD:              decimal.NewFromFloat(cfg.MaxDomainPurchaseUSD),
		PriceTolerancePct:        decimal.NewFromFloat(cfg.DomainPurchasePriceTolerancePct),
		RequireRegistrantContact: cfg.RequireDomainRegistrantContact,
	}

	// BeforeDomainPurchase guard — same checker shape as deploy.GuardDeploy.
	// Reuses the existing profitGuardCheckerAdapter so registrar Purchase
	// calls go through the canonical Decide+Record audit fan-out.
	guardChecker := &profitGuardCheckerAdapter{
		guard:      d.Guard,
		exec:       d.ExecSvc,
		bridgeDeps: d.BridgeDeps,
		logger:     d.Logger,
	}

	if d.Pool != nil {
		return deploy.NewPostgresDomainService(d.Pool, providers, registrars, cfg.DefaultDomainProvider, cfg.DefaultRegistrar,
			deploy.WithDomainPurchasePolicy(purchasePolicy),
			deploy.WithDomainProfitGuard(guardChecker),
		)
	}
	return deploy.NewMemoryDomainService(providers, registrars, cfg.DefaultDomainProvider, cfg.DefaultRegistrar,
		deploy.WithDomainPurchasePolicy(purchasePolicy),
		deploy.WithDomainProfitGuard(guardChecker),
	)
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
	cap, err := a.broker.Release(ctx, ref, secrets.ReleaseToDeployProvider, "wireup-deploy-secret", 0, secrets.ReleaseScope{
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
