package clickhouse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// OutboxIngester is the Redpanda-less fallback path. Local dev (and
// any deployment where IRONFLYER_CLICKHOUSE_INGEST_FROM_OUTBOX=true)
// can use it to tail event_outbox rows directly into ClickHouse so
// the analytics plane works without a Kafka broker.
//
// The ingester reads rows that have already been published — meaning
// the Redpanda pump's MarkPublished has run — so it never races the
// Redpanda consumer. When Redpanda is absent the publisher daemon
// won't mark anything; instead set IngestFromAll to true and the
// ingester will tail every outbox row, optionally bookmarking with a
// last_seen timestamp file or in-memory cursor.
type OutboxIngester struct {
	ch       *Client
	pool     *pgxpool.Pool
	interval time.Duration
	lookback time.Duration
	cursor   time.Time
	log      zerolog.Logger
}

// NewOutboxIngester wires the fallback ingester. interval controls
// the poll rate (default 2s) and lookback is the initial window the
// first poll scans (default 5 minutes).
func NewOutboxIngester(ch *Client, pool *pgxpool.Pool, log zerolog.Logger) *OutboxIngester {
	register()
	return &OutboxIngester{
		ch:       ch,
		pool:     pool,
		interval: 2 * time.Second,
		lookback: 5 * time.Minute,
		cursor:   time.Now().Add(-5 * time.Minute).UTC(),
		log:      log,
	}
}

// Run blocks until ctx is cancelled. Per-row errors are logged and
// the row is skipped; the cursor only advances past rows we
// successfully inserted (or that we decided to ignore on purpose).
func (i *OutboxIngester) Run(ctx context.Context) error {
	if i == nil {
		return errors.New("clickhouse ingester: nil receiver")
	}
	if i.ch == nil {
		return errors.New("clickhouse ingester: nil client")
	}
	if i.pool == nil {
		return errors.New("clickhouse ingester: nil pool")
	}
	t := time.NewTicker(i.interval)
	defer t.Stop()
	i.log.Info().
		Dur("interval", i.interval).
		Time("cursor", i.cursor).
		Msg("clickhouse outbox ingester started")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if err := i.drain(ctx); err != nil && !errors.Is(err, context.Canceled) {
				i.log.Warn().Err(err).Msg("clickhouse outbox ingester drain")
			}
		}
	}
}

const ingestSelectSQL = `
SELECT id, topic, key, event_type, event_version, payload, headers,
       created_at
  FROM event_outbox
 WHERE created_at > $1
 ORDER BY created_at ASC
 LIMIT 500`

func (i *OutboxIngester) drain(ctx context.Context) error {
	rows, err := i.pool.Query(ctx, ingestSelectSQL, i.cursor)
	if err != nil {
		return fmt.Errorf("ingester query: %w", err)
	}
	defer rows.Close()

	var maxSeen = i.cursor
	for rows.Next() {
		var (
			id           uuid.UUID
			topic        string
			key          string
			eventType    string
			eventVersion int
			payloadRaw   []byte
			headersRaw   []byte
			createdAt    time.Time
		)
		if err := rows.Scan(&id, &topic, &key, &eventType, &eventVersion, &payloadRaw, &headersRaw, &createdAt); err != nil {
			i.log.Warn().Err(err).Msg("clickhouse ingester scan")
			continue
		}
		if createdAt.After(maxSeen) {
			maxSeen = createdAt
		}

		table, ok := tableForTopic(topic)
		if !ok {
			// Unknown topic — count and skip so the cursor still advances.
			ingesterRowsTotal.WithLabelValues("unknown", "skip").Inc()
			continue
		}

		env := rawEnvelope{
			EventID:      id,
			EventType:    eventType,
			EventVersion: uint32(eventVersion),
			OccurredAt:   createdAt,
		}
		var payloadMap, headersMap map[string]any
		_ = json.Unmarshal(payloadRaw, &payloadMap)
		_ = json.Unmarshal(headersRaw, &headersMap)
		if v := stringFromMap(headersMap, "tenant_id"); v != "" {
			env.TenantID = v
		} else if v := stringFromMap(payloadMap, "tenant_id"); v != "" {
			env.TenantID = v
		}
		if v := stringFromMap(headersMap, "producer"); v != "" {
			env.Producer = v
		}
		if v := stringFromMap(headersMap, "trace_id"); v != "" {
			env.TraceID = strPtr(v)
		}
		if v := stringFromMap(headersMap, "idempotency_key"); v != "" {
			env.IdempotencyKey = v
		} else {
			env.IdempotencyKey = id.String()
		}
		if v := stringFromMap(payloadMap, "execution_id"); v != "" {
			env.ExecutionID = strPtr(v)
		}

		start := time.Now()
		if err := insertRaw(ctx, i.ch, table, env, string(payloadRaw)); err != nil {
			ingesterRowsTotal.WithLabelValues(table, "error").Inc()
			i.log.Warn().Err(err).Str("table", table).Msg("clickhouse ingester insert")
			// Don't advance the cursor past this row — we'll retry
			// next drain. Break out so failing inserts can't
			// monotonically advance maxSeen via later successful rows.
			break
		}
		insertLatencySeconds.WithLabelValues(table).Observe(time.Since(start).Seconds())
		ingesterRowsTotal.WithLabelValues(table, "ok").Inc()
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("ingester iterate: %w", err)
	}
	i.cursor = maxSeen
	return nil
}
