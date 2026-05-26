package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ironflyer/core/runtime/internal/customer/auth"
)

// requestIDKey scopes the request ID inside the request context.
type requestIDKey struct{}

// requestIDHeader is honoured for upstream propagation and surfaced on
// every response so a developer can correlate a 500 to a log line.
const requestIDHeader = "X-Request-Id"

// requestIDMiddleware mints a UUID per request unless an upstream proxy
// already set one. The id is exported via X-Request-Id and the context.
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
// want to embed it in error payloads.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

// accessLogMiddleware emits one structured log line per request. We
// skip /healthz, /readyz, terminal WS upgrades, and the preview proxy —
// each of those is either too noisy or already covered by a focused log.
func accessLogMiddleware(logger zerolog.Logger, previewPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipAccessLog(r.URL.Path, previewPrefix) {
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

func skipAccessLog(path, previewPrefix string) bool {
	switch path {
	case "/healthz", "/readyz", "/health":
		return true
	}
	if strings.HasSuffix(path, "/terminal") {
		return true
	}
	if previewPrefix != "" && strings.HasPrefix(path, previewPrefix+"/") {
		return true
	}
	return false
}
