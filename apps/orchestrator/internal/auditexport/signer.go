// Package auditexport — signed download tokens.
//
// The previous BuildDownloadURL stub emitted a random 16-byte hex
// token that nothing on the verification side ever validated: an
// attacker could mint their own URL by guessing the path shape and
// the orchestrator had no way to reject it. This file replaces that
// stub with a real HMAC-SHA256 signer.
//
// Token shape: base64url(<json-body>) "." base64url(hmac_sha256(body, k))
//
//   - body carries TenantID + Format + NotAfter so verification can
//     refuse expired or scope-mismatched tokens.
//   - body and signature are URL-safe so the token survives being
//     spliced into "/audit/export/<token>.<ext>" without escaping.
//   - constant-time HMAC compare prevents byte-by-byte timing oracle.
//
// HMAC is enough — the body is not secret (clients see the URL), only
// the signature must be unforgeable. We intentionally do NOT JWE/JWS
// here: the token never escapes the orchestrator's REST exception
// list, so the additional JWT envelope overhead is wasted bytes.
package auditexport

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// SignedToken is the data the audit-export REST exception trusts
// after a successful Verify. The resolver layer constructs one,
// hands it to Signer.Sign, and never touches the wire form directly.
type SignedToken struct {
	TenantID string    `json:"t"`
	Format   Format    `json:"f"`
	NotAfter time.Time `json:"e"`
}

// Signer is the seam main.go wires from env-derived secret material.
// Both Sign and Verify operate on the canonical token string defined
// in the package doc — implementations MUST produce a value that
// round-trips through their own Verify and reject anything else
// (tampered HMAC, expired NotAfter, malformed body) with
// ErrInvalidToken or ErrExpiredToken.
type Signer interface {
	Sign(t SignedToken) string
	Verify(token string) (SignedToken, error)
}

// HMACSigner is the production Signer. Construct via NewHMACSigner
// so the secret-length invariant is enforced — a zero-length or
// too-short secret silently disables the entire authn boundary if
// the constructor is skipped.
type HMACSigner struct {
	secret []byte
}

// minSecretBytes is the lower bound the constructor enforces.
// SHA-256 outputs 32 bytes; an HMAC key smaller than the digest
// trades preimage resistance for nothing. Operators must provision
// a 32-byte (or longer) random key — typically 64 hex chars or 44
// base64 chars in the IRONFLYER_AUDIT_EXPORT_HMAC_SECRET env var.
const minSecretBytes = 32

// NewHMACSigner returns a Signer keyed on secret. Returns
// ErrSignerNotConfigured when secret is empty (operator forgot to
// set the env var) and ErrSignerSecretTooShort when secret is non-
// empty but below minSecretBytes — fail fast at boot rather than
// after a download URL has already been minted with a weak key.
func NewHMACSigner(secret []byte) (*HMACSigner, error) {
	if len(secret) == 0 {
		return nil, ErrSignerNotConfigured
	}
	if len(secret) < minSecretBytes {
		return nil, ErrSignerSecretTooShort
	}
	// Defensive copy so the caller cannot mutate the key later.
	key := make([]byte, len(secret))
	copy(key, secret)
	return &HMACSigner{secret: key}, nil
}

// Sign encodes t as canonical JSON, base64url-encodes both the body
// and the HMAC-SHA256(body, secret), then joins them with a '.'
// separator. The result is URL-safe and roundtrips through Verify.
func (s *HMACSigner) Sign(t SignedToken) string {
	// time.Time JSON-marshals to RFC3339 with sub-second precision
	// preserved, so Verify can reconstruct the exact NotAfter we
	// signed without rounding drift.
	body, _ := json.Marshal(t)
	bodyB64 := base64.RawURLEncoding.EncodeToString(body)
	sig := hmacSHA256(s.secret, []byte(bodyB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return bodyB64 + "." + sigB64
}

// Verify splits the token, recomputes the HMAC under the signer's
// secret, constant-time compares, JSON-decodes the body, and
// finally enforces NotAfter. Any failure resolves to either
// ErrInvalidToken (tampered HMAC, malformed body, wrong shape) or
// ErrExpiredToken (HMAC valid but NotAfter in the past).
func (s *HMACSigner) Verify(token string) (SignedToken, error) {
	dot := strings.IndexByte(token, '.')
	if dot <= 0 || dot == len(token)-1 {
		return SignedToken{}, ErrInvalidToken
	}
	bodyB64 := token[:dot]
	sigB64 := token[dot+1:]
	expected := hmacSHA256(s.secret, []byte(bodyB64))
	got, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return SignedToken{}, ErrInvalidToken
	}
	if !hmac.Equal(expected, got) {
		return SignedToken{}, ErrInvalidToken
	}
	body, err := base64.RawURLEncoding.DecodeString(bodyB64)
	if err != nil {
		return SignedToken{}, ErrInvalidToken
	}
	var out SignedToken
	if err := json.Unmarshal(body, &out); err != nil {
		return SignedToken{}, ErrInvalidToken
	}
	if !out.NotAfter.IsZero() && time.Now().After(out.NotAfter) {
		return SignedToken{}, ErrExpiredToken
	}
	return out, nil
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}
