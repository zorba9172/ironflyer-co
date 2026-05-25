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

	// Parallel critic — observability for the live-critique pipeline that
	// runs alongside a streaming Coder. `critic_partial_emitted_total`
	// climbs every time the in-flight critic produces a finding event;
	// `critic_blocker_aborted_total` only ticks when a partial critique
	// fires a `severity: blocker` and the Coder context is cancelled mid-
	// stream. `critic_partial_latency_seconds` is the wall time between
	// dispatching a partial critique provider call and emitting the
	// resulting event so operators can size the 5s tick budget.
	criticPartialEmitted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ironflyer_critic_partial_emitted_total",
		Help: "Live `critic_partial` SSE events emitted by the parallel critic.",
	})
	criticBlockerAborted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ironflyer_critic_blocker_aborted_total",
		Help: "Coder runs aborted mid-stream by a `severity: blocker` partial critique.",
	})
	criticPartialLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ironflyer_critic_partial_latency_seconds",
		Help:    "Wall time of one partial-critique provider round-trip.",
		Buckets: prometheus.ExponentialBuckets(0.5, 2, 8), // 500ms → ~64s
	})

	// Metered Stripe billing — observability for the usage-record pipeline.
	meteredFlushTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ironflyer_metered_flush_total",
		Help: "Successful Stripe UsageRecord POSTs from the metered reporter.",
	})
	meteredFlushErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ironflyer_metered_flush_errors_total",
		Help: "Failed Stripe UsageRecord POSTs from the metered reporter (post-retry).",
	})
	meteredBufferDrop = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ironflyer_metered_buffer_drop_total",
		Help: "Metered events dropped because the per-user buffer hit its cap.",
	})

	// Inline completions — Cursor-style ghost-text. The accept counter is
	// driven by the editor (/completions/inline/accept) so accept-rate is
	// computable as accepts / requests. Latency tracks first-token wall
	// time so the 400ms p95 target is enforceable in dashboards.
	inlineCompletionRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_inline_completion_requests_total",
		Help: "Inline-completion (ghost-text) requests served by the orchestrator, partitioned by outcome.",
	}, []string{"outcome"})
	inlineCompletionAccepts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ironflyer_inline_completion_accepts_total",
		Help: "Inline-completion suggestions accepted by the user (tab-to-accept).",
	})
	inlineCompletionLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ironflyer_inline_completion_latency_seconds",
		Help:    "Wall time from request to first token for inline completions.",
		Buckets: prometheus.ExponentialBuckets(0.05, 2, 10), // 50ms → ~25s
	})

	// Webhook delivery — retry pipeline observability. The attempts
	// counter is partitioned by outcome ("delivered" | "retrying" |
	// "dead") so dashboards can plot success rate. `dead` is a
	// dedicated counter so paging policies can alarm cleanly on a
	// non-zero rate.
	webhookDeliveryAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_webhook_delivery_attempts_total",
		Help: "Webhook delivery attempts, partitioned by outcome.",
	}, []string{"outcome"})
	webhookDeliveryDead = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ironflyer_webhook_delivery_dead_total",
		Help: "Webhook deliveries that exhausted retries and were dead-lettered.",
	})
	webhookDeliveryDur = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ironflyer_webhook_delivery_duration_seconds",
		Help:    "Wall time of one webhook POST attempt.",
		Buckets: prometheus.ExponentialBuckets(0.05, 2, 10), // 50ms → ~25s
	})

	// GraphQL hardening — request observability for the production
	// endpoint. `operation` is the GraphQL operation name (or
	// "anonymous"); `result` is "ok" | "error" | "complexity_rejected"
	// | "depth_rejected". The histogram buckets start at 5ms so the
	// fast end stays dense — most authenticated queries land under
	// 100ms once caches warm.
	graphqlRequestDur = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "graphql_request_duration_seconds",
		Help:    "GraphQL request latency partitioned by operation + result.",
		Buckets: prometheus.ExponentialBuckets(0.005, 2, 12), // 5ms → ~10s
	}, []string{"operation", "result"})
	graphqlComplexityRejected = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "graphql_complexity_rejected_total",
		Help: "GraphQL operations rejected because their calculated complexity exceeded the configured limit.",
	})
	graphqlDepthRejected = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "graphql_depth_rejected_total",
		Help: "GraphQL operations rejected because the document depth exceeded the configured limit.",
	})
	graphqlAPQRegister = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "graphql_apq_register_total",
		Help: "APQ register attempts (full-query+hash uploads), partitioned by allow/throttled outcome.",
	}, []string{"result"})

	// Cross-pod event bus — observability for the Redis-backed fan-out
	// underneath GraphQL subscriptions and SSE streams. topic_kind is the
	// first colon-segment of the topic (bounded cardinality:
	// collab_presence, collab_cursors, collab_chat, cost, finisher_run,
	// deploy, figma, inline). source is "local" (publish on this pod) or
	// "remote" (message arrived from another pod via Redis).
	busPublished = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bus_messages_published_total",
		Help: "Messages published to the cross-pod bus, partitioned by topic kind.",
	}, []string{"topic_kind"})
	busReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bus_messages_received_total",
		Help: "Messages delivered to bus subscribers, partitioned by topic kind and source (local|remote).",
	}, []string{"topic_kind", "source"})
	busSubDrop = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bus_subscriber_drop_total",
		Help: "Messages dropped because a subscriber's channel was full (slow consumer).",
	}, []string{"topic_kind"})
	busActiveSubs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bus_active_subscribers",
		Help: "Current count of in-process bus subscribers, partitioned by topic kind.",
	}, []string{"topic_kind"})

	// Provider circuit breakers — observability for the per-provider
	// breaker layer that sits between the failover chain and each
	// upstream. State is exposed as a per-state gauge so dashboards can
	// draw "anthropic is open" with a single PromQL series rather than
	// decoding numeric enum values. Trips is a counter so paging policies
	// can alarm on a sudden burst (a provider that flaps open/closed many
	// times in a few minutes is a different kind of incident than a
	// provider that opens once and stays open).
	providerBreakerState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ironflyer_provider_breaker_state",
		Help: "Current circuit-breaker state per provider (1 = matches the given state label, 0 otherwise).",
	}, []string{"provider", "state"})
	providerBreakerTrips = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_provider_breaker_trips_total",
		Help: "Cumulative count of breaker open transitions per provider.",
	}, []string{"provider"})
	providerRequestDur = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ironflyer_provider_request_duration_seconds",
		Help:    "Wall time of one provider call, partitioned by provider + outcome (success|fail|circuit_open).",
		Buckets: prometheus.ExponentialBuckets(0.05, 2, 12), // 50ms → ~100s
	}, []string{"provider", "outcome"})

	// Provider registration — gauge set to 1 at startup for every
	// provider whose credentials were present, 0 for providers that
	// were gated off due to missing creds. Dashboards page on this
	// to answer "which LLM backends are actually online right now?"
	// without grepping the boot logs.
	providerRegistered = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ironflyer_provider_registered",
		Help: "1 when the provider is registered with credentials present at startup; 0 when gated off due to missing creds.",
	}, []string{"provider"})

	// Vercel deploy adapter — outcome counter + wall-time histogram.
	// `outcome` is one of "success", "failed", "ensure_project_failed",
	// "create_deployment_failed", "stream_error", "status_error". The
	// histogram covers the full deploy lifetime from Deploy() entry
	// through the terminal event so dashboards can alarm on slow deploys
	// in addition to outright failures.
	vercelDeployTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_vercel_deploy_total",
		Help: "Vercel deploy attempts handled by the VercelAdapter, partitioned by outcome.",
	}, []string{"outcome"})
	vercelDeployDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ironflyer_vercel_deploy_duration_seconds",
		Help:    "Wall time of one Vercel deploy from adapter entry to terminal event.",
		Buckets: prometheus.ExponentialBuckets(0.5, 2, 12), // 500ms → ~30m
	})

	// Memory subsystem — per-backend operation counters + search latency.
	// `backend` is one of "memory", "surreal", "pgvector"; `op` is one of
	// "add", "get", "search", "list", "delete". Operators alarm on a sudden
	// dip in `op="search"` rate (signals embedder outage) and on a P95
	// blow-up in `memory_search_duration_seconds` (HNSW index degradation).
	memoryOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_memory_operations_total",
		Help: "Memory-store operations, partitioned by backend and op.",
	}, []string{"backend", "op"})
	memorySearchDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ironflyer_memory_search_duration_seconds",
		Help:    "Wall time of a memory search/list call.",
		Buckets: prometheus.ExponentialBuckets(0.005, 2, 12), // 5ms → ~10s
	}, []string{"backend"})

	// Patch lifecycle — kind + size counters consumed by the patch
	// engine's Propose path. patch_kind_total partitions over the
	// {create,update,delete,replace,insert_after,symbol_replace}
	// vocabulary so the dashboard can show which Op the AI prefers;
	// patch_size_bytes is the body+replacement+symbol-source size so
	// operators can spot pathological mega-patches.
	patchKindTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_patch_kind_total",
		Help: "Patch changes proposed, partitioned by Op kind.",
	}, []string{"kind"})
	patchSizeBytes = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ironflyer_patch_size_bytes",
		Help:    "Per-change content + replacement + symbol-source byte size at Propose time.",
		Buckets: prometheus.ExponentialBuckets(64, 2, 16), // 64B → ~2MB
	})
	patchApplyOutcome = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_patch_apply_outcome_total",
		Help: "Patch Apply outcomes, partitioned by status (applied|rejected|conflict|rollback).",
	}, []string{"outcome"})

	// Gate telemetry — per-gate duration histogram + per-severity findings
	// counter. The finisher engine wires these around each Gate.Check()
	// call so dashboards can answer "which gate is slowest?" and "which
	// gate produces the most fails by severity?" without re-deriving the
	// data from log scrapes. Bucket layout starts at 50ms because a
	// regex scan over a small project really does finish that fast; the
	// long tail reaches ~30 minutes for slow real-world build/test runs.
	gateDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ironflyer_gate_duration_seconds",
		Help:    "Wall time of one finisher Gate.Check call, partitioned by gate and outcome (pass|fail|warn|error).",
		Buckets: prometheus.ExponentialBuckets(0.05, 2, 16), // 50ms → ~30m
	}, []string{"gate", "outcome"})
	gateFindingsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_gate_findings_total",
		Help: "Findings produced by finisher gates, partitioned by gate and severity (critical|high|medium|low|info).",
	}, []string{"gate", "severity"})

	// Billing commercial surface — counters + gauges added in Round 14
	// alongside Stripe Tax / Customer Portal / refund / dunning /
	// invoice list. kind partitions {checkout, cancel, refund,
	// dunning_entered, dunning_cleared, dunning_paused, portal,
	// invoice_listed, tax_applied}. The dunning gauge is set every
	// minute by the reconciler so dashboards can plot "users currently
	// in dunning" as a single PromQL series.
	billingEventTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_billing_event_total",
		Help: "Billing events partitioned by kind (checkout|cancel|refund|dunning_*|portal|invoice_listed|tax_applied).",
	}, []string{"kind"})
	billingDunningActiveUsers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ironflyer_billing_dunning_active_users",
		Help: "Users currently in any dunning state (retry_1|retry_2|giving_up|paused).",
	})
	billingInvoiceAmountCents = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ironflyer_billing_invoice_amount_cents",
		Help:    "Distribution of Stripe invoice amounts at payment time, in cents.",
		Buckets: prometheus.ExponentialBuckets(100, 2, 16), // $1 → ~$32k
	})

	// Auth subsystem — commercial table-stakes (verification + reset +
	// MFA + sessions + email change). `kind` mirrors the audit
	// vocabulary so dashboards can correlate one Counter row to one
	// audit chain entry.
	authEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_auth_event_total",
		Help: "Auth lifecycle events, partitioned by kind (signup_completed, email_verified, password_reset_requested, password_reset_completed, mfa_enrolled, mfa_confirmed, mfa_disabled, mfa_recovery_used, session_revoked, email_change_requested, email_change_completed).",
	}, []string{"kind"})
	authMfaEnrolledUsers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ironflyer_auth_mfa_enrolled_users",
		Help: "Sampled count of users with MFA confirmed_at IS NOT NULL.",
	})
	authSessionAgeSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ironflyer_auth_session_age_seconds",
		Help:    "Age of a session at the moment of revocation or expiry.",
		Buckets: prometheus.ExponentialBuckets(60, 4, 12), // 1m → ~4w
	})

	registry = prometheus.NewRegistry()
)

func init() {
	registry.MustRegister(
		httpReqs, httpDur, agentRuns, agentDur, gateRuns, chargeUSD,
		criticPartialEmitted, criticBlockerAborted, criticPartialLatency,
		meteredFlushTotal, meteredFlushErrors, meteredBufferDrop,
		inlineCompletionRequests, inlineCompletionAccepts, inlineCompletionLatency,
		webhookDeliveryAttempts, webhookDeliveryDead, webhookDeliveryDur,
		graphqlRequestDur, graphqlComplexityRejected, graphqlDepthRejected, graphqlAPQRegister,
		busPublished, busReceived, busSubDrop, busActiveSubs,
		providerBreakerState, providerBreakerTrips, providerRequestDur,
		providerRegistered,
		vercelDeployTotal, vercelDeployDuration,
		memoryOperations, memorySearchDuration,
		patchKindTotal, patchSizeBytes, patchApplyOutcome,
		gateDurationSeconds, gateFindingsTotal,
		billingEventTotal, billingDunningActiveUsers, billingInvoiceAmountCents,
		authEvents, authMfaEnrolledUsers, authSessionAgeSeconds,
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

// ObserveCriticPartialEmitted records one live `critic_partial` event so the
// parallel-critic dashboard can plot critique throughput against Coder
// streaming progress.
func ObserveCriticPartialEmitted() { criticPartialEmitted.Inc() }

// ObserveCriticBlockerAborted records a mid-stream Coder abort triggered by a
// `severity: blocker` partial critique. Operators alarm on a sudden uptick —
// either the Coder is degrading or the Critic is over-eager.
func ObserveCriticBlockerAborted() { criticBlockerAborted.Inc() }

// ObserveCriticPartialLatency records the wall time of one partial-critique
// provider round-trip so the 5s tick budget can be tuned against the
// observed P95.
func ObserveCriticPartialLatency(dur time.Duration) {
	criticPartialLatency.Observe(dur.Seconds())
}

// ObserveMeteredFlush bumps the success counter for the metered reporter.
func ObserveMeteredFlush() { meteredFlushTotal.Inc() }

// ObserveMeteredFlushError bumps the error counter for the metered reporter.
func ObserveMeteredFlushError() { meteredFlushErrors.Inc() }

// ObserveMeteredBufferDrop bumps the drop counter when a per-user buffer is
// full. Operators alarm on a non-zero rate.
func ObserveMeteredBufferDrop() { meteredBufferDrop.Inc() }

// ObserveInlineCompletionRequest records one inline-completion attempt.
// `outcome` is one of "served" (stream completed), "dropped" (exceeded
// the 1.5s latency budget), "budget" (BillingGuard refused), or "error"
// (provider failure).
func ObserveInlineCompletionRequest(outcome string) {
	inlineCompletionRequests.WithLabelValues(outcome).Inc()
}

// ObserveInlineCompletionAccept bumps the accept counter. The extension
// hits POST /completions/inline/accept on tab so accept-rate is just
// accepts / requests in the dashboard.
func ObserveInlineCompletionAccept() { inlineCompletionAccepts.Inc() }

// ObserveInlineCompletionLatency records the wall time to first token.
// Target p95 is 400ms; the histogram bucket layout starts at 50ms so the
// short end is dense.
func ObserveInlineCompletionLatency(dur time.Duration) {
	inlineCompletionLatency.Observe(dur.Seconds())
}

// ObserveWebhookDeliveryAttempt records one webhook POST outcome.
// `outcome` is one of "delivered" (2xx response), "retrying"
// (transient failure, will retry), or "dead" (final failure after
// all retries exhausted).
func ObserveWebhookDeliveryAttempt(outcome string) {
	webhookDeliveryAttempts.WithLabelValues(outcome).Inc()
}

// ObserveWebhookDeliveryDead bumps the dead-letter counter. Operators
// alarm on a non-zero rate — a healthy fleet should never have a
// dead-lettered delivery.
func ObserveWebhookDeliveryDead() { webhookDeliveryDead.Inc() }

// ObserveWebhookDeliveryDuration records the wall time of one
// webhook POST attempt (network round-trip + body read).
func ObserveWebhookDeliveryDuration(dur time.Duration) {
	webhookDeliveryDur.Observe(dur.Seconds())
}

// ObserveGraphQLRequest records one GraphQL operation's wall time and
// outcome. `operation` falls back to "anonymous" for unnamed queries so
// the cardinality stays bounded; `result` is one of "ok", "error",
// "complexity_rejected", "depth_rejected".
func ObserveGraphQLRequest(operation, result string, dur time.Duration) {
	if operation == "" {
		operation = "anonymous"
	}
	if result == "" {
		result = "ok"
	}
	graphqlRequestDur.WithLabelValues(operation, result).Observe(dur.Seconds())
}

// ObserveGraphQLComplexityRejected bumps the counter for a complexity
// rejection. Operators alarm on a non-zero rate — sustained traffic
// hitting the limit usually signals either a misbehaving client or a
// limit that's too tight.
func ObserveGraphQLComplexityRejected() { graphqlComplexityRejected.Inc() }

// ObserveGraphQLDepthRejected bumps the counter for a depth rejection.
// A non-zero rate is suspicious — clients should not legitimately ask
// for documents deeper than the configured limit.
func ObserveGraphQLDepthRejected() { graphqlDepthRejected.Inc() }

// ObserveGraphQLAPQRegister records one APQ register attempt. `result`
// is "allow" when the register went through the per-IP rate limiter or
// "throttled" when it was blocked.
func ObserveGraphQLAPQRegister(result string) {
	if result == "" {
		result = "allow"
	}
	graphqlAPQRegister.WithLabelValues(result).Inc()
}

// ObserveBusPublish bumps the publish counter for the cross-pod event
// bus. `kind` is the first colon-segment of the topic so cardinality is
// bounded by the small set of known kinds (collab_presence, cost, etc).
func ObserveBusPublish(kind string) {
	if kind == "" {
		kind = "unknown"
	}
	busPublished.WithLabelValues(kind).Inc()
}

// ObserveBusReceive bumps the delivery counter for a bus message
// reaching a local subscriber. `source` is "local" when the publisher
// was on this pod (same-pod fan-out) or "remote" when the message came
// in over Redis from another pod.
func ObserveBusReceive(kind, source string) {
	if kind == "" {
		kind = "unknown"
	}
	if source != "remote" {
		source = "local"
	}
	busReceived.WithLabelValues(kind, source).Inc()
}

// ObserveBusSubscriberDrop ticks every time a per-subscriber buffer
// overflows and the bus drops the oldest message. A non-zero rate
// signals either a stuck consumer or under-sized SubBuffer.
func ObserveBusSubscriberDrop(kind string) {
	if kind == "" {
		kind = "unknown"
	}
	busSubDrop.WithLabelValues(kind).Inc()
}

// SetProviderBreakerState updates the per-state gauge so that the
// gauge for `state` reads 1 and the gauges for the other two states
// read 0. Operators graph `ironflyer_provider_breaker_state{provider=
// "anthropic",state="open"} == 1` to see when a provider is locked
// out. Valid states: "closed", "open", "half_open".
func SetProviderBreakerState(provider, state string) {
	if provider == "" {
		provider = "unknown"
	}
	for _, s := range []string{"closed", "open", "half_open"} {
		v := 0.0
		if s == state {
			v = 1
		}
		providerBreakerState.WithLabelValues(provider, s).Set(v)
	}
}

// ObserveProviderBreakerTrip ticks the trip counter when a provider's
// breaker opens (i.e. transitions to the open state). Alarm on a
// non-zero rate per provider.
func ObserveProviderBreakerTrip(provider string) {
	if provider == "" {
		provider = "unknown"
	}
	providerBreakerTrips.WithLabelValues(provider).Inc()
}

// ObserveProviderRequest records one provider call's wall time and
// outcome. `outcome` is one of "success", "fail", "circuit_open".
// "fail" covers every error classification (5xx, 429, network,
// timeout, and caller-side 4xx) — the per-state breakdown lives in
// the existing failover penalty logs; this histogram is a coarse
// latency signal partitioned only by win/lose.
func ObserveProviderRequest(provider, outcome string, dur time.Duration) {
	if provider == "" {
		provider = "unknown"
	}
	switch outcome {
	case "success", "fail", "circuit_open":
	default:
		outcome = "fail"
	}
	providerRequestDur.WithLabelValues(provider, outcome).Observe(dur.Seconds())
}

// SetProviderRegistered records whether a provider is online. Called
// once at startup per known provider name: `enabled=true` when its
// credentials were present and the provider was registered with the
// router, `false` when the provider was gated off (missing API key,
// missing required config). Operators can query the gauge in PromQL
// to confirm which LLM backends are live in any given environment.
func SetProviderRegistered(provider string, enabled bool) {
	if provider == "" {
		provider = "unknown"
	}
	v := 0.0
	if enabled {
		v = 1
	}
	providerRegistered.WithLabelValues(provider).Set(v)
}

// SetBusActiveSubscribers updates the gauge of currently-registered
// in-process bus subscribers for a topic kind. Called by the
// Multiplexer on every Subscribe/cancel.
func SetBusActiveSubscribers(kind string, n int) {
	if kind == "" {
		kind = "unknown"
	}
	busActiveSubs.WithLabelValues(kind).Set(float64(n))
}

// ObserveVercelDeploy records one Vercel deploy attempt's outcome and
// wall time. `outcome` is one of "success", "failed",
// "ensure_project_failed", "create_deployment_failed", "stream_error",
// "status_error". Operators alarm on `outcome="failed"` over a window
// and use the duration histogram for slow-deploy warnings.
func ObserveVercelDeploy(outcome string, dur time.Duration) {
	if outcome == "" {
		outcome = "unknown"
	}
	vercelDeployTotal.WithLabelValues(outcome).Inc()
	vercelDeployDuration.Observe(dur.Seconds())
}

// ObservePatchKind ticks the patch-kind counter for one FileChange Op.
// Called from the patch engine's Propose path.
func ObservePatchKind(kind string) {
	if kind == "" {
		kind = "unknown"
	}
	patchKindTotal.WithLabelValues(kind).Inc()
}

// ObservePatchSize records the byte size of one FileChange's payload
// (content + replacement + symbol-source) so dashboards can spot
// pathological mega-patches before they hit Apply.
func ObservePatchSize(bytes int) {
	if bytes < 0 {
		return
	}
	patchSizeBytes.Observe(float64(bytes))
}

// ObservePatchApplyOutcome ticks the apply-outcome counter. Callers
// pass one of "applied", "rejected", "conflict", "rollback"; anything
// else collapses to "unknown" so cardinality stays bounded.
func ObservePatchApplyOutcome(outcome string) {
	switch outcome {
	case "applied", "rejected", "conflict", "rollback":
	default:
		outcome = "unknown"
	}
	patchApplyOutcome.WithLabelValues(outcome).Inc()
}

// ObserveBillingEvent ticks the billing-event counter for one operation
// kind. Callers pass one of: "checkout", "cancel", "refund",
// "dunning_entered", "dunning_cleared", "dunning_paused", "portal",
// "invoice_listed", "tax_applied". Unknown kinds collapse to "unknown"
// so cardinality stays bounded.
func ObserveBillingEvent(kind string) {
	if kind == "" {
		kind = "unknown"
	}
	billingEventTotal.WithLabelValues(kind).Inc()
}

// SetBillingDunningActiveUsers updates the gauge of users currently in
// any dunning state. Called once per reconciler tick (default 1 min).
func SetBillingDunningActiveUsers(n int) {
	if n < 0 {
		n = 0
	}
	billingDunningActiveUsers.Set(float64(n))
}

// ObserveBillingInvoiceAmount records one invoice amount (cents) so the
// distribution histogram fills in. Negative values are silently ignored
// (refunds land on the billing-event counter instead).
func ObserveBillingInvoiceAmount(cents int64) {
	if cents <= 0 {
		return
	}
	billingInvoiceAmountCents.Observe(float64(cents))
}

// ObserveGateDuration records the wall time of one finisher Gate.Check
// call. `outcome` is one of "pass", "fail", "warn", "error"; unknown
// values collapse to "error" so the dashboard never has a phantom bucket.
func ObserveGateDuration(gate, outcome string, dur time.Duration) {
	if gate == "" {
		gate = "unknown"
	}
	switch outcome {
	case "pass", "fail", "warn", "error":
	default:
		outcome = "error"
	}
	gateDurationSeconds.WithLabelValues(gate, outcome).Observe(dur.Seconds())
}

// ObserveGateFinding ticks the per-gate, per-severity findings counter.
// `severity` is normalised to one of {critical, high, medium, low,
// info}; the legacy domain.Severity vocabulary maps as:
//
//	critical → critical
//	error    → high
//	warning  → medium
//	info     → info
//
// Unknown labels collapse to "info" so cardinality stays bounded.
func ObserveGateFinding(gate, severity string) {
	if gate == "" {
		gate = "unknown"
	}
	switch severity {
	case "critical", "high", "medium", "low", "info":
	default:
		severity = "info"
	}
	gateFindingsTotal.WithLabelValues(gate, severity).Inc()
}

// ObserveMemoryOp records one memory-store operation. `backend` is the
// resolved backend ("memory" | "surreal" | "pgvector"); `op` is one of
// "add", "get", "search", "list", "delete". `dur` is the wall time of
// the call — it lands in the search-duration histogram for op="search"
// and op="list" so operators can spot HNSW / index slowdowns.
func ObserveMemoryOp(backend, op string, dur time.Duration) {
	if backend == "" {
		backend = "unknown"
	}
	if op == "" {
		op = "unknown"
	}
	memoryOperations.WithLabelValues(backend, op).Inc()
	if op == "search" || op == "list" {
		memorySearchDuration.WithLabelValues(backend).Observe(dur.Seconds())
	}
}

// ObserveAuthEvent bumps the auth-lifecycle counter. `kind` is one of
// the constants documented on authEvents. Resolvers call this after the
// audit row lands so the counter never out-runs the audit chain.
func ObserveAuthEvent(kind string) {
	if kind == "" {
		return
	}
	authEvents.WithLabelValues(kind).Inc()
}

// SetMfaEnrolledUsers updates the sampled gauge of users with MFA
// active. The orchestrator runs a slow background sweep that calls this
// every few minutes so the gauge stays cheap to query.
func SetMfaEnrolledUsers(count int) {
	if count < 0 {
		return
	}
	authMfaEnrolledUsers.Set(float64(count))
}

// ObserveSessionAge records the age of a session at the moment of
// revocation (or expiry). `since` is the issued_at timestamp.
func ObserveSessionAge(since time.Time) {
	if since.IsZero() {
		return
	}
	authSessionAgeSeconds.Observe(time.Since(since).Seconds())
}
