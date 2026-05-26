// Package auth provides the shared-token middleware that fronts every
// bridge route. The orchestrator and the runtime hold the same secret
// and present it as X-Ironflyer-Bridge-Token. Browser-facing endpoints
// (the signaling WebSocket) also accept it as ?token= because browsers
// cannot set custom headers on WebSocket handshakes.
package auth

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

// HeaderName is the canonical HTTP header carrying the shared bridge
// token. Lowercased deliberately — http.Header normalises on Set.
const HeaderName = "X-Ironflyer-Bridge-Token"

// QueryParam mirrors HeaderName for the WebSocket handshake case.
const QueryParam = "token"

// RequireBridgeToken returns a chi-compatible middleware that rejects
// any request whose token doesn't match secret. When secret is empty
// the middleware short-circuits to 503: a bridge with no secret is a
// misconfiguration, never a public service.
func RequireBridgeToken(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.TrimSpace(secret) == "" {
				writeErr(w, http.StatusServiceUnavailable, "bridge token not configured")
				return
			}
			provided := r.Header.Get(HeaderName)
			if provided == "" {
				provided = r.URL.Query().Get(QueryParam)
			}
			if subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
				writeErr(w, http.StatusUnauthorized, "invalid bridge token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// VerifyToken is the standalone helper used by the WebSocket upgrader
// where the middleware pipeline doesn't directly apply (the upgrader
// owns the response writer after Accept).
func VerifyToken(r *http.Request, secret string) bool {
	if strings.TrimSpace(secret) == "" {
		return false
	}
	provided := r.Header.Get(HeaderName)
	if provided == "" {
		provided = r.URL.Query().Get(QueryParam)
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) == 1
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
