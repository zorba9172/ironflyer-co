package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// DLQTopicFor builds the DLQ topic name from a source topic + consumer
// name. Per ARCHITECTURE_EVENTS.md:
//
//	ifly.<env>.dlq.<source-domain>.<source-stream>.<consumer>.v1
//
// sourceTopic must match the ifly.<env>.<domain>.<stream>.v<N> shape.
// consumer is the consumer-group/worker identifier (e.g. "outbox-
// publisher"). Returns an error when either input is malformed so the
// caller can fall back to logging-only.
func DLQTopicFor(sourceTopic string, consumer string) (string, error) {
	if err := ValidateTopic(sourceTopic); err != nil {
		return "", fmt.Errorf("events: dlq source topic invalid: %w", err)
	}
	consumer = strings.TrimSpace(consumer)
	if consumer == "" {
		return "", errors.New("events: dlq consumer required")
	}
	// Strip the leading "ifly.<env>." and trailing ".v<N>" so we can
	// re-stamp them. topicRe already constrained the shape.
	parts := strings.Split(sourceTopic, ".")
	// parts: ["ifly", env, domain, stream, "v<N>"]
	if len(parts) != 5 {
		return "", fmt.Errorf("events: dlq source topic %q malformed", sourceTopic)
	}
	env := parts[1]
	domain := parts[2]
	stream := parts[3]
	// Per spec DLQs are pinned at v1 — the consumer's own contract
	// drives compatibility, not the source schema version.
	return fmt.Sprintf("ifly.%s.dlq.%s.%s.%s.v1", env, domain, stream, idSafeConsumer(consumer)), nil
}

// idSafeConsumer collapses a consumer name down to the topic-token
// character set so DLQ topics still pass ValidateTopic-style readers.
// The DLQ taxonomy doesn't share the producer topicRe (it has an extra
// segment), but we keep lowercase + [a-z0-9_] characters so Kafka
// admin tools don't choke on the auto-created topic.
func idSafeConsumer(c string) string {
	c = strings.ToLower(c)
	var b strings.Builder
	b.Grow(len(c))
	for _, r := range c {
		switch {
		case r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '_':
			b.WriteRune(r)
		case r == '-' || r == '.' || r == '/':
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "consumer"
	}
	return b.String()
}

// DLQRecord matches the ARCHITECTURE_EVENTS.md DLQ payload spec. It
// captures enough provenance for an operator to replay the original
// event back onto its source topic.
type DLQRecord struct {
	OriginalTopic     string            `json:"original_topic"`
	OriginalPartition int32             `json:"original_partition,omitempty"`
	OriginalOffset    int64             `json:"original_offset,omitempty"`
	OriginalKey       string            `json:"original_key,omitempty"`
	Headers           map[string]string `json:"headers"`
	EventID           string            `json:"event_id"`
	Consumer          string            `json:"consumer"`
	ConsumerVersion   string            `json:"consumer_version"`
	FailureClass      string            `json:"failure_class"` // "schema" | "transient" | "permanent"
	AttemptCount      int               `json:"attempt_count"`
	FinalError        string            `json:"final_error"` // truncated to 1KB
	FirstFailureTime  string            `json:"first_failure_time"`
	DLQTime           string            `json:"dlq_time"`
	Payload           json.RawMessage   `json:"payload"`
}

// dlqMaxErrorBytes caps FinalError at 1KB per spec — replay tooling
// does not need a stack trace, just a fingerprint of why the event
// died on the original consumer.
const dlqMaxErrorBytes = 1024

// classifyFailure decides which bucket the DLQ record belongs to.
// Schema validation errors are permanent and must DLQ immediately;
// kafka transient errors retried until MaxAttempts; everything else
// permanent.
func classifyFailure(err error) string {
	if err == nil {
		return "permanent"
	}
	if errors.Is(err, ErrSchemaValidation) {
		return "schema"
	}
	var kErr kafka.Error
	if errors.As(err, &kErr) {
		if kErr.Temporary() {
			return "transient"
		}
		return "permanent"
	}
	// Generic temporary interface (net errors, broker errors that don't
	// surface as kafka.Error but implement Temporary()).
	type temporary interface{ Temporary() bool }
	var te temporary
	if errors.As(err, &te) && te.Temporary() {
		return "transient"
	}
	return "permanent"
}

// truncateError shortens the diagnostic text to dlqMaxErrorBytes
// without splitting a multi-byte rune at the boundary.
func truncateError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) <= dlqMaxErrorBytes {
		return s
	}
	cut := dlqMaxErrorBytes
	for cut > 0 && (s[cut]&0xC0) == 0x80 {
		cut--
	}
	return s[:cut] + "...[truncated]"
}

// buildDLQRecord assembles the record from an outbox event and the
// final publish error. firstFailure is the row's created_at as a
// reasonable proxy when the publisher doesn't track the actual first
// failure time per-row.
func buildDLQRecord(e Event, consumer, consumerVersion string, finalErr error, firstFailure time.Time) DLQRecord {
	headers := map[string]string{}
	for k, v := range e.Headers {
		if s, ok := v.(string); ok {
			headers[k] = s
		} else {
			b, _ := json.Marshal(v)
			headers[k] = string(b)
		}
	}
	if firstFailure.IsZero() {
		firstFailure = e.CreatedAt
	}
	payload := e.RawPayload
	if len(payload) == 0 {
		if b, err := json.Marshal(e.Payload); err == nil {
			payload = b
		} else {
			payload = []byte("{}")
		}
	}
	return DLQRecord{
		OriginalTopic:    e.Topic,
		OriginalKey:      e.Key,
		Headers:          headers,
		EventID:          e.ID.String(),
		Consumer:         consumer,
		ConsumerVersion:  consumerVersion,
		FailureClass:     classifyFailure(finalErr),
		AttemptCount:     e.Attempts,
		FinalError:       truncateError(finalErr),
		FirstFailureTime: firstFailure.UTC().Format(time.RFC3339Nano),
		DLQTime:          time.Now().UTC().Format(time.RFC3339Nano),
		Payload:          payload,
	}
}

// dlqEventFor wraps a DLQRecord into an events.Event so the existing
// Publisher can ship it through the same Redpanda writer. The wrapping
// Event is keyed on the original event id so an operator can grep by
// either id or original topic.
func dlqEventFor(dlqTopic string, rec DLQRecord) (Event, error) {
	payload, err := json.Marshal(rec)
	if err != nil {
		return Event{}, fmt.Errorf("events: marshal dlq record: %w", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(payload, &doc); err != nil {
		return Event{}, fmt.Errorf("events: dlq record envelope: %w", err)
	}
	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		id = uuid.New()
	}
	return Event{
		ID:        id,
		Topic:     dlqTopic,
		Key:       rec.EventID,
		Type:      "dlq.record.v1",
		Version:   1,
		Payload:   doc,
		Headers:   map[string]any{"tenant_id": rec.Headers["tenant_id"], "producer": rec.Consumer},
		CreatedAt: now,
	}, nil
}
