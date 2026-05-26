// Package workspaces holds the scale-ready workspace lifecycle: the
// persistent registry (Postgres / Memory), the EFS-backed file storage,
// the S3 archival path, and the Redis pod-ownership registry.
//
// The legacy `internal/sandbox` package still owns the driver interface
// + per-process in-memory workspace map. This package layers durability
// + cross-pod coordination on top: every workspace mutation that needs
// to survive a pod restart lands here first, then is mirrored into the
// driver via the existing sandbox.Manager.
package workspaces

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Status is the persistent lifecycle status — distinct from sandbox.Status
// because it includes the cross-pod states (idle, archived) the in-memory
// driver doesn't reason about.
type Status string

const (
	StatusCreating  Status = "creating"
	StatusRunning   Status = "running"
	StatusIdle      Status = "idle"
	StatusArchived  Status = "archived"
	StatusDestroyed Status = "destroyed"
)

// Record is one row of the `workspaces` table. JSON tags mirror the
// runtime HTTP API so we can hand a Record straight to writeJSON without
// an adapter type.
type Record struct {
	ID            string     `json:"id"`
	OwnerID       string     `json:"ownerId"`
	ProjectID     string     `json:"projectId,omitempty"`
	Driver        string     `json:"driver"`
	Status        Status     `json:"status"`
	EFSPath       string     `json:"efsPath,omitempty"`
	S3ArchiveKey  string     `json:"s3ArchiveKey,omitempty"`
	ActivePod     string     `json:"activePod,omitempty"`
	LastActiveAt  *time.Time `json:"lastActiveAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// ErrNotFound matches the sentinel sandbox.ErrNotFound returns so the
// HTTP layer can map either to 404 with a single check.
var ErrNotFound = errors.New("workspace not found")

// Store is the durable registry contract. Implementations: Memory
// (single-pod / dev) and Postgres (multi-pod / prod).
type Store interface {
	Insert(ctx context.Context, rec Record) error
	Get(ctx context.Context, id string) (Record, error)
	List(ctx context.Context, ownerID string) ([]Record, error)
	UpdateStatus(ctx context.Context, id string, status Status) error
	UpdateActivePod(ctx context.Context, id, podIP string) error
	UpdateEFSPath(ctx context.Context, id, path string) error
	UpdateArchive(ctx context.Context, id, s3Key string) error
	TouchActive(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	// IdleCandidates returns workspaces whose last_active_at is older
	// than the cutoff and status is `running` or `idle` — the archival
	// scanner uses this to find work.
	IdleCandidates(ctx context.Context, olderThan time.Time, limit int) ([]Record, error)
}

// --- Memory implementation ---------------------------------------------

// MemoryStore is a single-pod in-process registry. Sufficient for dev
// and single-replica deployments; production should always wire
// PostgresStore.
type MemoryStore struct {
	mu   sync.RWMutex
	rows map[string]Record
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{rows: make(map[string]Record)}
}

func (m *MemoryStore) Insert(_ context.Context, rec Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rows[rec.ID]; ok {
		return errors.New("workspace already exists")
	}
	rec.CreatedAt = nowUTC()
	rec.UpdatedAt = rec.CreatedAt
	m.rows[rec.ID] = rec
	return nil
}

func (m *MemoryStore) Get(_ context.Context, id string) (Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.rows[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	return rec, nil
}

func (m *MemoryStore) List(_ context.Context, ownerID string) ([]Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Record, 0, len(m.rows))
	for _, r := range m.rows {
		if ownerID != "" && r.OwnerID != ownerID {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (m *MemoryStore) UpdateStatus(_ context.Context, id string, status Status) error {
	return m.mutate(id, func(r *Record) { r.Status = status })
}

func (m *MemoryStore) UpdateActivePod(_ context.Context, id, podIP string) error {
	return m.mutate(id, func(r *Record) { r.ActivePod = podIP })
}

func (m *MemoryStore) UpdateEFSPath(_ context.Context, id, path string) error {
	return m.mutate(id, func(r *Record) { r.EFSPath = path })
}

func (m *MemoryStore) UpdateArchive(_ context.Context, id, s3Key string) error {
	return m.mutate(id, func(r *Record) {
		r.S3ArchiveKey = s3Key
		r.EFSPath = ""
		r.Status = StatusArchived
	})
}

func (m *MemoryStore) TouchActive(_ context.Context, id string) error {
	return m.mutate(id, func(r *Record) {
		now := nowUTC()
		r.LastActiveAt = &now
	})
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

func (m *MemoryStore) IdleCandidates(_ context.Context, olderThan time.Time, limit int) ([]Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Record
	for _, r := range m.rows {
		if r.Status != StatusRunning && r.Status != StatusIdle {
			continue
		}
		if r.LastActiveAt == nil || r.LastActiveAt.Before(olderThan) {
			out = append(out, r)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (m *MemoryStore) mutate(id string, fn func(*Record)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return ErrNotFound
	}
	fn(&r)
	r.UpdatedAt = nowUTC()
	m.rows[id] = r
	return nil
}

// --- Postgres implementation -------------------------------------------

// PostgresStore persists workspace metadata in the `workspaces` table.
// The schema is created by core/runtime/migrations/00001_workspaces.sql.
type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

const workspaceColumns = "id, owner_id, COALESCE(project_id,''), driver, status, COALESCE(efs_path,''), COALESCE(s3_archive_key,''), COALESCE(active_pod,''), last_active_at, created_at, updated_at"

func scanRecord(row pgx.Row) (Record, error) {
	var r Record
	var status string
	if err := row.Scan(&r.ID, &r.OwnerID, &r.ProjectID, &r.Driver, &status,
		&r.EFSPath, &r.S3ArchiveKey, &r.ActivePod,
		&r.LastActiveAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return Record{}, err
	}
	r.Status = Status(status)
	return r, nil
}

func (p *PostgresStore) Insert(ctx context.Context, rec Record) error {
	now := nowUTC()
	_, err := p.pool.Exec(ctx, `
		INSERT INTO workspaces (id, owner_id, project_id, driver, status, efs_path,
			s3_archive_key, active_pod, last_active_at, created_at, updated_at)
		VALUES ($1,$2, NULLIF($3,''), $4, $5, NULLIF($6,''), NULLIF($7,''), NULLIF($8,''),
			$9, $10, $10)
	`, rec.ID, rec.OwnerID, rec.ProjectID, rec.Driver, string(rec.Status),
		rec.EFSPath, rec.S3ArchiveKey, rec.ActivePod, rec.LastActiveAt, now)
	return err
}

func (p *PostgresStore) Get(ctx context.Context, id string) (Record, error) {
	row := p.pool.QueryRow(ctx, `SELECT `+workspaceColumns+` FROM workspaces WHERE id=$1`, id)
	rec, err := scanRecord(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	return rec, err
}

func (p *PostgresStore) List(ctx context.Context, ownerID string) ([]Record, error) {
	q := `SELECT ` + workspaceColumns + ` FROM workspaces`
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

func (p *PostgresStore) UpdateStatus(ctx context.Context, id string, status Status) error {
	_, err := p.pool.Exec(ctx, `UPDATE workspaces SET status=$2, updated_at=now() WHERE id=$1`, id, string(status))
	return err
}

func (p *PostgresStore) UpdateActivePod(ctx context.Context, id, podIP string) error {
	_, err := p.pool.Exec(ctx, `UPDATE workspaces SET active_pod=NULLIF($2,''), updated_at=now() WHERE id=$1`, id, podIP)
	return err
}

func (p *PostgresStore) UpdateEFSPath(ctx context.Context, id, path string) error {
	_, err := p.pool.Exec(ctx, `UPDATE workspaces SET efs_path=NULLIF($2,''), updated_at=now() WHERE id=$1`, id, path)
	return err
}

func (p *PostgresStore) UpdateArchive(ctx context.Context, id, s3Key string) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE workspaces
		   SET s3_archive_key=NULLIF($2,''),
		       efs_path=NULL,
		       status=$3,
		       updated_at=now()
		 WHERE id=$1
	`, id, s3Key, string(StatusArchived))
	return err
}

func (p *PostgresStore) TouchActive(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `UPDATE workspaces SET last_active_at=now(), updated_at=now() WHERE id=$1`, id)
	return err
}

func (p *PostgresStore) Delete(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM workspaces WHERE id=$1`, id)
	return err
}

func (p *PostgresStore) IdleCandidates(ctx context.Context, olderThan time.Time, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := p.pool.Query(ctx, `
		SELECT `+workspaceColumns+` FROM workspaces
		WHERE status IN ('running','idle')
		  AND (last_active_at IS NULL OR last_active_at < $1)
		ORDER BY last_active_at NULLS FIRST
		LIMIT $2
	`, olderThan, limit)
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

func nowUTC() time.Time { return time.Now().UTC() }

var _ Store = (*MemoryStore)(nil)
var _ Store = (*PostgresStore)(nil)
