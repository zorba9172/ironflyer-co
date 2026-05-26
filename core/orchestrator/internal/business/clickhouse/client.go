package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/business/clickhouse/schema"
)

// Client is the orchestrator-facing wrapper around the clickhouse-go
// v2 native connection. It owns a pool that any goroutine can share.
//
// The wrapper exists so the rest of the orchestrator never imports
// the upstream driver type directly — that lets us swap drivers,
// inject a no-op for tests, or wrap every call with metrics later
// without touching call sites.
type Client struct {
	conn driver.Conn
	log  zerolog.Logger
	cfg  Config
}

// NewClient opens a native ClickHouse connection. Returns (nil, nil)
// when Enabled is false so wireup code can stay branch-light:
//
//	cli, err := clickhouse.NewClient(cfg, log)
//	if err != nil { ... }
//	if cli == nil { /* analytics plane disabled */ }
func NewClient(cfg Config, log zerolog.Logger) (*Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	opts := &ch.Options{
		Addr: cfg.Hosts,
		Auth: ch.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout: time.Duration(cfg.DialTimeoutMS) * time.Millisecond,
		// Async insert is the high-throughput knob ClickHouse exposes for
		// streaming pipelines: the server batches in-flight rows up to
		// the table's async_insert_max_data_size / async_insert_busy_timeout
		// before flushing to the on-disk part. wait_for_async_insert=0
		// means the client returns the moment the row is in the in-memory
		// buffer; combined with ReplacingMergeTree dedup this is safe
		// because crash-on-flush is recoverable from the outbox.
		Settings: ch.Settings{
			"max_execution_time":       60,
			"async_insert":             1,
			"wait_for_async_insert":    0,
			"async_insert_busy_timeout_ms": 1000,
		},
		// Native protocol negotiates LZ4 when both sides advertise it.
		Compression: &ch.Compression{Method: ch.CompressionLZ4},
	}
	if !cfg.Compression {
		opts.Compression = nil
	}
	conn, err := ch.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: open: %w", err)
	}
	return &Client{conn: conn, log: log, cfg: cfg}, nil
}

// Conn exposes the underlying driver.Conn for code that needs the
// PrepareBatch fast-path. Most callers should prefer Exec/QueryRows.
func (c *Client) Conn() driver.Conn {
	if c == nil {
		return nil
	}
	return c.conn
}

// Ping verifies the connection is alive. Used by /readyz when the
// integration agent wires it in.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.conn == nil {
		return errors.New("clickhouse: client not configured")
	}
	return c.conn.Ping(ctx)
}

// Exec runs a one-shot DDL/DML statement. Safe to call concurrently.
func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
	if c == nil || c.conn == nil {
		return errors.New("clickhouse: client not configured")
	}
	return c.conn.Exec(ctx, query, args...)
}

// QueryRows is the read path used by the dashboard adapters. The
// returned Rows must be Close()d by the caller.
func (c *Client) QueryRows(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	if c == nil || c.conn == nil {
		return nil, errors.New("clickhouse: client not configured")
	}
	return c.conn.Query(ctx, query, args...)
}

// Close shuts down the pool. Idempotent.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Bootstrap applies the embedded schema files in lexical order
// (01_raw → 02_facts → 03_rollups). Every statement uses
// IF NOT EXISTS, so repeated boots are idempotent.
//
// Error semantics:
//
//   - Returns ErrDisabled when the client is nil (analytics plane off).
//   - Aborts on the first statement that fails and returns a
//     wrapped error identifying file + statement prefix.
//   - Successful statements stay applied — there is no transactional
//     rollback (ClickHouse DDL is not transactional).
//
// CreateDatabase is run first so a fresh cluster gains the
// configured database before any CREATE TABLE references it.
func (c *Client) Bootstrap(ctx context.Context) error {
	if c == nil || c.conn == nil {
		return ErrDisabled
	}
	if c.cfg.Database != "" {
		if err := c.conn.Exec(ctx, "CREATE DATABASE IF NOT EXISTS "+quoteIdent(c.cfg.Database)); err != nil {
			return fmt.Errorf("clickhouse bootstrap: create database %s: %w", c.cfg.Database, err)
		}
	}
	stmts, err := schema.Load()
	if err != nil {
		return fmt.Errorf("clickhouse bootstrap: load schema: %w", err)
	}
	for _, s := range stmts {
		if err := c.conn.Exec(ctx, s.Body); err != nil {
			head := s.Body
			if len(head) > 120 {
				head = head[:120] + "..."
			}
			head = strings.ReplaceAll(head, "\n", " ")
			return fmt.Errorf("clickhouse bootstrap: %s: %s: %w", s.File, head, err)
		}
		c.log.Debug().Str("file", s.File).Msg("clickhouse ddl applied")
	}
	c.log.Info().Int("statements", len(stmts)).Msg("clickhouse schema bootstrap complete")
	return nil
}

// ErrDisabled is returned by client methods when the analytics plane
// is intentionally off (no IRONFLYER_CLICKHOUSE_HOSTS configured).
// Callers should treat it as a soft "no-op" condition, not a failure.
var ErrDisabled = errors.New("clickhouse: client disabled (no hosts configured)")

// quoteIdent wraps a ClickHouse identifier in backticks, escaping
// embedded backticks. Used for the database name in Bootstrap; every
// other identifier in the package is a static literal in the SQL
// files.
func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
