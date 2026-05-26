package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/operations/logctx"
	"ironflyer/core/orchestrator/internal/operations/metrics"
)

// Postgres is the durable implementation of Service backed by the
// `executions` + `execution_events` tables (migration 00026).
//
// Status-changing operations open a transaction, fetch the row with
// SELECT … FOR UPDATE, check CanTransition, then UPDATE in the same
// tx. AddCost/AddRevenue/Reserve also run under FOR UPDATE so the
// numeric columns cannot race against a concurrent finisher tick.
//
// Events are inserted into execution_events AND fanned out through
// the in-process broker. A real cross-process subscription path would
// add a LISTEN goroutine that NOTIFY-bridges into the same broker;
// the in-process broker alone is sufficient for the single-replica
// orchestrator we ship today, and a future hook can call broker.publish
// from a LISTEN handler without changing the public Service contract.
type Postgres struct {
	pool   *pgxpool.Pool
	broker *broker
}

// NewPostgres wraps a pgxpool.Pool and returns the durable Service
// implementation.
func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool, broker: newBroker()}
}

var _ Service = (*Postgres)(nil)

const executionColumns = `id, tenant_id, COALESCE(project_id::text, ''),
    COALESCE(blueprint_id, ''), status,
    budget_usd::text, reserved_usd::text, spent_usd::text,
    refunded_usd::text, revenue_usd::text,
    provider_cost_usd::text, sandbox_cost_usd::text,
    storage_cost_usd::text, deployment_cost_usd::text,
    completion_score, completion_score_initial,
    gross_margin_pct::text, expected_completion_delta, risk_score,
    stop_loss_usd::text, COALESCE(prompt_summary, ''),
    COALESCE(failure_reason, ''), metadata,
    created_at, admitted_at, started_at, ended_at,
    COALESCE(workspace_id, '')`

// scanExecution decodes one row (selected via executionColumns) into
// an Execution. Used by both QueryRow and the row-loop in
// ListByTenant.
func scanExecution(row pgx.Row) (Execution, error) {
	var (
		e                Execution
		projectID        string
		blueprintID      string
		budget           string
		reserved         string
		spent            string
		refunded         string
		revenue          string
		providerCost     string
		sandboxCost      string
		storageCost      string
		deploymentCost   string
		grossMarginPct   *string
		expectedDelta    *float64
		riskScore        *float64
		stopLossUSD      *string
		promptSummary    string
		failureReason    string
		metadata         []byte
		workspaceID      string
	)
	if err := row.Scan(
		&e.ID, &e.TenantID, &projectID, &blueprintID, &e.Status,
		&budget, &reserved, &spent, &refunded, &revenue,
		&providerCost, &sandboxCost, &storageCost, &deploymentCost,
		&e.CompletionScore, &e.CompletionScoreInitial,
		&grossMarginPct, &expectedDelta, &riskScore, &stopLossUSD,
		&promptSummary, &failureReason, &metadata,
		&e.CreatedAt, &e.AdmittedAt, &e.StartedAt, &e.EndedAt,
		&workspaceID,
	); err != nil {
		return Execution{}, err
	}
	e.ProjectID = projectID
	e.BlueprintID = blueprintID
	e.PromptSummary = promptSummary
	e.FailureReason = failureReason
	e.WorkspaceID = workspaceID
	e.BudgetUSD = decStr(budget)
	e.ReservedUSD = decStr(reserved)
	e.SpentUSD = decStr(spent)
	e.RefundedUSD = decStr(refunded)
	e.RevenueUSD = decStr(revenue)
	e.ProviderCostUSD = decStr(providerCost)
	e.SandboxCostUSD = decStr(sandboxCost)
	e.StorageCostUSD = decStr(storageCost)
	e.DeploymentCostUSD = decStr(deploymentCost)
	if grossMarginPct != nil {
		d := decStr(*grossMarginPct)
		e.GrossMarginPct = &d
	}
	e.ExpectedCompletionDelta = expectedDelta
	e.RiskScore = riskScore
	if stopLossUSD != nil {
		d := decStr(*stopLossUSD)
		e.StopLossUSD = &d
	}
	if len(metadata) > 0 {
		e.Metadata = json.RawMessage(metadata)
	}
	return e, nil
}

func decStr(s string) decimal.Decimal {
	if s == "" {
		return decimal.Zero
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

// Create inserts a new execution row and returns the populated
// projection.
func (p *Postgres) Create(ctx context.Context, in CreateInput) (Execution, error) {
	if in.TenantID == "" || !in.BudgetUSD.IsPositive() {
		return Execution{}, ErrInvalidAmount
	}
	metadata := in.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	var (
		projectID   any
		blueprintID any
		stopLoss    any
		prompt      any
	)
	if in.ProjectID != "" {
		projectID = in.ProjectID
	}
	if in.BlueprintID != "" {
		blueprintID = in.BlueprintID
	}
	if in.StopLossUSD != nil {
		stopLoss = in.StopLossUSD.String()
	}
	if in.PromptSummary != "" {
		prompt = in.PromptSummary
	}

	var id string
	if err := p.pool.QueryRow(ctx, `
        INSERT INTO executions(tenant_id, project_id, blueprint_id, status,
                               budget_usd, stop_loss_usd, prompt_summary, metadata)
        VALUES ($1, $2, $3, 'created', $4, $5, $6, $7)
        RETURNING id`,
		in.TenantID, projectID, blueprintID,
		in.BudgetUSD.String(), stopLoss, prompt, metadata,
	).Scan(&id); err != nil {
		return Execution{}, err
	}

	row, err := p.Get(ctx, id)
	if err != nil {
		return Execution{}, err
	}
	p.recordAndEmit(ctx, id, EventCreated, nil)
	return row, nil
}

func (p *Postgres) Get(ctx context.Context, id string) (Execution, error) {
	row := p.pool.QueryRow(ctx, `SELECT `+executionColumns+` FROM executions WHERE id = $1`, id)
	e, err := scanExecution(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Execution{}, ErrNotFound
		}
		return Execution{}, err
	}
	return e, nil
}

func (p *Postgres) GetState(ctx context.Context, id string) (State, error) {
	row, err := p.Get(ctx, id)
	if err != nil {
		return State{}, err
	}
	return State{
		Execution:           row,
		BudgetRemaining:     budgetRemaining(row.BudgetUSD, row.SpentUSD, row.ReservedUSD),
		CompletionPerDollar: completionPerDollar(row.CompletionScore, row.CompletionScoreInitial, row.SpentUSD),
	}, nil
}

func (p *Postgres) ListByTenant(ctx context.Context, tenant string, limit, offset int) ([]Execution, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := p.pool.Query(ctx, `
        SELECT `+executionColumns+`
        FROM executions
        WHERE tenant_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3`, tenant, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Execution, 0)
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (p *Postgres) ListByTenantAndProject(ctx context.Context, tenant, projectID string, limit, offset int) ([]Execution, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := p.pool.Query(ctx, `
        SELECT `+executionColumns+`
        FROM executions
        WHERE tenant_id = $1 AND project_id = $2
        ORDER BY created_at DESC
        LIMIT $3 OFFSET $4`, tenant, projectID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Execution, 0)
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// txTransition is the helper every status-changing op funnels through.
// It opens a tx, locks the row, validates the FSM, runs the
// caller-supplied UPDATE, then commits.
func (p *Postgres) txTransition(ctx context.Context, id string, to Status, updateSQL string, args ...any) error {
	// Empty IDs cannot match any row; short-circuit before Postgres
	// rejects the `WHERE id = ''` lookup as "invalid input syntax for
	// type uuid: \"\"" (SQLSTATE 22P02). The historical crash here
	// pointed at a caller invoking Admit/Start/Stop before Create's
	// RETURNING id had been read. Treat it as ErrNotFound so the
	// caller surfaces the correct GraphQL error.
	if id == "" {
		return ErrNotFound
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return logPGErr(ctx, "tx_begin", id, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current Status
	if err := tx.QueryRow(ctx, `SELECT status FROM executions WHERE id = $1 FOR UPDATE`, id).Scan(&current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return logPGErr(ctx, "tx_select_for_update", id, err)
	}
	if !CanTransition(current, to) {
		return ErrIllegalTransition
	}
	if _, err := tx.Exec(ctx, updateSQL, args...); err != nil {
		return logPGErr(ctx, "tx_update", id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return logPGErr(ctx, "tx_commit", id, err)
	}
	return nil
}

// logPGErr stamps the failing Postgres op on the ctx-aware logger
// before returning the error to the caller. Centralises the
// log-before-return pattern so every execution-store mutation lands
// in the diagnostics ring buffer with execution_id + request_id
// attached automatically.
func logPGErr(ctx context.Context, op, executionID string, err error) error {
	if err == nil {
		return nil
	}
	l := logctx.From(ctx)
	l.Error().
		Err(err).
		Str("op", op).
		Str("execution_id", executionID).
		Msg("execution postgres op failed")
	return err
}

func (p *Postgres) Admit(ctx context.Context, id string) error {
	err := p.txTransition(ctx, id, StatusAdmitted,
		`UPDATE executions SET status = 'admitted', admitted_at = now() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	p.recordAndEmit(ctx, id, EventAdmitted, nil)
	return nil
}

func (p *Postgres) Start(ctx context.Context, id string) error {
	err := p.txTransition(ctx, id, StatusRunning,
		`UPDATE executions SET status = 'running', started_at = now() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	metrics.ObserveExecutionStarted()
	p.recordAndEmit(ctx, id, EventStarted, nil)
	return nil
}

func (p *Postgres) Pause(ctx context.Context, id, reason string) error {
	err := p.txTransition(ctx, id, StatusPausedForBudget,
		`UPDATE executions SET status = 'paused_for_budget' WHERE id = $1`, id)
	if err != nil {
		return err
	}
	p.recordAndEmit(ctx, id, EventPaused, mustJSON(map[string]string{"reason": reason}))
	return nil
}

func (p *Postgres) Resume(ctx context.Context, id string) error {
	err := p.txTransition(ctx, id, StatusRunning,
		`UPDATE executions SET status = 'running' WHERE id = $1`, id)
	if err != nil {
		return err
	}
	p.recordAndEmit(ctx, id, EventResumed, nil)
	return nil
}

func (p *Postgres) Succeed(ctx context.Context, id string) error {
	err := p.txTransition(ctx, id, StatusSucceeded,
		`UPDATE executions SET status = 'succeeded', ended_at = now() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	metrics.ObserveExecutionCompleted("success")
	p.recordAndEmit(ctx, id, EventSucceeded, nil)
	return nil
}

func (p *Postgres) Fail(ctx context.Context, id, reason string) error {
	err := p.txTransition(ctx, id, StatusFailed,
		`UPDATE executions SET status = 'failed', ended_at = now(), failure_reason = $2 WHERE id = $1`,
		id, reason)
	if err != nil {
		return err
	}
	metrics.ObserveExecutionCompleted(classifyTerminalOutcome("failure", reason))
	p.recordAndEmit(ctx, id, EventFailed, mustJSON(map[string]string{"reason": reason}))
	return nil
}

func (p *Postgres) Stop(ctx context.Context, id, reason string) error {
	err := p.txTransition(ctx, id, StatusStopped,
		`UPDATE executions SET status = 'stopped', ended_at = now(), failure_reason = $2 WHERE id = $1`,
		id, reason)
	if err != nil {
		return err
	}
	metrics.ObserveExecutionCompleted(classifyTerminalOutcome("failure", reason))
	p.recordAndEmit(ctx, id, EventStopped, mustJSON(map[string]string{"reason": reason}))
	return nil
}

func (p *Postgres) Kill(ctx context.Context, id, reason string) error {
	err := p.txTransition(ctx, id, StatusKilled,
		`UPDATE executions SET status = 'killed', ended_at = now(), failure_reason = $2 WHERE id = $1`,
		id, reason)
	if err != nil {
		return err
	}
	metrics.ObserveExecutionCompleted(classifyTerminalOutcome("killed", reason))
	p.recordAndEmit(ctx, id, EventKilled, mustJSON(map[string]string{"reason": reason}))
	return nil
}

func (p *Postgres) Refund(ctx context.Context, id string, amount decimal.Decimal) error {
	if !amount.IsPositive() {
		return ErrInvalidAmount
	}
	err := p.txTransition(ctx, id, StatusRefunded,
		`UPDATE executions SET status = 'refunded', refunded_usd = refunded_usd + $2 WHERE id = $1`,
		id, amount.String())
	if err != nil {
		return err
	}
	p.recordAndEmit(ctx, id, EventRefunded, mustJSON(map[string]any{"amount_usd": amount.String()}))
	return nil
}

// txMutate is the helper for numeric updates that do NOT change
// status. It locks the row, refuses to proceed if the execution is
// already terminal, runs the UPDATE, then commits.
func (p *Postgres) txMutate(ctx context.Context, id string, allowSucceeded bool, updateSQL string, args ...any) error {
	if id == "" {
		return ErrNotFound
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current Status
	if err := tx.QueryRow(ctx, `SELECT status FROM executions WHERE id = $1 FOR UPDATE`, id).Scan(&current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if IsTerminal(current) {
		if !(allowSucceeded && current == StatusSucceeded) {
			return ErrFinalised
		}
	}
	if _, err := tx.Exec(ctx, updateSQL, args...); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) Reserve(ctx context.Context, id string, amount decimal.Decimal) error {
	if !amount.IsPositive() {
		return ErrInvalidAmount
	}
	err := p.txMutate(ctx, id, false,
		`UPDATE executions SET reserved_usd = reserved_usd + $2 WHERE id = $1`,
		id, amount.String())
	if err != nil {
		return err
	}
	p.recordAndEmit(ctx, id, EventReserved, mustJSON(map[string]any{"amount_usd": amount.String()}))
	return nil
}

func (p *Postgres) AddCost(ctx context.Context, id string, kind CostKind, amount decimal.Decimal, provider string) error {
	if !amount.IsPositive() {
		return ErrInvalidAmount
	}
	col := columnForCost(kind)
	if col == "" {
		return ErrInvalidAmount
	}
	// Build the UPDATE dynamically — `col` is from a closed enum
	// (columnForCost), so there is no SQL-injection surface.
	sql := fmt.Sprintf(`UPDATE executions SET
            %s = %s + $2,
            spent_usd = spent_usd + $2,
            gross_margin_pct = CASE WHEN revenue_usd > 0
                THEN (revenue_usd - (spent_usd + $2)) / revenue_usd * 100
                ELSE NULL END
        WHERE id = $1`, col, col)
	if err := p.txMutate(ctx, id, true, sql, id, amount.String()); err != nil {
		return err
	}
	p.recordAndEmit(ctx, id, EventCostAdded, mustJSON(map[string]any{
		"kind":       string(kind),
		"amount_usd": amount.String(),
		"provider":   provider,
	}))
	return nil
}

func (p *Postgres) AddRevenue(ctx context.Context, id string, amount decimal.Decimal) error {
	if !amount.IsPositive() {
		return ErrInvalidAmount
	}
	err := p.txMutate(ctx, id, true,
		`UPDATE executions SET
            revenue_usd = revenue_usd + $2,
            gross_margin_pct = CASE WHEN (revenue_usd + $2) > 0
                THEN ((revenue_usd + $2) - spent_usd) / (revenue_usd + $2) * 100
                ELSE NULL END
         WHERE id = $1`,
		id, amount.String())
	if err != nil {
		return err
	}
	p.recordAndEmit(ctx, id, EventRevenueAdded, mustJSON(map[string]any{"amount_usd": amount.String()}))
	return nil
}

func (p *Postgres) SetCompletionScore(ctx context.Context, id string, score float64) error {
	if score < 0 || score > 1 {
		return ErrInvalidScore
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current Status
	var prev float64
	if err := tx.QueryRow(ctx,
		`SELECT status, completion_score FROM executions WHERE id = $1 FOR UPDATE`,
		id).Scan(&current, &prev); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if IsTerminal(current) {
		return ErrFinalised
	}
	if _, err := tx.Exec(ctx,
		`UPDATE executions SET completion_score = $2 WHERE id = $1`,
		id, score); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	p.recordAndEmit(ctx, id, EventScoreUpdated, mustJSON(map[string]any{
		"previous": prev,
		"current":  score,
		"delta":    score - prev,
	}))
	return nil
}

func (p *Postgres) SetExpectation(ctx context.Context, id string, expectedDelta, risk float64) error {
	err := p.txMutate(ctx, id, false,
		`UPDATE executions SET expected_completion_delta = $2, risk_score = $3 WHERE id = $1`,
		id, expectedDelta, risk)
	if err != nil {
		return err
	}
	p.recordAndEmit(ctx, id, EventExpectationUpdated, mustJSON(map[string]any{
		"expected_completion_delta": expectedDelta,
		"risk_score":                risk,
	}))
	return nil
}

func (p *Postgres) RecordEvent(ctx context.Context, id, eventType string, payload json.RawMessage) error {
	// Verify the row exists so we can return ErrNotFound up-front
	// instead of bubbling a FK violation from the INSERT.
	var exists bool
	if err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM executions WHERE id = $1)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	p.recordAndEmit(ctx, id, eventType, payload)
	return nil
}

func (p *Postgres) SubscribeEvents(ctx context.Context, id string) (<-chan Event, error) {
	var exists bool
	if err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM executions WHERE id = $1)`, id).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	return p.broker.subscribeWithContext(ctx, id), nil
}

// ActiveCount counts executions in 'running' status across all tenants.
func (p *Postgres) ActiveCount(ctx context.Context) (int, error) {
	var n int
	if err := p.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM executions WHERE status = 'running'`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// QueuedCount counts executions waiting to start (created + admitted).
func (p *Postgres) QueuedCount(ctx context.Context) (int, error) {
	var n int
	if err := p.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM executions WHERE status IN ('created', 'admitted')`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// AverageQueueWaitSec returns the mean (admitted_at - created_at) in
// seconds across executions admitted on or after `since`. Returns 0
// when no rows match (avoids a NULL-from-AVG corner case).
func (p *Postgres) AverageQueueWaitSec(ctx context.Context, since time.Time) (float64, error) {
	var avg *float64
	if err := p.pool.QueryRow(ctx, `
        SELECT AVG(EXTRACT(EPOCH FROM (admitted_at - created_at)))
        FROM executions
        WHERE admitted_at IS NOT NULL
          AND admitted_at >= $1`, since).Scan(&avg); err != nil {
		return 0, err
	}
	if avg == nil {
		return 0, nil
	}
	return *avg, nil
}

// LatestSecurityFindings — Postgres implementation.
//
// Reads the recent execution_events payloads whose event_type sits in
// the closed security-findings set (gate.security.finding.v1,
// patch.security_scan.v1, security.findings.v1). Newest first, capped
// at 500 — matches the contract in execution.Service.
//
// Tolerant on read errors: a payload that fails to JSON-decode is
// skipped (not surfaced as an error), so a single malformed row never
// blanks the customer report.
func (p *Postgres) LatestSecurityFindings(ctx context.Context, executionID string) ([]map[string]any, error) {
	rows, err := p.pool.Query(ctx, `
        SELECT payload FROM execution_events
        WHERE execution_id = $1
          AND event_type IN ('gate.security.finding.v1',
                             'patch.security_scan.v1',
                             'security.findings.v1')
        ORDER BY created_at DESC
        LIMIT 500`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil || decoded == nil {
			continue
		}
		out = append(out, decoded)
	}
	return out, rows.Err()
}

// GateEventsByExecution — Postgres implementation.
//
// Reads execution_events rows whose event_type sits in the closed
// gate.* family. We project the payload into a GateEvent so the
// wow-loop builder can stay decoupled from the row shape.
//
// Status normalisation: gate.verdict.v1 carries an explicit "status"
// field; the older marker types (gate.failed.v1, gate.passed.v1,
// gate.skipped.v1, gate.repaired.v1) imply the status from the type
// itself.
func (p *Postgres) GateEventsByExecution(ctx context.Context, executionID string) ([]GateEvent, error) {
	rows, err := p.pool.Query(ctx, `
        SELECT event_type, payload, created_at
        FROM execution_events
        WHERE execution_id = $1
          AND event_type IN ('gate.verdict.v1',
                             'gate.failed.v1',
                             'gate.passed.v1',
                             'gate.skipped.v1',
                             'gate.repaired.v1')
        ORDER BY created_at ASC`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GateEvent, 0)
	for rows.Next() {
		var (
			eventType string
			raw       []byte
			created   time.Time
		)
		if err := rows.Scan(&eventType, &raw, &created); err != nil {
			return nil, err
		}
		ge, ok := decodeGateEvent(eventType, raw, created)
		if !ok {
			continue
		}
		out = append(out, ge)
	}
	return out, rows.Err()
}

// PatchAppliedEventsByExecution — Postgres implementation.
func (p *Postgres) PatchAppliedEventsByExecution(ctx context.Context, executionID string) ([]PatchAppliedEvent, error) {
	rows, err := p.pool.Query(ctx, `
        SELECT payload, created_at
        FROM execution_events
        WHERE execution_id = $1
          AND event_type = 'patch.applied.v1'
        ORDER BY created_at ASC`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PatchAppliedEvent, 0)
	for rows.Next() {
		var (
			raw     []byte
			created time.Time
		)
		if err := rows.Scan(&raw, &created); err != nil {
			return nil, err
		}
		pe, ok := decodePatchAppliedEvent(raw, created)
		if !ok {
			continue
		}
		out = append(out, pe)
	}
	return out, rows.Err()
}

// RecoveryAttemptsByExecution — Postgres implementation.
func (p *Postgres) RecoveryAttemptsByExecution(ctx context.Context, executionID string) ([]RecoveryAttempt, error) {
	rows, err := p.pool.Query(ctx, `
        SELECT event_type, payload, created_at
        FROM execution_events
        WHERE execution_id = $1
          AND event_type IN ('recovery.recipe_hit.v1',
                             'recovery.recipe_applied.v1')
        ORDER BY created_at ASC`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RecoveryAttempt, 0)
	for rows.Next() {
		var (
			eventType string
			raw       []byte
			created   time.Time
		)
		if err := rows.Scan(&eventType, &raw, &created); err != nil {
			return nil, err
		}
		ra, ok := decodeRecoveryAttempt(eventType, raw, created)
		if !ok {
			continue
		}
		out = append(out, ra)
	}
	return out, rows.Err()
}

// PendingRefinements — Postgres implementation.
//
// Returns every studio.refine.v1 row on the execution that has not
// yet been acknowledged by a sibling studio.refine.consumed.v1 row
// (matched by the consumed marker's payload->>'refine_id' = the
// originating row id). Oldest first.
func (p *Postgres) PendingRefinements(ctx context.Context, executionID string) ([]Refinement, error) {
	rows, err := p.pool.Query(ctx, `
        SELECT id::text, payload, created_at
        FROM execution_events
        WHERE execution_id = $1
          AND event_type = 'studio.refine.v1'
          AND NOT EXISTS (
              SELECT 1 FROM execution_events e2
              WHERE e2.execution_id = $1
                AND e2.event_type = 'studio.refine.consumed.v1'
                AND e2.payload->>'refine_id' = execution_events.id::text
          )
        ORDER BY created_at ASC`, executionID)
	if err != nil {
		return nil, logPGErr(ctx, "pending_refinements", executionID, err)
	}
	defer rows.Close()
	out := make([]Refinement, 0)
	for rows.Next() {
		var (
			id      string
			raw     []byte
			created time.Time
		)
		if err := rows.Scan(&id, &raw, &created); err != nil {
			return nil, err
		}
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			continue
		}
		msg, _ := payload["message"].(string)
		if msg == "" {
			continue
		}
		out = append(out, Refinement{
			ID:       id,
			Message:  msg,
			QueuedAt: created,
		})
	}
	return out, rows.Err()
}

// DrainRefinements — Postgres implementation.
//
// Reads the pending set then atomically stamps a
// studio.refine.consumed.v1 marker for each refine_id. The marker
// is what the NOT EXISTS predicate in PendingRefinements keys off
// of, so re-Drains return empty until a fresh studio.refine.v1
// arrives.
func (p *Postgres) DrainRefinements(ctx context.Context, executionID string) ([]Refinement, error) {
	pending, err := p.PendingRefinements(ctx, executionID)
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
		// Best-effort: a failed insert just means this refinement
		// will surface again on the next Drain (idempotent retry).
		_ = p.RecordEvent(ctx, executionID, EventStudioRefineConsumedV1, payload)
	}
	return pending, nil
}

// SetWorkspaceID stamps the runtime workspace bound to the execution
// onto the row. Idempotent — calling twice with the same value is a
// no-op success at the SQL level (UPDATE rewrites the column with the
// same text). A different value overwrites; A63 picks overwrite over
// reject for simplicity since workspace rebinding is rare but does
// happen (sandbox restart, pod migration).
//
// Returns ErrNotFound when the execution row does not exist so callers
// can distinguish "no such execution" from "set succeeded".
func (p *Postgres) SetWorkspaceID(ctx context.Context, executionID, workspaceID string) error {
	if executionID == "" {
		return ErrNotFound
	}
	tag, err := p.pool.Exec(ctx,
		`UPDATE executions SET workspace_id = $2 WHERE id = $1`,
		executionID, workspaceID)
	if err != nil {
		return logPGErr(ctx, "set_workspace_id", executionID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// recordAndEmit inserts the row into execution_events AND publishes
// onto the broker. Errors on the INSERT are swallowed and printed via
// the context-bound logger if any (we deliberately do not block the
// happy path on the audit insert — the parent op has already
// committed). Subscribers always see the broker event so a downed
// audit insert does not stall the UI.
func (p *Postgres) recordAndEmit(ctx context.Context, id, eventType string, payload json.RawMessage) {
	now := time.Now().UTC()
	body := payload
	if len(body) == 0 {
		body = json.RawMessage(`{}`)
	}
	_, _ = p.pool.Exec(ctx,
		`INSERT INTO execution_events(execution_id, event_type, payload, created_at)
         VALUES ($1, $2, $3, $4)`,
		id, eventType, body, now)
	p.broker.publish(Event{
		ExecutionID: id,
		EventType:   eventType,
		Payload:     body,
		CreatedAt:   now,
	})
}
