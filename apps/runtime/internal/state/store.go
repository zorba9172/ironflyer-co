// Package state owns the workspace metadata layer for the portable
// runtime. The previous `workspaces.Store` package modeled a single-pod
// EFS-backed registry with archive-on-idle semantics. Portability flips
// the model: every workspace has an explicit *current pod* and a fresh
// *heartbeat*, and any pod can atomically claim a workspace whose
// heartbeat has lapsed.
//
// Three layers participate in workspace state:
//
//  1. Metadata layer (this package) — Postgres row keyed by workspace
//     ID. Source of truth for ownership and lifecycle.
//  2. Content layer (apps/runtime/internal/snapshot) — workspace
//     filesystem as a gzip tarball in S3. Pulled into a local working
//     directory on Start, pushed back on checkpoint / Stop.
//  3. Live layer — the running Docker container itself. Always local to
//     whichever pod currently owns the workspace.
//
// The Store interface has two backends:
//
//   - MemoryStore for dev / single-pod installs. Claim semantics still
//     enforced in-process so callers don't need conditional code paths.
//   - PostgresStore for production. The interesting verb is Claim — an
//     UPDATE ... WHERE current_pod_id = '' that returns the row on
//     success and nothing on contention. Reclaim is the same pattern,
//     gated on stale heartbeat instead of empty owner.
package state

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Status enumerates the lifecycle states a workspace can be in. Driving
// the state machine through the metadata row (rather than each pod's
// in-memory map) is what lets a different pod take over after a crash.
type Status string

const (
	StatusCreated Status = "created"
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusErrored Status = "errored"
)

// Record is one workspace row. Field names mirror the schema in
// apps/orchestrator/migrations/00016_workspaces_state.sql.
type Record struct {
	ID              string     `json:"id"`
	OwnerID         string     `json:"ownerId"`
	ProjectID       string     `json:"projectId,omitempty"`
	Driver          string     `json:"driver"`
	Image           string     `json:"image,omitempty"`
	Status          Status     `json:"status"`
	CurrentPodID    string     `json:"currentPodId,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	LastHeartbeatAt *time.Time `json:"lastHeartbeatAt,omitempty"`
}

// ErrNotFound is returned by Get + Delete when the workspace row doesn't
// exist. Callers map this to 404.
var ErrNotFound = errors.New("workspace not found")

// ErrNotClaimable is returned by Claim when the row exists but a
// different pod already owns it (and the heartbeat hasn't gone stale).
var ErrNotClaimable = errors.New("workspace already claimed")

// Store is the metadata contract.
type Store interface {
	// Create inserts a brand-new workspace row in StatusCreated with no
	// current pod. Idempotent on the ID: calling Create twice with the
	// same ID returns ErrAlreadyExists.
	Create(ctx context.Context, rec Record) error

	// Get loads a row by ID.
	Get(ctx context.Context, id string) (Record, error)

	// List returns workspaces owned by ownerID, newest first. Empty
	// ownerID returns every row (admin use only).
	List(ctx context.Context, ownerID string) ([]Record, error)

	// Claim atomically transitions the workspace to be owned by podID.
	// Succeeds when current_pod_id = '' AND status IN ('created',
	// 'stopped'). On success the row is returned with CurrentPodID =
	// podID, Status = running, LastHeartbeatAt = now.
	Claim(ctx context.Context, id, podID string) (Record, error)

	// Heartbeat refreshes LastHeartbeatAt only when podID still owns the
	// row. Returns true when the heartbeat landed.
	Heartbeat(ctx context.Context, id, podID string) (bool, error)

	// Release relinquishes ownership and sets status. Only succeeds when
	// podID still owns the row.
	Release(ctx context.Context, id, podID string, status Status) error

	// Reap reclaims any workspace whose LastHeartbeatAt is older than
	// staleAfter, in a single atomic UPDATE per row. Returns the list of
	// IDs that were freed. Run as a background goroutine on every pod —
	// the first pod to win the UPDATE owns the reclaim.
	Reap(ctx context.Context, staleAfter time.Duration) ([]string, error)

	// OwnedBy returns every workspace whose current_pod_id matches
	// podID. Used by the heartbeat loop and graceful drain.
	OwnedBy(ctx context.Context, podID string) ([]Record, error)

	// SetStatus updates status without touching ownership. Used to mark
	// errored / mid-checkpoint states.
	SetStatus(ctx context.Context, id string, status Status) error

	// Delete removes the row outright. Caller owns deleting the S3
	// prefix before invoking this.
	Delete(ctx context.Context, id string) error
}

// ErrAlreadyExists is returned by Create when the ID is taken.
var ErrAlreadyExists = errors.New("workspace already exists")

// nowUTC is the single time source. Tests would override; this codebase
// has the "no tests" rule, so it's just here for readability.
func nowUTC() time.Time { return time.Now().UTC() }

// ---------------- MemoryStore ------------------------------------------

// MemoryStore is the in-process implementation. It still enforces
// ownership semantics (claim, heartbeat, reap) so single-pod installs
// exercise the same code paths.
type MemoryStore struct {
	mu   sync.Mutex
	rows map[string]Record
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{rows: make(map[string]Record)}
}

func (m *MemoryStore) Create(_ context.Context, rec Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rows[rec.ID]; ok {
		return ErrAlreadyExists
	}
	now := nowUTC()
	rec.CreatedAt = now
	rec.UpdatedAt = now
	if rec.Status == "" {
		rec.Status = StatusCreated
	}
	m.rows[rec.ID] = rec
	return nil
}

func (m *MemoryStore) Get(_ context.Context, id string) (Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	return r, nil
}

func (m *MemoryStore) List(_ context.Context, ownerID string) ([]Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Record, 0, len(m.rows))
	for _, r := range m.rows {
		if ownerID != "" && r.OwnerID != ownerID {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (m *MemoryStore) Claim(_ context.Context, id, podID string) (Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	if r.CurrentPodID != "" && r.CurrentPodID != podID {
		return Record{}, ErrNotClaimable
	}
	if r.Status != StatusCreated && r.Status != StatusStopped && r.Status != StatusRunning {
		return Record{}, ErrNotClaimable
	}
	now := nowUTC()
	r.CurrentPodID = podID
	r.Status = StatusRunning
	r.LastHeartbeatAt = &now
	r.UpdatedAt = now
	m.rows[id] = r
	return r, nil
}

func (m *MemoryStore) Heartbeat(_ context.Context, id, podID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return false, ErrNotFound
	}
	if r.CurrentPodID != podID {
		return false, nil
	}
	now := nowUTC()
	r.LastHeartbeatAt = &now
	r.UpdatedAt = now
	m.rows[id] = r
	return true, nil
}

func (m *MemoryStore) Release(_ context.Context, id, podID string, status Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return ErrNotFound
	}
	if r.CurrentPodID != podID {
		return nil // already released or owned by someone else
	}
	r.CurrentPodID = ""
	r.Status = status
	r.UpdatedAt = nowUTC()
	r.LastHeartbeatAt = nil
	m.rows[id] = r
	return nil
}

func (m *MemoryStore) Reap(_ context.Context, staleAfter time.Duration) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := nowUTC().Add(-staleAfter)
	var freed []string
	for id, r := range m.rows {
		if r.CurrentPodID == "" {
			continue
		}
		if r.LastHeartbeatAt == nil || r.LastHeartbeatAt.Before(cutoff) {
			r.CurrentPodID = ""
			r.Status = StatusStopped
			r.UpdatedAt = nowUTC()
			r.LastHeartbeatAt = nil
			m.rows[id] = r
			freed = append(freed, id)
		}
	}
	return freed, nil
}

func (m *MemoryStore) OwnedBy(_ context.Context, podID string) ([]Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Record
	for _, r := range m.rows {
		if r.CurrentPodID == podID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *MemoryStore) SetStatus(_ context.Context, id string, status Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return ErrNotFound
	}
	r.Status = status
	r.UpdatedAt = nowUTC()
	m.rows[id] = r
	return nil
}

func (m *MemoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rows[id]; !ok {
		return ErrNotFound
	}
	delete(m.rows, id)
	return nil
}

// ---------------- PostgresStore ----------------------------------------

// PostgresStore is the production implementation. Schema is provisioned
// by the orchestrator's goose migrations (00016_workspaces_state.sql) —
// the runtime doesn't own the schema, it just consumes the table.
//
// BootstrapPostgres can be called at startup to apply a thin idempotent
// CREATE-IF-NOT-EXISTS fallback in case the runtime boots before the
// orchestrator has finished migrating. It mirrors the orchestrator
// migration, but never destroys data.
type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

const stateColumns = `
    id,
    owner_id,
    COALESCE(project_id,''),
    driver,
    COALESCE(image,''),
    status,
    COALESCE(current_pod_id,''),
    created_at,
    updated_at,
    last_heartbeat_at
`

func scanRecord(row pgx.Row) (Record, error) {
	var r Record
	var status string
	if err := row.Scan(&r.ID, &r.OwnerID, &r.ProjectID, &r.Driver, &r.Image,
		&status, &r.CurrentPodID, &r.CreatedAt, &r.UpdatedAt, &r.LastHeartbeatAt); err != nil {
		return Record{}, err
	}
	r.Status = Status(status)
	return r, nil
}

// BootstrapPostgres ensures the workspaces table exists. Safe to call
// from every pod on every startup. Mirrors the canonical orchestrator
// migration; never drops or alters existing columns.
func BootstrapPostgres(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS workspaces_state (
            id                TEXT        PRIMARY KEY,
            owner_id          TEXT        NOT NULL,
            project_id        TEXT        NULL,
            driver            TEXT        NOT NULL,
            image             TEXT        NULL,
            status            TEXT        NOT NULL,
            current_pod_id    TEXT        NULL,
            created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
            last_heartbeat_at TIMESTAMPTZ NULL
        );
        CREATE INDEX IF NOT EXISTS idx_workspaces_state_owner ON workspaces_state(owner_id);
        CREATE INDEX IF NOT EXISTS idx_workspaces_state_pod ON workspaces_state(current_pod_id);
        CREATE INDEX IF NOT EXISTS idx_workspaces_state_heartbeat ON workspaces_state(last_heartbeat_at);
    `)
	return err
}

func (p *PostgresStore) Create(ctx context.Context, rec Record) error {
	if rec.Status == "" {
		rec.Status = StatusCreated
	}
	_, err := p.pool.Exec(ctx, `
        INSERT INTO workspaces_state (id, owner_id, project_id, driver, image, status)
        VALUES ($1, $2, NULLIF($3,''), $4, NULLIF($5,''), $6)
    `, rec.ID, rec.OwnerID, rec.ProjectID, rec.Driver, rec.Image, string(rec.Status))
	if err != nil {
		// pgx surfaces duplicate-key as a *pgconn.PgError; the simplest
		// portable signal is the SQLState 23505 prefix in the message.
		if isUnique(err) {
			return ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (p *PostgresStore) Get(ctx context.Context, id string) (Record, error) {
	row := p.pool.QueryRow(ctx, `SELECT `+stateColumns+` FROM workspaces_state WHERE id=$1`, id)
	rec, err := scanRecord(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	return rec, err
}

func (p *PostgresStore) List(ctx context.Context, ownerID string) ([]Record, error) {
	q := `SELECT ` + stateColumns + ` FROM workspaces_state`
	args := []any{}
	if ownerID != "" {
		q += ` WHERE owner_id=$1`
		args = append(args, ownerID)
	}
	q += ` ORDER BY created_at DESC LIMIT 500`
	rows, err := p.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *PostgresStore) Claim(ctx context.Context, id, podID string) (Record, error) {
	row := p.pool.QueryRow(ctx, `
        UPDATE workspaces_state
           SET current_pod_id = $2,
               status = 'running',
               last_heartbeat_at = now(),
               updated_at = now()
         WHERE id = $1
           AND (current_pod_id IS NULL OR current_pod_id = '' OR current_pod_id = $2)
           AND status IN ('created','stopped','running')
        RETURNING `+stateColumns+`
    `, id, podID)
	rec, err := scanRecord(row)
	if errors.Is(err, pgx.ErrNoRows) {
		// Two cases: row missing, or row owned by another pod. Disambiguate.
		existing, gerr := p.Get(ctx, id)
		if errors.Is(gerr, ErrNotFound) {
			return Record{}, ErrNotFound
		}
		if gerr != nil {
			return Record{}, gerr
		}
		if existing.CurrentPodID != "" && existing.CurrentPodID != podID {
			return Record{}, ErrNotClaimable
		}
		return Record{}, ErrNotClaimable
	}
	return rec, err
}

func (p *PostgresStore) Heartbeat(ctx context.Context, id, podID string) (bool, error) {
	tag, err := p.pool.Exec(ctx, `
        UPDATE workspaces_state
           SET last_heartbeat_at = now(), updated_at = now()
         WHERE id = $1 AND current_pod_id = $2
    `, id, podID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (p *PostgresStore) Release(ctx context.Context, id, podID string, status Status) error {
	_, err := p.pool.Exec(ctx, `
        UPDATE workspaces_state
           SET current_pod_id = NULL,
               status = $3,
               last_heartbeat_at = NULL,
               updated_at = now()
         WHERE id = $1 AND current_pod_id = $2
    `, id, podID, string(status))
	return err
}

func (p *PostgresStore) Reap(ctx context.Context, staleAfter time.Duration) ([]string, error) {
	// Cast the interval through Go's time.Duration so the SQL uses
	// seconds (avoids locale-dependent interval parsing on the server).
	secs := int64(staleAfter.Seconds())
	if secs <= 0 {
		secs = 60
	}
	rows, err := p.pool.Query(ctx, `
        UPDATE workspaces_state
           SET current_pod_id = NULL,
               status = 'stopped',
               last_heartbeat_at = NULL,
               updated_at = now()
         WHERE current_pod_id IS NOT NULL
           AND current_pod_id <> ''
           AND (last_heartbeat_at IS NULL
                OR last_heartbeat_at < now() - make_interval(secs => $1::int))
        RETURNING id
    `, secs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (p *PostgresStore) OwnedBy(ctx context.Context, podID string) ([]Record, error) {
	rows, err := p.pool.Query(ctx, `
        SELECT `+stateColumns+`
          FROM workspaces_state
         WHERE current_pod_id = $1
    `, podID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *PostgresStore) SetStatus(ctx context.Context, id string, status Status) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE workspaces_state SET status=$2, updated_at=now() WHERE id=$1`,
		id, string(status))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresStore) Delete(ctx context.Context, id string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM workspaces_state WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// isUnique inspects an error for a Postgres SQLSTATE 23505. We avoid
// importing pgconn directly so the package compiles even when the test
// build tags strip pgx integration symbols.
func isUnique(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "23505") || contains(msg, "duplicate key")
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	// Naive substring scan — only used on error strings (tiny).
	n := len(s) - len(sub)
	for i := 0; i <= n; i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

var _ Store = (*MemoryStore)(nil)
var _ Store = (*PostgresStore)(nil)
