package budget

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// BootstrapPostgres creates the ledger + vault tables if they don't exist.
//
// Deprecated: schema for this package now lives in
// core/orchestrator/migrations/00001_init_budget.sql. New schema changes
// MUST land as a numbered goose migration in that directory — not inline
// here. This function is retained only as a fallback for callers that
// don't yet route through the goose runner.
const bootstrapSQL = `
CREATE TABLE IF NOT EXISTS budget_ledger (
    id            UUID        PRIMARY KEY,
    user_id       TEXT        NOT NULL,
    project_id    TEXT        NULL,
    provider      TEXT        NOT NULL,
    model         TEXT        NOT NULL,
    input_tokens  INT         NOT NULL DEFAULT 0,
    output_tokens INT         NOT NULL DEFAULT 0,
    cache_read    INT         NOT NULL DEFAULT 0,
    cache_create  INT         NOT NULL DEFAULT 0,
    cost_usd      NUMERIC(20,10) NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_budget_ledger_user_period ON budget_ledger(user_id, created_at);

CREATE TABLE IF NOT EXISTS budget_vault (
    id         UUID        PRIMARY KEY,
    kind       TEXT        NOT NULL,
    user_id    TEXT        NULL,
    amount     NUMERIC(20,10) NOT NULL,
    note       TEXT        NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_budget_vault_kind ON budget_vault(kind);
`

func BootstrapPostgres(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, bootstrapSQL)
	return err
}

// PostgresLedger persists ledger entries in `budget_ledger`.
type PostgresLedger struct{ pool *pgxpool.Pool }

func NewPostgresLedger(pool *pgxpool.Pool) *PostgresLedger {
	return &PostgresLedger{pool: pool}
}

func (l *PostgresLedger) Charge(ctx context.Context, e LedgerEntry) (LedgerEntry, error) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	_, err := l.pool.Exec(ctx, `
        INSERT INTO budget_ledger(id, user_id, project_id, provider, model,
                                  input_tokens, output_tokens, cache_read,
                                  cache_create, cost_usd, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		e.ID, e.UserID, nullable(e.ProjectID), e.Provider, e.Model,
		e.InputTokens, e.OutputTokens, e.CacheRead, e.CacheCreate,
		e.CostUSD.String(), e.CreatedAt)
	if err != nil {
		return LedgerEntry{}, err
	}
	return e, nil
}

func (l *PostgresLedger) SpentByUser(ctx context.Context, userID string) (decimal.Decimal, error) {
	row := l.pool.QueryRow(ctx, `
        SELECT COALESCE(SUM(cost_usd), 0)::text
        FROM budget_ledger
        WHERE user_id = $1 AND created_at >= date_trunc('month', now() AT TIME ZONE 'UTC')`, userID)
	return scanDecimal(row)
}

func (l *PostgresLedger) SpentTotal(ctx context.Context) (decimal.Decimal, error) {
	row := l.pool.QueryRow(ctx, `
        SELECT COALESCE(SUM(cost_usd), 0)::text
        FROM budget_ledger
        WHERE created_at >= date_trunc('month', now() AT TIME ZONE 'UTC')`)
	return scanDecimal(row)
}

func (l *PostgresLedger) EntriesByUser(ctx context.Context, userID string) ([]LedgerEntry, error) {
	rows, err := l.pool.Query(ctx, `
        SELECT id, user_id, COALESCE(project_id, ''), provider, model,
               input_tokens, output_tokens, cache_read, cache_create,
               cost_usd::text, created_at
        FROM budget_ledger WHERE user_id = $1
        ORDER BY created_at DESC LIMIT 500`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		var cost string
		if err := rows.Scan(&e.ID, &e.UserID, &e.ProjectID, &e.Provider, &e.Model,
			&e.InputTokens, &e.OutputTokens, &e.CacheRead, &e.CacheCreate,
			&cost, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.CostUSD, _ = decimal.NewFromString(cost)
		out = append(out, e)
	}
	return out, rows.Err()
}

// PostgresVault persists vault movements in `budget_vault`.
type PostgresVault struct{ pool *pgxpool.Pool }

func NewPostgresVault(pool *pgxpool.Pool) *PostgresVault {
	return &PostgresVault{pool: pool}
}

func (v *PostgresVault) Record(ctx context.Context, e VaultEntry) (VaultEntry, error) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	_, err := v.pool.Exec(ctx, `
        INSERT INTO budget_vault(id, kind, user_id, amount, note, created_at)
        VALUES ($1,$2,$3,$4,$5,$6)`,
		e.ID, string(e.Kind), nullable(e.UserID), e.Amount.String(),
		nullable(e.Note), e.CreatedAt)
	if err != nil {
		return VaultEntry{}, err
	}
	return e, nil
}

func (v *PostgresVault) Balance(ctx context.Context) (decimal.Decimal, error) {
	row := v.pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount), 0)::text FROM budget_vault`)
	return scanDecimal(row)
}

func (v *PostgresVault) Snapshot(ctx context.Context) (VaultSnapshot, error) {
	rows, err := v.pool.Query(ctx, `
        SELECT kind, COALESCE(SUM(amount), 0)::text
        FROM budget_vault GROUP BY kind`)
	if err != nil {
		return VaultSnapshot{}, err
	}
	defer rows.Close()
	s := VaultSnapshot{}
	for rows.Next() {
		var kind, sum string
		if err := rows.Scan(&kind, &sum); err != nil {
			return VaultSnapshot{}, err
		}
		amt, _ := decimal.NewFromString(sum)
		switch VaultEntryKind(kind) {
		case VaultRevenue:
			s.Revenue = amt
		case VaultProviderCost:
			s.ProviderCost = amt.Abs()
		case VaultRefund:
			s.Refunds = amt.Abs()
		case VaultAdjustment:
			s.Adjustments = amt
		}
	}
	s.Margin = s.Revenue.Sub(s.ProviderCost).Sub(s.Refunds).Add(s.Adjustments)
	return s, rows.Err()
}

func scanDecimal(row pgx.Row) (decimal.Decimal, error) {
	var s string
	if err := row.Scan(&s); err != nil {
		return decimal.Zero, err
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, err
	}
	return d, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var (
	_ LedgerStore = (*PostgresLedger)(nil)
	_ VaultStore  = (*PostgresVault)(nil)
)

// ConnectPostgres builds a pgxpool.Pool and verifies connectivity.
func ConnectPostgres(ctx context.Context, url string) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, errors.New("postgres URL empty")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx2); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
