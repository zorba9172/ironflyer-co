package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/events"
	"ironflyer/apps/orchestrator/internal/outboxhooks"
)

// PostgresService is the production-grade Service implementation
// backed by the ledger_entries table created in
// migrations/00025_ledger.sql. The write path is a single INSERT;
// the read paths are parameterised queries — no string-interpolated
// SQL is ever assembled in this file.
type PostgresService struct {
	pool          *pgxpool.Pool
	outboxEnabled bool
}

// NewPostgresService constructs a PostgresService over the given
// pool. The pool's lifecycle is owned by the caller.
func NewPostgresService(pool *pgxpool.Pool) *PostgresService {
	return &PostgresService{pool: pool}
}

// WithOutbox makes every ledger write enqueue a durable event_outbox row
// in the same Postgres transaction. The background events.Pump publishes
// those rows to Redpanda for ClickHouse, SurrealDB projections, and other
// consumers.
func (p *PostgresService) WithOutbox() *PostgresService {
	if p != nil {
		p.outboxEnabled = true
	}
	return p
}

const insertSQL = `
INSERT INTO ledger_entries
    (id, tenant_id, execution_id, entry_type, direction, amount_usd,
     provider, billable, margin_relevant, metadata, op_key, created_at)
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11, ''), $12)
RETURNING id, tenant_id, execution_id, entry_type, direction, amount_usd,
          provider, billable, margin_relevant, metadata, COALESCE(op_key, ''),
          created_at
`

// selectByOpKeySQL is the dedupe replay path. Returns the existing
// ledger_entries row for a given op_key so a retried Write with the
// same OpKey returns the prior row without inserting twice. The
// uniq_ledger_op_key partial unique index (migrations/00037) makes
// this lookup index-only.
const selectByOpKeySQL = `
SELECT id, tenant_id, execution_id, entry_type, direction, amount_usd,
       provider, billable, margin_relevant, metadata, COALESCE(op_key, ''),
       created_at
FROM ledger_entries
WHERE op_key = $1
`

// Write persists one entry. The migration's CHECK on entry_type +
// direction + amount_usd is a defence-in-depth backstop; validate()
// here keeps the failure local for the common mistakes.
//
// V22 idempotency: when e.OpKey is set, Write first probes
// ledger_entries by op_key. A hit returns the prior row without
// inserting a second one — Temporal-safe at-least-once writes.
func (p *PostgresService) Write(ctx context.Context, e Entry) (Entry, error) {
	if err := validate(e); err != nil {
		return Entry{}, err
	}
	if e.OpKey != "" {
		if prior, ok, err := p.lookupByOpKey(ctx, e.OpKey); err != nil {
			return Entry{}, err
		} else if ok {
			return prior, nil
		}
	}
	e = stamp(e)

	metaJSON, err := json.Marshal(e.Metadata)
	if err != nil {
		return Entry{}, fmt.Errorf("ledger: marshal metadata: %w", err)
	}

	if !p.outboxEnabled {
		row := p.pool.QueryRow(ctx, insertSQL,
			e.ID, e.TenantID, nullableUUID(e.ExecutionID),
			string(e.EntryType), string(e.Direction), e.AmountUSD,
			nullableString(e.Provider), e.Billable, e.MarginRelevant,
			metaJSON, e.OpKey, e.CreatedAt,
		)
		stored, err := scanEntry(row)
		if err != nil {
			// Unique-violation on op_key means a concurrent retry won
			// the race; re-read the winning row and return it.
			if e.OpKey != "" {
				if prior, ok, lerr := p.lookupByOpKey(ctx, e.OpKey); lerr == nil && ok {
					return prior, nil
				}
			}
			return Entry{}, fmt.Errorf("ledger: insert: %w", err)
		}
		observeWrite(stored)
		return stored, nil
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return Entry{}, fmt.Errorf("ledger: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, insertSQL,
		e.ID, e.TenantID, nullableUUID(e.ExecutionID),
		string(e.EntryType), string(e.Direction), e.AmountUSD,
		nullableString(e.Provider), e.Billable, e.MarginRelevant,
		metaJSON, e.OpKey, e.CreatedAt,
	)
	stored, err := scanEntry(row)
	if err != nil {
		if e.OpKey != "" {
			if prior, ok, lerr := p.lookupByOpKey(ctx, e.OpKey); lerr == nil && ok {
				return prior, nil
			}
		}
		return Entry{}, fmt.Errorf("ledger: insert: %w", err)
	}
	if err := outboxhooks.WriteEventInTx(ctx, tx, ledgerEvent(stored)); err != nil {
		return Entry{}, fmt.Errorf("ledger: enqueue event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Entry{}, fmt.Errorf("ledger: commit: %w", err)
	}
	observeWrite(stored)
	return stored, nil
}

// lookupByOpKey returns the prior ledger row that landed under this
// op_key, if any. Used by Write to short-circuit retried inserts.
func (p *PostgresService) lookupByOpKey(ctx context.Context, opKey string) (Entry, bool, error) {
	row := p.pool.QueryRow(ctx, selectByOpKeySQL, opKey)
	stored, err := scanEntry(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, false, nil
		}
		return Entry{}, false, fmt.Errorf("ledger: lookup op_key: %w", err)
	}
	return stored, true, nil
}

// WriteWithOutbox performs the ledger INSERT plus a caller-supplied
// outbox event in one tx the caller already owns. Use this from the
// Settler and from any other writer that needs to atomically commit a
// ledger row alongside a domain event (e.g. execution.settled.v1)
// without two competing transactions racing each other.
//
// outboxEvent.Topic, Type, and Key MUST be set; the rest is stamped
// by outboxhooks.WriteEventInTx.
func (p *PostgresService) WriteWithOutbox(ctx context.Context, tx pgx.Tx, e Entry, outboxEvent events.Event) (Entry, error) {
	if tx == nil {
		return Entry{}, fmt.Errorf("ledger: nil tx")
	}
	if err := validate(e); err != nil {
		return Entry{}, err
	}
	// V22 idempotency: if a prior row landed under e.OpKey, return it
	// without inserting twice. The caller's tx still commits cleanly —
	// only the ledger row is reused; the caller's outbox event still
	// publishes so the workflow signal isn't swallowed.
	if e.OpKey != "" {
		if prior, ok, lerr := p.lookupByOpKey(ctx, e.OpKey); lerr == nil && ok {
			if outboxEvent.Topic != "" {
				if werr := outboxhooks.WriteEventInTx(ctx, tx, outboxEvent); werr != nil {
					return Entry{}, fmt.Errorf("ledger: enqueue caller event: %w", werr)
				}
			}
			return prior, nil
		}
	}
	e = stamp(e)
	metaJSON, err := json.Marshal(e.Metadata)
	if err != nil {
		return Entry{}, fmt.Errorf("ledger: marshal metadata: %w", err)
	}
	row := tx.QueryRow(ctx, insertSQL,
		e.ID, e.TenantID, nullableUUID(e.ExecutionID),
		string(e.EntryType), string(e.Direction), e.AmountUSD,
		nullableString(e.Provider), e.Billable, e.MarginRelevant,
		metaJSON, e.OpKey, e.CreatedAt,
	)
	stored, err := scanEntry(row)
	if err != nil {
		return Entry{}, fmt.Errorf("ledger: insert: %w", err)
	}
	// Always emit the billing.ledger.* row for the entry itself so the
	// downstream finance projection sees every ledger fact.
	if err := outboxhooks.WriteEventInTx(ctx, tx, ledgerEvent(stored)); err != nil {
		return Entry{}, fmt.Errorf("ledger: enqueue ledger event: %w", err)
	}
	if outboxEvent.Topic != "" {
		if err := outboxhooks.WriteEventInTx(ctx, tx, outboxEvent); err != nil {
			return Entry{}, fmt.Errorf("ledger: enqueue caller event: %w", err)
		}
	}
	observeWrite(stored)
	return stored, nil
}

func ledgerEvent(e Entry) events.Event {
	payload := map[string]any{
		"ledger_entry_id": e.ID.String(),
		"tenant_id":       e.TenantID.String(),
		"entry_type":      string(e.EntryType),
		"direction":       string(e.Direction),
		"amount_usd":      e.AmountUSD.String(),
		"provider":        e.Provider,
		"billable":        e.Billable,
		"margin_relevant": e.MarginRelevant,
		"metadata":        e.Metadata,
		"created_at":      e.CreatedAt.Format(time.RFC3339Nano),
	}
	key := e.TenantID.String()
	if e.ExecutionID != nil {
		key = e.ExecutionID.String()
		payload["execution_id"] = e.ExecutionID.String()
	}
	return events.Event{
		ID:      e.ID,
		Topic:   events.TopicFor("", "billing", "ledger", 1),
		Key:     key,
		Type:    "ledger." + string(e.EntryType) + ".v1",
		Version: 1,
		Payload: payload,
		Headers: map[string]any{
			"tenant_id": e.TenantID.String(),
		},
		CreatedAt: e.CreatedAt,
	}
}

// ListByTenant assembles a parameterised WHERE clause from the
// Filter and runs it. The query builder uses positional placeholders
// ($1, $2, …) and a parallel args slice — never %s formatting of
// user-supplied values.
func (p *PostgresService) ListByTenant(ctx context.Context, tenantID uuid.UUID, f Filter) ([]Entry, error) {
	var (
		clauses = []string{"tenant_id = $1"}
		args    = []any{tenantID}
	)

	if !f.Since.IsZero() {
		args = append(args, f.Since)
		clauses = append(clauses, "created_at >= $"+strconv.Itoa(len(args)))
	}
	if !f.Until.IsZero() {
		args = append(args, f.Until)
		clauses = append(clauses, "created_at <= $"+strconv.Itoa(len(args)))
	}
	if f.ExecutionID != nil {
		args = append(args, *f.ExecutionID)
		clauses = append(clauses, "execution_id = $"+strconv.Itoa(len(args)))
	}
	if len(f.EntryTypes) > 0 {
		typeStrs := make([]string, 0, len(f.EntryTypes))
		for _, t := range f.EntryTypes {
			typeStrs = append(typeStrs, string(t))
		}
		args = append(args, typeStrs)
		clauses = append(clauses, "entry_type = ANY($"+strconv.Itoa(len(args))+")")
	}

	sql := `
SELECT id, tenant_id, execution_id, entry_type, direction, amount_usd,
       provider, billable, margin_relevant, metadata, COALESCE(op_key, ''), created_at
FROM ledger_entries
WHERE ` + strings.Join(clauses, " AND ") + `
ORDER BY created_at DESC`

	if f.Limit > 0 {
		args = append(args, f.Limit)
		sql += " LIMIT $" + strconv.Itoa(len(args))
	}
	if f.Offset > 0 {
		args = append(args, f.Offset)
		sql += " OFFSET $" + strconv.Itoa(len(args))
	}

	rows, err := p.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("ledger: query tenant: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

// ListByExecution returns the per-execution timeline, oldest-first.
func (p *PostgresService) ListByExecution(ctx context.Context, executionID uuid.UUID) ([]Entry, error) {
	const sql = `
SELECT id, tenant_id, execution_id, entry_type, direction, amount_usd,
       provider, billable, margin_relevant, metadata, COALESCE(op_key, ''), created_at
FROM ledger_entries
WHERE execution_id = $1
ORDER BY created_at ASC`

	rows, err := p.pool.Query(ctx, sql, executionID)
	if err != nil {
		return nil, fmt.Errorf("ledger: query execution: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

// SumByType returns per-EntryType totals computed in Postgres so we
// don't ship every row across the wire for a dashboard rollup.
func (p *PostgresService) SumByType(ctx context.Context, tenantID uuid.UUID, types []EntryType, since, until time.Time) (map[EntryType]decimal.Decimal, error) {
	var (
		clauses = []string{"tenant_id = $1"}
		args    = []any{tenantID}
	)

	if !since.IsZero() {
		args = append(args, since)
		clauses = append(clauses, "created_at >= $"+strconv.Itoa(len(args)))
	}
	if !until.IsZero() {
		args = append(args, until)
		clauses = append(clauses, "created_at <= $"+strconv.Itoa(len(args)))
	}
	if len(types) > 0 {
		typeStrs := make([]string, 0, len(types))
		for _, t := range types {
			typeStrs = append(typeStrs, string(t))
		}
		args = append(args, typeStrs)
		clauses = append(clauses, "entry_type = ANY($"+strconv.Itoa(len(args))+")")
	}

	sql := `
SELECT entry_type, COALESCE(SUM(amount_usd), 0)
FROM ledger_entries
WHERE ` + strings.Join(clauses, " AND ") + `
GROUP BY entry_type`

	rows, err := p.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("ledger: sum by type: %w", err)
	}
	defer rows.Close()

	sums := make(map[EntryType]decimal.Decimal, len(AllEntryTypes))
	for rows.Next() {
		var (
			t   string
			amt decimal.Decimal
		)
		if err := rows.Scan(&t, &amt); err != nil {
			return nil, fmt.Errorf("ledger: scan sum: %w", err)
		}
		sums[EntryType(t)] = amt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate sum: %w", err)
	}
	return sums, nil
}

// TenantRollup pulls the per-type sums in a single grouped query and
// folds them into a Rollup. Cheaper than ListByTenant + Build for
// dashboards because the totals are computed inside Postgres.
func (p *PostgresService) TenantRollup(ctx context.Context, tenantID uuid.UUID, since, until time.Time) (Rollup, error) {
	sums, err := p.SumByType(ctx, tenantID, nil, since, until)
	if err != nil {
		return Rollup{}, err
	}
	var r Rollup
	r.RevenueUSD = sums[EntryWalletTopup]
	r.ProviderCostUSD = sums[EntryProviderInferenceCost]
	r.SandboxCostUSD = sums[EntrySandboxCost]
	r.StorageCostUSD = sums[EntryStorageCost]
	r.DeploymentCostUSD = sums[EntryDeploymentCost]
	r.PremiumReasoningCostUSD = sums[EntryPremiumReasoningCharge]
	r.RefundsUSD = sums[EntryRefund]
	r.PlatformMarginUSD = sums[EntryPlatformMargin]
	r.MobileBuildCostUSD = sums[EntryMobileBuildMin]
	r.EmulatorCostUSD = sums[EntryEmulatorMin]
	r.MacWorkspaceCostUSD = sums[EntryMacWorkspaceMin]
	r.EASBuildCostUSD = sums[EntryEASBuildCredit]
	r.AppetizeCostUSD = sums[EntryAppetizeMin]
	r.MobileCostUSD = mobileTotal(r)

	allCosts := decimal.Zero.
		Add(r.ProviderCostUSD).
		Add(r.SandboxCostUSD).
		Add(r.StorageCostUSD).
		Add(r.DeploymentCostUSD).
		Add(r.PremiumReasoningCostUSD).
		Add(r.RefundsUSD).
		Add(r.MobileCostUSD)

	if r.RevenueUSD.IsZero() {
		r.GrossMarginPct = decimal.Zero
		return r, nil
	}
	gross := r.RevenueUSD.Sub(allCosts)
	r.GrossMarginPct = gross.Div(r.RevenueUSD).Mul(decimal.NewFromInt(100))
	return r, nil
}

// --- scan helpers ---------------------------------------------------------

// scanEntry decodes one row from a QueryRow / Rows source into Entry.
// rowScanner is the smallest interface that pgx.Row and pgx.Rows both
// satisfy so the same scan logic services single-row and multi-row
// calls.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanEntry(r rowScanner) (Entry, error) {
	var (
		e        Entry
		execID   *uuid.UUID
		provider *string
		metaRaw  []byte
		opKey    string
	)
	if err := r.Scan(
		&e.ID, &e.TenantID, &execID,
		&e.EntryType, &e.Direction, &e.AmountUSD,
		&provider, &e.Billable, &e.MarginRelevant,
		&metaRaw, &opKey, &e.CreatedAt,
	); err != nil {
		return Entry{}, err
	}
	e.OpKey = opKey
	e.ExecutionID = execID
	if provider != nil {
		e.Provider = *provider
	}
	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &e.Metadata); err != nil {
			return Entry{}, fmt.Errorf("ledger: unmarshal metadata: %w", err)
		}
	}
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}
	return e, nil
}

func scanRows(rows pgx.Rows) ([]Entry, error) {
	out := make([]Entry, 0)
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("ledger: scan row: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate rows: %w", err)
	}
	return out, nil
}

// nullableUUID converts a *uuid.UUID into a value pgx will store as
// SQL NULL when nil, so callers don't have to thread sql.NullX types.
func nullableUUID(u *uuid.UUID) any {
	if u == nil {
		return nil
	}
	return *u
}

// nullableString converts an empty string into SQL NULL.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
