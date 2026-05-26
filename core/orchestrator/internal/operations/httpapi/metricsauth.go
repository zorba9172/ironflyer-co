package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// metricsAuth wraps the /metrics handler with a constant-time bearer
// check when a token is configured. An empty token leaves the route
// open — main.go enforces non-empty in prod and warns otherwise.
func metricsAuth(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	expected := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		got := []byte(strings.TrimSpace(header[len(prefix):]))
		if subtle.ConstantTimeCompare(got, expected) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
