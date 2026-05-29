package learning

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/outboxhooks"
	"ironflyer/core/orchestrator/internal/operations/events"
)

// LearningTopic is the dedicated Redpanda topic for outcome events.
// Stamped via events.TopicFor so dev/staging/prod inherit the canonical
// env prefix.
func LearningTopic() string {
	return events.TopicFor("", "learning", "outcomes", 1)
}

// PatternTopic carries miner outputs back to strategy adapters. It
// sits on the same domain so consumers can subscribe with a glob on
// "ifly.*.learning.*.v1".
func PatternTopic() string {
	return events.TopicFor("", "learning", "patterns", 1)
}

// Publisher writes OutcomeEvents to the durable event_outbox. It is
// safe to call Publish from any goroutine; the publisher itself is a
// thin wrapper that lazily acquires a tx — call sites that already own
// a tx should use PublishInTx instead so the outcome write commits
// atomically with the business write.
//
// In dev / boot-without-Postgres the publisher falls back to an
// in-memory ring that the miner can still read from; production
// always goes through Postgres.
type Publisher struct {
	outbox events.Outbox
	log    zerolog.Logger

	// observer is set by the in-memory adapter so a dev build still
	// gets miner-readable events without standing up Postgres.
	obsMu    sync.RWMutex
	observer func(OutcomeEvent)

	// patternObserver is the in-process fan-out for miner-derived
	// PatternObservations. PublishPattern always writes to the outbox
	// (durable, cross-process); this hook lets same-process strategy
	// adapters (e.g. the ProfitGuard PolicyAdapter) react without a
	// Redpanda round-trip. Nil-safe: no observer → outbox-only.
	patObsMu        sync.RWMutex
	patternObserver func(PatternObservation)

	// stats counters power the snapshot fallback when no ClickHouse is
	// available — read by the memory store.
	today    atomic.Int64
	allTime  atomic.Int64
	lastDate atomic.Value // time.Time, used to roll `today` at midnight UTC
}

// NewPublisher constructs a publisher that writes through the supplied
// outbox. outbox MAY be nil — Publish then degrades to observer-only
// (dev). The log is used for non-fatal warnings; production callers
// should pass a child logger.
func NewPublisher(outbox events.Outbox, log zerolog.Logger) *Publisher {
	p := &Publisher{outbox: outbox, log: log}
	p.lastDate.Store(time.Now().UTC().Truncate(24 * time.Hour))
	return p
}

// SetObserver registers a synchronous callback invoked on every
// Publish — used by the memory-backed Store to maintain its own
// in-process projection. Pass nil to unregister.
func (p *Publisher) SetObserver(fn func(OutcomeEvent)) {
	if p == nil {
		return
	}
	p.obsMu.Lock()
	p.observer = fn
	p.obsMu.Unlock()
}

// SetPatternObserver registers a synchronous callback invoked on every
// PublishPattern — used by same-process strategy adapters (the
// ProfitGuard PolicyAdapter) to react to miner output without a
// Redpanda round-trip. Pass nil to unregister. Nil-safe on a nil
// Publisher.
func (p *Publisher) SetPatternObserver(fn func(PatternObservation)) {
	if p == nil {
		return
	}
	p.patObsMu.Lock()
	p.patternObserver = fn
	p.patObsMu.Unlock()
}

// Publish enqueues evt onto the durable outbox. Missing fields are
// stamped from defaults (ID, Timestamp, TenantID via outboxhooks ctx).
// Errors are logged at warn — the producer keeps moving.
func (p *Publisher) Publish(ctx context.Context, evt OutcomeEvent) {
	if p == nil {
		return
	}
	evt = normalize(ctx, evt)
	p.fanout(evt)
	p.tick(evt)
	if p.outbox == nil {
		return
	}
	e := buildOutboxEvent(evt)
	if _, err := p.outbox.Enqueue(ctx, e); err != nil {
		p.log.Warn().Err(err).
			Str("kind", string(evt.Kind)).
			Str("execution_id", evt.ExecutionID).
			Msg("learning publish: outbox enqueue failed")
	}
}

// PublishInTx writes the outcome inside the caller's pgx.Tx so the
// business write and the learning row commit atomically. This is the
// preferred path inside Postgres-backed services — the outbox-only
// Publish path lacks transactional guarantees.
func (p *Publisher) PublishInTx(ctx context.Context, tx pgx.Tx, evt OutcomeEvent) error {
	if p == nil {
		return nil
	}
	evt = normalize(ctx, evt)
	e := buildOutboxEvent(evt)
	if _, err := events.EnqueueTx(ctx, tx, e); err != nil {
		return fmt.Errorf("learning publish in tx: %w", err)
	}
	p.fanout(evt)
	p.tick(evt)
	return nil
}

// PublishPattern emits a miner-derived PatternObservation onto the
// pattern topic so the strategy adapter can react.
func (p *Publisher) PublishPattern(ctx context.Context, obs PatternObservation) {
	if p == nil {
		return
	}
	if obs.ID == "" {
		obs.ID = uuid.NewString()
	}
	if obs.ObservedAt.IsZero() {
		obs.ObservedAt = time.Now().UTC()
	}
	if obs.TenantID == "" {
		obs.TenantID = outboxhooks.TenantID(ctx)
	}
	// In-process fan-out first so an observer-only (nil outbox) dev
	// build still drives the strategy adapters.
	p.fanoutPattern(obs)
	if p.outbox == nil {
		return
	}
	payload := map[string]any{
		"id":          obs.ID,
		"tenant_id":   obs.TenantID,
		"pattern":     obs.Pattern,
		"target":      obs.Target,
		"direction":   obs.Direction,
		"confidence":  obs.Confidence,
		"evidence":    obs.Evidence,
		"observed_at": obs.ObservedAt.UTC().Format(time.RFC3339Nano),
	}
	e := events.Event{
		Topic:   PatternTopic(),
		Key:     obs.TenantID,
		Type:    "learning.pattern.v1",
		Version: 1,
		Payload: payload,
		Headers: map[string]any{"tenant_id": obs.TenantID},
	}
	if _, err := p.outbox.Enqueue(ctx, e); err != nil {
		p.log.Warn().Err(err).Str("pattern", obs.Pattern).
			Msg("learning publish: pattern enqueue failed")
	}
}

// Today returns the running count of outcomes published since the last
// midnight-UTC roll. Used by the memory Store fallback.
func (p *Publisher) Today() int64 {
	if p == nil {
		return 0
	}
	return p.today.Load()
}

// AllTime returns the lifetime publish count since process start.
func (p *Publisher) AllTime() int64 {
	if p == nil {
		return 0
	}
	return p.allTime.Load()
}

func (p *Publisher) tick(evt OutcomeEvent) {
	p.allTime.Add(1)
	day := time.Now().UTC().Truncate(24 * time.Hour)
	last, _ := p.lastDate.Load().(time.Time)
	if !last.Equal(day) {
		p.lastDate.Store(day)
		p.today.Store(0)
	}
	p.today.Add(1)
	_ = evt
}

func (p *Publisher) fanout(evt OutcomeEvent) {
	p.obsMu.RLock()
	fn := p.observer
	p.obsMu.RUnlock()
	if fn != nil {
		// Best-effort, never block.
		go func() {
			defer func() { _ = recover() }()
			fn(evt)
		}()
	}
}

func (p *Publisher) fanoutPattern(obs PatternObservation) {
	p.patObsMu.RLock()
	fn := p.patternObserver
	p.patObsMu.RUnlock()
	if fn != nil {
		// Best-effort, never block the miner tick.
		go func() {
			defer func() { _ = recover() }()
			fn(obs)
		}()
	}
}

func normalize(ctx context.Context, evt OutcomeEvent) OutcomeEvent {
	if evt.ID == "" {
		evt.ID = uuid.NewString()
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}
	if evt.TenantID == "" {
		evt.TenantID = outboxhooks.TenantID(ctx)
	}
	if evt.Attributes == nil {
		evt.Attributes = map[string]any{}
	}
	if evt.Tags == nil {
		evt.Tags = map[string]string{}
	}
	return evt
}

func buildOutboxEvent(evt OutcomeEvent) events.Event {
	payload := map[string]any{
		"id":           evt.ID,
		"tenant_id":    evt.TenantID,
		"execution_id": evt.ExecutionID,
		"kind":         string(evt.Kind),
		"timestamp":    evt.Timestamp.UTC().Format(time.RFC3339Nano),
		"attributes":   evt.Attributes,
		"tags":         evt.Tags,
	}
	if evt.Success != nil {
		payload["success"] = *evt.Success
	}
	if evt.CostUSD != nil {
		payload["cost_usd"] = evt.CostUSD.String()
	}
	if evt.MarginUSD != nil {
		payload["margin_usd"] = evt.MarginUSD.String()
	}
	id, err := uuid.Parse(evt.ID)
	if err != nil {
		id = uuid.New()
	}
	return events.Event{
		ID:      id,
		Topic:   LearningTopic(),
		Key:     keyFor(evt),
		Type:    "learning.outcome." + string(evt.Kind) + ".v1",
		Version: 1,
		Payload: payload,
		Headers: map[string]any{"tenant_id": evt.TenantID},
	}
}

func keyFor(evt OutcomeEvent) string {
	if evt.ExecutionID != "" {
		return evt.ExecutionID
	}
	if evt.TenantID != "" {
		return evt.TenantID
	}
	return evt.ID
}

// BoolPtr is a tiny helper so call sites don't have to declare a
// throwaway local for the Success pointer field.
func BoolPtr(v bool) *bool { return &v }

// DecimalPtr is the matching helper for CostUSD/MarginUSD. Returns
// nil when the value is the zero decimal so we don't litter facts
// with $0 noise.
func DecimalPtr(v decimal.Decimal) *decimal.Decimal {
	if v.IsZero() {
		return nil
	}
	out := v
	return &out
}
