package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/auth"
	"ironflyer/apps/orchestrator/internal/ratelimit"
)

// chatLimiter caps how often a single user can hit POST /chat — the route
// that spends provider tokens. 6 per minute sustained, 3 burst keeps the
// experience responsive without letting a stuck-tab refresh loop drain a
// user's budget in 30 seconds.
var chatLimiter = ratelimit.New(0.10, 3) // 6/min sustained, 3 burst

// generalLimiter is a softer per-IP bucket used for unauthenticated public
// endpoints (signup, login, lead capture). 60/min sustained, 30 burst —
// fast enough that real humans never feel it, slow enough that a script
// spraying signups gives up.
var generalLimiter = ratelimit.New(1.0, 30)

// withChatRateLimit returns a middleware that runs `chatLimiter` keyed by
// the authenticated user id. Falls back to the client IP if no user is
// authenticated (dev mode with AuthOptional).
func (a *API) withChatRateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := rateLimitKey(r)
		if ok, wait := chatLimiter.Allow(key); !ok {
			retryAfter := int(wait.Round(time.Second).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			w.Header().Set("X-RateLimit-Bucket", "chat")
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":      "rate limit exceeded — slow down or switch effort to Lite",
				"retryAfter": retryAfter,
			})
			return
		}
		next(w, r)
	}
}

// withSignupRateLimit caps anonymous signup/login/lead floods. Keyed by IP
// rather than user (since the user isn't known yet); shares the bucket
// across the three routes intentionally — a bot spraying signups looks
// the same as a bot spraying logins.
func (a *API) withSignupRateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := "ip:" + clientIP(r)
		if ok, wait := generalLimiter.Allow(key); !ok {
			retryAfter := int(wait.Round(time.Second).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			w.Header().Set("X-RateLimit-Bucket", "anon")
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":      "too many requests — try again shortly",
				"retryAfter": retryAfter,
			})
			return
		}
		next(w, r)
	}
}

// rateLimitKey is "user:<id>" when authenticated, "ip:<addr>" otherwise.
// Falls back gracefully so dev mode (AuthOptional) still gets isolation.
func rateLimitKey(r *http.Request) string {
	if u, ok := auth.FromContext(r.Context()); ok && u.ID != "" {
		return "user:" + u.ID
	}
	return "ip:" + clientIP(r)
}

// shareLimitsAcross is a no-op placeholder kept so the route registration
// reads consistently when we later collapse the two limiters into a single
// budget. Strings only — never used in conditional logic.
var _ = strings.TrimSpace
