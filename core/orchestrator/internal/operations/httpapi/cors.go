package httpapi

// CORS middleware for the orchestrator HTTP surface.
//
// The web SPA runs on a different origin (localhost:3000 in dev, an
// app.* subdomain in prod) than the orchestrator (localhost:8080 / api.*).
// The browser blocks credentialed POST requests unless the server
// explicitly opts in. We reflect the request Origin into Access-Control-
// Allow-Origin (the wildcard `*` is invalid when Access-Control-Allow-
// Credentials is true) and allow the headers the SPA sends, including
// the double-submit CSRF header used by gqlhardening.CSRFMiddleware.
//
// Allowlist:
//   • empty list → reflect any Origin (dev convenience; prod MUST set
//     IRONFLYER_CORS_ORIGINS to a closed list).
//   • non-empty  → only reflect origins that exact-match an entry.

import (
	"net/http"
	"strings"
)

const (
	corsAllowMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsAllowHeaders = "Authorization, Content-Type, X-Requested-With, " +
		"X-Ironflyer-CSRF, Apollo-Require-Preflight, X-Apollo-Operation-Name, " +
		"X-Apollo-Tracing"
	corsExposeHeaders = "X-Request-Id, Deprecation, Sunset, Link"
	corsMaxAge        = "600"
)

func corsMiddleware(allowed []string) func(http.Handler) http.Handler {
	// Normalize allowlist once at construction time so per-request work
	// stays a single map lookup.
	allowMap := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		allowMap[o] = struct{}{}
	}
	openMode := len(allowMap) == 0

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				_, ok := allowMap[origin]
				if openMode || ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", appendVary(w.Header().Get("Vary"), "Origin"))
					if !openMode {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}
					w.Header().Set("Access-Control-Expose-Headers", corsExposeHeaders)
				}
			}
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				// Preflight. Echo the requested method/headers explicitly so
				// the browser sees a tight allowlist instead of a wildcard.
				w.Header().Set("Access-Control-Allow-Methods", corsAllowMethods)
				w.Header().Set("Access-Control-Allow-Headers", corsAllowHeaders)
				w.Header().Set("Access-Control-Max-Age", corsMaxAge)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func appendVary(existing, value string) string {
	if existing == "" {
		return value
	}
	for _, part := range strings.Split(existing, ",") {
		if strings.EqualFold(strings.TrimSpace(part), value) {
			return existing
		}
	}
	return existing + ", " + value
}
