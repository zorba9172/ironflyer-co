// Package memory — owner-scoped federation.
//
// Federation lets the SAME user's projects share execution memory: the
// Critic / Coder / Planner running against Project B can read records
// (decisions, failure→fix lineage, patterns) that were captured against
// Project A — but ONLY when the user has opted both projects into the
// federation pool, and ONLY for memories that user owns.
//
// Federation is NEVER cross-user. Every read path that consults the
// federation set re-verifies record.UserID == caller.UserID before the
// record is surfaced to an agent or HTTP caller. This file owns the
// membership table; the per-record owner check lives in
// memory.Query.IncludeFederated (see memory.go + surreal.go).

package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FederationMember is one row in a user's federation set. A project is
// in the set when its owner has explicitly opted it in.
type FederationMember struct {
	UserID    string    `json:"userId"`
	ProjectID string    `json:"projectId"`
	JoinedAt  time.Time `json:"joinedAt"`
	// Role is a free-form label the dashboard uses to colour the
	// project's contribution: "source", "consumer", or empty for the
	// default bidirectional behaviour.
	Role string `json:"role,omitempty"`
}

// FederationStore tracks per-user federation membership. Implementations
// MUST enforce that Add / Remove only ever mutate rows for the supplied
// UserID — never accept a payload that names another user.
type FederationStore interface {
	// List returns the projectIDs the given user has opted in.
	List(ctx context.Context, userID string) ([]FederationMember, error)
	// Add inserts (or upserts) a (userID, projectID) row.
	Add(ctx context.Context, userID, projectID, role string) (FederationMember, error)
	// Remove drops a (userID, projectID) row. Idempotent — removing an
	// unknown row returns nil so the HTTP layer can map DELETE to 204
	// unconditionally.
	Remove(ctx context.Context, userID, projectID string) error
	// IsMember is the fast-path predicate used by the read pipeline.
	IsMember(ctx context.Context, userID, projectID string) (bool, error)
}

// ErrFederationOwnerMismatch is returned by FederationStore implementations
// when a caller tries to mutate another user's federation set. The HTTP
// layer should not see this error in practice (it always pins userID to
// the authenticated user before calling) but the contract is enforced
// defensively.
var ErrFederationOwnerMismatch = errors.New("federation: owner mismatch")

// ---------------------------------------------------------------------------
// In-memory implementation — used in dev + when Postgres isn't configured.
// ---------------------------------------------------------------------------

// MemoryFederationStore is a process-local FederationStore.
type MemoryFederationStore struct {
	mu   sync.RWMutex
	rows map[string]map[string]FederationMember // userID -> projectID -> member
}

// NewMemoryFederationStore returns a fresh in-process store.
func NewMemoryFederationStore() *MemoryFederationStore {
	return &MemoryFederationStore{rows: map[string]map[string]FederationMember{}}
}

func (m *MemoryFederationStore) List(_ context.Context, userID string) ([]FederationMember, error) {
	if userID == "" {
		return nil, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	bucket := m.rows[userID]
	out := make([]FederationMember, 0, len(bucket))
	for _, m := range bucket {
		out = append(out, m)
	}
	return out, nil
}

func (m *MemoryFederationStore) Add(_ context.Context, userID, projectID, role string) (FederationMember, error) {
	if userID == "" || projectID == "" {
		return FederationMember{}, errors.New("userID and projectID required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rows[userID] == nil {
		m.rows[userID] = map[string]FederationMember{}
	}
	mem := FederationMember{
		UserID:    userID,
		ProjectID: projectID,
		Role:      role,
		JoinedAt:  time.Now().UTC(),
	}
	m.rows[userID][projectID] = mem
	return mem, nil
}

func (m *MemoryFederationStore) Remove(_ context.Context, userID, projectID string) error {
	if userID == "" || projectID == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if bucket := m.rows[userID]; bucket != nil {
		delete(bucket, projectID)
	}
	return nil
}

func (m *MemoryFederationStore) IsMember(_ context.Context, userID, projectID string) (bool, error) {
	if userID == "" || projectID == "" {
		return false, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	bucket := m.rows[userID]
	if bucket == nil {
		return false, nil
	}
	_, ok := bucket[projectID]
	return ok, nil
}

// ProjectIDsFor is a convenience wrapper that returns the federation set
// as a string slice — handy when populating memory.Query.FederatedProjectIDs.
func ProjectIDsFor(ctx context.Context, fs FederationStore, userID string) ([]string, error) {
	if fs == nil || userID == "" {
		return nil, nil
	}
	members, err := fs.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(members))
	for _, m := range members {
		out = append(out, m.ProjectID)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Postgres implementation.
// ---------------------------------------------------------------------------

// PostgresFederationBootstrap is the idempotent DDL. Mirrors the pattern
// used by other Postgres-backed subsystems (affiliates, domains, …).
const PostgresFederationBootstrap = `
CREATE TABLE IF NOT EXISTS memory_federation (
    user_id    TEXT        NOT NULL,
    project_id TEXT        NOT NULL,
    role       TEXT        NOT NULL DEFAULT '',
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, project_id)
);
CREATE INDEX IF NOT EXISTS idx_memory_federation_user ON memory_federation(user_id);
`

// BootstrapFederationPostgres installs the memory_federation table.
// Idempotent — safe on every boot.
//
// Deprecated: schema lives in core/orchestrator/migrations/00006_init_memory_federation.sql.
// Add follow-up schema as a new numbered goose migration.
func BootstrapFederationPostgres(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return nil
	}
	_, err := pool.Exec(ctx, PostgresFederationBootstrap)
	return err
}

// PostgresFederationStore persists federation membership in Postgres.
type PostgresFederationStore struct {
	pool *pgxpool.Pool
}

// NewPostgresFederationStore wraps a pgx pool.
func NewPostgresFederationStore(pool *pgxpool.Pool) *PostgresFederationStore {
	return &PostgresFederationStore{pool: pool}
}

func (p *PostgresFederationStore) List(ctx context.Context, userID string) ([]FederationMember, error) {
	if p == nil || p.pool == nil || userID == "" {
		return nil, nil
	}
	rows, err := p.pool.Query(ctx,
		`SELECT user_id, project_id, role, joined_at
		 FROM memory_federation
		 WHERE user_id = $1
		 ORDER BY joined_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FederationMember{}
	for rows.Next() {
		var m FederationMember
		if err := rows.Scan(&m.UserID, &m.ProjectID, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (p *PostgresFederationStore) Add(ctx context.Context, userID, projectID, role string) (FederationMember, error) {
	if p == nil || p.pool == nil {
		return FederationMember{}, errors.New("postgres pool not configured")
	}
	if userID == "" || projectID == "" {
		return FederationMember{}, errors.New("userID and projectID required")
	}
	now := time.Now().UTC()
	_, err := p.pool.Exec(ctx, `
		INSERT INTO memory_federation(user_id, project_id, role, joined_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, project_id) DO UPDATE SET role = EXCLUDED.role`,
		userID, projectID, role, now)
	if err != nil {
		return FederationMember{}, err
	}
	return FederationMember{
		UserID: userID, ProjectID: projectID, Role: role, JoinedAt: now,
	}, nil
}

func (p *PostgresFederationStore) Remove(ctx context.Context, userID, projectID string) error {
	if p == nil || p.pool == nil || userID == "" || projectID == "" {
		return nil
	}
	_, err := p.pool.Exec(ctx,
		`DELETE FROM memory_federation WHERE user_id = $1 AND project_id = $2`,
		userID, projectID)
	return err
}

func (p *PostgresFederationStore) IsMember(ctx context.Context, userID, projectID string) (bool, error) {
	if p == nil || p.pool == nil || userID == "" || projectID == "" {
		return false, nil
	}
	var exists bool
	err := p.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM memory_federation WHERE user_id = $1 AND project_id = $2)`,
		userID, projectID).Scan(&exists)
	return exists, err
}

// compile-time interface checks.
var (
	_ FederationStore = (*MemoryFederationStore)(nil)
	_ FederationStore = (*PostgresFederationStore)(nil)
)
