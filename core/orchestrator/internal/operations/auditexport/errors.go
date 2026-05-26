package auditexport

import "errors"

// Sentinel errors callers can branch on. The resolver layer maps
// ErrTenantRequired → 403/4xx ("operator scope required"), the others
// map to 400.
var (
	ErrTenantRequired = errors.New("auditexport: TenantID is mandatory; use TenantWildcard for operator-scoped exports")
	ErrInvalidWindow  = errors.New("auditexport: time window must be non-empty and chronological")
	ErrUnknownFormat  = errors.New("auditexport: format must be csv or jsonl")

	// ErrSignerNotConfigured is returned by NewHMACSigner when the
	// caller passed an empty secret, and by Config.BuildDownloadURL
	// when Config.Signer is nil. Either way the operator is
	// missing the IRONFLYER_AUDIT_EXPORT_HMAC_SECRET wiring and
	// the resolver should surface a hard 500-class error rather
	// than mint an unsigned URL.
	ErrSignerNotConfigured = errors.New("auditexport: HMAC signer not configured (set IRONFLYER_AUDIT_EXPORT_HMAC_SECRET)")

	// ErrSignerSecretTooShort is returned by NewHMACSigner when
	// the supplied key is below the 32-byte SHA-256 digest size.
	// Fail fast at boot instead of issuing weak download URLs.
	ErrSignerSecretTooShort = errors.New("auditexport: HMAC signer secret must be at least 32 bytes")

	// ErrInvalidToken — the wire token failed shape / HMAC /
	// JSON validation. The REST exception that serves the actual
	// download must map this to 401/403 (never 400, to avoid
	// leaking the difference between "tampered" and "malformed").
	ErrInvalidToken = errors.New("auditexport: signed token is invalid")

	// ErrExpiredToken — HMAC valid but NotAfter has passed. Maps
	// to 410 Gone or 403; the client should re-request a fresh
	// preview from the GraphQL surface.
	ErrExpiredToken = errors.New("auditexport: signed token has expired")
)
