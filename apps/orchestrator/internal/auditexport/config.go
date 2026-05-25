package auditexport

import (
	"fmt"
	"strings"
	"time"
)

// Config carries operator-tunable knobs the StoreExporter respects.
// Kept in its own file so the integration agent can wire from env
// without growing exporter.go.
type Config struct {
	// MaxEntries caps a single Stream/ChainProof window. Zero defers
	// to the 100k default baked into NewStoreExporter.
	MaxEntries int

	// SignedURLTTL is how long BuildDownloadURL honours a minted
	// token before Signer.Verify rejects it as expired. The
	// resolver computes ExpiresAt = now + TTL and surfaces it on
	// the GraphQL preview so clients know when to re-request.
	SignedURLTTL time.Duration

	// SignedURLBase is the externally visible base URL the resolver
	// prepends to the signed token when building the download link.
	// Falls back to "https://orch.local" when empty so dev boxes work.
	SignedURLBase string

	// Signer mints + verifies the HMAC token embedded in the
	// download URL. main.go constructs an HMACSigner from
	// IRONFLYER_AUDIT_EXPORT_HMAC_SECRET (min 32 bytes) and
	// injects it here at boot. When nil, BuildDownloadURL fails
	// with ErrSignerNotConfigured rather than emit an unsigned
	// URL.
	Signer Signer
}

// DefaultConfig is the conservative production baseline. Signer is
// left nil intentionally — there is no safe default secret; main.go
// must inject one or audit export refuses to mint URLs.
func DefaultConfig() Config {
	return Config{
		MaxEntries:    100_000,
		SignedURLTTL:  5 * time.Minute,
		SignedURLBase: "https://orch.local",
	}
}

// BuildDownloadURL signs a SignedToken scoped to (tenantID, format,
// now+SignedURLTTL) and returns the externally-visible download URL
// plus the absolute expiry the resolver should surface as ExpiresAt.
// Returns ErrSignerNotConfigured when c.Signer is nil so the caller
// fails loudly rather than mint a forgeable link.
func (c Config) BuildDownloadURL(tenantID string, format Format) (string, time.Time, error) {
	if c.Signer == nil {
		return "", time.Time{}, ErrSignerNotConfigured
	}
	base := strings.TrimRight(c.SignedURLBase, "/")
	if base == "" {
		base = "https://orch.local"
	}
	notAfter := time.Now().Add(c.SignedURLTTL).UTC()
	token := c.Signer.Sign(SignedToken{
		TenantID: tenantID,
		Format:   format,
		NotAfter: notAfter,
	})
	url := fmt.Sprintf("%s/audit/export/%s.%s",
		base, token, formatExtension(format))
	return url, notAfter, nil
}

// formatExtension is the file extension paired with each Format.
// Kept here so the URL constructor and any future REST handler that
// parses the extension agree on the canonical mapping.
func formatExtension(f Format) string {
	if f == FormatJSONL {
		return "jsonl"
	}
	return "csv"
}
