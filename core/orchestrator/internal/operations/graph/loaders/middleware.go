// Per-request middleware wiring for the graph-gophers dataloaders.
//
// The middleware constructs a fresh *Loaders on every HTTP request and
// stashes it on the request context. Resolvers read it back via
// FromContext. The lifetime is intentionally request-scoped: dataloader
// caches are not invalidated when the underlying store mutates so a
// process-wide cache would serve stale data. The win is intra-request
// deduplication ("50 projects, each rendering owner.email" becomes one
// user batch instead of fifty point reads).

package loaders

import (
	"context"
	"net/http"
)

// ctxKey is unexported so consumers must use FromContext to retrieve
// the loaders.
type ctxKey struct{}

var loadersKey = ctxKey{}

// WithLoaders returns an http middleware that constructs a fresh set of
// loaders per request and stuffs them on ctx. The middleware is safe
// to mount unconditionally — when deps are zero-valued the loaders
// return ErrNotFound per key so resolvers still behave consistently.
//
// Usage from the GraphQL handler:
//
//	chain := loaders.WithLoaders(deps)(graphHandler)
//	r.Method("POST", "/graphql", chain)
func WithLoaders(deps LoaderDeps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := Inject(r.Context(), NewLoaders(deps))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Inject puts ldrs on ctx. Exposed so non-HTTP entry points (websocket
// upgrade, background workers) can also opt into per-call loaders
// without going through the http middleware.
func Inject(ctx context.Context, ldrs *Loaders) context.Context {
	if ldrs == nil {
		return ctx
	}
	return context.WithValue(ctx, loadersKey, ldrs)
}

// FromContext returns the per-request loaders, or nil when the
// middleware did not run for this request. Callers should nil-check —
// a nil *Loaders means "fall back to the un-batched store call" so the
// orchestrator stays robust even if a future code path forgets to mount
// the middleware.
func FromContext(ctx context.Context) *Loaders {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(loadersKey).(*Loaders)
	return v
}
