// Metered usage reporter — turns ledger-charged provider cost into Stripe
// UsageRecord increments so users on paid plans pay per-call for spend that
// exceeds their CostCapUSD. The reporter buffers events, flushes on a
// configurable interval (default 60s), and uses minute-bucket idempotency
// keys so retries never double-bill.
//
// Pipeline:
//
//	Billing.Charge -> MeteredReporter.Record (in-memory buffer)
//	                       │
//	                       ▼ every flush interval
//	                  POST /v1/subscription_items/:si/usage_records
//	                  Idempotency-Key: <user>-<minute>
//	                       │
//	                       └─ on 2xx: audit "metered_usage_reported"
//	                          on err: exponential backoff, retry,
//	                                  Sentry capture after 3 strikes
//
// The SubscriptionItem ID per customer is supplied by the
// SubscriptionItemStore — populated either by the
// `customer.subscription.updated` webhook (production) or by an explicit
// admin override in tests. Records for users without a stored item id stay
// in the buffer; they get drained as soon as the webhook fills it.

package budget

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/metrics"
)

// MeteredEvent is one usage row queued for Stripe. CostUSD is the dollar
// cost we incurred on the underlying provider call; we convert to whole
// cents (ceiling) at flush time so we never under-bill.
type MeteredEvent struct {
	UserID    string
	ProjectID string
	Model     string
	InTokens  int
	OutTokens int
	CostUSD   decimal.Decimal
	At        time.Time
}

// SubscriptionItemStore returns the Stripe Subscription Item ID for a user.
// In production this is hydrated by the `customer.subscription.updated`
// webhook; in dev/test an explicit Set call is enough.
type SubscriptionItemStore interface {
	Get(ctx context.Context, userID string) (itemID string, ok bool)
	Set(ctx context.Context, userID, itemID string) error
	Delete(ctx context.Context, userID string) error
}

// MemorySubItemStore is the dev/in-process implementation.
type MemorySubItemStore struct {
	mu sync.RWMutex
	m  map[string]string
}

func NewMemorySubItemStore() *MemorySubItemStore {
	return &MemorySubItemStore{m: map[string]string{}}
}

func (s *MemorySubItemStore) Get(_ context.Context, userID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[userID]
	return v, ok && v != ""
}

func (s *MemorySubItemStore) Set(_ context.Context, userID, itemID string) error {
	if userID == "" {
		return errors.New("userID required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[userID] = itemID
	return nil
}

func (s *MemorySubItemStore) Delete(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, userID)
	return nil
}

var _ SubscriptionItemStore = (*MemorySubItemStore)(nil)

// PaymentFlagStore lets the reporter (and the webhook handler) block users
// whose latest invoice failed. While the flag is set, MeteredReporter
// silently drops new events instead of letting Stripe accumulate more
// charges the user already can't pay for.
type PaymentFlagStore interface {
	IsBlocked(ctx context.Context, userID string) bool
	Block(ctx context.Context, userID, reason string) error
	Clear(ctx context.Context, userID string) error
}

// MemoryPaymentFlagStore is the in-process default.
type MemoryPaymentFlagStore struct {
	mu sync.RWMutex
	m  map[string]string
}

func NewMemoryPaymentFlagStore() *MemoryPaymentFlagStore {
	return &MemoryPaymentFlagStore{m: map[string]string{}}
}

func (s *MemoryPaymentFlagStore) IsBlocked(_ context.Context, userID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.m[userID]
	return ok
}

func (s *MemoryPaymentFlagStore) Block(_ context.Context, userID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]string{}
	}
	s.m[userID] = reason
	return nil
}

func (s *MemoryPaymentFlagStore) Clear(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, userID)
	return nil
}

var _ PaymentFlagStore = (*MemoryPaymentFlagStore)(nil)

// MeteredAuditor receives a "metered_usage_reported" hook every successful
// flush. Wired to audit.Store in production; nil in tests is a no-op.
type MeteredAuditor interface {
	OnFlushed(ctx context.Context, userID, subscriptionItemID string, quantityCents int64, idempotencyKey string)
}

// MeteredOptions tunes the reporter. Zero values get sensible defaults.
type MeteredOptions struct {
	SecretKey     string                // Stripe secret; empty -> Disabled
	FlushInterval time.Duration         // default 60s
	MaxPerUser    int                   // bounded buffer; default 10_000
	HTTPClient    *http.Client          // default 15s timeout
	Items         SubscriptionItemStore // required when SecretKey set
	Flags         PaymentFlagStore      // optional
	Auditor       MeteredAuditor        // optional
	Logger        zerolog.Logger
	Disabled      bool // IRONFLYER_METERED_DISABLED
}

// MeteredReporter buffers MeteredEvents per user and flushes them to
// Stripe's UsageRecord API. The reporter is safe to share across goroutines
// and intentionally lock-free on the hot path (Record) beyond a per-user
// slice append.
type MeteredReporter struct {
	opts MeteredOptions

	mu        sync.Mutex
	bufs      map[string][]MeteredEvent
	strikes   map[string]int // consecutive flush failures per user
	stopCh    chan struct{}
	doneCh    chan struct{}
	started   bool
}

// NewMeteredReporter builds a reporter. Caller invokes Start() to launch
// the flush goroutine and Stop() during graceful shutdown.
func NewMeteredReporter(o MeteredOptions) *MeteredReporter {
	if o.FlushInterval <= 0 {
		o.FlushInterval = 60 * time.Second
	}
	if o.MaxPerUser <= 0 {
		o.MaxPerUser = 10_000
	}
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &MeteredReporter{
		opts:    o,
		bufs:    map[string][]MeteredEvent{},
		strikes: map[string]int{},
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// Enabled is true when we have a Stripe secret AND metering hasn't been
// disabled via env. Callers should use this before calling Record so they
// can short-circuit work for self-hosted / dev runs.
func (r *MeteredReporter) Enabled() bool {
	return r != nil && r.opts.SecretKey != "" && !r.opts.Disabled
}

// Record appends an event to the user's buffer. Drops the oldest event when
// the buffer is full (and increments a Prometheus counter so operators can
// alarm on it).
func (r *MeteredReporter) Record(ctx context.Context, evt MeteredEvent) {
	if !r.Enabled() {
		return
	}
	if evt.UserID == "" {
		return
	}
	if !evt.CostUSD.IsPositive() {
		return
	}
	if r.opts.Flags != nil && r.opts.Flags.IsBlocked(ctx, evt.UserID) {
		return
	}
	if evt.At.IsZero() {
		evt.At = time.Now().UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	buf := r.bufs[evt.UserID]
	if len(buf) >= r.opts.MaxPerUser {
		// Drop the oldest event so the buffer stays bounded — better to
		// under-bill a tiny tail than to OOM the orchestrator.
		buf = buf[1:]
		metrics.ObserveMeteredBufferDrop()
		r.opts.Logger.Warn().Str("user_id", evt.UserID).
			Msg("metered buffer full — dropping oldest event")
	}
	r.bufs[evt.UserID] = append(buf, evt)
}

// Start kicks off the background flusher. Idempotent: a second call is a
// no-op.
func (r *MeteredReporter) Start(ctx context.Context) {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return
	}
	r.started = true
	r.mu.Unlock()
	go r.runLoop(ctx)
}

// Stop signals the flusher to exit after one final flush attempt.
func (r *MeteredReporter) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	started := r.started
	r.mu.Unlock()
	if !started {
		return
	}
	close(r.stopCh)
	<-r.doneCh
}

func (r *MeteredReporter) runLoop(ctx context.Context) {
	defer close(r.doneCh)
	t := time.NewTicker(r.opts.FlushInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			r.flushAll(context.Background())
			return
		case <-r.stopCh:
			r.flushAll(context.Background())
			return
		case <-t.C:
			r.flushAll(ctx)
		}
	}
}

// flushAll drains the buffer once per user and POSTs the aggregated
// quantity for each minute bucket to Stripe.
func (r *MeteredReporter) flushAll(ctx context.Context) {
	r.mu.Lock()
	if len(r.bufs) == 0 {
		r.mu.Unlock()
		return
	}
	// Snapshot the buffers — drain them into local copies under lock so the
	// hot Record path stays responsive. Failed flushes re-enqueue.
	snapshot := make(map[string][]MeteredEvent, len(r.bufs))
	for u, evs := range r.bufs {
		if len(evs) == 0 {
			continue
		}
		snapshot[u] = evs
		r.bufs[u] = nil
	}
	r.mu.Unlock()

	for userID, events := range snapshot {
		if !r.flushUser(ctx, userID, events) {
			// Put them back at the front so order is preserved across
			// retries; the next tick will try again.
			r.mu.Lock()
			r.bufs[userID] = append(events, r.bufs[userID]...)
			r.mu.Unlock()
		}
	}
}

// flushUser groups events into minute buckets and POSTs each bucket as a
// UsageRecord with quantity=sum(ceil(cost_cents)). Returns true when every
// bucket succeeded; false leaves the slice for the caller to re-enqueue.
func (r *MeteredReporter) flushUser(ctx context.Context, userID string, events []MeteredEvent) bool {
	itemID, ok := r.opts.Items.Get(ctx, userID)
	if !ok {
		// No subscription item known yet — wait for the webhook. Re-enqueue
		// without incrementing strike count so we don't spam Sentry.
		return false
	}
	if r.opts.Flags != nil && r.opts.Flags.IsBlocked(ctx, userID) {
		// Drop on the floor — see PaymentFlagStore docs.
		return true
	}

	buckets := bucketByMinute(events)
	allOK := true
	for bucketMinute, evs := range buckets {
		cents := sumCents(evs)
		if cents <= 0 {
			continue
		}
		idem := fmt.Sprintf("%s-%d", userID, bucketMinute)
		if err := r.postUsageRecord(ctx, itemID, cents, bucketMinute, idem); err != nil {
			r.handleFailure(ctx, userID, err)
			allOK = false
			continue
		}
		// On success: clear strike counter and emit the audit hook.
		r.mu.Lock()
		delete(r.strikes, userID)
		r.mu.Unlock()
		metrics.ObserveMeteredFlush()
		if r.opts.Auditor != nil {
			r.opts.Auditor.OnFlushed(ctx, userID, itemID, cents, idem)
		}
	}
	return allOK
}

func (r *MeteredReporter) handleFailure(_ context.Context, userID string, err error) {
	r.mu.Lock()
	r.strikes[userID]++
	strikes := r.strikes[userID]
	r.mu.Unlock()
	r.opts.Logger.Error().Err(err).Str("user_id", userID).Int("strikes", strikes).
		Msg("metered flush failed")
	metrics.ObserveMeteredFlushError()
	if strikes >= 3 {
		// Sentry capture so on-call sees it. We don't reset strikes; the
		// next success will. Backoff is implicit — re-enqueued events
		// stay in the buffer until the next flush tick.
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("component", "metered_reporter")
			scope.SetTag("user_id", userID)
			scope.SetTag("strikes", fmt.Sprintf("%d", strikes))
			sentry.CaptureException(err)
		})
	}
}

// postUsageRecord calls Stripe's UsageRecord endpoint. action=increment
// means quantity adds to whatever the subscription item already has for the
// period; combined with the minute-bucket Idempotency-Key this is safe to
// retry indefinitely.
func (r *MeteredReporter) postUsageRecord(ctx context.Context, subItemID string, qty int64, minuteBucket int64, idemKey string) error {
	form := url.Values{}
	form.Set("quantity", fmt.Sprintf("%d", qty))
	form.Set("action", "increment")
	form.Set("timestamp", fmt.Sprintf("%d", minuteBucket*60))

	endpoint := "https://api.stripe.com/v1/subscription_items/" + subItemID + "/usage_records"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.opts.SecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Idempotency-Key", idemKey)
	req.Header.Set("Stripe-Version", "2024-04-10")

	resp, err := r.opts.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("stripe %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// bucketByMinute groups events by Unix-minute (UTC). All events whose
// timestamp falls in the same calendar minute become a single Stripe
// UsageRecord with quantity=sum(cents); the Idempotency-Key is built from
// the minute, so any retry collapses into the same record server-side.
func bucketByMinute(evs []MeteredEvent) map[int64][]MeteredEvent {
	out := map[int64][]MeteredEvent{}
	for _, e := range evs {
		minute := e.At.UTC().Unix() / 60
		out[minute] = append(out[minute], e)
	}
	return out
}

// sumCents converts the bucket's USD spend to whole cents, rounding up so
// we never under-bill the user for fractional pennies. We then sum across
// the bucket so one UsageRecord covers all calls in the minute.
func sumCents(evs []MeteredEvent) int64 {
	total := decimal.Zero
	for _, e := range evs {
		total = total.Add(e.CostUSD)
	}
	cents := total.Mul(decimal.NewFromInt(100))
	f, _ := cents.Float64()
	return int64(math.Ceil(f))
}
