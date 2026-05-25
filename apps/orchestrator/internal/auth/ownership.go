// ownership.go centralises the tenant-isolation invariant for resources that
// carry an owner column (user_id / owner_id). Every store row in the
// orchestrator MUST be tagged with the user that owns it, and every read
// path MUST clamp by that owner before returning. The two helpers below —
// the Resource interface and EnsureOwner — give handlers and stores a
// single place to fail-closed when an owner mismatch is detected.
//
// Contract (documented in docs/security/TENANT_ISOLATION_INVARIANTS.md):
//
//   - Every store row carries an owner string (UserID or OwnerID).
//   - Every Get/List/Update/Delete MUST scope by owner. List methods that
//     don't take an owner are bugs.
//   - Foreign resources surface as ErrNotFound (HTTP 404), NEVER as
//     ErrForbidden on the wire — the existence of a row owned by someone
//     else must not leak. ErrForbidden is internal; HTTP translates it
//     to 404.
//
// EnsureOwner is the single helper every owner-check call site uses.
// Callers that hold a typed resource implement Resource; callers that have
// the raw owner string can use EnsureOwnerString. Either way the error
// surface is identical so audit + middleware code branch on one sentinel.
package auth

import (
	"context"
	"errors"
)

// ErrForbidden is the internal sentinel for an owner mismatch. HTTP
// handlers MUST translate this to 404 (not 403) before writing the
// response — 403 leaks existence of the requested resource.
var ErrForbidden = errors.New("forbidden: resource owner mismatch")

// Resource is implemented by any value that knows who owns it. The two
// methods kept separate so callers can fall through (e.g. "no owner =
// public") without an extra interface dance.
type Resource interface {
	GetOwner() string
}

// EnsureOwner is the canonical owner-check helper. Returns nil when the
// supplied ownerID matches the resource owner OR when the resource is
// public (empty owner). Returns ErrForbidden otherwise.
//
// ctx is accepted for future audit-event emission — current
// implementations are pure but the signature keeps the call sites
// forward-compatible with structured logging of forbidden hits.
func EnsureOwner(ctx context.Context, ownerID string, resource Resource) error {
	if resource == nil {
		return ErrForbidden
	}
	return EnsureOwnerString(ctx, ownerID, resource.GetOwner())
}

// EnsureOwnerString is the lower-level helper for call sites that hold
// the raw owner string instead of a Resource. Empty resourceOwner is
// treated as "public" — matches the orchestrator's existing posture for
// the seed demo project.
func EnsureOwnerString(_ context.Context, ownerID, resourceOwner string) error {
	if resourceOwner == "" {
		// Public row — anyone (including anonymous) may read it.
		return nil
	}
	if ownerID == "" {
		// Authenticated user is anonymous but the resource is owned —
		// fail closed.
		return ErrForbidden
	}
	if ownerID != resourceOwner {
		return ErrForbidden
	}
	return nil
}

// MustOwn is a convenience wrapper that returns true on access granted
// and false otherwise. Useful inside template-style filters where
// returning an error is awkward (e.g. slice predicate).
func MustOwn(ownerID, resourceOwner string) bool {
	return EnsureOwnerString(context.Background(), ownerID, resourceOwner) == nil
}

// ErrNotFound is the public-facing sentinel resolver code returns when
// either the resource truly doesn't exist OR the caller can't see it.
// The shared 404-vs-403 posture (don't leak existence) means GraphQL
// resolvers translate every ownership miss to this single error.
var ErrNotFound = errors.New("not found")

// RequireProjectAccess is the shared owner-check helper for GraphQL
// resolvers (and any other call site) that operate on a project. It
// loads the project from `projects.Get(projectID)`, verifies the
// authenticated user can read it via the project's own
// IsAccessibleBy method, and returns ErrNotFound on both unknown id
// AND foreign id — the same posture as the REST requireProjectAccess
// helper. Callers MUST surface ErrNotFound verbatim so existence of
// other users' projects doesn't leak via differential error messages.
//
// Generics are used so the helper stays free of the store/domain
// imports (which would force every package importing auth to pull
// the heavier graph in transitively). Concrete callers pass their
// store.Store directly; Go infers S and P.
func RequireProjectAccess[S interface {
	Get(id string) (P, error)
}, P interface {
	IsAccessibleBy(userID string) bool
}](_ context.Context, projects S, userID, projectID string) (P, error) {
	var zero P
	p, err := projects.Get(projectID)
	if err != nil {
		return zero, ErrNotFound
	}
	if !p.IsAccessibleBy(userID) {
		return zero, ErrNotFound
	}
	return p, nil
}
