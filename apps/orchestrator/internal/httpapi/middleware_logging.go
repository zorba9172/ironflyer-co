package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/auth"
)

// requestIDKey scopes the request ID inside the request context so
// handlers can pull it without colliding with chi's middleware.
type requestIDKey struct{}

// requestIDHeader is honoured for incoming traffic (so an upstream proxy
// can inject its own correlation id) and echoed back on every response.
const requestIDHeader = "X-Request-Id"

// requestIDMiddleware mints a UUID per request when no upstream id is
// present, propagates it through the context, and surfaces it in the
// response header so callers can quote it in a bug report.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if rid == "" {
			rid = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, rid)
		ctx := context.WithValue(r.Context(), requestIDKey{}, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext exposes the active request id to handlers that
// want to embed it into a structured error response.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

// accessLogMiddleware emits a single zerolog line per request. We skip
// /healthz + /readyz to keep the access log signal-to-noise high; those
// endpoints are scraped multiple times per minute by Kubernetes and
// drown out actual traffic when included.
func accessLogMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipAccessLog(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			uid := ""
			if u, ok := auth.FromContext(r.Context()); ok {
				uid = u.ID
			}
			logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.Status()).
				Int("bytes", ww.BytesWritten()).
				Dur("dur", time.Since(start)).
				Str("user_id", uid).
				Str("req_id", RequestIDFromContext(r.Context())).
				Msg("access")
		})
	}
}

func skipAccessLog(path string) bool {
	switch path {
	case "/healthz", "/readyz", "/health", "/metrics":
		return true
	}
	return false
}
