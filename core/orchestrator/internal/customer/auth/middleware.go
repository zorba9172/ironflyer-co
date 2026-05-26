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
	"context"
	"net/http"
	"strings"
)

// jtiCtxKeyType is the per-request key for the bearer token's jti claim.
// Resolvers that mutate the session registry (revokeSession,
// revokeAllOtherSessions, mySessions' "current" flag) call JTIFromContext
// to find out which session row belongs to the caller.
type jtiCtxKeyType struct{}

var jtiCtxKey = jtiCtxKeyType{}

// WithJTI attaches the bearer token's jti to the request context.
func WithJTI(ctx context.Context, jti string) context.Context {
	if jti == "" {
		return ctx
	}
	return context.WithValue(ctx, jtiCtxKey, jti)
}

// JTIFromContext returns the bearer token's jti, or "" when the
// request was not authenticated or the token carried no jti.
func JTIFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(jtiCtxKey).(string); ok {
		return v
	}
	return ""
}

// requestInfoCtxKeyType is the per-request key for the (IP, UA) pair
// every authenticated path can read. SignIn / oauth-callback / MFA
// enrollment use it to dispatch the NewDeviceLogin notification on the
// matching trigger without having to thread the raw *http.Request all
// the way down to the resolver.
type requestInfoCtxKeyType struct{}

var requestInfoCtxKey = requestInfoCtxKeyType{}

// RequestInfo carries the per-request IP + UserAgent. Empty fields are
// legal — callers must accept partial values (X-Forwarded-For is the
// industry-standard proxy header but local dev may not set it).
type RequestInfo struct {
	IPAddress string
	UserAgent string
}

// WithRequestInfo attaches a RequestInfo to the context. Empty pairs
// are ignored so call sites do not need to branch.
func WithRequestInfo(ctx context.Context, info RequestInfo) context.Context {
	if info.IPAddress == "" && info.UserAgent == "" {
		return ctx
	}
	return context.WithValue(ctx, requestInfoCtxKey, info)
}

// RequestInfoFromContext returns the stamped (IP, UA) pair, or the zero
// value when nothing was attached.
func RequestInfoFromContext(ctx context.Context) RequestInfo {
	if v, ok := ctx.Value(requestInfoCtxKey).(RequestInfo); ok {
		return v
	}
	return RequestInfo{}
}

// bearerCtxKeyType is the per-request key for the raw bearer token.
// Long-lived background work (e.g. the finisher Engine.Run goroutine
// kicked from describeIdea) needs to re-present the user's JWT to the
// workspace runtime to allocate a sandbox; the runtime enforces owner
// checks via the same JWT. Resolvers stamp this on the bg context
// alongside finisher.WithBearer so downstream gate code can read it
// via bearerFromCtx without re-parsing the Authorization header.
type bearerCtxKeyType struct{}

var bearerCtxKey = bearerCtxKeyType{}

// WithBearer attaches the raw JWT to the request context. Empty
// tokens are ignored so callers don't have to branch.
func WithBearer(ctx context.Context, token string) context.Context {
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, bearerCtxKey, token)
}

// BearerFromContext returns the raw JWT, or "" when none is attached.
func BearerFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(bearerCtxKey).(string); ok {
		return v
	}
	return ""
}

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
			u, jti, err := svc.VerifyWithJTI(r.Context(), tok)
			if err != nil {
				writeAuthError(w, "invalid token")
				return
			}
			ctx := WithUser(r.Context(), u)
			ctx = WithJTI(ctx, jti)
			ctx = WithBearer(ctx, tok)
			ctx = WithRequestInfo(ctx, RequestInfo{IPAddress: extractClientIP(r), UserAgent: r.UserAgent()})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Optional is like Middleware but does not 401 on missing/invalid tokens.
// Used on endpoints that work both authenticated and anonymous (e.g.
// /budget/plans listing).
func Optional(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Stamp RequestInfo unconditionally so anonymous resolvers
			// (signIn, signUp, oauth-callback) can still read the
			// caller's (IP, UA) pair for new-device detection.
			r = r.WithContext(WithRequestInfo(r.Context(), RequestInfo{
				IPAddress: extractClientIP(r),
				UserAgent: r.UserAgent(),
			}))
			if tok := extractToken(r); tok != "" {
				if u, jti, err := svc.VerifyWithJTI(r.Context(), tok); err == nil {
					ctx := WithUser(r.Context(), u)
					ctx = WithJTI(ctx, jti)
					ctx = WithBearer(ctx, tok)
					ctx = WithRequestInfo(ctx, RequestInfo{IPAddress: extractClientIP(r), UserAgent: r.UserAgent()})
					r = r.WithContext(ctx)
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

// RequestInfoMiddleware stamps every incoming request with a
// RequestInfo carrying the client IP and User-Agent. Mounted ahead of
// the auth middleware so /graphql handlers (signIn / signUp / oauth
// callback) can read the pair even before authentication runs.
func RequestInfoMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithRequestInfo(r.Context(), RequestInfo{
			IPAddress: extractClientIP(r),
			UserAgent: r.UserAgent(),
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractClientIP walks the standard proxy headers, falling back to
// the raw RemoteAddr. Matches the behaviour of the oauth handler's
// clientIP — kept here so every auth-aware path resolves the IP the
// same way.
func extractClientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i > 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	if v := r.Header.Get("X-Real-Ip"); v != "" {
		return v
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	return host
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
