// Package clickhouse owns the orchestrator's analytics plane.
//
// Responsibilities:
//
//   - Connect to a ClickHouse cluster (`Client`).
//   - Apply the embedded DDL at boot (`Client.Bootstrap`).
//   - Ingest Redpanda events into the raw_* tables (`Consumer`).
//   - Fall back to direct Postgres-outbox tailing when Redpanda is
//     disabled (`OutboxIngester`).
//   - Implement the dashboards.* source interfaces against rollup
//     tables (`adapters.go`).
//
// Postgres remains the source of truth; ClickHouse is a replayable
// projection.
package clickhouse

import (
	"os"

	"ironflyer/core/orchestrator/internal/pkg/env"
)

// Config bundles every IRONFLYER_CLICKHOUSE_* env knob. Enabled is
// derived from the host list — when no host is configured the rest of
// the package becomes a no-op so dev and CI work without a CH cluster.
type Config struct {
	Hosts            []string // IRONFLYER_CLICKHOUSE_HOSTS — comma-separated host:port list
	Database         string   // IRONFLYER_CLICKHOUSE_DATABASE — defaults to "ironflyer"
	Username         string   // IRONFLYER_CLICKHOUSE_USERNAME — defaults to "default"
	Password         string   // IRONFLYER_CLICKHOUSE_PASSWORD
	Compression      bool     // IRONFLYER_CLICKHOUSE_COMPRESSION — defaults true
	DialTimeoutMS    int      // IRONFLYER_CLICKHOUSE_DIAL_TIMEOUT_MS — defaults to 5000
	Secure           bool     // IRONFLYER_CLICKHOUSE_SECURE — TLS on the wire, defaults false
	IngestFromOutbox bool     // IRONFLYER_CLICKHOUSE_INGEST_FROM_OUTBOX — direct outbox→CH fallback
	Enabled          bool     // derived: at least one host non-empty
}

// LoadConfig reads every IRONFLYER_CLICKHOUSE_* env var with sane
// defaults. Hosts is the only mandatory knob — without it the package
// is disabled and all wired-up callers must be tolerant of nil/no-op
// returns.
func LoadConfig() Config {
	c := Config{
		Hosts:            env.StringCSV("IRONFLYER_CLICKHOUSE_HOSTS"),
		Database:         env.String("IRONFLYER_CLICKHOUSE_DATABASE", "ironflyer"),
		Username:         env.String("IRONFLYER_CLICKHOUSE_USERNAME", "default"),
		Password:         os.Getenv("IRONFLYER_CLICKHOUSE_PASSWORD"),
		Compression:      env.Bool("IRONFLYER_CLICKHOUSE_COMPRESSION", true),
		DialTimeoutMS:    env.Int("IRONFLYER_CLICKHOUSE_DIAL_TIMEOUT_MS", 5000),
		Secure:           env.Bool("IRONFLYER_CLICKHOUSE_SECURE", false),
		IngestFromOutbox: env.Bool("IRONFLYER_CLICKHOUSE_INGEST_FROM_OUTBOX", false),
	}
	c.Enabled = len(c.Hosts) > 0
	return c
}
