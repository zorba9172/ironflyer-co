package notify

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

const (
	defaultWorkerIntervalMS = 2000
	defaultWorkerBatch      = 16
	maxAttempts             = 5
	baseBackoff             = 30 * time.Second
	maxBackoff              = 30 * time.Minute
)

// Hub is the in-process pub/sub fan-out the GraphQL subscription
// reads from. The Worker calls Publish after a successful in-app
// delivery so connected subscribers receive the new Notification
// without a polling delay.
type Hub interface {
	Publish(userID string, n Notification)
}

// Worker drains the OutboxStore on a tick, delivers each row to the
// email + in-app channels, and stamps the row accordingly. Failures
// schedule a retry with exponential backoff; max-attempts is the
// dead-letter ceiling.
type Worker struct {
	store    NotificationStore
	outbox   OutboxStore
	sender   EmailSender
	hub      Hub
	prefs    PrefsStore
	logger   zerolog.Logger
	from     string
	dashURL  string
	interval time.Duration
	batch    int
}

// WorkerOpts holds optional knobs. Zero values pick env / package
// defaults.
type WorkerOpts struct {
	Interval     time.Duration
	Batch        int
	From         string
	DashboardURL string
}

// NewWorker wires the worker. Any of store / outbox / sender may be
// nil; the worker no-ops if outbox is nil so dev boots cleanly.
func NewWorker(store NotificationStore, outbox OutboxStore, sender EmailSender, prefs PrefsStore, hub Hub, opts WorkerOpts, logger zerolog.Logger) *Worker {
	if sender == nil {
		sender = NewNoopSender(logger)
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = time.Duration(intEnv("IRONFLYER_NOTIFY_WORKER_INTERVAL_MS", defaultWorkerIntervalMS)) * time.Millisecond
	}
	batch := opts.Batch
	if batch <= 0 {
		batch = intEnv("IRONFLYER_NOTIFY_WORKER_BATCH", defaultWorkerBatch)
	}
	return &Worker{
		store:    store,
		outbox:   outbox,
		sender:   sender,
		prefs:    prefs,
		hub:      hub,
		logger:   logger,
		from:     opts.From,
		dashURL:  opts.DashboardURL,
		interval: interval,
		batch:    batch,
	}
}

// Run blocks on a ticker until ctx is cancelled, draining the outbox
// every tick. A nil outbox returns immediately (dev no-op).
func (w *Worker) Run(ctx context.Context) {
	if w == nil || w.outbox == nil {
		return
	}
	w.logger.Info().
		Dur("interval", w.interval).
		Int("batch", w.batch).
		Msg("notify: outbox worker started")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("notify: outbox worker stopped")
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// tick drains one batch.
func (w *Worker) tick(ctx context.Context) {
	items, err := w.outbox.Claim(ctx, w.batch)
	if err != nil {
		w.logger.Warn().Err(err).Msg("notify: outbox claim failed")
		return
	}
	for i := range items {
		w.deliver(ctx, items[i])
	}
}

// deliver handles a single OutboxItem. Per-channel success is recorded
// independently so a partial failure retries only the failed leg.
func (w *Worker) deliver(ctx context.Context, item OutboxItem) {
	var firstErr error

	if item.InAppTarget && item.InAppSentAt == nil {
		if err := w.deliverInApp(ctx, item); err != nil {
			w.logger.Warn().Err(err).Str("kind", string(item.Kind)).Str("user", item.UserID).
				Msg("notify: in-app deliver failed")
			firstErr = err
		}
	}

	if item.EmailTarget && item.EmailSentAt == nil {
		if err := w.deliverEmail(ctx, item); err != nil {
			w.logger.Warn().Err(err).Str("kind", string(item.Kind)).Str("user", item.UserID).
				Msg("notify: email deliver failed")
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if firstErr == nil {
		if err := w.outbox.MarkDelivered(ctx, item.ID); err != nil {
			w.logger.Warn().Err(err).Str("id", item.ID).Msg("notify: mark delivered failed")
		}
		return
	}

	nextAttempts := item.Attempts + 1
	if nextAttempts >= maxAttempts {
		if err := w.outbox.MarkDeadLettered(ctx, item.ID, firstErr.Error()); err != nil {
			w.logger.Warn().Err(err).Str("id", item.ID).Msg("notify: mark dead-lettered failed")
		}
		w.logger.Error().Str("id", item.ID).Str("kind", string(item.Kind)).Int("attempts", nextAttempts).
			Err(firstErr).Msg("notify: dead-lettered after max attempts")
		return
	}
	backoff := computeBackoff(nextAttempts)
	if err := w.outbox.MarkFailed(ctx, item.ID, firstErr.Error(), backoff); err != nil {
		w.logger.Warn().Err(err).Str("id", item.ID).Msg("notify: mark failed update")
	}
}

// deliverInApp writes the durable row + publishes on the hub.
func (w *Worker) deliverInApp(ctx context.Context, item OutboxItem) error {
	if w.store == nil {
		return errors.New("in-app store not wired")
	}
	n, err := renderInApp(item.Kind, item.Payload, w.dashURL)
	if err != nil {
		return err
	}
	n.UserID = item.UserID
	if err := w.store.Create(ctx, n); err != nil {
		return err
	}
	if err := w.outbox.MarkInAppSent(ctx, item.ID); err != nil {
		return err
	}
	if w.hub != nil {
		w.hub.Publish(item.UserID, n)
	}
	return nil
}

// deliverEmail renders + sends + stamps email_sent_at.
func (w *Worker) deliverEmail(ctx context.Context, item OutboxItem) error {
	to := w.lookupEmail(ctx, item.UserID)
	if to == "" {
		return errors.New("no email on file")
	}
	content, err := renderEmail(item.Kind, item.Payload, w.dashURL)
	if err != nil {
		return err
	}
	if err := w.sender.Send(ctx, to, content.Subject, content.HTMLBody, content.TextBody); err != nil {
		return err
	}
	return w.outbox.MarkEmailSent(ctx, item.ID)
}

// lookupEmail returns the configured email for the user from PrefsStore.
// Empty string when prefs aren't wired or the user has no email on file.
func (w *Worker) lookupEmail(ctx context.Context, userID string) string {
	if w.prefs == nil {
		return ""
	}
	rule, err := w.prefs.Get(ctx, userID)
	if err != nil {
		return ""
	}
	return rule.Email
}

// computeBackoff returns min(2^attempts * baseBackoff, maxBackoff).
func computeBackoff(attempts int) time.Duration {
	d := baseBackoff
	for i := 1; i < attempts; i++ {
		d *= 2
		if d >= maxBackoff {
			return maxBackoff
		}
	}
	return d
}

func intEnv(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
