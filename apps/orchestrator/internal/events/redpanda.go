package events

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// RedpandaPublisher writes outbox events to a Kafka-compatible Redpanda
// cluster. The writer is topic-less; each Event supplies its topic.
type RedpandaPublisher struct {
	writer *kafka.Writer
}

func NewRedpandaPublisher(brokers []string) (*RedpandaPublisher, error) {
	clean := make([]string, 0, len(brokers))
	for _, b := range brokers {
		b = strings.TrimSpace(b)
		if b != "" {
			clean = append(clean, b)
		}
	}
	if len(clean) == 0 {
		return nil, errors.New("events: redpanda brokers required")
	}
	return &RedpandaPublisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(clean...),
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
			BatchTimeout: 50 * time.Millisecond,
		},
	}, nil
}

func (p *RedpandaPublisher) Publish(ctx context.Context, e Event) error {
	if p == nil || p.writer == nil {
		return errors.New("events: redpanda publisher not configured")
	}
	e = Normalize(e)
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	headers := []kafka.Header{
		{Key: "event_id", Value: []byte(e.ID.String())},
		{Key: "event_type", Value: []byte(e.Type)},
		{Key: "event_version", Value: []byte(strconv.Itoa(e.Version))},
	}
	for k, v := range e.Headers {
		if s, ok := v.(string); ok {
			headers = append(headers, kafka.Header{Key: k, Value: []byte(s)})
		}
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic:   e.Topic,
		Key:     []byte(e.Key),
		Value:   payload,
		Headers: headers,
		Time:    e.CreatedAt,
	})
}

func (p *RedpandaPublisher) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}
