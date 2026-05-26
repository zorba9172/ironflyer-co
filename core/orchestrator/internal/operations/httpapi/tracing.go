package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"ironflyer/core/orchestrator/internal/operations/tracing"
)

// otelHTTP opens a span per HTTP request using the orchestrator's
// global tracer. Span name is the chi route pattern when the request
// matched a route (bounded cardinality), or "<method> unmatched"
// otherwise. Inline rather than pulling in otelhttp because the
// orchestrator does not already depend on go.opentelemetry.io/contrib.
func otelHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		name := r.Method + " " + route
		ctx, span := tracing.StartSpan(r.Context(), name,
			semconv.HTTPRequestMethodKey.String(r.Method),
			semconv.URLPathKey.String(r.URL.Path),
			attribute.String("http.route", route),
		)
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
