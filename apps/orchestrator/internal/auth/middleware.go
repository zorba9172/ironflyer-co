// CSRF posture: this orchestrator authenticates EVERY protected route via
// the Authorization: Bearer <jwt> header (with a ?token= query-string
// fallback for SSE/EventSource, which cannot set headers — see
// extractToken below). There is NO cookie path: the JWT is never read
// from, nor written to, an HTTP cookie anywhere in this package. CSRF
// attacks rely on the browser auto-attaching credentials (cookies, HTTP
// auth) cross-origin; Authorization headers are not auto-attached, so
// the API is structurally immune. CSRF middleware (gorilla/csrf, double-
// submit tokens, SameSite cookies) is therefore intentionally not wired
// in. If ANY future endpoint ever accepts the JWT from a cookie, this
// invariant breaks and a CSRF token mechanism MUST be added before that
// endpoint ships.
package auth

import (
	"net/http"
	"strings"
)

// Middleware extracts the bearer token (header or `token` query param so the
// SSE EventSource — which can't set headers — still authenticates), verifies
// it, and attaches the User to context. Unauthenticated requests are 401.
func Middleware(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := extractToken(r)
			if tok == "" {
				writeAuthError(w, "missing token")
				return
			}
			u, err := svc.Verify(r.Context(), tok)
			if err != nil {
				writeAuthError(w, "invalid token")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
		})
	}
}

// Optional is like Middleware but does not 401 on missing/invalid tokens.
// Used on endpoints that work both authenticated and anonymous (e.g.
// /budget/plans listing).
func Optional(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tok := extractToken(r); tok != "" {
				if u, err := svc.Verify(r.Context(), tok); err == nil {
					r = r.WithContext(WithUser(r.Context(), u))
				} else if r.URL.Path == "/graphql" || r.URL.Path == "/graphql/" {
					// Surface verification errors on the /graphql plane
					// only — every authenticated GraphQL call that
					// silently degrades to anonymous lands as a
					// confusing POLICY_DENY in logs. The header lets
					// the operator immediately see "wrong JWT secret"
					// or "user row missing" without enabling debug
					// logging. Removed from prod via a build tag in a
					// follow-up; for now it's information that costs
					// nothing.
					w.Header().Set("X-Ironflyer-Auth-Optional-Error", err.Error())
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(strings.ToLower(h), "bearer ") {
			return strings.TrimSpace(h[7:])
		}
	}
	// SSE/EventSource can't set Authorization. Allow ?token= as a fallback.
	if q := r.URL.Query().Get("token"); q != "" {
		return q
	}
	return ""
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="ironflyer"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
