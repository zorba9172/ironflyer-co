package events

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
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

// EnsureDLQTopics is an idempotent admin call that creates the per-
// source DLQ topic for every V22 producer topic, against the given
// consumer name (the same one the PublisherDaemon stamps on emitDLQ).
//
// The current publish path swallowed the create error and only
// surfaced "Unknown Topic Or Partition" at publish time. Calling this
// at boot — right after the publisher daemon starts — primes the
// broker so the first dead row finds its DLQ topic ready.
//
// Best-effort: brokers that already own the topic, or where the user
// lacks CreateTopics ACL, return nil after a Warn log. Unknown brokers
// surface as a Warn and the function returns nil so startup stays
// non-fatal.
func EnsureDLQTopics(ctx context.Context, brokers []string, consumer string, log zerolog.Logger) error {
	clean := make([]string, 0, len(brokers))
	for _, b := range brokers {
		b = strings.TrimSpace(b)
		if b != "" {
			clean = append(clean, b)
		}
	}
	if len(clean) == 0 {
		return errors.New("events: brokers required")
	}
	if strings.TrimSpace(consumer) == "" {
		consumer = "outbox-publisher"
	}

	// Resolve a controller broker. kafka-go's admin requires connecting
	// to the controller for CreateTopics; we dial the first broker, ask
	// it for the controller, then issue the request against that node.
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	dialer := &kafka.Dialer{Timeout: 5 * time.Second, DualStack: true}
	conn, err := dialer.DialContext(dialCtx, "tcp", clean[0])
	if err != nil {
		log.Warn().Err(err).Str("broker", clean[0]).
			Msg("events: dlq bootstrap dial failed; broker may not be reachable yet")
		return nil
	}
	defer func() { _ = conn.Close() }()

	controller, err := conn.Controller()
	if err != nil {
		log.Warn().Err(err).Msg("events: dlq bootstrap controller lookup failed")
		return nil
	}
	addr := net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))
	ctrlConn, err := dialer.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		log.Warn().Err(err).Str("controller", addr).
			Msg("events: dlq bootstrap controller dial failed")
		return nil
	}
	defer func() { _ = ctrlConn.Close() }()

	// Build one TopicConfig per V22 source topic. The runtime topics
	// are env-aware so we register against ifly.<env>.<domain>.<stream>.v1
	// — that's what the publisher sees and what DLQTopicFor maps from.
	env := CurrentEnv()
	stems := []string{
		"execution.lifecycle.v1",
		"execution.steps.v1",
		"gates.results.v1",
		"patches.lifecycle.v1",
		"billing.ledger.v1",
		"profitguard.decisions.v1",
		"deploy.lifecycle.v1",
		"memory.indexing.v1",
		"audit.security.v1",
	}
	cfgs := make([]kafka.TopicConfig, 0, len(stems))
	for _, stem := range stems {
		src := "ifly." + env + "." + stem
		dlq, derr := DLQTopicFor(src, consumer)
		if derr != nil {
			log.Warn().Err(derr).Str("topic", src).
				Msg("events: dlq bootstrap could not derive dlq topic")
			continue
		}
		cfgs = append(cfgs, kafka.TopicConfig{
			Topic:             dlq,
			NumPartitions:     1,
			ReplicationFactor: 1,
		})
	}
	if len(cfgs) == 0 {
		return nil
	}

	// CreateTopics is idempotent at the broker level — a request for a
	// topic that already exists returns Topic_already_exists which we
	// treat as success.
	if err := ctrlConn.CreateTopics(cfgs...); err != nil {
		// Re-classify per-topic errors. kafka-go returns a single error
		// for the whole batch; "Topic with this name already exists" is
		// not actionable, everything else is logged at Warn.
		if isTopicExistsErr(err) {
			log.Debug().Int("topics", len(cfgs)).
				Msg("events: dlq bootstrap: topics already exist")
			return nil
		}
		log.Warn().Err(err).Int("topics", len(cfgs)).
			Msg("events: dlq bootstrap CreateTopics returned an error; continuing")
		return nil
	}
	names := make([]string, 0, len(cfgs))
	for _, c := range cfgs {
		names = append(names, c.Topic)
	}
	log.Info().Strs("topics", names).Str("consumer", consumer).
		Msg("events: dlq topics ensured")
	return nil
}

// isTopicExistsErr matches the broker's "Topic with this name already
// exists" classification so EnsureDLQTopics can treat it as success.
func isTopicExistsErr(err error) bool {
	if err == nil {
		return false
	}
	var kErr kafka.Error
	if errors.As(err, &kErr) {
		// Kafka error code 36 = TOPIC_ALREADY_EXISTS.
		if int(kErr) == 36 {
			return true
		}
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "already exists")
}

