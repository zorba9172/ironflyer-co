package operator

import (
	"context"
	"strings"

	"ironflyer/core/orchestrator/internal/customer/auth"
)

// OperatorPlan is the legacy User.Plan value the orchestrator used
// before migration 00038 introduced a real role plane. It is kept as
// a transitional shortcut for one release so accounts that have not
// yet been migrated to the new users.roles[] column still resolve
// as operators. New code MUST grant auth.RolePlatformOperator
// instead of mutating the Plan column.
//
// Deprecated: assign auth.RolePlatformOperator via
// auth.RoleSetter.SetRoles and drop the Plan-based check in the
// next release.
const OperatorPlan = "operator"

// IsOperator reports whether the authenticated user on ctx is a
// platform operator. The canonical signal is the role plane
// (auth.RolePlatformOperator carried in User.Roles); the Plan
// shortcut is honoured only as a backwards-compat bridge.
//
// Anonymous contexts (no auth.User attached) always return false —
// operator access requires positive proof of identity.
//
// This is the canonical implementation the gqlhardening middleware
// (IntrospectionGate, PersistedQueriesMiddleware) and the GraphQL
// operator resolvers both consume so the trust boundary is computed
// in exactly one place.
func IsOperator(ctx context.Context) bool {
	if auth.IsPlatformOperatorContext(ctx) {
		return true
	}
	// Backwards-compat: keep Plan="operator" as a transitional
	// shortcut for one release until every operator account has
	// been migrated to the users.roles[] plane.
	if u, ok := auth.FromContext(ctx); ok &&
		strings.EqualFold(strings.TrimSpace(u.Plan), OperatorPlan) {
		return true
	}
	return false
}

// RequireOperator returns ErrNotOperator unless IsOperator(ctx) is
// true. Operator-only service methods invoke this as their first line
// so the rule lives in one place rather than being re-implemented in
// every entry point.
func RequireOperator(ctx context.Context) error {
	if !IsOperator(ctx) {
		return ErrNotOperator
	}
	return nil
}
