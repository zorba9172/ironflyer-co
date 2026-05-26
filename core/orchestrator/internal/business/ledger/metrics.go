package ledger

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// ironflyer_ledger_entries_total is the operator-facing pulse on the
// V22 ledger. Every successful Write across every backend ticks it,
// partitioned by entry_type + direction so a Grafana panel can plot
// "wallet top-ups vs. provider cost vs. platform margin" without
// scanning Postgres.
//
// Registration is done once via sync.Once so multiple test binaries
// (or, in production, multiple Service constructions sharing a
// process) don't double-register and panic the default registry.
var (
	entriesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_ledger_entries_total",
		Help: "Append-only ledger entries written by the orchestrator, partitioned by entry_type + direction.",
	}, []string{"type", "direction"})

	registerOnce sync.Once
)

// registerMetrics is called lazily from the first Write so the
// metrics surface exists even when no Service constructor was used
// (the in-memory backend is sometimes constructed via a struct
// literal in cold paths).
func registerMetrics() {
	registerOnce.Do(func() {
		// Use a Register-and-tolerate-duplicate pattern so importing
		// the package in two distinct binaries within the same
		// process (rare, but possible in cmd/) does not panic.
		if err := prometheus.Register(entriesTotal); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				if existing, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
					entriesTotal = existing
				}
			}
		}
	})
}

// observeWrite increments the per-(type,direction) counter. Safe to
// call from any goroutine.
func observeWrite(e Entry) {
	registerMetrics()
	entriesTotal.WithLabelValues(string(e.EntryType), string(e.Direction)).Inc()
}
