// Package auth handles user identity: signup, login, JWT issuance and
// verification, and the chi middleware that materialises the authenticated
// user on the request context.
package auth

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"
)

// User is the identity record. Password hashes never leave the auth package.
//
// TelemetryOptOut, when true, instructs the orchestrator to skip per-user
// OTel spans and Sentry events for this user. The flag is currently
// derived from the IRONFLYER_TELEMETRY_OPT_OUT environment variable (which
// disables telemetry for every authenticated user on the instance) via
// WithUser. A future iteration will let an individual user toggle it from
// their privacy settings; the storage shape is forward-compatible.
type User struct {
	ID              string    `json:"id"`
	Email           string    `json:"email"`
	Name            string    `json:"name,omitempty"`
	Plan            string    `json:"plan,omitempty"`
	// OrgID is the tenant the user belongs to. Empty on personal accounts
	// (the legacy default); populated by SAML provisioning and by admin
	// org-management endpoints. The IP allowlist middleware uses this to
	// look up the per-org rules.
	OrgID           string    `json:"orgId,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	TelemetryOptOut bool      `json:"telemetryOptOut,omitempty"`
	// EmailVerifiedAt is set when the user clicks the post-signup
	// verification link (and after a confirmEmailChange). nil means the
	// account is still "pending verification" — they can sign in but
	// gated mutations (paid plan, deploy, custom domains) refuse via
	// safeerror.NotVerified.
	EmailVerifiedAt *time.Time `json:"emailVerifiedAt,omitempty"`
	// MfaEnabled is true once confirmMfaEnrollment lands a verified
	// TOTP code. Sign-in for an MfaEnabled user returns an mfa-challenge
	// envelope rather than a session token; completeMfaSignIn finishes.
	MfaEnabled bool `json:"mfaEnabled,omitempty"`
	// Roles carries the role plane introduced by migration 00038
	// (users.roles[] — TEXT[] with a GIN index). Canonical names are
	// the auth.Role* constants in role.go. Authorisation gates
	// (operator.IsOperator, audit export, admin resolvers) must
	// consult this field via HasRole / HasAnyRole / IsPlatformOperator
	// instead of branching on Plan — Plan is a billing tier, not a
	// trust boundary.
	Roles []string `json:"roles,omitempty"`
}

// UserStore is the persistence contract. Memory and Postgres back it.
type UserStore interface {
	Create(ctx context.Context, email, name, passwordHash string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, string /*hash*/, error)
	GetByID(ctx context.Context, id string) (User, error)
	// GetByIDs batch-loads users by id. Returned map is keyed by id and
	// only contains rows that exist — missing ids are silently omitted so
	// callers (notably the GraphQL dataloader) can decide whether to
	// surface ErrUserNotFound per key.
	GetByIDs(ctx context.Context, ids []string) (map[string]User, error)
	SetPlan(ctx context.Context, id, plan string) error
	// SetTelemetryOptOut persists the per-user telemetry preference. The
	// instance-wide IRONFLYER_TELEMETRY_OPT_OUT env still trumps this
	// (WithUser forces opt-out=true when set); the per-user flag only
	// matters when the env flag is not set.
	SetTelemetryOptOut(ctx context.Context, id string, opt bool) error
	Delete(ctx context.Context, id string) error
}

// PasswordlessHash is a marker stored in the password_hash column for users
// created via external OAuth (no password login). Bcrypt will never produce
// this string, so password auth against it always fails.
const PasswordlessHash = "!oauth-only"

var (
	ErrUserExists   = errors.New("user already exists")
	ErrUserNotFound = errors.New("user not found")
	ErrBadPassword  = errors.New("invalid email or password")
)

// ctxKey is unexported so callers must use the package's helpers.
type ctxKey struct{}

var userCtxKey = ctxKey{}

// WithUser attaches a user to the context. If the instance-wide
// IRONFLYER_TELEMETRY_OPT_OUT environment variable is truthy, the
// attached user is forced into the opt-out state so downstream
// middleware (OTel, Sentry) can suppress per-user capture without
// each call site re-reading the env.
func WithUser(ctx context.Context, u User) context.Context {
	if envTelemetryOptOut() {
		u.TelemetryOptOut = true
	}
	return context.WithValue(ctx, userCtxKey, u)
}

// envTelemetryOptOut reports whether the orchestrator-wide opt-out flag is
// set. Truthy values: 1, true, yes, on (case-insensitive). Anything else
// — including unset — is false.
func envTelemetryOptOut() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("IRONFLYER_TELEMETRY_OPT_OUT"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// TelemetryEnabled is the helper telemetry call sites should use before
// emitting per-user spans or Sentry events. It returns false when the
// authenticated user has opted out (per-user or instance-wide). Calls
// that have no authenticated user fall back to enabled — anonymous
// telemetry is governed by separate config.
func TelemetryEnabled(ctx context.Context) bool {
	u, ok := FromContext(ctx)
	if !ok {
		return true
	}
	return !u.TelemetryOptOut
}

// FromContext returns the authenticated user or false.
func FromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userCtxKey).(User)
	return u, ok
}
