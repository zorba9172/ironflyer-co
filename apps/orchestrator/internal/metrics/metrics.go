// Package metrics exposes Prometheus-format counters / histograms for the
// orchestrator. Wire `Handler()` to /metrics and the `HTTP` middleware
// onto the chi router; agents + gates + billing can call the helpers
// directly to record domain events.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpReqs = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_http_requests_total",
		Help: "HTTP requests received by the orchestrator, partitioned by route + status class.",
	}, []string{"method", "route", "status"})

	httpDur = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ironflyer_http_request_duration_seconds",
		Help:    "HTTP request latency for the orchestrator.",
		Buckets: prometheus.ExponentialBuckets(0.005, 2, 12), // 5ms → ~10s
	}, []string{"method", "route"})

	agentRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_agent_runs_total",
		Help: "Agent invocations, partitioned by role + provider + outcome.",
	}, []string{"role", "provider", "outcome"})

	agentDur = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ironflyer_agent_duration_seconds",
		Help:    "Wall time of an agent invocation, end-to-end.",
		Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 100ms → ~50s
	}, []string{"role", "provider"})

	gateRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_gate_runs_total",
		Help: "Finisher gate checks, partitioned by gate + status.",
	}, []string{"gate", "status"})

	chargeUSD = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_charge_usd_total",
		Help: "Total provider cost charged to ledger, in USD, partitioned by provider + model.",
	}, []string{"provider", "model"})

	registry = prometheus.NewRegistry()
)

func init() {
	registry.MustRegister(
		httpReqs, httpDur, agentRuns, agentDur, gateRuns, chargeUSD,
		// Stock collectors — Go runtime + process stats.
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)
}

// Handler returns an http.Handler that serves the Prometheus text exposition
// format. Mount at /metrics; kept registry-scoped so we control exactly
// what's exposed (no default registry leakage).
func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		Registry:          registry,
	})
}

// HTTP is the chi middleware that records each request's count + latency.
// We use chi's RoutePattern for the label so cardinality stays bounded —
// dynamic IDs in the URL collapse into the static pattern that registered
// the handler.
func HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		httpReqs.WithLabelValues(r.Method, route, strconv.Itoa(ww.Status())).Inc()
		httpDur.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// ObserveAgent records an agent run's outcome + duration. Call from the
// place that actually drains the stream; outcome is "ok" | "error".
func ObserveAgent(role, provider, outcome string, dur time.Duration) {
	agentRuns.WithLabelValues(role, provider, outcome).Inc()
	agentDur.WithLabelValues(role, provider).Observe(dur.Seconds())
}

// ObserveGate records a finisher gate check outcome.
func ObserveGate(gate, status string) {
	gateRuns.WithLabelValues(gate, status).Inc()
}

// ObserveCharge records a billing charge. Called from budget.Billing.Charge
// after the ledger entry persists successfully.
func ObserveCharge(provider, model string, usd float64) {
	if usd <= 0 {
		return
	}
	chargeUSD.WithLabelValues(provider, model).Add(usd)
}
