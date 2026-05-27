package compliance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// attestationClaims is the payload of the JWT shipped inside the
// AuditBundle. The token is the "Ironflyer Verified" attestation an
// external auditor can verify against the public key we publish; the
// fields are deliberately compact so the JWT stays human-grokkable.
type attestationClaims struct {
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	Audience  string `json:"aud"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Framework string `json:"framework"`
	Verdict   string `json:"verdict"`
	Tenant    string `json:"tenant"`
}

// jwtHeader is the JOSE header for an HS256 token. Kept as a string
// constant so we don't repeat the marshalling on every sign call.
var jwtHeader = base64URL([]byte(`{"alg":"HS256","typ":"JWT"}`))

// signAttestation produces a compact HS256 JWT carrying the claims.
// secret MUST be non-empty — the Service.ExportAuditBundle caller
// returns ErrAttestationDisabled before reaching this function.
func signAttestation(secret string, claims attestationClaims) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("compliance: empty attestation secret")
	}
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := jwtHeader + "." + base64URL(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := base64URL(mac.Sum(nil))
	return payload + "." + sig, nil
}

// base64URL is the unpadded base64url encoding the JWT spec mandates.
func base64URL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
