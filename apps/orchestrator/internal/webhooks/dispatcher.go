package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/domain"
)

// Defaults tuned for a single orchestrator node. The pool keeps a hard cap
// on concurrency so a slow receiver can't starve faster ones.
const (
	defaultConcurrency      = 16
	defaultRetries          = 3
	defaultInitialBackoff   = 500 * time.Millisecond
	defaultRequestTimeout   = 10 * time.Second
	defaultDisableThreshold = 10 // consecutive failures before auto-disable
)

// FailureNotifier is the optional hook the dispatcher calls when a
// subscription is auto-disabled after too many consecutive failures. The
// notify package implements this with an email + in-app message so the user
// finds out before they wonder why Slack stopped pinging.
type FailureNotifier interface {
	WebhookDisabled(ctx context.Context, userID, webhookURL string, failures int)
}

// EventEnvelope is the JSON shape POSTed to subscriber URLs. We intentionally
// keep it minimal and stable — receivers will pin against this contract.
type EventEnvelope struct {
	DeliveryID  string       `json:"deliveryId"`
	Type        string       `json:"type"`         // e.g. "run_complete", "gate_failed", "webhook_test"
	ProjectID   string       `json:"projectId"`
	OccurredAt  time.Time    `json:"occurredAt"`
	Event       domain.Event `json:"event"`        // the raw orchestrator event
	WebhookID   string       `json:"webhookId"`
	Attempt     int          `json:"attempt"`      // 1-based
}

// Dispatcher fans events out to webhook subscriptions with retry + signing.
// Use NewDispatcher and call Stop on shutdown to drain in-flight goroutines.
type Dispatcher struct {
	store       Store
	client      *http.Client
	logger      zerolog.Logger
	notifier    FailureNotifier
	concurrency chan struct{}
	retries     int
	backoff     time.Duration
	disableAt   int
}

// NewDispatcher constructs a dispatcher with sensible defaults.
func NewDispatcher(store Store, logger zerolog.Logger) *Dispatcher {
	return &Dispatcher{
		store:       store,
		client:      &http.Client{Timeout: defaultRequestTimeout},
		logger:      logger,
		concurrency: make(chan struct{}, defaultConcurrency),
		retries:     defaultRetries,
		backoff:     defaultInitialBackoff,
		disableAt:   defaultDisableThreshold,
	}
}

// WithNotifier attaches a failure notifier for auto-disable events. Returns
// the dispatcher for chained configuration during startup.
func (d *Dispatcher) WithNotifier(n FailureNotifier) *Dispatcher {
	d.notifier = n
	return d
}

// Dispatch matches subscriptions and fires deliveries asynchronously. Returns
// the number of subscriptions queued so callers (HTTP handlers, tests) can
// confirm the fan-out actually fanned.
func (d *Dispatcher) Dispatch(ctx context.Context, userID, projectID string, evt domain.Event) int {
	subs, err := d.store.ListMatching(ctx, userID, projectID)
	if err != nil {
		d.logger.Warn().Err(err).Str("user", userID).Msg("webhook list failed")
		return 0
	}
	queued := 0
	for _, s := range subs {
		if !s.Matches(eventName(evt)) {
			continue
		}
		sub := s
		go d.deliver(context.Background(), sub, projectID, evt)
		queued++
	}
	return queued
}

// DeliverSynthetic is the entry point used by /webhooks/{id}/test — it
// bypasses the store filter and pushes a synthetic event so the user can
// verify their endpoint without waiting for a real run.
func (d *Dispatcher) DeliverSynthetic(ctx context.Context, sub Subscription, projectID string) {
	evt := domain.Event{
		ID:        "webhook-test-" + uuid.NewString(),
		Step:      "webhook_test",
		Message:   "synthetic test event from Ironflyer",
		Status:    "done",
		CreatedAt: time.Now().UTC(),
	}
	go d.deliver(ctx, sub, projectID, evt)
}

// deliver runs a single subscription delivery with retry/backoff. It is the
// only function that mutates Subscription stats — all callers go through
// store.UpdateStats here, never elsewhere.
func (d *Dispatcher) deliver(ctx context.Context, sub Subscription, projectID string, evt domain.Event) {
	d.concurrency <- struct{}{}
	defer func() { <-d.concurrency }()

	deliveryID := uuid.NewString()
	body := EventEnvelope{
		DeliveryID: deliveryID,
		Type:       eventName(evt),
		ProjectID:  projectID,
		OccurredAt: evt.CreatedAt,
		Event:      evt,
		WebhookID:  sub.ID,
	}

	var lastErr error
	for attempt := 1; attempt <= d.retries; attempt++ {
		body.Attempt = attempt
		payload, _ := json.Marshal(body)
		err := d.post(ctx, sub, payload, body.Type, deliveryID)
		if err == nil {
			_ = d.store.UpdateStats(ctx, sub.ID, time.Now().UTC(), 0, false)
			return
		}
		lastErr = err
		d.logger.Debug().Err(err).Str("sub", sub.ID).Int("attempt", attempt).Msg("webhook delivery failed")
		// Exponential backoff: 0.5s, 1s, 2s ...
		sleep := d.backoff * time.Duration(1<<(attempt-1))
		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			return
		}
	}

	// All retries failed → bump failure count, maybe auto-disable.
	failures := sub.FailureCount + 1
	disabled := failures >= d.disableAt
	if err := d.store.UpdateStats(ctx, sub.ID, time.Now().UTC(), failures, disabled); err != nil {
		d.logger.Warn().Err(err).Str("sub", sub.ID).Msg("webhook updateStats failed")
	}
	if disabled && d.notifier != nil {
		d.notifier.WebhookDisabled(ctx, sub.UserID, sub.URL, failures)
	}
	d.logger.Warn().Err(lastErr).Str("sub", sub.ID).Int("failures", failures).
		Bool("disabled", disabled).Msg("webhook giving up")
}

func (d *Dispatcher) post(ctx context.Context, sub Subscription, payload []byte, eventType, deliveryID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Ironflyer-Webhooks/1.0")
	req.Header.Set("X-Ironflyer-Event", eventType)
	req.Header.Set("X-Ironflyer-Delivery", deliveryID)
	req.Header.Set("Idempotency-Key", deliveryID)
	if sub.Secret != "" {
		mac := hmac.New(sha256.New, []byte(sub.Secret))
		mac.Write(payload)
		req.Header.Set("X-Ironflyer-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Drain so the connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("webhook responded %d", resp.StatusCode)
}

// eventName picks a stable, machine-friendly name for the event so receivers
// can filter without parsing the free-form Message string. We use a small
// allowlist of known steps and fall back to Step itself.
func eventName(evt domain.Event) string {
	switch evt.Step {
	case "run":
		if evt.Status == "done" {
			return "run_complete"
		}
		if evt.Status == "failed" {
			return "run_failed"
		}
		return "run_running"
	case "gate":
		if evt.Status == "failed" {
			return "gate_failed"
		}
		if evt.Status == "done" {
			return "gate_passed"
		}
		return "gate_running"
	case "patch":
		return "patch_" + evt.Status
	case "webhook_test":
		return "webhook_test"
	}
	if evt.Step != "" {
		return evt.Step + "_" + evt.Status
	}
	return "event"
}
