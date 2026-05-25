package events

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

// PublisherConfig knobs the daemon loop. Defaults are tuned for the
// V22 spine: 100-row claim window, 250ms poll, 30s lease so a crashed
// publisher's lease expires fast, 12 attempts (~30 min with capped
// exp backoff) before dead-lettering.
type PublisherConfig struct {
	BatchSize     int
	PollInterval  time.Duration
	LeaseDuration time.Duration
	MaxAttempts   int
	WorkerID      string
	Logger        zerolog.Logger
	// Pool, when non-nil, is used to compute oldest_unpublished_age
	// without going through the Outbox interface (it needs raw SQL).
	// Callers that pass a *PostgresOutbox to NewPublisherDaemon can
	// leave this nil — the daemon extracts the pool via PoolGetter.
	Pool *pgxpool.Pool
	// Observer is invoked after each successful publish with the event.
	// Best-effort; observer panics are recovered and logged. Observers
	// must not block — the daemon runs them on a bounded goroutine.
	Observer func(ctx context.Context, e Event)
	// ConsumerName stamps the DLQ topic suffix when MaxAttempts is
	// exhausted. Defaults to "outbox-publisher".
	ConsumerName string
	// ConsumerVersion stamps the DLQ record's consumer_version field.
	// Defaults to ProducerName equivalent ("orchestrator/v22").
	ConsumerVersion string
}

// PoolGetter is the optional escape hatch that lets PublisherDaemon
// reach into a Postgres-backed Outbox for the age-gauge query without
// widening the Outbox interface for in-memory or test backends.
type PoolGetter interface {
	Pool() *pgxpool.Pool
}

// Pool exposes the underlying pgxpool when callers need to issue raw
// observability queries (oldest-unpublished-age gauge).
func (p *PostgresOutbox) Pool() *pgxpool.Pool {
	if p == nil {
		return nil
	}
	return p.pool
}

// PublisherDaemon is the long-running outbox → Redpanda pump. It owns
// the claim/publish/mark loop and exports Prometheus metrics so the
// operator dashboards can spot a stalled or DLQ-bound topic.
//
// PublisherDaemon is intentionally separate from the older Pump:
// integration agents wire NewPublisherDaemon(...).Run(ctx) into main.go
// at boot; Pump remains as a thinner shim for tests that don't care
// about metrics.
type PublisherDaemon struct {
	outbox Outbox
	pub    Publisher
	cfg    PublisherConfig
	log    zerolog.Logger
	pool   *pgxpool.Pool

	obsMu    sync.RWMutex
	observer func(ctx context.Context, e Event)
}

// NewPublisherDaemon constructs the daemon with defaults applied.
// outbox and pub MUST be non-nil; Run returns an error otherwise.
func NewPublisherDaemon(outbox Outbox, pub Publisher, cfg PublisherConfig) *PublisherDaemon {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 250 * time.Millisecond
	}
	if cfg.LeaseDuration <= 0 {
		cfg.LeaseDuration = 30 * time.Second
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 12
	}
	if cfg.WorkerID == "" {
		cfg.WorkerID = "orchestrator-pub-" + uuid.NewString()
	}
	if cfg.ConsumerName == "" {
		cfg.ConsumerName = "outbox-publisher"
	}
	if cfg.ConsumerVersion == "" {
		cfg.ConsumerVersion = "orchestrator/v22"
	}
	pool := cfg.Pool
	if pool == nil {
		if g, ok := outbox.(PoolGetter); ok {
			pool = g.Pool()
		}
	}
	registerPublisherMetrics()
	return &PublisherDaemon{
		outbox:   outbox,
		pub:      pub,
		cfg:      cfg,
		log:      cfg.Logger,
		pool:     pool,
		observer: cfg.Observer,
	}
}

// SetObserver swaps the observer at runtime. Late wiring is the
// expected path: the MemoryGraph writer is built after the publisher
// daemon, then attached via SetObserver. Pass nil to unsubscribe.
func (d *PublisherDaemon) SetObserver(fn func(ctx context.Context, e Event)) {
	if d == nil {
		return
	}
	d.obsMu.Lock()
	d.observer = fn
	d.obsMu.Unlock()
}

// notifyObserver invokes the registered observer on a bounded
// goroutine with panic recovery. Observers MUST NOT block the publish
// loop — slow consumers should buffer internally.
func (d *PublisherDaemon) notifyObserver(ctx context.Context, e Event) {
	d.obsMu.RLock()
	fn := d.observer
	d.obsMu.RUnlock()
	if fn == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				d.log.Error().Interface("panic", r).
					Str("event_id", e.ID.String()).
					Str("event_type", e.Type).
					Msg("events publisher observer panic recovered")
			}
		}()
		fn(ctx, e)
	}()
}

// Run blocks until ctx is cancelled. Errors inside one drain pass are
// logged and the loop continues — the only fatal condition is a nil
// outbox or publisher.
func (d *PublisherDaemon) Run(ctx context.Context) error {
	if d == nil || d.outbox == nil || d.pub == nil {
		return errors.New("events: publisher daemon not configured")
	}
	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()
	ageTicker := time.NewTicker(10 * time.Second)
	defer ageTicker.Stop()

	d.observeAge(ctx)
	for {
		if err := d.drainOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			d.log.Warn().Err(err).Msg("events publisher drain")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ageTicker.C:
			d.observeAge(ctx)
		case <-ticker.C:
		}
	}
}

func (d *PublisherDaemon) drainOnce(ctx context.Context) error {
	batch, err := d.outbox.Claim(ctx, d.cfg.WorkerID, d.cfg.BatchSize, d.cfg.LeaseDuration)
	if err != nil {
		return err
	}
	for _, e := range batch {
		topicLabel := e.Topic
		if vErr := ValidateTopic(e.Topic); vErr != nil {
			// Schema/topic validation failure: dead-letter immediately
			// so a malformed row never blocks the partition.
			if markErr := d.outbox.MarkFailed(ctx, e.ID, vErr, time.Hour, true); markErr != nil {
				d.log.Warn().Err(markErr).Str("event_id", e.ID.String()).Msg("events publisher mark dead")
			}
			outboxDeadTotal.WithLabelValues(topicLabel, "topic_invalid").Inc()
			// Best-effort DLQ even when topic name itself is invalid:
			// the source topic is malformed so we can't compute a DLQ
			// topic for it. Log and move on — operator alerts via
			// outboxDeadTotal{reason="topic_invalid"}.
			continue
		}
		if err := d.pub.Publish(ctx, e); err != nil {
			dead := e.Attempts >= d.cfg.MaxAttempts
			retry := publisherBackoff(e.Attempts)
			if markErr := d.outbox.MarkFailed(ctx, e.ID, err, retry, dead); markErr != nil {
				d.log.Warn().Err(markErr).Str("event_id", e.ID.String()).Msg("events publisher mark failed")
			}
			outboxFailedTotal.WithLabelValues(topicLabel).Inc()
			if dead {
				outboxDeadTotal.WithLabelValues(topicLabel, "publish_exhausted").Inc()
				d.emitDLQ(ctx, e, err)
			}
			continue
		}
		if err := d.outbox.MarkPublished(ctx, e.ID); err != nil {
			d.log.Warn().Err(err).Str("event_id", e.ID.String()).Msg("events publisher mark published")
			continue
		}
		outboxPublishedTotal.WithLabelValues(topicLabel).Inc()
		d.notifyObserver(ctx, e)
	}
	return nil
}

// emitDLQ pushes the failed event to its consumer-suffixed DLQ topic.
// Best-effort: a failed DLQ publish is logged at Error and the dead
// row stays put — we MUST NOT retry the DLQ publish from the same
// loop or a wedged broker would dead-letter the DLQ event into
// itself.
func (d *PublisherDaemon) emitDLQ(ctx context.Context, e Event, finalErr error) {
	dlqTopic, err := DLQTopicFor(e.Topic, d.cfg.ConsumerName)
	if err != nil {
		d.log.Error().Err(err).Str("event_id", e.ID.String()).Str("topic", e.Topic).
			Msg("events publisher: cannot derive DLQ topic")
		return
	}
	rec := buildDLQRecord(e, d.cfg.ConsumerName, d.cfg.ConsumerVersion, finalErr, e.CreatedAt)
	dlqEvt, err := dlqEventFor(dlqTopic, rec)
	if err != nil {
		d.log.Error().Err(err).Str("event_id", e.ID.String()).Str("dlq_topic", dlqTopic).
			Msg("events publisher: build DLQ event failed")
		return
	}
	if pubErr := d.pub.Publish(ctx, dlqEvt); pubErr != nil {
		d.log.Error().Err(pubErr).
			Str("event_id", e.ID.String()).
			Str("dlq_topic", dlqTopic).
			Str("original_topic", e.Topic).
			Msg("events publisher: DLQ publish failed; dead row retained")
		return
	}
	outboxDLQTotal.WithLabelValues(e.Topic, rec.FailureClass).Inc()
	d.log.Warn().
		Str("event_id", e.ID.String()).
		Str("original_topic", e.Topic).
		Str("dlq_topic", dlqTopic).
		Str("failure_class", rec.FailureClass).
		Int("attempts", e.Attempts).
		Msg("events publisher: DLQ record emitted")
}

func (d *PublisherDaemon) observeAge(ctx context.Context) {
	if d.pool == nil {
		return
	}
	var ageSec *float64
	if err := d.pool.QueryRow(ctx, `
        SELECT EXTRACT(EPOCH FROM (now() - MIN(created_at)))::double precision
        FROM event_outbox
        WHERE status = 'pending'`).Scan(&ageSec); err != nil {
		return
	}
	v := 0.0
	if ageSec != nil {
		v = *ageSec
	}
	outboxOldestAge.Set(v)
}

// publisherBackoff returns capped exponential backoff. attempt 1 → 1s,
// 2 → 2s, … capped at 5 minutes so a wedged broker doesn't push
// retries past the half-hour MaxAttempts window.
func publisherBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	seconds := math.Pow(2, float64(attempts-1))
	if seconds > 300 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

// --- metrics --------------------------------------------------------------

var (
	outboxPublishedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_outbox_published_total",
		Help: "Outbox rows successfully published to Redpanda, by topic.",
	}, []string{"topic"})

	outboxFailedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_outbox_failed_total",
		Help: "Outbox rows that failed a publish attempt, by topic.",
	}, []string{"topic"})

	outboxDeadTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_outbox_dead_total",
		Help: "Outbox rows moved to dead status (exhausted retries or topic invalid), by topic and reason.",
	}, []string{"topic", "reason"})

	outboxDLQTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ironflyer_outbox_dlq_total",
		Help: "DLQ records emitted by the outbox publisher, by original topic and failure class.",
	}, []string{"topic", "failure_class"})

	outboxOldestAge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ironflyer_outbox_oldest_unpublished_age_seconds",
		Help: "Age in seconds of the oldest pending outbox row, observed by the publisher daemon.",
	})

	publisherMetricsOnce sync.Once
)

func registerPublisherMetrics() {
	publisherMetricsOnce.Do(func() {
		for _, c := range []prometheus.Collector{outboxPublishedTotal, outboxFailedTotal, outboxDeadTotal, outboxDLQTotal, outboxOldestAge} {
			if err := prometheus.Register(c); err != nil {
				if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
					switch existing := are.ExistingCollector.(type) {
					case *prometheus.CounterVec:
						switch c {
						case outboxPublishedTotal:
							outboxPublishedTotal = existing
						case outboxFailedTotal:
							outboxFailedTotal = existing
						case outboxDeadTotal:
							outboxDeadTotal = existing
						case outboxDLQTotal:
							outboxDLQTotal = existing
						}
					case prometheus.Gauge:
						if c == outboxOldestAge {
							outboxOldestAge = existing
						}
					}
				}
			}
		}
	})
}
