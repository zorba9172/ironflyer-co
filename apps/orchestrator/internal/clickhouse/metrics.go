package clickhouse

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus metrics for the analytics plane. The orchestrator's
// metrics handler already mounts the global registry, so we just
// register on package init via registerOnce.
var (
	consumerEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_clickhouse_consumer_events_total",
		Help: "Events consumed from Redpanda and inserted into ClickHouse raw_* tables, by topic and outcome.",
	}, []string{"topic", "outcome"})

	consumerLagSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ironflyer_clickhouse_consumer_lag_seconds",
		Help: "Age of the most recent committed event in the consumer loop, by topic.",
	}, []string{"topic"})

	ingesterRowsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_clickhouse_ingester_rows_total",
		Help: "Outbox rows tailed by the fallback ingester and inserted into ClickHouse, by table and outcome.",
	}, []string{"table", "outcome"})

	insertLatencySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ironflyer_clickhouse_insert_latency_seconds",
		Help:    "Latency of a ClickHouse INSERT batch, by raw_* table.",
		Buckets: prometheus.DefBuckets,
	}, []string{"table"})

	registerOnce sync.Once
)

// register installs the metrics on the global registry, tolerating
// AlreadyRegisteredError so multi-instance test harnesses or repeated
// init paths do not panic.
func register() {
	registerOnce.Do(func() {
		for _, c := range []prometheus.Collector{
			consumerEventsTotal,
			consumerLagSeconds,
			ingesterRowsTotal,
			insertLatencySeconds,
		} {
			if err := prometheus.Register(c); err != nil {
				if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
					switch existing := are.ExistingCollector.(type) {
					case *prometheus.CounterVec:
						switch c {
						case consumerEventsTotal:
							consumerEventsTotal = existing
						case ingesterRowsTotal:
							ingesterRowsTotal = existing
						}
					case *prometheus.GaugeVec:
						if c == consumerLagSeconds {
							consumerLagSeconds = existing
						}
					case *prometheus.HistogramVec:
						if c == insertLatencySeconds {
							insertLatencySeconds = existing
						}
					}
				}
			}
		}
	})
}
