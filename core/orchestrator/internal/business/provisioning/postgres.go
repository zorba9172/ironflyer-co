package provisioning

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// PostgresService is the production-grade Service backed by Postgres.
// Schema: migrations/00047_provisioning.sql. Owner isolation is
// enforced inline on every read/write via the tenant_id column —
// rows owned by other tenants surface as ErrResourceNotFound so the
// API never leaks existence to the wrong tenant.
type PostgresService struct {
	pool *pgxpool.Pool
}

// NewPostgresService wires the provisioning service to an existing
// pgxpool. The migration MUST have been applied before the first call.
func NewPostgresService(pool *pgxpool.Pool) *PostgresService {
	return &PostgresService{pool: pool}
}

// Provision implements Service. Idempotent on (tenant_id, external_id)
// via the UNIQUE constraint in the migration — a duplicate insert
// returns the existing row.
func (s *PostgresService) Provision(ctx context.Context, r ProvisionedResource) (ProvisionedResource, error) {
	if r.TenantID == "" || r.ProjectID == "" || r.Kind == "" {
		return ProvisionedResource{}, ErrUnknownKind
	}
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.Status == "" {
		r.Status = StatusPending
	}
	// ON CONFLICT (tenant_id, external_id) folds a re-onboarding onto
	// the existing row so the same Stripe AccountLinks flow can be
	// re-triggered without creating a duplicate ProvisionedResource.
	row := s.pool.QueryRow(ctx, `
        INSERT INTO provisioned_resources(id, tenant_id, project_id, kind, external_id, status)
        VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
        ON CONFLICT (tenant_id, external_id) DO UPDATE
          SET updated_at = now()
        RETURNING id, tenant_id, project_id, kind, COALESCE(external_id, ''),
                  status, created_at, updated_at`,
		r.ID, r.TenantID, r.ProjectID, r.Kind, r.ExternalID, r.Status)
	out, err := scanResource(row)
	if err != nil {
		return ProvisionedResource{}, fmt.Errorf("provisioning: insert: %w", err)
	}
	publishProvisioned(ctx, out)
	return out, nil
}

// Get implements Service.
func (s *PostgresService) Get(ctx context.Context, tenant, id string) (ProvisionedResource, error) {
	row := s.pool.QueryRow(ctx, `
        SELECT id, tenant_id, project_id, kind, COALESCE(external_id, ''),
               status, created_at, updated_at
        FROM provisioned_resources
        WHERE id = $1 AND tenant_id = $2`, id, tenant)
	r, err := scanResource(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProvisionedResource{}, ErrResourceNotFound
		}
		return ProvisionedResource{}, fmt.Errorf("provisioning: get: %w", err)
	}
	return r, nil
}

// List implements Service.
func (s *PostgresService) List(ctx context.Context, tenant, project string) ([]ProvisionedResource, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT id, tenant_id, project_id, kind, COALESCE(external_id, ''),
               status, created_at, updated_at
        FROM provisioned_resources
        WHERE tenant_id = $1 AND project_id = $2
        ORDER BY created_at DESC`, tenant, project)
	if err != nil {
		return nil, fmt.Errorf("provisioning: list: %w", err)
	}
	defer rows.Close()
	out := []ProvisionedResource{}
	for rows.Next() {
		r, err := scanResource(rows)
		if err != nil {
			return nil, fmt.Errorf("provisioning: scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateStatus implements Service. The WHERE tenant_id filter doubles
// as the owner-check: a wrong-tenant request returns ErrResourceNotFound
// (rendered identically to a not-found by the resolver).
func (s *PostgresService) UpdateStatus(ctx context.Context, tenant, id, status string) (ProvisionedResource, error) {
	row := s.pool.QueryRow(ctx, `
        UPDATE provisioned_resources
           SET status = $3, updated_at = now()
         WHERE id = $1 AND tenant_id = $2
         RETURNING id, tenant_id, project_id, kind, COALESCE(external_id, ''),
                   status, created_at, updated_at`,
		id, tenant, status)
	r, err := scanResource(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProvisionedResource{}, ErrResourceNotFound
		}
		return ProvisionedResource{}, fmt.Errorf("provisioning: update status: %w", err)
	}
	publishStatusChange(ctx, r)
	return r, nil
}

// RecordRevenue implements Service. The UNIQUE (resource_id,
// external_ref) index makes redelivery a no-op — we surface
// ErrDuplicateEvent so the caller can short-circuit telemetry without
// erroring the webhook.
func (s *PostgresService) RecordRevenue(ctx context.Context, e RevenueEvent) (RevenueEvent, error) {
	if !e.GrossAmountUSD.IsPositive() {
		return RevenueEvent{}, ErrInvalidAmount
	}
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	// Fetch the resource row to validate existence + capture the
	// tenant/kind for the OutcomeEvent emission. A missing resource is
	// a hard error — we never want orphan revenue rows.
	var resource ProvisionedResource
	row := s.pool.QueryRow(ctx, `
        SELECT id, tenant_id, project_id, kind, COALESCE(external_id, ''),
               status, created_at, updated_at
        FROM provisioned_resources WHERE id = $1`, e.ResourceID)
	resource, err := scanResource(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RevenueEvent{}, ErrResourceNotFound
		}
		return RevenueEvent{}, fmt.Errorf("provisioning: revenue resource lookup: %w", err)
	}
	inserted := s.pool.QueryRow(ctx, `
        INSERT INTO revenue_events(id, resource_id, occurred_at,
                                   gross_amount_usd, ironflyer_cut_usd,
                                   external_ref, ledger_entry_id)
        VALUES ($1, $2, $3, $4::numeric, $5::numeric,
                NULLIF($6, ''), NULLIF($7, ''))
        ON CONFLICT (resource_id, external_ref) DO NOTHING
        RETURNING id, resource_id, occurred_at,
                  gross_amount_usd::text, ironflyer_cut_usd::text,
                  COALESCE(external_ref, ''), COALESCE(ledger_entry_id, '')`,
		e.ID, e.ResourceID, e.OccurredAt,
		e.GrossAmountUSD.String(), e.IronflyerCutUSD.String(),
		e.ExternalRef, e.LedgerEntryID)
	stored, err := scanRevenue(inserted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RevenueEvent{}, ErrDuplicateEvent
		}
		return RevenueEvent{}, fmt.Errorf("provisioning: insert revenue: %w", err)
	}
	publishRevenue(ctx, resource, stored)
	return stored, nil
}

// ListRevenue implements Service. The JOIN onto provisioned_resources
// enforces owner-isolation in one shot — a wrong-tenant resourceID
// returns ErrResourceNotFound after the empty rowset.
func (s *PostgresService) ListRevenue(ctx context.Context, tenant, resourceID string, limit int) ([]RevenueEvent, error) {
	if _, err := s.Get(ctx, tenant, resourceID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
        SELECT id, resource_id, occurred_at,
               gross_amount_usd::text, ironflyer_cut_usd::text,
               COALESCE(external_ref, ''), COALESCE(ledger_entry_id, '')
        FROM revenue_events
        WHERE resource_id = $1
        ORDER BY occurred_at DESC
        LIMIT $2`, resourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("provisioning: list revenue: %w", err)
	}
	defer rows.Close()
	out := []RevenueEvent{}
	for rows.Next() {
		e, err := scanRevenue(rows)
		if err != nil {
			return nil, fmt.Errorf("provisioning: scan revenue: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SumRevenue implements Service.
func (s *PostgresService) SumRevenue(ctx context.Context, tenant, resourceID string) (CutTotals, error) {
	if _, err := s.Get(ctx, tenant, resourceID); err != nil {
		return CutTotals{}, err
	}
	var gross, cut string
	var count int
	var firstAt, lastAt *time.Time
	err := s.pool.QueryRow(ctx, `
        SELECT COALESCE(SUM(gross_amount_usd), 0)::text,
               COALESCE(SUM(ironflyer_cut_usd), 0)::text,
               COUNT(*)::int,
               MIN(occurred_at),
               MAX(occurred_at)
        FROM revenue_events WHERE resource_id = $1`, resourceID).
		Scan(&gross, &cut, &count, &firstAt, &lastAt)
	if err != nil {
		return CutTotals{}, fmt.Errorf("provisioning: sum revenue: %w", err)
	}
	g, err := decimal.NewFromString(gross)
	if err != nil {
		return CutTotals{}, fmt.Errorf("provisioning: parse gross: %w", err)
	}
	c, err := decimal.NewFromString(cut)
	if err != nil {
		return CutTotals{}, fmt.Errorf("provisioning: parse cut: %w", err)
	}
	return CutTotals{
		GrossUSD:     g,
		CutUSD:       c,
		EventCount:   count,
		FirstEventAt: firstAt,
		LastEventAt:  lastAt,
	}, nil
}

// scanResource is the shared row scanner. The ::text casts on numeric
// columns are not needed here (no numerics on this table), but the
// shape mirrors wallet/postgres.go scanWallet so future column adds
// stay consistent.
func scanResource(row pgx.Row) (ProvisionedResource, error) {
	var r ProvisionedResource
	err := row.Scan(&r.ID, &r.TenantID, &r.ProjectID, &r.Kind, &r.ExternalID,
		&r.Status, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

// scanRevenue is the shared RevenueEvent scanner. NUMERIC columns
// come back as text so decimal.NewFromString can parse without a
// pgtype intermediate.
func scanRevenue(row pgx.Row) (RevenueEvent, error) {
	var e RevenueEvent
	var gross, cut string
	if err := row.Scan(&e.ID, &e.ResourceID, &e.OccurredAt,
		&gross, &cut, &e.ExternalRef, &e.LedgerEntryID); err != nil {
		return RevenueEvent{}, err
	}
	g, err := decimal.NewFromString(gross)
	if err != nil {
		return RevenueEvent{}, fmt.Errorf("parse gross: %w", err)
	}
	c, err := decimal.NewFromString(cut)
	if err != nil {
		return RevenueEvent{}, fmt.Errorf("parse cut: %w", err)
	}
	e.GrossAmountUSD = g
	e.IronflyerCutUSD = c
	return e, nil
}

// recordProvisioningOp is the Postgres-side idempotency primitive.
// Mirrors recordWalletOpTx in wallet/postgres.go — wireup calls this
// inside the same tx as the mutation so a redelivered webhook does not
// double-record a RevenueEvent under a different op_key. Returns true
// when the op was newly recorded (caller proceeds with the mutation)
// or false when a prior row already exists (caller short-circuits).
func (s *PostgresService) recordProvisioningOp(ctx context.Context, tx pgx.Tx, opKey string, opType OpType, ref string) (bool, error) {
	if opKey == "" {
		return true, nil
	}
	ct, err := tx.Exec(ctx, `
        INSERT INTO provisioning_operations(op_key, op_type, external_ref)
        VALUES ($1, $2, NULLIF($3, ''))
        ON CONFLICT (op_key) DO NOTHING`,
		opKey, string(opType), ref)
	if err != nil {
		return false, fmt.Errorf("provisioning: record op: %w", err)
	}
	return ct.RowsAffected() == 1, nil
}
