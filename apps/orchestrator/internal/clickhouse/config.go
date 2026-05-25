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
	"strconv"
	"strings"
)

// Config bundles every IRONFLYER_CLICKHOUSE_* env knob. Enabled is
// derived from the host list — when no host is configured the rest of
// the package becomes a no-op so dev and CI work without a CH cluster.
type Config struct {
	Hosts         []string // IRONFLYER_CLICKHOUSE_HOSTS — comma-separated host:port list
	Database      string   // IRONFLYER_CLICKHOUSE_DATABASE — defaults to "ironflyer"
	Username      string   // IRONFLYER_CLICKHOUSE_USERNAME — defaults to "default"
	Password      string   // IRONFLYER_CLICKHOUSE_PASSWORD
	Compression   bool     // IRONFLYER_CLICKHOUSE_COMPRESSION — defaults true
	DialTimeoutMS int      // IRONFLYER_CLICKHOUSE_DIAL_TIMEOUT_MS — defaults to 5000
	Secure        bool     // IRONFLYER_CLICKHOUSE_SECURE — TLS on the wire, defaults false
	IngestFromOutbox bool  // IRONFLYER_CLICKHOUSE_INGEST_FROM_OUTBOX — direct outbox→CH fallback
	Enabled       bool     // derived: at least one host non-empty
}

// LoadConfig reads every IRONFLYER_CLICKHOUSE_* env var with sane
// defaults. Hosts is the only mandatory knob — without it the package
// is disabled and all wired-up callers must be tolerant of nil/no-op
// returns.
func LoadConfig() Config {
	c := Config{
		Hosts:         splitCSV(os.Getenv("IRONFLYER_CLICKHOUSE_HOSTS")),
		Database:      strings.TrimSpace(os.Getenv("IRONFLYER_CLICKHOUSE_DATABASE")),
		Username:      strings.TrimSpace(os.Getenv("IRONFLYER_CLICKHOUSE_USERNAME")),
		Password:      os.Getenv("IRONFLYER_CLICKHOUSE_PASSWORD"),
		Compression:   parseBool(os.Getenv("IRONFLYER_CLICKHOUSE_COMPRESSION"), true),
		DialTimeoutMS: parseInt(os.Getenv("IRONFLYER_CLICKHOUSE_DIAL_TIMEOUT_MS"), 5000),
		Secure:        parseBool(os.Getenv("IRONFLYER_CLICKHOUSE_SECURE"), false),
		IngestFromOutbox: parseBool(os.Getenv("IRONFLYER_CLICKHOUSE_INGEST_FROM_OUTBOX"), false),
	}
	if c.Database == "" {
		c.Database = "ironflyer"
	}
	if c.Username == "" {
		c.Username = "default"
	}
	c.Enabled = len(c.Hosts) > 0
	return c
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseBool(s string, def bool) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return def
	}
	switch s {
	case "1", "t", "true", "yes", "y", "on":
		return true
	case "0", "f", "false", "no", "n", "off":
		return false
	}
	return def
}

func parseInt(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
