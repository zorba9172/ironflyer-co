package shippass

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/ai/learning"
	"ironflyer/core/orchestrator/internal/business/wallet"
)

// PostgresService is the persistent backend. Schema lives in
// migrations/00047_shippass.sql.
//
// Atomicity guarantees:
//   - Purchase runs (uniqueness check, wallet hold, pass insert) in a
//     single tx so a concurrent buy on the same project loses on the
//     partial unique index instead of stacking two holds.
//   - RecordGateVerdict runs (progress insert, latest-per-gate scan,
//     debit, pass update) in a single tx so a flapping verdict cannot
//     race the settlement.
//   - ExpireDue processes one pass per tx so a single tenant's wallet
//     error does not roll back the rest of the sweep.
//
// All wallet mutations route through wallet.IdempotentService with
// stable op keys derived from the pass id; retries land once.
type PostgresService struct {
	pool   *pgxpool.Pool
	wallet wallet.IdempotentService
	log    zerolog.Logger
}

// NewPostgresService wires the backend to an existing pgx pool and
// the live wallet service. The pool is expected to point at the
// orchestrator's primary database (migrations/00047_shippass.sql
// runs there).
func NewPostgresService(pool *pgxpool.Pool, walletSvc wallet.IdempotentService, log zerolog.Logger) *PostgresService {
	return &PostgresService{pool: pool, wallet: walletSvc, log: log}
}

// Quote — preview only; identical SQL to memory but materialised
// against wallet.Get rather than a snapshot.
func (s *PostgresService) Quote(ctx context.Context, tenant, _ string, tierKey string) (Quote, error) {
	tier, ok := TierByKey(tierKey)
	if !ok {
		return Quote{}, ErrInvalidTier
	}
	w, err := s.wallet.Get(ctx, tenant)
	if err != nil {
		return Quote{}, err
	}
	shortfall := tier.PriceUSD.Sub(w.AvailableUSD())
	if shortfall.IsNegative() {
		shortfall = decimal.Zero
	}
	return Quote{
		TierKey:         tier.Key,
		PriceUSD:        tier.PriceUSD,
		RequiredGates:   tier.sortedGates(),
		DeadlineDays:    tier.DeadlineDays,
		WalletShortfall: shortfall,
	}, nil
}

const sqlInsertPass = `
INSERT INTO ship_passes
    (id, tenant_id, project_id, tier_key, price_usd, status, deadline_at,
     created_at, updated_at, hold_op_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9)
RETURNING id`

const sqlCheckActiveForProject = `
SELECT id FROM ship_passes
WHERE tenant_id = $1 AND project_id = $2 AND status = 'active'
LIMIT 1`

const sqlSelectPass = `
SELECT id, tenant_id, project_id, tier_key, price_usd, status,
       deadline_at, created_at, updated_at, settled_at,
       hold_op_key, COALESCE(debit_op_key, ''), COALESCE(refund_op_key, '')
FROM ship_passes
WHERE id = $1 AND tenant_id = $2`

const sqlSelectActiveForProject = `
SELECT id, tenant_id, project_id, tier_key, price_usd, status,
       deadline_at, created_at, updated_at, settled_at,
       hold_op_key, COALESCE(debit_op_key, ''), COALESCE(refund_op_key, '')
FROM ship_passes
WHERE tenant_id = $1 AND project_id = $2 AND status = 'active'
LIMIT 1`

const sqlListByTenant = `
SELECT id, tenant_id, project_id, tier_key, price_usd, status,
       deadline_at, created_at, updated_at, settled_at,
       hold_op_key, COALESCE(debit_op_key, ''), COALESCE(refund_op_key, '')
FROM ship_passes
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2`

const sqlInsertProgress = `
INSERT INTO ship_pass_progress (id, ship_pass_id, gate, passed, reason, observed_at)
VALUES ($1, $2, $3, $4, $5, $6)`

const sqlSelectLatestProgress = `
SELECT DISTINCT ON (gate) gate, passed
FROM ship_pass_progress
WHERE ship_pass_id = $1
ORDER BY gate, observed_at DESC`

const sqlSelectProgressForPass = `
SELECT id, ship_pass_id, gate, passed, COALESCE(reason, ''), observed_at
FROM ship_pass_progress
WHERE ship_pass_id = $1
ORDER BY observed_at ASC`

const sqlMarkShipped = `
UPDATE ship_passes
SET status = 'shipped', updated_at = $2, settled_at = $2, debit_op_key = $3
WHERE id = $1 AND status = 'active'
RETURNING tenant_id, project_id, tier_key, price_usd, deadline_at, created_at, hold_op_key`

const sqlMarkRefunded = `
UPDATE ship_passes
SET status = 'refunded', updated_at = $2, settled_at = $2, refund_op_key = $3
WHERE id = $1 AND status = 'active'
RETURNING id`

const sqlMarkCancelled = `
UPDATE ship_passes
SET status = 'cancelled', updated_at = $2, settled_at = $2, refund_op_key = $3
WHERE id = $1 AND tenant_id = $4 AND status = 'active'
RETURNING id`

const sqlSelectActiveDue = `
SELECT id, tenant_id, price_usd
FROM ship_passes
WHERE status = 'active' AND deadline_at <= $1
ORDER BY deadline_at ASC
LIMIT 200`

const sqlLifetimeStats = `
SELECT status, COUNT(*) AS n, COALESCE(SUM(price_usd) FILTER (WHERE status = 'shipped'), 0) AS revenue
FROM ship_passes
WHERE tenant_id = $1
GROUP BY status`

// Purchase opens a tx, ensures no active pass exists, places the
// wallet hold (via IdempotentService — wallet operates against its
// own connection pool so this is two coordinated stores, not one),
// and inserts the row.
//
// The wallet hold is intentionally placed BEFORE the pass insert
// even though they share a request id. The hold is idempotent on
// op_key; if the pass insert fails for any reason (uniqueness
// race, transient db error) a retry sees the same op_key and the
// wallet treats the second call as a no-op, so the user is never
// double-held.
func (s *PostgresService) Purchase(ctx context.Context, tenant, projectID, tierKey, requestID string) (ShipPass, error) {
	tier, ok := TierByKey(tierKey)
	if !ok {
		return ShipPass{}, ErrInvalidTier
	}
	if requestID == "" {
		requestID = uuid.NewString()
	}
	holdKey := "shippass-hold-" + requestID

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ShipPass{}, err
	}
	defer tx.Rollback(ctx)

	var existing string
	err = tx.QueryRow(ctx, sqlCheckActiveForProject, tenant, projectID).Scan(&existing)
	if err == nil && existing != "" {
		return ShipPass{ID: existing}, ErrPassNotActive
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ShipPass{}, err
	}

	if err := s.wallet.HoldWithKey(ctx, tenant, tier.PriceUSD, holdKey); err != nil {
		return ShipPass{}, err
	}

	now := time.Now().UTC()
	id := uuid.NewString()
	deadline := tier.Deadline(now)
	if _, err := tx.Exec(ctx, sqlInsertPass,
		id, tenant, projectID, tier.Key, tier.PriceUSD, string(StatusActive),
		deadline, now, holdKey,
	); err != nil {
		return ShipPass{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ShipPass{}, err
	}

	row := ShipPass{
		ID:         id,
		TenantID:   tenant,
		ProjectID:  projectID,
		TierKey:    tier.Key,
		PriceUSD:   tier.PriceUSD,
		Status:     StatusActive,
		DeadlineAt: deadline,
		CreatedAt:  now,
		UpdatedAt:  now,
		HoldOpKey:  holdKey,
	}
	publishLifecycle(ctx, row, "purchased", nil)
	return row, nil
}

// Cancel releases the hold and updates the row. Idempotent on the
// passID-derived refund key.
func (s *PostgresService) Cancel(ctx context.Context, tenant, passID string) (ShipPass, error) {
	row, err := s.Get(ctx, tenant, passID)
	if err != nil {
		return ShipPass{}, err
	}
	if row.Status != StatusActive {
		return row, ErrPassNotActive
	}
	refundKey := "shippass-cancel-" + passID
	if err := s.wallet.ReleaseWithKey(ctx, tenant, row.PriceUSD, refundKey); err != nil {
		return row, err
	}
	now := time.Now().UTC()
	var id string
	if err := s.pool.QueryRow(ctx, sqlMarkCancelled, passID, now, refundKey, tenant).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return row, ErrPassNotActive
		}
		return row, err
	}
	row.Status = StatusCancelled
	row.UpdatedAt = now
	row.SettledAt = &now
	row.RefundOpKey = refundKey
	publishLifecycle(ctx, row, "cancelled", nil)
	return row, nil
}

// Get returns the row owned by tenant.
func (s *PostgresService) Get(ctx context.Context, tenant, passID string) (ShipPass, error) {
	row, err := s.scanOne(ctx, sqlSelectPass, passID, tenant)
	if errors.Is(err, pgx.ErrNoRows) {
		return ShipPass{}, ErrPassNotFound
	}
	return row, err
}

// ActiveForProject returns the in-flight pass for a project.
func (s *PostgresService) ActiveForProject(ctx context.Context, tenant, projectID string) (ShipPass, error) {
	row, err := s.scanOne(ctx, sqlSelectActiveForProject, tenant, projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ShipPass{}, ErrPassNotFound
	}
	return row, err
}

// List returns recent passes for the tenant.
func (s *PostgresService) List(ctx context.Context, tenant string, limit int) ([]ShipPass, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := s.pool.Query(ctx, sqlListByTenant, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ShipPass{}
	for rows.Next() {
		var r ShipPass
		var settled *time.Time
		if err := rows.Scan(&r.ID, &r.TenantID, &r.ProjectID, &r.TierKey, &r.PriceUSD,
			&r.Status, &r.DeadlineAt, &r.CreatedAt, &r.UpdatedAt, &settled,
			&r.HoldOpKey, &r.DebitOpKey, &r.RefundOpKey); err != nil {
			return nil, err
		}
		r.SettledAt = settled
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecordGateVerdict appends progress and, when the required set is
// fully covered, settles the pass.
func (s *PostgresService) RecordGateVerdict(ctx context.Context, passID string, gate domain.GateName, passed bool, reason string, observedAt time.Time) (ShipPass, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ShipPass{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, sqlInsertProgress, uuid.NewString(), passID, string(gate), passed, reason, observedAt.UTC()); err != nil {
		return ShipPass{}, err
	}

	// Re-fetch pass under tx so we settle atomically with the latest
	// progress snapshot.
	var (
		tenantID  string
		projectID string
		tierKey   string
		priceUSD  decimal.Decimal
		status    string
		deadline  time.Time
		created   time.Time
		holdKey   string
	)
	const selectInTx = `
SELECT tenant_id, project_id, tier_key, price_usd, status, deadline_at, created_at, hold_op_key
FROM ship_passes WHERE id = $1 FOR UPDATE`
	if err := tx.QueryRow(ctx, selectInTx, passID).Scan(
		&tenantID, &projectID, &tierKey, &priceUSD, &status, &deadline, &created, &holdKey,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ShipPass{}, ErrPassNotFound
		}
		return ShipPass{}, err
	}
	if status != string(StatusActive) {
		_ = tx.Commit(ctx)
		return s.Get(ctx, tenantID, passID)
	}

	tier, ok := TierByKey(tierKey)
	if !ok {
		_ = tx.Commit(ctx)
		return s.Get(ctx, tenantID, passID)
	}
	required := tier.requiredGateSet()
	if _, inScope := required[gate]; !inScope {
		_ = tx.Commit(ctx)
		return s.Get(ctx, tenantID, passID)
	}

	rows, err := tx.Query(ctx, sqlSelectLatestProgress, passID)
	if err != nil {
		return ShipPass{}, err
	}
	latest := map[domain.GateName]bool{}
	for rows.Next() {
		var g string
		var p bool
		if err := rows.Scan(&g, &p); err != nil {
			rows.Close()
			return ShipPass{}, err
		}
		latest[domain.GateName(g)] = p
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return ShipPass{}, err
	}
	for g := range required {
		if !latest[g] {
			_ = tx.Commit(ctx)
			return s.Get(ctx, tenantID, passID)
		}
	}

	debitKey := "shippass-debit-" + passID
	if err := s.wallet.DebitWithKey(ctx, tenantID, priceUSD, debitKey); err != nil {
		return ShipPass{}, err
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, sqlMarkShipped, passID, now, debitKey); err != nil {
		return ShipPass{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ShipPass{}, err
	}
	row := ShipPass{
		ID:         passID,
		TenantID:   tenantID,
		ProjectID:  projectID,
		TierKey:    tierKey,
		PriceUSD:   priceUSD,
		Status:     StatusShipped,
		DeadlineAt: deadline,
		CreatedAt:  created,
		UpdatedAt:  now,
		SettledAt:  &now,
		HoldOpKey:  holdKey,
		DebitOpKey: debitKey,
	}
	publishLifecycle(ctx, row, "shipped", &priceUSD)
	return row, nil
}

// ProgressFor returns the full observation log for the pass.
func (s *PostgresService) ProgressFor(ctx context.Context, tenant, passID string) ([]GateProgress, error) {
	// Owner check via Get.
	if _, err := s.Get(ctx, tenant, passID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, sqlSelectProgressForPass, passID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GateProgress{}
	for rows.Next() {
		var r GateProgress
		var g string
		if err := rows.Scan(&r.ID, &r.ShipPassID, &g, &r.Passed, &r.Reason, &r.ObservedAt); err != nil {
			return nil, err
		}
		r.Gate = domain.GateName(g)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ExpireDue processes one pass per tx so a single tenant's wallet
// error never blocks the rest of the sweep.
func (s *PostgresService) ExpireDue(ctx context.Context, now time.Time) ([]ShipPass, error) {
	rows, err := s.pool.Query(ctx, sqlSelectActiveDue, now)
	if err != nil {
		return nil, err
	}
	type due struct {
		id     string
		tenant string
		price  decimal.Decimal
	}
	due_ := []due{}
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.tenant, &d.price); err != nil {
			rows.Close()
			return nil, err
		}
		due_ = append(due_, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	expired := []ShipPass{}
	for _, d := range due_ {
		refundKey := "shippass-expire-" + d.id
		if err := s.wallet.ReleaseWithKey(ctx, d.tenant, d.price, refundKey); err != nil {
			s.log.Warn().Err(err).Str("pass_id", d.id).Msg("shippass expire: release failed")
			continue
		}
		var rowID string
		ts := now.UTC()
		if err := s.pool.QueryRow(ctx, sqlMarkRefunded, d.id, ts, refundKey).Scan(&rowID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			s.log.Warn().Err(err).Str("pass_id", d.id).Msg("shippass expire: mark refunded failed")
			continue
		}
		full, err := s.scanOne(ctx, sqlSelectPass, d.id, d.tenant)
		if err != nil {
			continue
		}
		expired = append(expired, full)
		publishLifecycle(ctx, full, "refunded", nil)
	}
	return expired, nil
}

// LifetimeStats projects the tenant's history into the dashboard
// counters in one round-trip.
func (s *PostgresService) LifetimeStats(ctx context.Context, tenant string) (LifetimeStats, error) {
	rows, err := s.pool.Query(ctx, sqlLifetimeStats, tenant)
	if err != nil {
		return LifetimeStats{}, err
	}
	defer rows.Close()
	stats := LifetimeStats{RevenueUSD: decimal.Zero}
	for rows.Next() {
		var status string
		var n int
		var revenue decimal.Decimal
		if err := rows.Scan(&status, &n, &revenue); err != nil {
			return LifetimeStats{}, err
		}
		switch Status(status) {
		case StatusActive, StatusShipped, StatusRefunded, StatusCancelled:
			stats.TotalPurchased += n
		}
		switch Status(status) {
		case StatusShipped:
			stats.TotalShipped = n
			stats.RevenueUSD = revenue
		case StatusRefunded:
			stats.TotalRefunded = n
		case StatusCancelled:
			stats.TotalCancelled = n
		}
	}
	return stats, rows.Err()
}

// scanOne reads a ship_passes row by query + args.
func (s *PostgresService) scanOne(ctx context.Context, sql string, args ...any) (ShipPass, error) {
	var r ShipPass
	var settled *time.Time
	err := s.pool.QueryRow(ctx, sql, args...).Scan(
		&r.ID, &r.TenantID, &r.ProjectID, &r.TierKey, &r.PriceUSD,
		&r.Status, &r.DeadlineAt, &r.CreatedAt, &r.UpdatedAt, &settled,
		&r.HoldOpKey, &r.DebitOpKey, &r.RefundOpKey,
	)
	if err != nil {
		return ShipPass{}, err
	}
	r.SettledAt = settled
	return r, nil
}

// ensure unused-import safety in case future edits remove a callsite.
var _ = uuid.NewString
var _ = learning.Publish
