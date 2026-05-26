package httpapi

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/operations/logctx"
)

// RequestIDMiddleware mints (or honours) X-Request-ID, stamps it on
// the response header and on ctx via logctx.WithRequestID, and forwards
// the canonical base logger so logctx.From(ctx) returns a decorated
// logger downstream. The middleware MUST run BEFORE auth so auth
// failures themselves carry a request_id in their log lines.
//
// The tenant id is best-effort: if auth resolves later in the chain
// the per-route resolvers will re-stamp via logctx.WithTenantID. For
// routes that do their own auth (the optional auth on /budget/plans
// etc.) the tenant id is left empty here.
func RequestIDMiddleware(baseLogger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
			if id == "" {
				id = uuid.NewString()
			}
			w.Header().Set("X-Request-ID", id)

			ctx := logctx.WithRequestID(r.Context(), id)
			// Auth runs AFTER this middleware on protected routes, but
			// the GraphQL handler runs auth.Optional implicitly (the
			// resolver layer calls auth.FromContext). If the upstream
			// chain has already attached a user (e.g. a future change),
			// surface tenant_id immediately.
			if u, ok := auth.FromContext(ctx); ok {
				tenant := u.OrgID
				if tenant == "" {
					tenant = u.ID
				}
				ctx = logctx.WithTenantID(ctx, tenant)
			}
			ctx = logctx.ContextWithLogger(ctx, baseLogger)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TenantStampMiddleware re-stamps tenant_id on ctx once auth has run.
// Mount this AFTER auth.Middleware on protected routes so the
// decorated logger sees tenant_id even though RequestIDMiddleware ran
// before auth had a chance to attach a user.
//
// On routes where this middleware never runs the resolver layer can
// still call logctx.WithTenantID directly.
func TenantStampMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if logctx.TenantID(ctx) == "" {
			if u, ok := auth.FromContext(ctx); ok {
				tenant := u.OrgID
				if tenant == "" {
					tenant = u.ID
				}
				ctx = logctx.WithTenantID(ctx, tenant)
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
