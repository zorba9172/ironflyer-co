package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/learning"
	"ironflyer/core/orchestrator/internal/operations/metrics"
)

// Memory is the in-process implementation of Service. It is used by
// dev/test bring-up and by the orchestrator when no Postgres URL is
// configured. All state lives behind a single RWMutex; the broker is
// shared with the postgres backend for symmetry.
type Memory struct {
	mu     sync.RWMutex
	rows   map[string]*Execution
	broker *broker
	// securityFindings is the per-execution ring of security-finding
	// event payloads (gate.security.finding.v1 + siblings). Lazily
	// allocated; capped per-execution in recordSecurityFindingIfApplicable.
	securityFindings map[string][]map[string]any
	// eventLog is the per-execution ring of raw events the wave-3
	// wow-loop adapters read back (gate.* / patch.applied.v1 /
	// recovery.*). The Postgres backend serves the same queries from
	// execution_events; Memory keeps an in-process ring so dev boxes
	// still get a live wow-loop view without spinning up Postgres.
	eventLog map[string][]Event
}

// NewMemory returns a fresh in-memory execution store.
func NewMemory() *Memory {
	return &Memory{
		rows:             make(map[string]*Execution),
		broker:           newBroker(),
		securityFindings: make(map[string][]map[string]any),
		eventLog:         make(map[string][]Event),
	}
}

var _ Service = (*Memory)(nil)

// Create inserts a new execution. The returned Execution is a copy of
// the stored row so the caller cannot mutate broker state.
func (m *Memory) Create(ctx context.Context, in CreateInput) (Execution, error) {
	if in.TenantID == "" {
		return Execution{}, ErrInvalidAmount // misuse — tenant is mandatory
	}
	if !in.BudgetUSD.IsPositive() {
		return Execution{}, ErrInvalidAmount
	}
	now := time.Now().UTC()
	row := &Execution{
		ID:            uuid.NewString(),
		TenantID:      in.TenantID,
		ProjectID:     in.ProjectID,
		BlueprintID:   in.BlueprintID,
		// WorkspaceID starts empty — the finisher engine stamps it via
		// SetWorkspaceID the moment a workspace is resolved/allocated.
		Status:        StatusCreated,
		BudgetUSD:     in.BudgetUSD,
		StopLossUSD:   in.StopLossUSD,
		PromptSummary: in.PromptSummary,
		Metadata:      cloneJSON(in.Metadata),
		CreatedAt:     now,
	}
	m.mu.Lock()
	m.rows[row.ID] = row
	m.mu.Unlock()
	m.emit(row.ID, EventCreated, nil)
	return *row, nil
}

func (m *Memory) Get(ctx context.Context, id string) (Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	row, ok := m.rows[id]
	if !ok {
		return Execution{}, ErrNotFound
	}
	return *row, nil
}

func (m *Memory) GetState(ctx context.Context, id string) (State, error) {
	row, err := m.Get(ctx, id)
	if err != nil {
		return State{}, err
	}
	return State{
		Execution:           row,
		BudgetRemaining:     budgetRemaining(row.BudgetUSD, row.SpentUSD, row.ReservedUSD),
		CompletionPerDollar: completionPerDollar(row.CompletionScore, row.CompletionScoreInitial, row.SpentUSD),
	}, nil
}

func (m *Memory) ListByTenant(ctx context.Context, tenant string, limit, offset int) ([]Execution, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Execution, 0)
	for _, row := range m.rows {
		if row.TenantID == tenant {
			out = append(out, *row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if offset >= len(out) {
		return []Execution{}, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}

func (m *Memory) ListByTenantAndProject(ctx context.Context, tenant, projectID string, limit, offset int) ([]Execution, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Execution, 0)
	for _, row := range m.rows {
		if row.TenantID == tenant && row.ProjectID == projectID {
			out = append(out, *row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if offset >= len(out) {
		return []Execution{}, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}

// transition is the helper every status-changing op funnels through.
// It validates the FSM move, applies the row mutation under the write
// lock, then emits the event after the lock is released.
func (m *Memory) transition(id string, to Status, mutate func(*Execution)) error {
	m.mu.Lock()
	row, ok := m.rows[id]
	if !ok {
		m.mu.Unlock()
		return ErrNotFound
	}
	if !CanTransition(row.Status, to) {
		m.mu.Unlock()
		return ErrIllegalTransition
	}
	row.Status = to
	if mutate != nil {
		mutate(row)
	}
	m.mu.Unlock()
	return nil
}

func (m *Memory) Admit(ctx context.Context, id string) error {
	if err := m.transition(id, StatusAdmitted, func(r *Execution) {
		now := time.Now().UTC()
		r.AdmittedAt = &now
	}); err != nil {
		return err
	}
	m.emit(id, EventAdmitted, nil)
	return nil
}

func (m *Memory) Start(ctx context.Context, id string) error {
	if err := m.transition(id, StatusRunning, func(r *Execution) {
		now := time.Now().UTC()
		r.StartedAt = &now
	}); err != nil {
		return err
	}
	metrics.ObserveExecutionStarted()
	m.emit(id, EventStarted, nil)
	return nil
}

func (m *Memory) Pause(ctx context.Context, id, reason string) error {
	if err := m.transition(id, StatusPausedForBudget, nil); err != nil {
		return err
	}
	m.emit(id, EventPaused, mustJSON(map[string]string{"reason": reason}))
	return nil
}

func (m *Memory) Resume(ctx context.Context, id string) error {
	if err := m.transition(id, StatusRunning, nil); err != nil {
		return err
	}
	m.emit(id, EventResumed, nil)
	return nil
}

func (m *Memory) Succeed(ctx context.Context, id string) error {
	if err := m.transition(id, StatusSucceeded, func(r *Execution) {
		now := time.Now().UTC()
		r.EndedAt = &now
	}); err != nil {
		return err
	}
	metrics.ObserveExecutionCompleted("success")
	m.emit(id, EventSucceeded, nil)
	m.publishTerminal(ctx, id, "succeeded", true, "")
	return nil
}

func (m *Memory) Fail(ctx context.Context, id, reason string) error {
	if err := m.transition(id, StatusFailed, func(r *Execution) {
		now := time.Now().UTC()
		r.EndedAt = &now
		r.FailureReason = reason
	}); err != nil {
		return err
	}
	metrics.ObserveExecutionCompleted(classifyTerminalOutcome("failure", reason))
	m.emit(id, EventFailed, mustJSON(map[string]string{"reason": reason}))
	m.publishTerminal(ctx, id, "failed", false, reason)
	return nil
}

func (m *Memory) Stop(ctx context.Context, id, reason string) error {
	if err := m.transition(id, StatusStopped, func(r *Execution) {
		now := time.Now().UTC()
		r.EndedAt = &now
		r.FailureReason = reason
	}); err != nil {
		return err
	}
	metrics.ObserveExecutionCompleted(classifyTerminalOutcome("failure", reason))
	m.emit(id, EventStopped, mustJSON(map[string]string{"reason": reason}))
	m.publishTerminal(ctx, id, "stopped", false, reason)
	return nil
}

func (m *Memory) Kill(ctx context.Context, id, reason string) error {
	if err := m.transition(id, StatusKilled, func(r *Execution) {
		now := time.Now().UTC()
		r.EndedAt = &now
		r.FailureReason = reason
	}); err != nil {
		return err
	}
	metrics.ObserveExecutionCompleted(classifyTerminalOutcome("killed", reason))
	m.emit(id, EventKilled, mustJSON(map[string]string{"reason": reason}))
	m.publishTerminal(ctx, id, "killed", false, reason)
	return nil
}

// publishTerminal emits a KindExecutionComplete OutcomeEvent so the
// Feedback Brain miner can learn from every terminal transition.
// Best-effort: missing globals or marshal errors are silently dropped
// so the FSM step stays a single read of (status, ended_at).
func (m *Memory) publishTerminal(ctx context.Context, id, finalStatus string, success bool, reason string) {
	row, err := m.Get(ctx, id)
	if err != nil {
		return
	}
	cost := learning.DecimalPtr(row.SpentUSD)
	var marginPct float64
	rev, _ := row.RevenueUSD.Float64()
	spent, _ := row.SpentUSD.Float64()
	if rev > 0 {
		marginPct = (rev - spent) / rev * 100
	}
	margin := row.RevenueUSD.Sub(row.SpentUSD)
	marginPtr := learning.DecimalPtr(margin)
	attrs := map[string]any{
		"final_status":     finalStatus,
		"blueprint_id":     row.BlueprintID,
		"completion_score": row.CompletionScore,
		"reason":           reason,
		"margin_pct":       marginPct,
	}
	learning.Publish(ctx, learning.OutcomeEvent{
		ExecutionID: id,
		TenantID:    row.TenantID,
		Kind:        learning.KindExecutionComplete,
		Attributes:  attrs,
		Success:     learning.BoolPtr(success),
		CostUSD:     cost,
		MarginUSD:   marginPtr,
		Tags: map[string]string{
			"blueprint_id": row.BlueprintID,
		},
	})
}

// classifyTerminalOutcome maps a Fail/Stop/Kill reason string to the
// metrics outcome vocabulary. Reasons that mention budget / wallet
// exhaustion lift into the dedicated "out_of_budget" bucket so the
// dashboard can plot economic refusals separately from operational
// failures or operator kills.
func classifyTerminalOutcome(defaultOutcome, reason string) string {
	r := strings.ToLower(reason)
	switch {
	case strings.Contains(r, "out_of_budget"),
		strings.Contains(r, "budget"),
		strings.Contains(r, "wallet"),
		strings.Contains(r, "pause_for_budget"):
		return "out_of_budget"
	}
	return defaultOutcome
}

func (m *Memory) Refund(ctx context.Context, id string, amount decimal.Decimal) error {
	if !amount.IsPositive() {
		return ErrInvalidAmount
	}
	if err := m.transition(id, StatusRefunded, func(r *Execution) {
		r.RefundedUSD = r.RefundedUSD.Add(amount)
	}); err != nil {
		return err
	}
	m.emit(id, EventRefunded, mustJSON(map[string]any{"amount_usd": amount.String()}))
	return nil
}

func (m *Memory) Reserve(ctx context.Context, id string, amount decimal.Decimal) error {
	if !amount.IsPositive() {
		return ErrInvalidAmount
	}
	m.mu.Lock()
	row, ok := m.rows[id]
	if !ok {
		m.mu.Unlock()
		return ErrNotFound
	}
	if IsTerminal(row.Status) {
		m.mu.Unlock()
		return ErrFinalised
	}
	row.ReservedUSD = row.ReservedUSD.Add(amount)
	m.mu.Unlock()
	m.emit(id, EventReserved, mustJSON(map[string]any{"amount_usd": amount.String()}))
	return nil
}

func (m *Memory) AddCost(ctx context.Context, id string, kind CostKind, amount decimal.Decimal, provider string) error {
	if !amount.IsPositive() {
		return ErrInvalidAmount
	}
	if columnForCost(kind) == "" {
		return ErrInvalidAmount
	}
	m.mu.Lock()
	row, ok := m.rows[id]
	if !ok {
		m.mu.Unlock()
		return ErrNotFound
	}
	if IsTerminal(row.Status) {
		m.mu.Unlock()
		return ErrFinalised
	}
	switch kind {
	case CostProvider, CostPremiumReasoning:
		row.ProviderCostUSD = row.ProviderCostUSD.Add(amount)
	case CostSandbox:
		row.SandboxCostUSD = row.SandboxCostUSD.Add(amount)
	case CostStorage:
		row.StorageCostUSD = row.StorageCostUSD.Add(amount)
	case CostDeployment:
		row.DeploymentCostUSD = row.DeploymentCostUSD.Add(amount)
	}
	row.SpentUSD = row.SpentUSD.Add(amount)
	row.GrossMarginPct = computeGrossMargin(row.RevenueUSD, row.SpentUSD)
	m.mu.Unlock()
	m.emit(id, EventCostAdded, mustJSON(map[string]any{
		"kind":       string(kind),
		"amount_usd": amount.String(),
		"provider":   provider,
	}))
	return nil
}

func (m *Memory) AddRevenue(ctx context.Context, id string, amount decimal.Decimal) error {
	if !amount.IsPositive() {
		return ErrInvalidAmount
	}
	m.mu.Lock()
	row, ok := m.rows[id]
	if !ok {
		m.mu.Unlock()
		return ErrNotFound
	}
	if IsTerminal(row.Status) && row.Status != StatusSucceeded {
		// Allow revenue to land after success (deferred billing) but
		// not after stopped/failed/killed/refunded.
		m.mu.Unlock()
		return ErrFinalised
	}
	row.RevenueUSD = row.RevenueUSD.Add(amount)
	row.GrossMarginPct = computeGrossMargin(row.RevenueUSD, row.SpentUSD)
	m.mu.Unlock()
	m.emit(id, EventRevenueAdded, mustJSON(map[string]any{"amount_usd": amount.String()}))
	return nil
}

func (m *Memory) SetCompletionScore(ctx context.Context, id string, score float64) error {
	if score < 0 || score > 1 {
		return ErrInvalidScore
	}
	m.mu.Lock()
	row, ok := m.rows[id]
	if !ok {
		m.mu.Unlock()
		return ErrNotFound
	}
	if IsTerminal(row.Status) {
		m.mu.Unlock()
		return ErrFinalised
	}
	prev := row.CompletionScore
	row.CompletionScore = score
	m.mu.Unlock()
	m.emit(id, EventScoreUpdated, mustJSON(map[string]any{
		"previous": prev,
		"current":  score,
		"delta":    score - prev,
	}))
	return nil
}

func (m *Memory) SetExpectation(ctx context.Context, id string, expectedDelta, risk float64) error {
	m.mu.Lock()
	row, ok := m.rows[id]
	if !ok {
		m.mu.Unlock()
		return ErrNotFound
	}
	if IsTerminal(row.Status) {
		m.mu.Unlock()
		return ErrFinalised
	}
	d := expectedDelta
	r := risk
	row.ExpectedCompletionDelta = &d
	row.RiskScore = &r
	m.mu.Unlock()
	m.emit(id, EventExpectationUpdated, mustJSON(map[string]any{
		"expected_completion_delta": expectedDelta,
		"risk_score":                risk,
	}))
	return nil
}

func (m *Memory) RecordEvent(ctx context.Context, id, eventType string, payload json.RawMessage) error {
	m.mu.RLock()
	_, ok := m.rows[id]
	m.mu.RUnlock()
	if !ok {
		return ErrNotFound
	}
	m.emit(id, eventType, payload)
	return nil
}

func (m *Memory) SubscribeEvents(ctx context.Context, id string) (<-chan Event, error) {
	m.mu.RLock()
	_, ok := m.rows[id]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	return m.broker.subscribeWithContext(ctx, id), nil
}

// ActiveCount returns the number of executions in StatusRunning
// across every tenant. Powers the Scale dashboard.
func (m *Memory) ActiveCount(_ context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, r := range m.rows {
		if r.Status == StatusRunning {
			n++
		}
	}
	return n, nil
}

// QueuedCount returns the number of executions waiting to start
// (created + admitted) across every tenant.
func (m *Memory) QueuedCount(_ context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, r := range m.rows {
		if r.Status == StatusCreated || r.Status == StatusAdmitted {
			n++
		}
	}
	return n, nil
}

// AverageQueueWaitSec returns the mean seconds between created_at
// and admitted_at across executions admitted on or after `since`.
// Returns 0 when there are no recent admissions.
func (m *Memory) AverageQueueWaitSec(_ context.Context, since time.Time) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var total float64
	var n int
	for _, r := range m.rows {
		if r.AdmittedAt == nil {
			continue
		}
		if r.AdmittedAt.Before(since) {
			continue
		}
		wait := r.AdmittedAt.Sub(r.CreatedAt).Seconds()
		if wait < 0 {
			wait = 0
		}
		total += wait
		n++
	}
	if n == 0 {
		return 0, nil
	}
	return total / float64(n), nil
}

// securityFindingEventTypes is the closed set of event types the
// securityreport.FindingSource adapter scans for. Adding a new type
// here makes it visible to the customer-facing security report
// without touching the adapter; remove with care — the adapter has
// no other way to discover finding payloads.
var securityFindingEventTypes = map[string]struct{}{
	"gate.security.finding.v1": {},
	"patch.security_scan.v1":   {},
	"security.findings.v1":     {},
}

// IsSecurityFindingEventType reports whether an event_type belongs
// to the closed security-findings set the FindingSource consumes.
// Exported so adapters in sibling packages can stay in lockstep with
// the canonical set defined here.
func IsSecurityFindingEventType(eventType string) bool {
	_, ok := securityFindingEventTypes[eventType]
	return ok
}

// LatestSecurityFindings — Memory implementation.
//
// The memory backend does not persist execution_events; the in-process
// broker is fire-and-forget. To still give the securityreport.Builder
// real signal on dev boxes, we keep a small per-execution ring of
// security-finding payloads on the Memory store itself. The emit path
// (memory.emit + Postgres.recordAndEmit) appends here when the
// event_type is in the closed security-findings set.
//
// Returns nil + nil when the execution is unknown (degraded path —
// "no data yet" is a valid pass-with-empty-findings result).
func (m *Memory) LatestSecurityFindings(_ context.Context, executionID string) ([]map[string]any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	payloads := m.securityFindings[executionID]
	if len(payloads) == 0 {
		return nil, nil
	}
	// Return newest-first, capped at 500 to match the Postgres
	// implementation's stable contract.
	out := make([]map[string]any, 0, len(payloads))
	for i := len(payloads) - 1; i >= 0 && len(out) < 500; i-- {
		// Deep-ish copy: callers may mutate map values, and we want
		// repeat reads to be deterministic.
		dup := make(map[string]any, len(payloads[i]))
		for k, v := range payloads[i] {
			dup[k] = v
		}
		out = append(out, dup)
	}
	return out, nil
}

// recordSecurityFindingIfApplicable appends `payload` to the
// per-execution security-findings ring when `eventType` is in the
// closed security set. Best-effort — payload that fails to JSON-decode
// into a map is silently dropped (the broker still publishes it).
func (m *Memory) recordSecurityFindingIfApplicable(id, eventType string, payload json.RawMessage) {
	if !IsSecurityFindingEventType(eventType) || len(payload) == 0 {
		return
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil || decoded == nil {
		return
	}
	m.mu.Lock()
	if m.securityFindings == nil {
		m.securityFindings = make(map[string][]map[string]any)
	}
	// Cap per-execution ring at 1024 entries — covers any realistic
	// scan output and bounds memory for long-running dev sessions.
	const ringCap = 1024
	ring := m.securityFindings[id]
	if len(ring) >= ringCap {
		ring = ring[len(ring)-ringCap+1:]
	}
	m.securityFindings[id] = append(ring, decoded)
	m.mu.Unlock()
}

// emit publishes an event on the broker. Wrapped so the call sites
// don't repeat the now-stamping + struct construction.
func (m *Memory) emit(id, eventType string, payload json.RawMessage) {
	now := time.Now().UTC()
	m.recordSecurityFindingIfApplicable(id, eventType, payload)
	m.appendEventLog(id, Event{
		ExecutionID: id,
		EventType:   eventType,
		Payload:     payload,
		CreatedAt:   now,
	})
	m.broker.publish(Event{
		ExecutionID: id,
		EventType:   eventType,
		Payload:     payload,
		CreatedAt:   now,
	})
}

// appendEventLog appends evt to the per-execution event ring used by
// the wow-loop adapters. Capped at 4096 events per execution to bound
// memory on long-running dev sessions; the Postgres backend has no
// such cap because execution_events is the durable record.
func (m *Memory) appendEventLog(id string, evt Event) {
	const ringCap = 4096
	m.mu.Lock()
	if m.eventLog == nil {
		m.eventLog = make(map[string][]Event)
	}
	ring := m.eventLog[id]
	if len(ring) >= ringCap {
		ring = ring[len(ring)-ringCap+1:]
	}
	m.eventLog[id] = append(ring, evt)
	m.mu.Unlock()
}

// snapshotEvents returns a copy of the per-execution event ring so
// the caller can iterate without holding the Memory mutex.
func (m *Memory) snapshotEvents(id string) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.eventLog[id]
	if len(src) == 0 {
		return nil
	}
	out := make([]Event, len(src))
	copy(out, src)
	return out
}

// GateEventsByExecution — Memory implementation.
//
// Walks the in-process event ring populated by emit, filters on the
// closed gate-verdict set, decodes each payload via the shared
// decodeGateEvent helper, and returns the result oldest-first to
// match the Postgres contract.
func (m *Memory) GateEventsByExecution(_ context.Context, executionID string) ([]GateEvent, error) {
	events := m.snapshotEvents(executionID)
	out := make([]GateEvent, 0)
	for _, evt := range events {
		if !IsGateEventType(evt.EventType) {
			continue
		}
		ge, ok := decodeGateEvent(evt.EventType, evt.Payload, evt.CreatedAt)
		if !ok {
			continue
		}
		out = append(out, ge)
	}
	return out, nil
}

// PatchAppliedEventsByExecution — Memory implementation. See
// GateEventsByExecution for the shared filter+decode pattern.
func (m *Memory) PatchAppliedEventsByExecution(_ context.Context, executionID string) ([]PatchAppliedEvent, error) {
	events := m.snapshotEvents(executionID)
	out := make([]PatchAppliedEvent, 0)
	for _, evt := range events {
		if !IsPatchAppliedEventType(evt.EventType) {
			continue
		}
		pe, ok := decodePatchAppliedEvent(evt.Payload, evt.CreatedAt)
		if !ok {
			continue
		}
		out = append(out, pe)
	}
	return out, nil
}

// RecoveryAttemptsByExecution — Memory implementation. See
// GateEventsByExecution for the shared filter+decode pattern.
func (m *Memory) RecoveryAttemptsByExecution(_ context.Context, executionID string) ([]RecoveryAttempt, error) {
	events := m.snapshotEvents(executionID)
	out := make([]RecoveryAttempt, 0)
	for _, evt := range events {
		if !IsRecoveryEventType(evt.EventType) {
			continue
		}
		ra, ok := decodeRecoveryAttempt(evt.EventType, evt.Payload, evt.CreatedAt)
		if !ok {
			continue
		}
		out = append(out, ra)
	}
	return out, nil
}

// PendingRefinements — Memory implementation.
//
// Walks the in-process event ring populated by emit, picks every
// studio.refine.v1 row whose payload has not been marked consumed
// by a sibling studio.refine.consumed.v1 row referencing the same
// refine_id. Oldest first.
func (m *Memory) PendingRefinements(_ context.Context, executionID string) ([]Refinement, error) {
	events := m.snapshotEvents(executionID)
	consumed := refinementConsumedSet(events)
	out := make([]Refinement, 0)
	for _, evt := range events {
		if evt.EventType != EventStudioRefineV1 {
			continue
		}
		ref, ok := decodeRefinement(evt)
		if !ok {
			continue
		}
		if _, ack := consumed[ref.ID]; ack {
			continue
		}
		out = append(out, ref)
	}
	return out, nil
}

// DrainRefinements — Memory implementation.
//
// Returns the same set PendingRefinements would return AND emits a
// studio.refine.consumed.v1 marker for each so subsequent Drains
// return an empty slice. The marker carries the refine_id (which is
// the originating event's id) so the Postgres implementation can
// detect double-consumption with a NOT EXISTS predicate.
func (m *Memory) DrainRefinements(ctx context.Context, executionID string) ([]Refinement, error) {
	pending, err := m.PendingRefinements(ctx, executionID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for i := range pending {
		ack := now
		pending[i].ConsumedAt = &ack
		payload := mustJSON(map[string]any{
			"refine_id":   pending[i].ID,
			"consumed_at": now.Format(time.RFC3339Nano),
		})
		// Best-effort: append directly via emit so the event ring
		// captures it and dedup works on the next Drain.
		m.emit(executionID, EventStudioRefineConsumedV1, payload)
	}
	return pending, nil
}

// refinementConsumedSet builds the set of refine_id values already
// acknowledged by a studio.refine.consumed.v1 marker on the same
// event ring.
func refinementConsumedSet(events []Event) map[string]struct{} {
	out := make(map[string]struct{})
	for _, evt := range events {
		if evt.EventType != EventStudioRefineConsumedV1 {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(evt.Payload, &payload); err != nil {
			continue
		}
		if id, ok := payload["refine_id"].(string); ok && id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

// refinementIDCounter mints synthetic refine IDs for the Memory
// backend so PendingRefinements / DrainRefinements have something
// stable to dedupe on. The Postgres backend uses the row's
// execution_events.id (UUID) instead.
var (
	refinementIDMu      sync.Mutex
	refinementIDCounter int
)

// decodeRefinement projects one studio.refine.v1 event into a
// Refinement. We mint a synthetic ID per row since the Memory event
// log does not carry execution_events.id. The same ID is then used
// by the studio.refine.consumed.v1 marker for dedup.
func decodeRefinement(evt Event) (Refinement, bool) {
	if evt.EventType != EventStudioRefineV1 {
		return Refinement{}, false
	}
	var payload map[string]any
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		return Refinement{}, false
	}
	msg, _ := payload["message"].(string)
	if msg == "" {
		return Refinement{}, false
	}
	// Prefer a refine_id baked into the payload (Postgres path can
	// stamp the row UUID into the payload at write time); fall back
	// to a stable synthetic per (execution, created_at) pair so the
	// dedup set in the Memory backend is consistent across reads.
	id, _ := payload["refine_id"].(string)
	if id == "" {
		refinementIDMu.Lock()
		refinementIDCounter++
		id = fmt.Sprintf("mem-refine-%s-%d-%d",
			evt.ExecutionID, evt.CreatedAt.UnixNano(), refinementIDCounter)
		refinementIDMu.Unlock()
	}
	return Refinement{
		ID:       id,
		Message:  msg,
		QueuedAt: evt.CreatedAt,
	}, true
}

// SetWorkspaceID stamps the runtime workspace bound to the execution
// onto the in-memory row. Idempotent — calling twice with the same
// value is a no-op success. A different value overwrites (matches the
// Postgres backend behaviour). Returns ErrNotFound when the execution
// is unknown.
func (m *Memory) SetWorkspaceID(_ context.Context, executionID, workspaceID string) error {
	if executionID == "" {
		return ErrNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.rows[executionID]
	if !ok {
		return ErrNotFound
	}
	row.WorkspaceID = workspaceID
	return nil
}

// mustJSON marshals a value to json.RawMessage. Used only with map[…]…
// inputs that cannot fail to marshal; falls back to an empty object on
// error so the event still propagates with valid JSON.
func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

// cloneJSON returns a copy of raw JSON bytes (or nil for empty input)
// so the caller cannot mutate the stored metadata through their input
// slice.
func cloneJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}
