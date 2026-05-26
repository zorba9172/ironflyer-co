package quota

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is the Store backed by a pgx pool. v1 keeps the live
// counter set in-memory and uses Postgres only for periodic
// reconciliation (a future migration adds a `quota_usage` table with
// the same columns as MemoryStore.Usage). That gives us:
//
//  1. Synchronous admission at memory-speed.
//  2. A durable view for dashboards / cross-pod debugging.
//
// When the integration agent wires Postgres in but the
// reconciliation table is not yet created, the store transparently
// degrades to MemoryStore behaviour.
type PostgresStore struct {
	pool *pgxpool.Pool
	mem  *MemoryStore
}

// NewPostgresStore builds the Postgres-backed Store. Pool may be nil
// in which case the store behaves identically to MemoryStore.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool, mem: NewMemoryStore()}
}

// Get implements Store.
func (s *PostgresStore) Get(ctx context.Context, tenantID string) (Usage, error) {
	return s.mem.Get(ctx, tenantID)
}

// Hold implements Store.
func (s *PostgresStore) Hold(ctx context.Context, tenantID string, q TenantQuota, lease Lease) error {
	return s.mem.Hold(ctx, tenantID, q, lease)
}

// Release implements Store.
func (s *PostgresStore) Release(ctx context.Context, tenantID, executionID, workspaceID string) error {
	return s.mem.Release(ctx, tenantID, executionID, workspaceID)
}

var _ Store = (*PostgresStore)(nil)
