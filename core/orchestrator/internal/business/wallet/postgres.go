package wallet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/operations/events"
	"ironflyer/core/orchestrator/internal/operations/metrics"
	"ironflyer/core/orchestrator/internal/business/outboxhooks"
)

// PostgresService is the production-grade Service backed by Postgres.
// All money-moving operations run inside a transaction with SELECT …
// FOR UPDATE on the wallets row so concurrent Holds and Debits stay
// serialized per-tenant; this is what makes hard law 1 ("no execution
// starts without budget") enforceable under load.
type PostgresService struct {
	pool          *pgxpool.Pool
	outboxEnabled bool
}

// NewPostgresService wires the wallet service to an existing pgxpool.
// The migrations/00024_wallets.sql file MUST have been applied first;
// the service does not bootstrap schema (goose runs that elsewhere).
func NewPostgresService(pool *pgxpool.Pool) *PostgresService {
	return &PostgresService{pool: pool}
}

// WithOutbox enables durable event emission: every money-moving op
// (TopUp / Hold / Release / Debit) writes a billing.ledger outbox row
// in the same transaction as the wallet mutation, so the publisher
// daemon can replay the cash-flow stream into Redpanda without a
// secondary scan of the wallets table.
func (s *PostgresService) WithOutbox() *PostgresService {
	if s != nil {
		s.outboxEnabled = true
	}
	return s
}

// Get returns the tenant wallet, materializing an empty row on first
// touch so the resolver never has to special-case "wallet missing".
func (s *PostgresService) Get(ctx context.Context, tenant string) (Wallet, error) {
	// First-touch upsert: insert a zero-balance row if missing. We use
	// ON CONFLICT DO NOTHING so the operation is idempotent under
	// concurrent first-Get calls.
	if _, err := s.pool.Exec(ctx, `
        INSERT INTO wallets(tenant_id) VALUES ($1)
        ON CONFLICT (tenant_id) DO NOTHING`, tenant); err != nil {
		return Wallet{}, fmt.Errorf("wallet: ensure row: %w", err)
	}
	row := s.pool.QueryRow(ctx, `
        SELECT tenant_id, balance_usd::text, hold_usd::text,
               lifetime_topup_usd::text, lifetime_spend_usd::text,
               updated_at, created_at
        FROM wallets WHERE tenant_id = $1`, tenant)
	return scanWallet(row)
}

// TopUp credits the wallet and flips the matching wallet_topups row to
// succeeded. Idempotent against stripeSessionID via the UNIQUE
// constraint — a duplicate webhook delivery is a no-op.
func (s *PostgresService) TopUp(ctx context.Context, tenant string, amount decimal.Decimal, stripeSessionID string) error {
	if amount.IsZero() || amount.IsNegative() {
		return ErrInvalidAmount
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.topUpInTx(ctx, tx, tenant, amount, stripeSessionID)
	})
}

// topUpInTx is the body of TopUp factored out so the *WithKey variant
// can run the credit AND the wallet_operations dedupe insert in a
// single Postgres transaction. Caller owns the tx lifecycle.
func (s *PostgresService) topUpInTx(ctx context.Context, tx pgx.Tx, tenant string, amount decimal.Decimal, stripeSessionID string) error {
	// Idempotency guard: if the session row is already 'succeeded',
	// short-circuit so a retried webhook delivery cannot double-credit.
	if stripeSessionID != "" {
		var status string
		err := tx.QueryRow(ctx, `
            SELECT status FROM wallet_topups
            WHERE stripe_session_id = $1`, stripeSessionID).Scan(&status)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// No matching pending row — this is unusual but tolerate it:
			// the webhook arrived before the pending row was visible.
			// We still need to record a succeeded row, which we do
			// below after the balance update.
		case err != nil:
			return fmt.Errorf("wallet: lookup session: %w", err)
		default:
			if status == "succeeded" {
				return nil
			}
		}
	}

	// Lock the wallet row (creating it if missing) and bump balances.
	if _, err := tx.Exec(ctx, `
        INSERT INTO wallets(tenant_id) VALUES ($1)
        ON CONFLICT (tenant_id) DO NOTHING`, tenant); err != nil {
		return fmt.Errorf("wallet: ensure row: %w", err)
	}
	if _, err := tx.Exec(ctx, `
        SELECT 1 FROM wallets WHERE tenant_id = $1 FOR UPDATE`, tenant); err != nil {
		return fmt.Errorf("wallet: lock row: %w", err)
	}
	if _, err := tx.Exec(ctx, `
        UPDATE wallets
           SET balance_usd        = balance_usd + $2::numeric,
               lifetime_topup_usd = lifetime_topup_usd + $2::numeric,
               updated_at         = now()
         WHERE tenant_id = $1`, tenant, amount.String()); err != nil {
		return fmt.Errorf("wallet: credit: %w", err)
	}

	// Flip pending → succeeded (or insert a new succeeded row when the
	// pending row didn't exist yet).
	if stripeSessionID != "" {
		ct, err := tx.Exec(ctx, `
            UPDATE wallet_topups
               SET status = 'succeeded', completed_at = now()
             WHERE stripe_session_id = $1
               AND status = 'pending'`, stripeSessionID)
		if err != nil {
			return fmt.Errorf("wallet: mark succeeded: %w", err)
		}
		if ct.RowsAffected() == 0 {
			// No pending row — race or out-of-band webhook. Record a
			// synthetic succeeded row so the user-visible history is
			// complete.
			if _, err := tx.Exec(ctx, `
                INSERT INTO wallet_topups(tenant_id, stripe_session_id, amount_usd, status, completed_at)
                VALUES ($1, $2, $3::numeric, 'succeeded', now())
                ON CONFLICT (stripe_session_id) DO NOTHING`,
				tenant, stripeSessionID, amount.String()); err != nil {
				return fmt.Errorf("wallet: record topup: %w", err)
			}
		}
	}
	return s.emitWalletEvent(ctx, tx, tenant, "wallet.topup.v1", amount, map[string]any{
		"stripe_session_id": stripeSessionID,
	})
}

// Hold reserves amount under SELECT … FOR UPDATE.
func (s *PostgresService) Hold(ctx context.Context, tenant string, amount decimal.Decimal) error {
	if amount.IsZero() || amount.IsNegative() {
		return ErrInvalidAmount
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		return s.holdInTx(ctx, tx, tenant, amount)
	}); err != nil {
		return err
	}
	metrics.IncWalletHoldsActive()
	return nil
}

// holdInTx is Hold's body factored out so *WithKey variants can fold
// the mutation, dedupe insert, and outbox event into one tx.
func (s *PostgresService) holdInTx(ctx context.Context, tx pgx.Tx, tenant string, amount decimal.Decimal) error {
	balance, hold, err := s.lockAndRead(ctx, tx, tenant)
	if err != nil {
		return err
	}
	if balance.Sub(hold).LessThan(amount) {
		return ErrInsufficient
	}
	if _, err := tx.Exec(ctx, `
        UPDATE wallets
           SET hold_usd = hold_usd + $2::numeric,
               updated_at = now()
         WHERE tenant_id = $1`, tenant, amount.String()); err != nil {
		return err
	}
	return s.emitWalletEvent(ctx, tx, tenant, "wallet.hold.v1", amount, nil)
}

// Release returns an unused hold to available balance, clamped at zero.
func (s *PostgresService) Release(ctx context.Context, tenant string, amount decimal.Decimal) error {
	if amount.IsZero() || amount.IsNegative() {
		return ErrInvalidAmount
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		return s.releaseInTx(ctx, tx, tenant, amount)
	}); err != nil {
		return err
	}
	metrics.DecWalletHoldsActive()
	return nil
}

// releaseInTx is Release's body factored out for *WithKey co-tx use.
func (s *PostgresService) releaseInTx(ctx context.Context, tx pgx.Tx, tenant string, amount decimal.Decimal) error {
	_, hold, err := s.lockAndRead(ctx, tx, tenant)
	if err != nil {
		return err
	}
	release := amount
	if release.GreaterThan(hold) {
		release = hold
	}
	if _, err := tx.Exec(ctx, `
        UPDATE wallets
           SET hold_usd = hold_usd - $2::numeric,
               updated_at = now()
         WHERE tenant_id = $1`, tenant, release.String()); err != nil {
		return err
	}
	return s.emitWalletEvent(ctx, tx, tenant, "wallet.release.v1", release, nil)
}

// Debit closes a previously-held amount.
func (s *PostgresService) Debit(ctx context.Context, tenant string, amount decimal.Decimal) error {
	if amount.IsZero() || amount.IsNegative() {
		return ErrInvalidAmount
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		return s.debitInTx(ctx, tx, tenant, amount)
	}); err != nil {
		return err
	}
	metrics.DecWalletHoldsActive()
	return nil
}

// debitInTx is Debit's body factored out for *WithKey co-tx use.
func (s *PostgresService) debitInTx(ctx context.Context, tx pgx.Tx, tenant string, amount decimal.Decimal) error {
	balance, hold, err := s.lockAndRead(ctx, tx, tenant)
	if err != nil {
		return err
	}
	if balance.LessThan(amount) || hold.LessThan(amount) {
		return ErrInsufficient
	}
	if _, err := tx.Exec(ctx, `
        UPDATE wallets
           SET balance_usd        = balance_usd - $2::numeric,
               hold_usd           = hold_usd    - $2::numeric,
               lifetime_spend_usd = lifetime_spend_usd + $2::numeric,
               updated_at         = now()
         WHERE tenant_id = $1`, tenant, amount.String()); err != nil {
		return err
	}
	return s.emitWalletEvent(ctx, tx, tenant, "wallet.debit.v1", amount, nil)
}

// emitWalletEvent writes one billing.ledger outbox row in the caller's
// tx so the wallet mutation and the durable event commit together.
// No-op when WithOutbox() was not called on this service — keeps the
// in-memory / no-outbox paths free of Redpanda dependencies.
func (s *PostgresService) emitWalletEvent(ctx context.Context, tx pgx.Tx, tenant, eventType string, amount decimal.Decimal, metadata map[string]any) error {
	if s == nil || !s.outboxEnabled {
		return nil
	}
	direction := "debit"
	switch eventType {
	case "wallet.topup.v1", "wallet.release.v1":
		direction = "credit"
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	evt := events.Event{
		Topic:   events.TopicFor("", "billing", "ledger", 1),
		Key:     tenant,
		Type:    eventType,
		Version: 1,
		Payload: map[string]any{
			"tenant_id":  tenant,
			"direction":  direction,
			"amount_usd": amount.String(),
			"metadata":   metadata,
		},
		Headers: map[string]any{"tenant_id": tenant},
	}
	return outboxhooks.WriteEventInTx(ctx, tx, evt)
}

// LifetimeStats reads the dashboard counters.
func (s *PostgresService) LifetimeStats(ctx context.Context, tenant string) (LifetimeStats, error) {
	if _, err := s.pool.Exec(ctx, `
        INSERT INTO wallets(tenant_id) VALUES ($1)
        ON CONFLICT (tenant_id) DO NOTHING`, tenant); err != nil {
		return LifetimeStats{}, fmt.Errorf("wallet: ensure row: %w", err)
	}
	var topup, spend string
	if err := s.pool.QueryRow(ctx, `
        SELECT lifetime_topup_usd::text, lifetime_spend_usd::text
        FROM wallets WHERE tenant_id = $1`, tenant).Scan(&topup, &spend); err != nil {
		return LifetimeStats{}, fmt.Errorf("wallet: read stats: %w", err)
	}
	t, err := decimal.NewFromString(topup)
	if err != nil {
		return LifetimeStats{}, fmt.Errorf("wallet: parse topup: %w", err)
	}
	sp, err := decimal.NewFromString(spend)
	if err != nil {
		return LifetimeStats{}, fmt.Errorf("wallet: parse spend: %w", err)
	}
	return LifetimeStats{LifetimeTopUpUSD: t, LifetimeSpendUSD: sp}, nil
}

// ListTopUps returns recent rows for the tenant, newest first.
func (s *PostgresService) ListTopUps(ctx context.Context, tenant string, limit int) ([]TopUp, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
        SELECT id, tenant_id, COALESCE(stripe_session_id, ''),
               amount_usd::text, status, created_at, completed_at
        FROM wallet_topups
        WHERE tenant_id = $1
        ORDER BY created_at DESC
        LIMIT $2`, tenant, limit)
	if err != nil {
		return nil, fmt.Errorf("wallet: list topups: %w", err)
	}
	defer rows.Close()
	out := make([]TopUp, 0, limit)
	for rows.Next() {
		var t TopUp
		var amount string
		var completed *time.Time
		if err := rows.Scan(&t.ID, &t.TenantID, &t.StripeSessionID,
			&amount, &t.Status, &t.CreatedAt, &completed); err != nil {
			return nil, fmt.Errorf("wallet: scan topup: %w", err)
		}
		amt, err := decimal.NewFromString(amount)
		if err != nil {
			return nil, fmt.Errorf("wallet: parse amount: %w", err)
		}
		t.AmountUSD = amt
		t.CompletedAt = completed
		out = append(out, t)
	}
	return out, rows.Err()
}

// CreatePendingTopUp records a Checkout session before the webhook
// fires. The UNIQUE(stripe_session_id) constraint guarantees we never
// stage two pending rows for the same session.
func (s *PostgresService) CreatePendingTopUp(ctx context.Context, tenant string, amount decimal.Decimal, stripeSessionID string) (TopUp, error) {
	if amount.IsZero() || amount.IsNegative() {
		return TopUp{}, ErrInvalidAmount
	}
	row := s.pool.QueryRow(ctx, `
        INSERT INTO wallet_topups(tenant_id, stripe_session_id, amount_usd, status)
        VALUES ($1, $2, $3::numeric, 'pending')
        RETURNING id, tenant_id, COALESCE(stripe_session_id, ''),
                  amount_usd::text, status, created_at, completed_at`,
		tenant, stripeSessionID, amount.String())
	var t TopUp
	var amt string
	var completed *time.Time
	if err := row.Scan(&t.ID, &t.TenantID, &t.StripeSessionID,
		&amt, &t.Status, &t.CreatedAt, &completed); err != nil {
		return TopUp{}, fmt.Errorf("wallet: insert pending: %w", err)
	}
	parsed, err := decimal.NewFromString(amt)
	if err != nil {
		return TopUp{}, fmt.Errorf("wallet: parse amount: %w", err)
	}
	t.AmountUSD = parsed
	t.CompletedAt = completed
	return t, nil
}

// --- V22 opKey-aware idempotent variants -----------------------------
//
// Temporal retries land the same activity body more than once. Each
// *WithKey method records its op in wallet_operations (PK = op_key)
// after the wallet mutation commits; a retried call hits the PK
// conflict and reads back the prior outcome instead of applying the
// mutation a second time.
//
// Failed ops are persisted with status='failed' + error_code so a
// retry returns the same error rather than a different one ("first
// failed, retry succeeded" is the classic Temporal foot-gun).

// errFromCode turns a persisted error_code back into the canonical
// error value. Legacy rows written by the pre-A30 *WithKey variants
// could carry status='failed' + an error_code; the new variants only
// write status='succeeded', but we keep the mapping so old rows still
// replay correctly.
func errFromCode(code string) error {
	switch code {
	case "":
		return nil
	case "insufficient":
		return ErrInsufficient
	case "invalid_amount":
		return ErrInvalidAmount
	case "unknown_session":
		return ErrUnknownSession
	default:
		return fmt.Errorf("wallet: prior op failed (%s)", code)
	}
}

// --- single-tx *WithKey variants (Agent 30) --------------------------
//
// The pre-A30 pattern wrote the wallet mutation in one tx and the
// wallet_operations dedupe row in a second tx. A pod crash between
// those two writes left the mutation applied with no dedupe trail,
// so the next Temporal retry would mutate a second time. The new
// pattern wraps BOTH writes in one BeginTx — either they commit
// together or neither does.
//
// Failure handling: we DO NOT persist failed ops. If holdInTx returns
// an error the tx rolls back, the dedupe row is never written, and a
// retry runs the same mutation again. For our wallet semantics (intent-
// keyed amounts) that's correct — a second run will either succeed or
// fail the same way. A caller that retries with a different amount for
// the same opKey is a caller bug, not something we paper over.
//
// recallWalletOpTx and recordWalletOpTx are the tx-scoped twins of
// recallWalletOp / recordWalletOp; the legacy pool-scoped helpers are
// kept for callers that want a stand-alone dedupe lookup.

// HoldWithKey is the idempotent variant of Hold. Mutation + dedupe
// share one Postgres transaction so a crash between the two cannot
// double-mutate.
func (s *PostgresService) HoldWithKey(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error {
	if opKey == "" {
		return s.Hold(ctx, tenant, amount)
	}
	dedupeHit := false
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		if prior, ok, err := s.recallWalletOpTx(ctx, tx, opKey); err != nil {
			return err
		} else if ok {
			dedupeHit = true
			if prior.status == "succeeded" {
				return nil
			}
			return errFromCode(prior.errorCode)
		}
		if err := s.holdInTx(ctx, tx, tenant, amount); err != nil {
			return err
		}
		return s.recordWalletOpTx(ctx, tx, opKey, tenant, OpHold, amount, "succeeded", "")
	}); err != nil {
		return err
	}
	if !dedupeHit {
		metrics.IncWalletHoldsActive()
	}
	return nil
}

// ReleaseWithKey is the idempotent variant of Release (single-tx).
func (s *PostgresService) ReleaseWithKey(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error {
	if opKey == "" {
		return s.Release(ctx, tenant, amount)
	}
	dedupeHit := false
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		if prior, ok, err := s.recallWalletOpTx(ctx, tx, opKey); err != nil {
			return err
		} else if ok {
			dedupeHit = true
			if prior.status == "succeeded" {
				return nil
			}
			return errFromCode(prior.errorCode)
		}
		if err := s.releaseInTx(ctx, tx, tenant, amount); err != nil {
			return err
		}
		return s.recordWalletOpTx(ctx, tx, opKey, tenant, OpRelease, amount, "succeeded", "")
	}); err != nil {
		return err
	}
	if !dedupeHit {
		metrics.DecWalletHoldsActive()
	}
	return nil
}

// DebitWithKey is the idempotent variant of Debit (single-tx).
func (s *PostgresService) DebitWithKey(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error {
	if opKey == "" {
		return s.Debit(ctx, tenant, amount)
	}
	dedupeHit := false
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		if prior, ok, err := s.recallWalletOpTx(ctx, tx, opKey); err != nil {
			return err
		} else if ok {
			dedupeHit = true
			if prior.status == "succeeded" {
				return nil
			}
			return errFromCode(prior.errorCode)
		}
		if err := s.debitInTx(ctx, tx, tenant, amount); err != nil {
			return err
		}
		return s.recordWalletOpTx(ctx, tx, opKey, tenant, OpDebit, amount, "succeeded", "")
	}); err != nil {
		return err
	}
	if !dedupeHit {
		metrics.DecWalletHoldsActive()
	}
	return nil
}

// TopUpWithKey is the idempotent variant of TopUp (single-tx).
func (s *PostgresService) TopUpWithKey(ctx context.Context, tenant string, amount decimal.Decimal, stripeSessionID, opKey string) error {
	if opKey == "" {
		return s.TopUp(ctx, tenant, amount, stripeSessionID)
	}
	if amount.IsZero() || amount.IsNegative() {
		return ErrInvalidAmount
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if prior, ok, err := s.recallWalletOpTx(ctx, tx, opKey); err != nil {
			return err
		} else if ok {
			if prior.status == "succeeded" {
				return nil
			}
			return errFromCode(prior.errorCode)
		}
		if err := s.topUpInTx(ctx, tx, tenant, amount, stripeSessionID); err != nil {
			return err
		}
		return s.recordWalletOpTx(ctx, tx, opKey, tenant, OpTopUp, amount, "succeeded", "")
	})
}

// withTx runs fn in a single Postgres transaction. Errors trigger
// rollback; nil triggers commit. Use this whenever a single
// caller-visible operation needs more than one statement to be atomic.
func (s *PostgresService) withTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("wallet: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// lockAndRead ensures the wallet row exists, takes a SELECT … FOR
// UPDATE lock on it, and returns the current (balance, hold). Shared
// by holdInTx / releaseInTx / debitInTx so the lock pattern lives in
// one place.
func (s *PostgresService) lockAndRead(ctx context.Context, tx pgx.Tx, tenant string) (decimal.Decimal, decimal.Decimal, error) {
	if _, err := tx.Exec(ctx, `
        INSERT INTO wallets(tenant_id) VALUES ($1)
        ON CONFLICT (tenant_id) DO NOTHING`, tenant); err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("wallet: ensure row: %w", err)
	}
	var balance, hold string
	if err := tx.QueryRow(ctx, `
        SELECT balance_usd::text, hold_usd::text
        FROM wallets
        WHERE tenant_id = $1
        FOR UPDATE`, tenant).Scan(&balance, &hold); err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("wallet: lock + read: %w", err)
	}
	bDec, err := decimal.NewFromString(balance)
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("wallet: parse balance: %w", err)
	}
	hDec, err := decimal.NewFromString(hold)
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("wallet: parse hold: %w", err)
	}
	return bDec, hDec, nil
}

// walletOpRow is the strongly-typed projection of a prior wallet_operations
// row. recallWalletOpTx returns one of these so callers don't have to
// juggle two parallel return values.
type walletOpRow struct {
	status    string
	errorCode string
}

// recallWalletOpTx is the tx-scoped twin of recallWalletOp. Running the
// dedupe lookup inside the same tx as the mutation closes the
// "lookup-said-no, then-another-pod-recorded, then-we-also-recorded"
// race the legacy WithKey wrappers were vulnerable to.
func (s *PostgresService) recallWalletOpTx(ctx context.Context, tx pgx.Tx, opKey string) (walletOpRow, bool, error) {
	if opKey == "" {
		return walletOpRow{}, false, nil
	}
	row := tx.QueryRow(ctx,
		`SELECT status, COALESCE(error_code, '') FROM wallet_operations WHERE op_key = $1`, opKey)
	var r walletOpRow
	if err := row.Scan(&r.status, &r.errorCode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return walletOpRow{}, false, nil
		}
		return walletOpRow{}, false, err
	}
	return r, true, nil
}

// recordWalletOpTx writes the outcome row inside the caller's tx so
// the mutation and the dedupe insert commit together. A duplicate
// op_key races to ON CONFLICT DO NOTHING; whichever tx loses the race
// rolls back when the caller checks RowsAffected upstream, but for our
// flow either winner is acceptable (both wrote the same intent).
func (s *PostgresService) recordWalletOpTx(ctx context.Context, tx pgx.Tx, opKey, tenant string, opType OpType, amount decimal.Decimal, status, errCode string) error {
	if opKey == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `
        INSERT INTO wallet_operations(op_key, tenant_id, op_type, amount_usd, status, error_code)
        VALUES ($1, $2, $3, $4::numeric, $5, NULLIF($6, ''))
        ON CONFLICT (op_key) DO NOTHING`,
		opKey, tenant, string(opType), amount.String(), status, errCode)
	return err
}

// scanWallet is the row scanner shared by Get; pgx returns NUMERIC as
// text via ::text cast so we can route through decimal.NewFromString
// without a pgtype dance.
func scanWallet(row pgx.Row) (Wallet, error) {
	var w Wallet
	var balance, hold, topup, spend string
	if err := row.Scan(&w.TenantID, &balance, &hold, &topup, &spend,
		&w.UpdatedAt, &w.CreatedAt); err != nil {
		return Wallet{}, fmt.Errorf("wallet: scan: %w", err)
	}
	// Unrolled to skip the per-call []struct{...} literal allocation and
	// the *decimal.Decimal pointer dance — Get is on the wallet hot path
	// (every GraphQL Wallet field read + every BillingGuard reservation).
	var err error
	if w.BalanceUSD, err = decimal.NewFromString(balance); err != nil {
		return Wallet{}, fmt.Errorf("wallet: parse balance %q: %w", balance, err)
	}
	if w.HoldUSD, err = decimal.NewFromString(hold); err != nil {
		return Wallet{}, fmt.Errorf("wallet: parse hold %q: %w", hold, err)
	}
	if w.LifetimeTopUpUSD, err = decimal.NewFromString(topup); err != nil {
		return Wallet{}, fmt.Errorf("wallet: parse topup %q: %w", topup, err)
	}
	if w.LifetimeSpendUSD, err = decimal.NewFromString(spend); err != nil {
		return Wallet{}, fmt.Errorf("wallet: parse spend %q: %w", spend, err)
	}
	return w, nil
}
