// Package allocator implements the 5-step admission order from
// ARCHITECTURE_RUNTIME_SCALE.md "Quotas And Admission":
//
//  1. Wallet hold exists (caller signals via ctx).
//  2. ProfitGuard expected margin positive (caller signals via ctx).
//  3. Tenant quota check (quota.Enforcer.Admit).
//  4. Warm slot available OR cold start meets SLA (warmpool.Pool.Lease).
//  5. Node pool capacity (v1: log-only).
//
// The allocator does NOT touch the wallet or ProfitGuard directly;
// the orchestrator already enforced those before dialing the runtime.
// Instead the allocator reads typed flags off context so the wireup
// stays explicit and audit-friendly.
package allocator

import (
	"context"
	"time"
)

// ctxKey is the local context key type to avoid collisions.
type ctxKey int

const (
	ctxWalletHoldOK ctxKey = iota + 1
	ctxProfitGuardPositive
	ctxRiskLabel
)

// WithWalletHold marks the context as carrying a confirmed wallet
// hold. Set this in the orchestrator's runtime-create RPC after
// wallet.Reserve() returns success.
func WithWalletHold(ctx context.Context, ok bool) context.Context {
	return context.WithValue(ctx, ctxWalletHoldOK, ok)
}

// WithProfitGuard marks the context as having a positive expected
// margin verdict from ProfitGuard.
func WithProfitGuard(ctx context.Context, positive bool) context.Context {
	return context.WithValue(ctx, ctxProfitGuardPositive, positive)
}

// WithRisk attaches a risk label ("low" / "medium" / "high") used by
// the RuntimeClass selector to pick stronger isolation when needed.
func WithRisk(ctx context.Context, risk string) context.Context {
	return context.WithValue(ctx, ctxRiskLabel, risk)
}

// walletHoldOK reads the marker; missing == false so the default is
// strict.
func walletHoldOK(ctx context.Context) bool {
	v, _ := ctx.Value(ctxWalletHoldOK).(bool)
	return v
}

// profitGuardPositive reads the marker; missing == false so a missing
// ProfitGuard verdict denies allocation.
func profitGuardPositive(ctx context.Context) bool {
	v, _ := ctx.Value(ctxProfitGuardPositive).(bool)
	return v
}

// riskOf reads the risk label; default "" lets the selector fall back
// to its low-risk preference.
func riskOf(ctx context.Context) string {
	v, _ := ctx.Value(ctxRiskLabel).(string)
	return v
}

// Config bundles allocator-wide knobs.
type Config struct {
	// ColdStartSLA is the SLA the allocator must beat to skip the
	// warm pool. If unset the allocator always tries warm first.
	ColdStartSLA time.Duration
	// AllowAnonymousTenant lets dev / mock paths run without a
	// tenant ID. Production should set this false.
	AllowAnonymousTenant bool
}
