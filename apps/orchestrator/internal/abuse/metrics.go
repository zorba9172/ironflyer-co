package abuse

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics surfaces the abuse engine on the orchestrator's Prometheus
// registry. They are kept in this package (rather than the central
// internal/metrics package) so the abuse engine remains
// self-contained — the integration agent wires the registry once in
// main.go.
var (
	signalsRecorded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "abuse_signals_recorded_total",
		Help: "Count of abuse signals recorded, keyed by signal type.",
	}, []string{"signal_type"})

	scoreChanged = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "abuse_score_changes_total",
		Help: "Count of tier transitions across all (tenant,user) pairs.",
	}, []string{"from_tier", "to_tier"})

	currentTier = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "abuse_current_tier",
		Help: "Number of (tenant,user) pairs currently in each tier.",
	}, []string{"tier"})

	registerOnce sync.Once
)

// RegisterMetrics is idempotent and safe to call from package init or
// from the wiring agent. It is a no-op on subsequent calls.
func RegisterMetrics(reg prometheus.Registerer) {
	if reg == nil {
		return
	}
	registerOnce.Do(func() {
		reg.MustRegister(signalsRecorded, scoreChanged, currentTier)
	})
}

func observeSignal(st SignalType) {
	signalsRecorded.WithLabelValues(string(st)).Inc()
}

func observeTransition(from, to Tier) {
	if from == to {
		return
	}
	scoreChanged.WithLabelValues(string(from), string(to)).Inc()
}
