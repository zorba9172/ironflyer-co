package clickhouse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"

	"ironflyer/core/orchestrator/internal/operations/events"
)

// Consumer is the Redpanda→ClickHouse pump. It reads from a
// consumer group across the configured topics, routes each event
// envelope to the matching raw_<domain>_events table by topic, and
// commits the offset only after the INSERT succeeds.
//
// Idempotency comes from ReplacingMergeTree(event_id): duplicate
// delivery — caused by a crash between INSERT and CommitMessages —
// collapses to a single row on the next merge.
type Consumer struct {
	ch      *Client
	brokers []string
	topics  []string
	group   string
	log     zerolog.Logger
}

// NewConsumer constructs the Redpanda→ClickHouse consumer. brokers
// and topics must be non-empty; group defaults to
// "ironflyer-clickhouse-ingest" so a misconfigured deploy doesn't
// accidentally pick up someone else's offsets.
func NewConsumer(ch *Client, brokers []string, topics []string, group string, log zerolog.Logger) *Consumer {
	register()
	if group == "" {
		group = "ironflyer-clickhouse-ingest"
	}
	return &Consumer{
		ch:      ch,
		brokers: brokers,
		topics:  topics,
		group:   group,
		log:     log,
	}
}

// Run blocks until ctx is cancelled or a fatal error occurs. Per-
// message errors are logged and the message is retried by NOT
// committing — kafka-go will redeliver after the consumer rejoin
// timeout. ClickHouse downtime therefore stalls the consumer rather
// than dropping events.
func (c *Consumer) Run(ctx context.Context) error {
	if c == nil {
		return errors.New("clickhouse consumer: nil receiver")
	}
	if c.ch == nil {
		return errors.New("clickhouse consumer: nil client")
	}
	if len(c.brokers) == 0 {
		return errors.New("clickhouse consumer: brokers required")
	}
	if len(c.topics) == 0 {
		return errors.New("clickhouse consumer: topics required")
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        c.brokers,
		GroupID:        c.group,
		GroupTopics:    c.topics,
		MinBytes:       1,
		MaxBytes:       10 << 20, // 10 MiB
		CommitInterval: 0,        // sync commits via CommitMessages
		MaxWait:        500 * time.Millisecond,
	})
	defer func() { _ = reader.Close() }()

	c.log.Info().
		Strs("topics", c.topics).
		Str("group", c.group).
		Msg("clickhouse consumer started")

	// One backoff timer, reused across loop iterations. time.After inside
	// a select that may not always pick the timer leaks a Timer per miss;
	// NewTimer + Reset is the leak-free pattern.
	backoff := time.NewTimer(time.Hour)
	if !backoff.Stop() {
		<-backoff.C
	}
	defer backoff.Stop()
	sleep := func() bool {
		backoff.Reset(time.Second)
		select {
		case <-ctx.Done():
			if !backoff.Stop() {
				<-backoff.C
			}
			return false
		case <-backoff.C:
			return true
		}
	}

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return ctx.Err()
			}
			c.log.Warn().Err(err).Msg("clickhouse consumer fetch")
			// Brief backoff so a wedged broker doesn't tight-loop.
			if !sleep() {
				return ctx.Err()
			}
			continue
		}

		if err := c.handle(ctx, msg); err != nil {
			c.log.Warn().Err(err).
				Str("topic", msg.Topic).
				Int64("offset", msg.Offset).
				Msg("clickhouse consumer handle")
			consumerEventsTotal.WithLabelValues(msg.Topic, "error").Inc()
			// Skip commit so the broker redelivers. Sleep first to
			// avoid a hot loop while ClickHouse is down.
			if !sleep() {
				return ctx.Err()
			}
			continue
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			c.log.Warn().Err(err).Msg("clickhouse consumer commit")
			// Even if commit fails we don't double-INSERT — the
			// ReplacingMergeTree collapses the duplicate on the next
			// merge. Keep going so we don't stall the partition.
		}
		consumerEventsTotal.WithLabelValues(msg.Topic, "ok").Inc()
		if !msg.Time.IsZero() {
			consumerLagSeconds.WithLabelValues(msg.Topic).
				Set(time.Since(msg.Time).Seconds())
		}
	}
}

// handle decodes one Kafka message, picks the matching raw_* table by
// topic, and INSERTs the envelope plus payload string. Decode failures
// are logged and skipped — the broker would otherwise re-deliver them
// forever and block the partition. A future revision can route bad
// rows to a DLQ topic.
func (c *Consumer) handle(ctx context.Context, msg kafka.Message) error {
	table, ok := tableForTopic(msg.Topic)
	if !ok {
		// Unknown topic in the subscription set — log once, skip.
		c.log.Debug().Str("topic", msg.Topic).Msg("clickhouse consumer: no raw table mapping")
		return nil
	}
	env, payload, err := decodeEnvelope(msg)
	if err != nil {
		c.log.Warn().Err(err).Str("topic", msg.Topic).Msg("clickhouse consumer: envelope decode")
		// Return nil so the offset commits — a malformed event would
		// otherwise wedge the partition. TODO: route to DLQ topic.
		return nil
	}
	start := time.Now()
	defer func() { insertLatencySeconds.WithLabelValues(table).Observe(time.Since(start).Seconds()) }()
	return insertRaw(ctx, c.ch, table, env, payload)
}

// rawEnvelope is the column-aligned struct for an inbound event. It
// is shared by the consumer and the outbox-tail ingester.
type rawEnvelope struct {
	EventID        uuid.UUID
	EventType      string
	EventVersion   uint32
	TenantID       string
	ExecutionID    *string
	OccurredAt     time.Time
	Producer       string
	TraceID        *string
	IdempotencyKey string
}

// decodeEnvelope unwraps the events.Event JSON written by the
// Redpanda publisher. The publisher stamps the full envelope as the
// message value, plus a few well-known headers (event_id, event_type).
func decodeEnvelope(msg kafka.Message) (rawEnvelope, string, error) {
	var e events.Event
	if err := json.Unmarshal(msg.Value, &e); err != nil {
		return rawEnvelope{}, "", fmt.Errorf("decode event: %w", err)
	}
	env := rawEnvelope{
		EventID:      e.ID,
		EventType:    e.Type,
		EventVersion: uint32(e.Version),
		OccurredAt:   e.CreatedAt,
	}
	if env.EventID == uuid.Nil {
		env.EventID = uuid.New()
	}
	if env.OccurredAt.IsZero() {
		env.OccurredAt = time.Now().UTC()
	}
	// Pull common fields from headers (preferred — explicit) then
	// fall back to the payload map.
	if v := headerString(msg, "tenant_id"); v != "" {
		env.TenantID = v
	} else if v := stringFromMap(e.Headers, "tenant_id"); v != "" {
		env.TenantID = v
	} else if v := stringFromMap(e.Payload, "tenant_id"); v != "" {
		env.TenantID = v
	}
	if v := headerString(msg, "producer"); v != "" {
		env.Producer = v
	} else if v := stringFromMap(e.Headers, "producer"); v != "" {
		env.Producer = v
	}
	if v := headerString(msg, "trace_id"); v != "" {
		env.TraceID = strPtr(v)
	} else if v := stringFromMap(e.Headers, "trace_id"); v != "" {
		env.TraceID = strPtr(v)
	}
	if v := headerString(msg, "idempotency_key"); v != "" {
		env.IdempotencyKey = v
	} else if v := stringFromMap(e.Headers, "idempotency_key"); v != "" {
		env.IdempotencyKey = v
	}
	if env.IdempotencyKey == "" {
		env.IdempotencyKey = env.EventID.String()
	}
	if v := stringFromMap(e.Payload, "execution_id"); v != "" {
		env.ExecutionID = strPtr(v)
	}

	// The raw payload column always stores the full payload JSON so a
	// downstream reprojection can recompute facts without re-reading
	// Redpanda.
	payloadBytes, err := json.Marshal(e.Payload)
	if err != nil {
		return rawEnvelope{}, "", fmt.Errorf("marshal payload: %w", err)
	}
	return env, string(payloadBytes), nil
}

// rawInsertQueries holds the one INSERT statement per raw_* table. The
// statements are constant strings (no fmt.Sprintf at hot-path) so the
// clickhouse-go v2 driver can cache the parsed plan instead of
// re-tokenising on every event. Adding a new raw_* table is a
// one-line append here.
var rawInsertQueries = map[string]string{
	"raw_execution_events": `INSERT INTO raw_execution_events
		 (event_id, event_type, event_version, tenant_id, execution_id, occurred_at, producer, trace_id, idempotency_key, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	"raw_ledger_events": `INSERT INTO raw_ledger_events
		 (event_id, event_type, event_version, tenant_id, execution_id, occurred_at, producer, trace_id, idempotency_key, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	"raw_agent_events": `INSERT INTO raw_agent_events
		 (event_id, event_type, event_version, tenant_id, execution_id, occurred_at, producer, trace_id, idempotency_key, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	"raw_gate_events": `INSERT INTO raw_gate_events
		 (event_id, event_type, event_version, tenant_id, execution_id, occurred_at, producer, trace_id, idempotency_key, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	"raw_runtime_events": `INSERT INTO raw_runtime_events
		 (event_id, event_type, event_version, tenant_id, execution_id, occurred_at, producer, trace_id, idempotency_key, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	"raw_deploy_events": `INSERT INTO raw_deploy_events
		 (event_id, event_type, event_version, tenant_id, execution_id, occurred_at, producer, trace_id, idempotency_key, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	"raw_security_events": `INSERT INTO raw_security_events
		 (event_id, event_type, event_version, tenant_id, execution_id, occurred_at, producer, trace_id, idempotency_key, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
}

// insertRaw runs the single-row INSERT for one envelope using a
// precompiled per-table query (see rawInsertQueries). Server-side
// async_insert (configured on the client) batches the in-flight rows
// so this single-row path keeps the kafka-go fetch loop simple while
// ClickHouse coalesces writes on its side.
func insertRaw(ctx context.Context, c *Client, table string, env rawEnvelope, payload string) error {
	query, ok := rawInsertQueries[table]
	if !ok {
		return fmt.Errorf("clickhouse: no insert query registered for table %q", table)
	}
	return c.Exec(ctx, query,
		env.EventID,
		env.EventType,
		env.EventVersion,
		env.TenantID,
		env.ExecutionID,
		env.OccurredAt,
		env.Producer,
		env.TraceID,
		env.IdempotencyKey,
		payload,
	)
}

// tableForTopic routes a topic name to its raw_* destination table.
// The mapping mirrors the topic taxonomy in
// docs/ARCHITECTURE_EVENTS.md. Unknown topics return ok=false so the
// consumer skips them rather than inventing a table.
func tableForTopic(topic string) (string, bool) {
	// Strip the ifly.<env>. prefix so dev/staging/prod all map to the
	// same raw table. Parse the dotted prefix without strings.Split so
	// the hot ingest path doesn't allocate a fresh []string per event.
	const prefix = "ifly."
	if !strings.HasPrefix(topic, prefix) {
		return "", false
	}
	rest := topic[len(prefix):] // <env>.<domain>.<rest>
	envEnd := strings.IndexByte(rest, '.')
	if envEnd < 0 {
		return "", false
	}
	rest = rest[envEnd+1:] // <domain>.<rest>
	domEnd := strings.IndexByte(rest, '.')
	if domEnd < 0 {
		return "", false
	}
	if len(rest[domEnd+1:]) == 0 {
		// Original guard required len(parts) >= 4 — at least one
		// trailing segment after the domain.
		return "", false
	}
	domain := rest[:domEnd]
	switch domain {
	case "execution":
		return "raw_execution_events", true
	case "billing":
		return "raw_ledger_events", true
	case "profitguard":
		return "raw_agent_events", true
	case "gates":
		return "raw_gate_events", true
	case "patches":
		return "raw_agent_events", true
	case "deploy":
		return "raw_deploy_events", true
	case "audit":
		return "raw_security_events", true
	case "memory":
		return "raw_agent_events", true
	case "integrations":
		return "raw_agent_events", true
	case "runtime":
		return "raw_runtime_events", true
	default:
		return "", false
	}
}

func headerString(msg kafka.Message, key string) string {
	for _, h := range msg.Headers {
		if strings.EqualFold(h.Key, key) {
			return string(h.Value)
		}
	}
	return ""
}

func stringFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func strPtr(s string) *string {
	return &s
}
