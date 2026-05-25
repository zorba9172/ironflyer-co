// Package auth — role plane.
//
// The orchestrator historically had no real role concept: operator
// status piggy-backed on User.Plan == "operator", and every gate
// re-implemented the same string comparison. That model breaks the
// moment a single principal needs multiple capabilities (e.g. a
// security_auditor who must read the audit chain but should not be
// allowed to mutate billing). This file introduces a first-class
// role plane: roles are a set carried on the User struct, persisted
// in users.roles[] (TEXT[] with a GIN index — migration 00038), and
// queried through HasRole / HasAnyRole / IsPlatformOperator helpers
// so every call site asks the same question the same way.
package auth

import (
	"context"
	"strings"
)

// Canonical role names. Stored verbatim in users.roles[] so the
// constants and the persisted strings must stay in lockstep — never
// rename without a data migration that rewrites existing rows.
//
//	platform_operator  — owns the audit/admin trust boundary, can
//	                     cross tenants and inspect the chain proof.
//	tenant_admin       — administers a single tenant: billing,
//	                     members, project ownership transfers.
//	security_auditor   — read-only against the audit log and
//	                     ProfitGuard ledger; cannot mutate state.
//	billing_admin      — wallet top-ups, plan changes, refunds.
const (
	RolePlatformOperator = "platform_operator"
	RoleTenantAdmin      = "tenant_admin"
	RoleSecurityAuditor  = "security_auditor"
	RoleBillingAdmin     = "billing_admin"
)

// HasRole reports whether u carries the named role exactly. Case- and
// whitespace-normalised so a stray "Platform_Operator " from a CSV
// import does not silently de-authorise a real operator.
func (u *User) HasRole(role string) bool {
	if u == nil {
		return false
	}
	want := normaliseRole(role)
	if want == "" {
		return false
	}
	for _, r := range u.Roles {
		if normaliseRole(r) == want {
			return true
		}
	}
	return false
}

// HasAnyRole returns true when u carries at least one of the listed
// roles. Empty role list returns false — "any of nothing" is no.
func (u *User) HasAnyRole(roles ...string) bool {
	if u == nil || len(roles) == 0 {
		return false
	}
	for _, r := range roles {
		if u.HasRole(r) {
			return true
		}
	}
	return false
}

// IsPlatformOperator is the fast-path the audit / admin gates use so
// the hot read does not re-allocate a string slice per call.
func (u *User) IsPlatformOperator() bool {
	return u.HasRole(RolePlatformOperator)
}

// RoleFromContext returns the roles attached to the authenticated user
// on ctx, plus an ok flag mirroring FromContext's semantics. The
// returned slice is a defensive copy so callers cannot mutate the
// context-carried User.
func RoleFromContext(ctx context.Context) ([]string, bool) {
	u, ok := FromContext(ctx)
	if !ok {
		return nil, false
	}
	if len(u.Roles) == 0 {
		return nil, true
	}
	out := make([]string, len(u.Roles))
	copy(out, u.Roles)
	return out, true
}

// IsPlatformOperatorContext is the call most gates actually want: a
// boolean answer to "does the ctx carry a platform_operator?" that
// folds the anonymous case (no auth.User) and the missing-role case
// into a single false.
func IsPlatformOperatorContext(ctx context.Context) bool {
	u, ok := FromContext(ctx)
	if !ok {
		return false
	}
	return u.IsPlatformOperator()
}

// RoleSetter is the persistence contract for promoting / demoting a
// user across roles. Memory and Postgres stores implement it; service
// layer code depends on this interface rather than the concrete store
// so a dev box and prod share the same call site.
type RoleSetter interface {
	SetRoles(ctx context.Context, userID string, roles []string) error
}

// normaliseRole lowercases and trims whitespace. Roles are stored
// canonically lowercase in users.roles[]; this helper makes the
// in-process comparison forgiving without changing what we persist.
func normaliseRole(r string) string {
	return strings.ToLower(strings.TrimSpace(r))
}
