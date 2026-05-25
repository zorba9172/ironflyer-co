package gqlhardening

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics surfacing — registered once via RegisterMetrics by the
// wiring agent. Kept local so the package stays self-contained.
var (
	depthRejects = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "graphql_depth_rejects_total",
		Help: "GraphQL operations rejected because depth exceeded the limit.",
	}, []string{"operation"})

	complexityRejects = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "graphql_complexity_rejects_total",
		Help: "GraphQL operations rejected because complexity exceeded the limit.",
	}, []string{"operation"})

	rateLimitRejects = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "graphql_rate_limit_rejects_total",
		Help: "GraphQL operations rejected by the per-tenant/operation/abuse-tier limiter.",
	}, []string{"operation", "tier"})

	introspectionRejects = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "graphql_introspection_rejects_total",
		Help: "GraphQL operations rejected because introspection is gated in production.",
	})

	persistedHits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "graphql_persisted_query_hits_total",
		Help: "Persisted-query allowlist hits / misses.",
	}, []string{"outcome"})

	csrfRejects = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "graphql_csrf_rejects_total",
		Help: "Browser GraphQL requests rejected by the CSRF double-submit check.",
	})

	originRejects = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "graphql_ws_origin_rejects_total",
		Help: "WebSocket upgrades rejected by the origin allowlist.",
	})

	once sync.Once
)

// RegisterMetrics is idempotent; called by the wiring agent during
// orchestrator startup.
func RegisterMetrics(reg prometheus.Registerer) {
	if reg == nil {
		return
	}
	once.Do(func() {
		reg.MustRegister(
			depthRejects,
			complexityRejects,
			rateLimitRejects,
			introspectionRejects,
			persistedHits,
			csrfRejects,
			originRejects,
		)
	})
}
