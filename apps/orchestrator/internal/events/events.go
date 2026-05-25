// Package events owns Ironflyer's durable event backbone.
//
// Durable product/business events are written to Postgres outbox rows
// first, then published to Redpanda/Kafka by a background pump. Redis
// remains for ephemeral live fan-out only.
package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event is the canonical outbox envelope. Payload must already be
// redacted: never place secrets, source-code bodies, or customer data
// blobs in durable infrastructure events.
type Event struct {
	ID          uuid.UUID       `json:"id"`
	Topic       string          `json:"topic"`
	Key         string          `json:"key"`
	Type        string          `json:"eventType"`
	Version     int             `json:"eventVersion"`
	Payload     map[string]any  `json:"payload"`
	Headers     map[string]any  `json:"headers"`
	Attempts    int             `json:"attempts"`
	CreatedAt   time.Time       `json:"createdAt"`
	PublishedAt *time.Time      `json:"publishedAt,omitempty"`
	LastError   string          `json:"lastError,omitempty"`
	LockedUntil *time.Time      `json:"lockedUntil,omitempty"`
	LockedBy    string          `json:"lockedBy,omitempty"`
	NextAttempt time.Time       `json:"nextAttemptAt"`
	RawPayload  json.RawMessage `json:"-"`
	RawHeaders  json.RawMessage `json:"-"`
}

// Outbox is the durable producer side.
type Outbox interface {
	Enqueue(ctx context.Context, e Event) (Event, error)
	Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]Event, error)
	MarkPublished(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, err error, retryAfter time.Duration, dead bool) error
}

// Publisher is the transport side. RedpandaPublisher implements this
// for Kafka-compatible Redpanda clusters.
type Publisher interface {
	Publish(ctx context.Context, e Event) error
	Close() error
}

// Normalize fills envelope defaults.
func Normalize(e Event) Event {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.Version <= 0 {
		e.Version = 1
	}
	if e.Payload == nil {
		e.Payload = map[string]any{}
	}
	if e.Headers == nil {
		e.Headers = map[string]any{}
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if e.NextAttempt.IsZero() {
		e.NextAttempt = e.CreatedAt
	}
	return e
}
