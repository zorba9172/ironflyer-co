// Package resolver helpers shared by every V22 resolver file. These
// utilities replace the much larger pre-purge helpers.go — V22 only
// needs the tenant resolver, the authenticated-user accessor, and a
// pair of decimal/float conversion seams that several resolvers use.
package resolver

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/shopspring/decimal"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"ironflyer/apps/orchestrator/internal/auth"
)

// errUnauthenticated is returned by the helpers when no authenticated
// user is on the context. Resolvers map this to a typed GraphQL error
// with extension {"code":"UNAUTHENTICATED"} so clients can route to the
// sign-in flow.
var errUnauthenticated = errors.New("unauthenticated")

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
			"topUpURL": webBaseURL + "/app/wallet",
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
