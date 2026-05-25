package secrets

import "errors"

// ErrBackendNotConfigured is returned when a backend was selected but
// its required dependencies (SDK, credentials, network address) are
// not available at this build/runtime. The broker treats this as a
// hard release failure: a secret cannot be released through a backend
// that cannot be reached.
var ErrBackendNotConfigured = errors.New("secrets: backend not configured")

// ErrUnknownBackend is returned when a SecretRef points at a backend
// name that the broker has no implementation for. Distinct from
// ErrBackendNotConfigured so callers can tell "you typo'd the name"
// apart from "the SDK isn't compiled in".
var ErrUnknownBackend = errors.New("secrets: unknown backend")

// ErrSecretNotFound is returned when the underlying backend has no
// material at the supplied reference. Surfacing this distinctly lets
// upstream UIs render "missing secret" empty states without leaking
// backend specifics.
var ErrSecretNotFound = errors.New("secrets: not found")

// ErrCapabilityExpired is returned by Resolve when the capability's
// ExpiresAt is in the past. Capabilities are short-lived by design;
// consumers must call Release again to obtain a fresh one.
var ErrCapabilityExpired = errors.New("secrets: capability expired")

// ErrCapabilityInvalid is returned by Resolve when the capability
// references a SecretRefID that the broker cannot find — typically
// because the underlying secret_refs row was deleted or rotated past
// the capability's snapshot.
var ErrCapabilityInvalid = errors.New("secrets: capability invalid")

// ErrPolicyDecisionRequired is returned by Release when the caller
// did not supply a policy decision ID. The broker never releases
// material without an upstream PDP allow attached to the call.
var ErrPolicyDecisionRequired = errors.New("secrets: policy decision id required")

// ErrInvalidReleaseTo is returned when the releaseTo target is not
// one of the broker's allowed audit categories.
var ErrInvalidReleaseTo = errors.New("secrets: invalid release target")

// ErrInvalidReleaseClass is returned when a SecretRef carries a class
// that is not part of the V22 contract.
var ErrInvalidReleaseClass = errors.New("secrets: invalid release class")
