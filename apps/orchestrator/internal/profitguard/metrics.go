package profitguard

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// ironflyer_profitguard_decisions_total is the operator-facing pulse
// on Profit Guard. Every Decide() that returns a verdict ticks it,
// partitioned by enforcement_point + action so Grafana can plot
// "kill_branch vs. switch_provider vs. pause_for_budget" across the
// fleet without scanning the audit table.
//
// Registration is gated by sync.Once so multiple Guard constructions
// in the same process do not double-register and panic the default
// registry.
var (
	decisionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_profitguard_decisions_total",
		Help: "Profit Guard decisions emitted, partitioned by enforcement_point + action.",
	}, []string{"enforcement_point", "action"})

	registerOnce sync.Once
)

// registerMetrics lazily registers the counter on first observation.
// We tolerate AlreadyRegistered so importing the package in two
// distinct cmd/ binaries within a single process doesn't panic.
func registerMetrics() {
	registerOnce.Do(func() {
		if err := prometheus.Register(decisionsTotal); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				if existing, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
					decisionsTotal = existing
				}
			}
		}
	})
}

// observeDecision increments the per-(point, action) counter. Safe to
// call from any goroutine.
func observeDecision(point EnforcementPoint, action Action) {
	registerMetrics()
	decisionsTotal.WithLabelValues(string(point), string(action)).Inc()
}
