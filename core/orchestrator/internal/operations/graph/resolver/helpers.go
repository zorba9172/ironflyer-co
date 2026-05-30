// Package resolver helpers shared by every V22 resolver file. These
// utilities replace the much larger pre-purge helpers.go — V22 only
// needs the tenant resolver, the authenticated-user accessor, and a
// pair of decimal/float conversion seams that several resolvers use.
package resolver

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/shopspring/decimal"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"ironflyer/core/orchestrator/internal/customer/auth"
)

// errUnauthenticated is returned by the helpers when no authenticated
// user is on the context. It carries extension {"code":"UNAUTHENTICATED"}
// so the gqlgen error presenter passes the code through to clients,
// which use it to flip cached tokens and route to the sign-in flow.
// Why a typed *gqlerror.Error rather than errors.New: the web
// errorLink (clients/web/src/lib/apollo.tsx) keys off extensions.code,
// not the message, so a bare "unauthenticated" string left users
// stranded on screens like Studio with "Could not generate" instead
// of bouncing them to /login when their JWT expired.
var errUnauthenticated = &gqlerror.Error{
	Message:    "unauthenticated",
	Extensions: map[string]any{"code": "UNAUTHENTICATED"},
}

// currentUser returns the authenticated user from ctx, or
// errUnauthenticated if the resolver was reached without auth.
func currentUser(ctx context.Context) (auth.User, error) {
	u, ok := auth.FromContext(ctx)
	if !ok || u.ID == "" {
		return auth.User{}, errUnauthenticated
	}
	return u, nil
}

// tenantFor returns the canonical tenant string for a user. V22 uses
// the org id when set, falling back to the user id for personal
// accounts. Everything per-tenant (wallet, ledger, executions,
// blueprints, profitguard) keys off this value.
func tenantFor(u auth.User) string {
	if u.OrgID != "" {
		return u.OrgID
	}
	return u.ID
}

// floatOfDecimal is the decimal→float seam every V22 GraphQL boundary
// uses. The conversion is lossy in the last few cents of precision
// (float64 has ~15 sig figs); resolvers that need exact decimals
// expose the Decimal scalar instead. Kept in one place so the lossy
// boundary is always reviewable.
func floatOfDecimal(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}

// gqlInsufficientFunds is the typed GraphQL error returned when a
// paid execution starts but the wallet hold fails. Carries a
// top_up_url extension so the client can route the user to Stripe
// Checkout via the wallet UI.
func gqlInsufficientFunds(webBaseURL string) *gqlerror.Error {
	if webBaseURL == "" {
		webBaseURL = "http://localhost:3000"
	}
	return &gqlerror.Error{
		Message: "insufficient wallet balance",
		Extensions: map[string]any{
			"code":     "INSUFFICIENT_FUNDS",
			"topUpURL": webBaseURL + "/wallet",
		},
	}
}

// gqlProfitGuardRefused is the typed GraphQL error returned when the
// ProfitGuard BeforeExecutionAdmit gate projects a margin below the
// platform floor and refuses the execution UP FRONT — before any wallet
// hold is taken, so no funds are locked on a doomed run (law 2).
//
// The user-facing surface is deliberately clean: a stable BUDGET_TOO_LOW
// code + an upgradeURL, and NO raw verdict math (margin %, floor). The
// raw decision is already recorded in the profitguard store + audit; the
// client turns the code into a clear, localized "your budget is too low —
// upgrade your plan" message.
func gqlProfitGuardRefused(webBaseURL string) *gqlerror.Error {
	if webBaseURL == "" {
		webBaseURL = "http://localhost:3000"
	}
	return &gqlerror.Error{
		Message: "budget too low for the requested build",
		Extensions: map[string]any{
			"code":       "BUDGET_TOO_LOW",
			"upgradeURL": webBaseURL + "/plans",
		},
	}
}

// gqlNotConfigured is the typed GraphQL error returned when an
// optional surface (Stripe, ProfitGuard, blueprints) is unwired. The
// resolver is reachable but cannot serve the request.
func gqlNotConfigured(feature string) *gqlerror.Error {
	return &gqlerror.Error{
		Message: feature + ": not configured",
		Extensions: map[string]any{
			"code":    "NOT_CONFIGURED",
			"feature": feature,
		},
	}
}

// _ keeps the gqlgen graphql import alive even when no current
// resolver references it directly — future helpers (path extraction,
// custom extensions) will lean on the package.
var _ = graphql.GetOperationContext
