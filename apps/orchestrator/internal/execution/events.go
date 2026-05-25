package execution

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// Event type names emitted onto the execution_events feed. Subscribers
// (the GraphQL executionFeed subscription, dashboards, ProfitGuard
// audit) switch on these strings.
const (
	EventCreated             = "created"
	EventAdmitted            = "admitted"
	EventStarted             = "started"
	EventReserved            = "reserved"
	EventCostAdded           = "cost_added"
	EventRevenueAdded        = "revenue_added"
	EventScoreUpdated        = "score_updated"
	EventExpectationUpdated  = "expectation_updated"
	EventPaused              = "paused"
	EventResumed             = "resumed"
	EventSucceeded           = "succeeded"
	EventFailed              = "failed"
	EventStopped             = "stopped"
	EventKilled              = "killed"
	EventRefunded            = "refunded"
	EventProfitGuardDecision = "profitguard_decision"
)

// broker is the in-process fan-out for execution events. It is keyed
// by execution_id and only holds subscribers for as long as the caller
// keeps the returned channel alive. The Postgres backend layers
// LISTEN/NOTIFY on top of this so cross-process subscribers also see
// the events; the memory backend uses the broker directly.
//
// The broker is intentionally tiny — no persistence, no replay,
// bounded per-channel buffer. If a subscriber lags past the buffer the
// event is dropped (the durable record is in execution_events anyway,
// so a lagging UI can backfill via a Query).
type broker struct {
	mu   sync.RWMutex
	subs map[string]map[*subscription]struct{}
}

type subscription struct {
	ch chan Event
}

// channelBufferSize is the per-subscriber send-buffer depth. Sized so
// that a brief UI render hiccup does not drop events for a typical
// execution (a handful of events per second).
const channelBufferSize = 64

func newBroker() *broker {
	return &broker{subs: make(map[string]map[*subscription]struct{})}
}

// subscribe registers a new subscriber for the given execution and
// returns the receive channel. The returned cleanup function MUST be
// called when the subscriber is done (typically deferred); without it
// the broker leaks the goroutine that drains the channel.
func (b *broker) subscribe(executionID string) (<-chan Event, func()) {
	sub := &subscription{ch: make(chan Event, channelBufferSize)}
	b.mu.Lock()
	if b.subs[executionID] == nil {
		b.subs[executionID] = make(map[*subscription]struct{})
	}
	b.subs[executionID][sub] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if set, ok := b.subs[executionID]; ok {
			delete(set, sub)
			if len(set) == 0 {
				delete(b.subs, executionID)
			}
		}
		b.mu.Unlock()
		close(sub.ch)
	}
	return sub.ch, cancel
}

// publish fans the event out to every live subscriber for the
// execution. Non-blocking: subscribers whose buffer is full silently
// drop the event (see channelBufferSize comment).
func (b *broker) publish(evt Event) {
	b.mu.RLock()
	set := b.subs[evt.ExecutionID]
	// Snapshot the subscriber list so we don't hold the lock across
	// channel sends. The sends themselves are non-blocking.
	subs := make([]*subscription, 0, len(set))
	for s := range set {
		subs = append(subs, s)
	}
	b.mu.RUnlock()

	for _, s := range subs {
		select {
		case s.ch <- evt:
		default:
			// Buffer full — drop and rely on execution_events table
			// for durable replay.
		}
	}
}

// decodeGateEvent projects one execution_events row into a GateEvent.
// gate.verdict.v1 carries an explicit "status" field; the older
// marker types (gate.failed.v1, gate.passed.v1, gate.skipped.v1,
// gate.repaired.v1) imply the status from the event_type itself.
// Returns (zero, false) when the payload is undecipherable so the
// caller can skip the row without surfacing the error.
func decodeGateEvent(eventType string, raw []byte, created time.Time) (GateEvent, bool) {
	payload := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			payload = map[string]any{}
		}
	}
	ge := GateEvent{OccurredAt: created}
	if name, ok := payload["gate"].(string); ok {
		ge.Gate = name
	} else if name, ok := payload["name"].(string); ok {
		ge.Gate = name
	}
	if n, ok := payload["issues_count"].(float64); ok {
		ge.IssuesCount = int(n)
	} else if n, ok := payload["issues"].(float64); ok {
		ge.IssuesCount = int(n)
	}
	switch eventType {
	case EventGatePassedV1:
		ge.Status = "pass"
	case EventGateFailedV1:
		ge.Status = "fail"
	case EventGateRepairedV1:
		ge.Status = "repaired"
	case EventGateSkippedV1:
		ge.Status = "skipped"
	case EventGateVerdictV1:
		if s, ok := payload["status"].(string); ok {
			ge.Status = normaliseGateStatus(s)
		}
	}
	if ge.Status == "" {
		return GateEvent{}, false
	}
	return ge, true
}

// decodePatchAppliedEvent projects one patch.applied.v1 row.
func decodePatchAppliedEvent(raw []byte, created time.Time) (PatchAppliedEvent, bool) {
	payload := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return PatchAppliedEvent{}, false
		}
	}
	pe := PatchAppliedEvent{AppliedAt: created}
	if id, ok := payload["patch_id"].(string); ok {
		pe.PatchID = id
	} else if id, ok := payload["id"].(string); ok {
		pe.PatchID = id
	}
	if rawPaths, ok := payload["affected_paths"].([]any); ok {
		for _, p := range rawPaths {
			if s, ok := p.(string); ok && s != "" {
				pe.AffectedPaths = append(pe.AffectedPaths, s)
			}
		}
	} else if rawPaths, ok := payload["paths"].([]any); ok {
		for _, p := range rawPaths {
			if s, ok := p.(string); ok && s != "" {
				pe.AffectedPaths = append(pe.AffectedPaths, s)
			}
		}
	}
	if pe.PatchID == "" && len(pe.AffectedPaths) == 0 {
		return PatchAppliedEvent{}, false
	}
	return pe, true
}

// decodeRecoveryAttempt projects one recovery.recipe_*.v1 row.
func decodeRecoveryAttempt(eventType string, raw []byte, created time.Time) (RecoveryAttempt, bool) {
	payload := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			payload = map[string]any{}
		}
	}
	ra := RecoveryAttempt{OccurredAt: created}
	if sig, ok := payload["failure_signature"].(string); ok {
		ra.FailureSignature = sig
	} else if sig, ok := payload["signature"].(string); ok {
		ra.FailureSignature = sig
	}
	if gate, ok := payload["gate"].(string); ok {
		ra.Gate = gate
	}
	if eventType == EventRecoveryApplyV1 {
		ra.Applied = true
		// "success" defaults to true for recipe_applied unless
		// payload explicitly says otherwise.
		ra.Success = true
		if v, ok := payload["success"].(bool); ok {
			ra.Success = v
		}
	} else {
		// recovery.recipe_hit.v1 — looked up but not necessarily
		// applied yet.
		if v, ok := payload["applied"].(bool); ok {
			ra.Applied = v
		}
		if v, ok := payload["success"].(bool); ok {
			ra.Success = v
		}
	}
	return ra, true
}

// normaliseGateStatus maps free-form status strings onto the wow-loop
// vocabulary: "pass" | "fail" | "repaired" | "skipped". Unknown
// strings fall through unchanged so the wow-loop builder can render
// them as-is rather than swallowing a future status it does not yet
// recognise.
func normaliseGateStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "pass", "passed":
		return "pass"
	case "fail", "failed":
		return "fail"
	case "repair", "repaired":
		return "repaired"
	case "skip", "skipped":
		return "skipped"
	default:
		return s
	}
}

// subscribeWithContext wires the broker subscription to a context so
// the caller gets a single <-chan Event that closes when ctx is done
// OR the broker tears down the subscription. This is the shape both
// backends return from Service.SubscribeEvents.
func (b *broker) subscribeWithContext(ctx context.Context, executionID string) <-chan Event {
	in, cancel := b.subscribe(executionID)
	out := make(chan Event, channelBufferSize)
	go func() {
		defer cancel()
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-in:
				if !ok {
					return
				}
				select {
				case out <- evt:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}
