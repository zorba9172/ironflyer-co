package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresOutbox struct {
	pool *pgxpool.Pool
}

func NewPostgresOutbox(pool *pgxpool.Pool) *PostgresOutbox {
	return &PostgresOutbox{pool: pool}
}

func (p *PostgresOutbox) Enqueue(ctx context.Context, e Event) (Event, error) {
	if p == nil || p.pool == nil {
		return Event{}, errors.New("events: postgres outbox not configured")
	}
	e = Normalize(e)
	payload, headers, err := marshalParts(e)
	if err != nil {
		return Event{}, err
	}
	var out Event
	row := p.pool.QueryRow(ctx, insertOutboxSQL, e.ID, e.Topic, e.Key, e.Type, e.Version, payload, headers, e.NextAttempt, e.CreatedAt)
	if err := scanEvent(row, &out); err != nil {
		return Event{}, fmt.Errorf("events: enqueue: %w", err)
	}
	return out, nil
}

const insertOutboxSQL = `
INSERT INTO event_outbox
  (id, topic, key, event_type, event_version, payload, headers,
   status, attempts, next_attempt_at, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,'pending',0,$8,$9)
RETURNING id, topic, key, event_type, event_version, payload, headers,
          attempts, created_at, published_at, last_error, locked_until,
          locked_by, next_attempt_at`

func EnqueueTx(ctx context.Context, tx pgx.Tx, e Event) (Event, error) {
	e = Normalize(e)
	payload, headers, err := marshalParts(e)
	if err != nil {
		return Event{}, err
	}
	var out Event
	row := tx.QueryRow(ctx, insertOutboxSQL, e.ID, e.Topic, e.Key, e.Type, e.Version, payload, headers, e.NextAttempt, e.CreatedAt)
	if err := scanEvent(row, &out); err != nil {
		return Event{}, fmt.Errorf("events: enqueue tx: %w", err)
	}
	return out, nil
}

func (p *PostgresOutbox) Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]Event, error) {
	if p == nil || p.pool == nil {
		return nil, errors.New("events: postgres outbox not configured")
	}
	if limit <= 0 {
		limit = 50
	}
	if lease <= 0 {
		lease = 30 * time.Second
	}
	const q = `
WITH picked AS (
  SELECT id
  FROM event_outbox
  WHERE status = 'pending'
    AND next_attempt_at <= now()
    AND (locked_until IS NULL OR locked_until < now())
  ORDER BY created_at ASC
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
UPDATE event_outbox e
SET locked_until = now() + ($2::double precision * interval '1 second'),
    locked_by = $3,
    attempts = attempts + 1
FROM picked
WHERE e.id = picked.id
RETURNING e.id, e.topic, e.key, e.event_type, e.event_version,
          e.payload, e.headers, e.attempts, e.created_at,
          e.published_at, e.last_error, e.locked_until,
          e.locked_by, e.next_attempt_at`
	rows, err := p.pool.Query(ctx, q, limit, lease.Seconds(), workerID)
	if err != nil {
		return nil, fmt.Errorf("events: claim: %w", err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

func (p *PostgresOutbox) MarkPublished(ctx context.Context, id uuid.UUID) error {
	if p == nil || p.pool == nil {
		return errors.New("events: postgres outbox not configured")
	}
	const q = `
UPDATE event_outbox
SET status = 'published',
    published_at = now(),
    locked_until = NULL,
    locked_by = NULL,
    last_error = NULL
WHERE id = $1`
	_, err := p.pool.Exec(ctx, q, id)
	return err
}

func (p *PostgresOutbox) MarkFailed(ctx context.Context, id uuid.UUID, cause error, retryAfter time.Duration, dead bool) error {
	if p == nil || p.pool == nil {
		return errors.New("events: postgres outbox not configured")
	}
	status := "pending"
	if dead {
		status = "dead"
	}
	if retryAfter <= 0 {
		retryAfter = time.Second
	}
	msg := ""
	if cause != nil {
		msg = cause.Error()
	}
	const q = `
UPDATE event_outbox
SET status = $2,
    next_attempt_at = now() + ($3::double precision * interval '1 second'),
    locked_until = NULL,
    locked_by = NULL,
    last_error = $4
WHERE id = $1`
	_, err := p.pool.Exec(ctx, q, id, status, retryAfter.Seconds(), msg)
	return err
}

func marshalParts(e Event) ([]byte, []byte, error) {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("events: marshal payload: %w", err)
	}
	headers, err := json.Marshal(e.Headers)
	if err != nil {
		return nil, nil, fmt.Errorf("events: marshal headers: %w", err)
	}
	return payload, headers, nil
}

type eventScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row eventScanner, out *Event) error {
	var payload, headers []byte
	if err := row.Scan(
		&out.ID, &out.Topic, &out.Key, &out.Type, &out.Version,
		&payload, &headers, &out.Attempts, &out.CreatedAt,
		&out.PublishedAt, &out.LastError, &out.LockedUntil,
		&out.LockedBy, &out.NextAttempt,
	); err != nil {
		return err
	}
	out.RawPayload = append(out.RawPayload[:0], payload...)
	out.RawHeaders = append(out.RawHeaders[:0], headers...)
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &out.Payload)
	}
	if len(headers) > 0 {
		_ = json.Unmarshal(headers, &out.Headers)
	}
	if out.Payload == nil {
		out.Payload = map[string]any{}
	}
	if out.Headers == nil {
		out.Headers = map[string]any{}
	}
	return nil
}

func scanEvents(rows pgx.Rows) ([]Event, error) {
	out := make([]Event, 0)
	for rows.Next() {
		var e Event
		if err := scanEvent(rows, &e); err != nil {
			return nil, fmt.Errorf("events: scan row: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("events: iterate rows: %w", err)
	}
	return out, nil
}
