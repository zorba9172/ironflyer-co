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
