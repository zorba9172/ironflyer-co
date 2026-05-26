// Package outboxhooks is the thin coupling layer between V22
// transactional writers (wallet, ledger, profitguard, execution
// settler, patch lifecycle, gate runner) and the durable event_outbox
// table. Each helper writes ONE outbox row inside the caller's pgx.Tx
// so the business state change and the durable event commit (or roll
// back) atomically.
//
// The constructors here exist so callers do not need to know the topic
// taxonomy, the standard header set, or the JSON shape of the payload.
// They build a fully-stamped events.Event ready for WriteEventInTx.
package outboxhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/operations/events"
)

// registryRef holds the optional Schema Registry. When nil, payloads
// are inserted without validation (the original V22 behaviour). Set
// via SetRegistry at startup.
var (
	registryMu sync.RWMutex
	registry   events.Registry
)

// SetRegistry installs the package-level Schema Registry used by
// WriteEventInTx. Pass nil to disable validation. Safe to call
// concurrently with WriteEventInTx — replacement is atomic per call.
func SetRegistry(r events.Registry) {
	registryMu.Lock()
	registry = r
	registryMu.Unlock()
}

// currentRegistry returns the installed registry, or nil.
func currentRegistry() events.Registry {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry
}

// ctxKey is the unexported context key type used for correlation
// propagation. Keeping the key type local prevents accidental
// collisions with other packages that stash a string in ctx.
type ctxKey int

const (
	correlationKey ctxKey = iota + 1
	tenantKey
)

// WithCorrelationID stamps a correlation id on ctx so every outbox
// row written downstream carries it in headers. Resolvers/HTTP
// middleware are expected to set this once per request.
func WithCorrelationID(ctx context.Context, corrID string) context.Context {
	if corrID == "" {
		return ctx
	}
	return context.WithValue(ctx, correlationKey, corrID)
}

// CorrelationID returns the correlation id stamped by an upstream
// handler, or "" when none is present.
func CorrelationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(correlationKey).(string); ok {
		return v
	}
	return ""
}

// WithTenantID stamps the tenant boundary on ctx so downstream outbox
// rows can default tenant_id without every call site threading it.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	if tenantID == "" {
		return ctx
	}
	return context.WithValue(ctx, tenantKey, tenantID)
}

// TenantID returns the tenant id stamped on ctx, or "" when absent.
func TenantID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(tenantKey).(string); ok {
		return v
	}
	return ""
}

// ProducerName is stamped into the producer header on every outbox
// row written by this orchestrator process. Bumped manually on V22
// schema-breaking releases.
const ProducerName = "orchestrator/v22"

// WriteEventInTx writes ONE outbox row inside the caller's tx. The
// row is validated against the topic taxonomy before insertion so a
// malformed topic never reaches the publisher.
//
// Required Event fields: Topic, Type, Key. Everything else is
// defaulted: ID defaults to UUIDv7, NextAttempt to now, headers are
// merged with the standard set (tenant_id, correlation_id, producer,
// idempotency_key). If e.Payload already contains a top-level
// tenant_id, it is mirrored to the tenant_id header.
func WriteEventInTx(ctx context.Context, tx pgx.Tx, e events.Event) error {
	if tx == nil {
		return fmt.Errorf("outboxhooks: nil tx")
	}
	if err := events.ValidateTopic(e.Topic); err != nil {
		return err
	}
	if e.Type == "" {
		return fmt.Errorf("outboxhooks: event type is required")
	}
	if e.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("outboxhooks: uuid v7: %w", err)
		}
		e.ID = id
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if e.NextAttempt.IsZero() {
		e.NextAttempt = e.CreatedAt
	}
	if e.Version <= 0 {
		e.Version = 1
	}
	if e.Payload == nil {
		e.Payload = map[string]any{}
	}
	e.Headers = stampHeaders(ctx, e)
	if reg := currentRegistry(); reg != nil {
		subject := events.SubjectFor(e.Topic, e.Type)
		// Stamp the envelope fields the schemas require (event_id /
		// tenant_id / occurred_at) into the payload so producers can keep
		// posting domain-only bodies. The originals (e.ID, e.CreatedAt,
		// headers.tenant_id) remain authoritative.
		if _, ok := e.Payload["event_id"]; !ok {
			e.Payload["event_id"] = e.ID.String()
		}
		if _, ok := e.Payload["occurred_at"]; !ok {
			e.Payload["occurred_at"] = e.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
		if _, ok := e.Payload["tenant_id"]; !ok {
			if t, ok2 := e.Headers["tenant_id"].(string); ok2 && t != "" {
				e.Payload["tenant_id"] = t
			} else if e.Key != "" {
				e.Payload["tenant_id"] = e.Key
			}
		}
		payloadBytes, mErr := json.Marshal(e.Payload)
		if mErr != nil {
			return fmt.Errorf("outboxhooks: marshal payload for validation: %w", mErr)
		}
		if vErr := reg.Validate(ctx, subject, payloadBytes); vErr != nil {
			switch {
			case errors.Is(vErr, events.ErrSchemaValidation):
				// Permanent failure: caller's tx will roll back its
				// state change along with this aborted insert.
				return vErr
			case errors.Is(vErr, events.ErrSchemaNotFound):
				// Subject not registered yet — preserve V22 backward
				// compatibility by logging and proceeding to insert.
				log.Warn().
					Str("subject", string(subject)).
					Str("event_type", e.Type).
					Str("topic", e.Topic).
					Msg("outboxhooks: schema subject not registered; inserting without validation")
			default:
				return fmt.Errorf("outboxhooks: schema validate: %w", vErr)
			}
		}
	}
	if _, err := events.EnqueueTx(ctx, tx, e); err != nil {
		return err
	}
	return nil
}

// stampHeaders applies the required envelope set per
// docs/ARCHITECTURE_EVENTS.md: tenant_id, correlation_id, producer,
// idempotency_key, plus the schema_subject pointer. Caller-supplied
// headers win over defaults so call sites can override (e.g. for
// causation_id when a consumer re-publishes a derived event).
func stampHeaders(ctx context.Context, e events.Event) map[string]any {
	h := map[string]any{}
	for k, v := range e.Headers {
		h[k] = v
	}
	if _, ok := h["tenant_id"]; !ok {
		if t := tenantFromEvent(ctx, e); t != "" {
			h["tenant_id"] = t
		}
	}
	if _, ok := h["correlation_id"]; !ok {
		if c := CorrelationID(ctx); c != "" {
			h["correlation_id"] = c
		}
	}
	if _, ok := h["producer"]; !ok {
		h["producer"] = ProducerName
	}
	if _, ok := h["idempotency_key"]; !ok {
		h["idempotency_key"] = e.ID.String()
	}
	if _, ok := h["schema_subject"]; !ok {
		// Per ARCHITECTURE_EVENTS.md the subject is <topic>-<event_type>.
		// Schema Registry client lookup is deferred (TODO Wave 2).
		h["schema_subject"] = e.Topic + "-" + e.Type
	}
	return h
}

func tenantFromEvent(ctx context.Context, e events.Event) string {
	if t := TenantID(ctx); t != "" {
		return t
	}
	if v, ok := e.Payload["tenant_id"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// --- typed event constructors -------------------------------------------
//
// Each helper returns a fully-shaped events.Event with topic, key,
// type, and payload set per the V22 spec. Callers then pass the event
// to WriteEventInTx inside their own transaction.

// ExecutionLifecycleEvent builds an execution lifecycle envelope.
// eventType examples: "execution.admitted.v1", "execution.settled.v1",
// "execution.failed.v1".
func ExecutionLifecycleEvent(execID, tenantID, eventType string, payload map[string]any) events.Event {
	out := map[string]any{}
	for k, v := range payload {
		out[k] = v
	}
	if execID != "" {
		out["execution_id"] = execID
	}
	if tenantID != "" {
		out["tenant_id"] = tenantID
	}
	return events.Event{
		Topic:   events.TopicFor("", "execution", "lifecycle", 1),
		Key:     execID,
		Type:    eventType,
		Version: 1,
		Payload: out,
		Headers: map[string]any{"tenant_id": tenantID},
	}
}

// BillingLedgerEvent mirrors one ledger_entries row to the durable
// billing stream. executionID is optional — callers pass nil for
// tenant-scoped entries like wallet_topup or refund.
func BillingLedgerEvent(tenantID string, executionID *string, entryType, direction string, amountUSD decimal.Decimal, metadata map[string]any) events.Event {
	payload := map[string]any{
		"tenant_id":  tenantID,
		"entry_type": entryType,
		"direction":  direction,
		"amount_usd": amountUSD.String(),
		"metadata":   metadata,
	}
	key := tenantID
	if executionID != nil && *executionID != "" {
		payload["execution_id"] = *executionID
		key = *executionID
	}
	return events.Event{
		Topic:   events.TopicFor("", "billing", "ledger", 1),
		Key:     key,
		Type:    "billing.ledger." + entryType + ".v1",
		Version: 1,
		Payload: payload,
		Headers: map[string]any{"tenant_id": tenantID},
	}
}

// ProfitGuardDecisionEvent records one Decide call so dashboards and
// policy analysis can replay the gate trail offline.
func ProfitGuardDecisionEvent(execID, enforcementPoint, action, reason string) events.Event {
	return events.Event{
		Topic:   events.TopicFor("", "profitguard", "decisions", 1),
		Key:     execID,
		Type:    "profitguard.decision.v1",
		Version: 1,
		Payload: map[string]any{
			"execution_id":      execID,
			"enforcement_point": enforcementPoint,
			"decision":          action,
			"reason":            reason,
		},
		Headers: map[string]any{},
	}
}

// PatchLifecycleEvent emits a patch state-transition event. phase is
// the patch status (proposed, previewed, approved, applied, rolled_back).
func PatchLifecycleEvent(projectID, patchID, phase string, payload map[string]any) events.Event {
	out := map[string]any{}
	for k, v := range payload {
		out[k] = v
	}
	out["project_id"] = projectID
	out["patch_id"] = patchID
	out["phase"] = phase
	return events.Event{
		Topic:   events.TopicPatchesLifecycle,
		Key:     projectID,
		Type:    "patch." + phase + ".v1",
		Version: 1,
		Payload: out,
		Headers: map[string]any{},
	}
}

// GateResultEvent emits one finisher gate verdict. verdict is one of
// pass / fail / skip / block.
func GateResultEvent(projectID, executionID, gate, verdict string, issues int) events.Event {
	return events.Event{
		Topic:   events.TopicGatesResults,
		Key:     projectID,
		Type:    "gate.result.v1",
		Version: 1,
		Payload: map[string]any{
			"project_id":   projectID,
			"execution_id": executionID,
			"gate":         gate,
			"verdict":      verdict,
			"issues":       issues,
		},
		Headers: map[string]any{},
	}
}
