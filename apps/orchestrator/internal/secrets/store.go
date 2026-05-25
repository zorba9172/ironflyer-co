package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the persistence contract for SecretRef + release rows.
// Implementations MUST enforce the (tenant_id, project_id, name)
// uniqueness from migration 00032 and MUST refuse to mutate the
// `releases` rows once recorded (the broker treats them as audit).
type Store interface {
	CreateRef(ctx context.Context, ref SecretRef) (SecretRef, error)
	GetRef(ctx context.Context, id string) (SecretRef, error)
	LookupRef(ctx context.Context, tenantID, projectID, name string) (SecretRef, error)
	ListRefs(ctx context.Context, tenantID string) ([]SecretRef, error)
	UpdateRef(ctx context.Context, id string, fn func(*SecretRef)) (SecretRef, error)

	RecordRelease(ctx context.Context, rel ReleaseRecord) (ReleaseRecord, error)
	ListReleases(ctx context.Context, refID string) ([]ReleaseRecord, error)
}

// ReleaseRecord is one row in secret_releases. The fields mirror the
// migration column for column so writes do not need a translation
// layer beyond JSON encoding for metadata.
type ReleaseRecord struct {
	ID               int64     `json:"id"`
	SecretRefID      string    `json:"secretRefId"`
	TenantID         string    `json:"tenantId"`
	ExecutionID      string    `json:"executionId,omitempty"`
	WorkspaceID      string    `json:"workspaceId,omitempty"`
	PolicyDecisionID string    `json:"policyDecisionId"`
	ReleasedTo       string    `json:"releasedTo"`
	ReleasedAt       time.Time `json:"releasedAt"`
	ExpiresAt        time.Time `json:"expiresAt"`
	RedactionProof   string    `json:"redactionProof"`
}

// PostgresStore is the production Store implementation. The pool's
// lifecycle is owned by the caller (orchestrator startup).
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore wraps a pgx pool. Migration 00032 must already be
// applied on the database before the broker dispatches its first call.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

const createRefSQL = `
INSERT INTO secret_refs
    (id, tenant_id, project_id, name, backend, backend_ref, release_class,
     version, rotated_at, last_released_at, metadata, created_at)
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, tenant_id, project_id, name, backend, backend_ref, release_class,
          version, rotated_at, last_released_at, metadata, created_at
`

func (p *PostgresStore) CreateRef(ctx context.Context, ref SecretRef) (SecretRef, error) {
	if ref.TenantID == "" || ref.Name == "" {
		return SecretRef{}, fmt.Errorf("secrets: tenant_id and name are required")
	}
	if !validReleaseClass(ref.ReleaseClass) {
		return SecretRef{}, ErrInvalidReleaseClass
	}
	if ref.ID == "" {
		ref.ID = uuid.NewString()
	}
	if ref.Version <= 0 {
		ref.Version = 1
	}
	if ref.CreatedAt.IsZero() {
		ref.CreatedAt = time.Now().UTC()
	}
	meta, err := json.Marshal(ref.Metadata)
	if err != nil {
		return SecretRef{}, fmt.Errorf("secrets: marshal metadata: %w", err)
	}
	row := p.pool.QueryRow(ctx, createRefSQL,
		ref.ID,
		ref.TenantID,
		nullableUUID(ref.ProjectID),
		ref.Name,
		string(ref.Backend),
		ref.BackendRef,
		string(ref.ReleaseClass),
		ref.Version,
		nullableTime(ref.RotatedAt),
		nullableTime(ref.LastReleasedAt),
		meta,
		ref.CreatedAt,
	)
	return scanRef(row)
}

const selectRefByIDSQL = `
SELECT id, tenant_id, project_id, name, backend, backend_ref, release_class,
       version, rotated_at, last_released_at, metadata, created_at
FROM secret_refs WHERE id = $1
`

func (p *PostgresStore) GetRef(ctx context.Context, id string) (SecretRef, error) {
	row := p.pool.QueryRow(ctx, selectRefByIDSQL, id)
	r, err := scanRef(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return SecretRef{}, ErrSecretNotFound
	}
	return r, err
}

const lookupRefSQL = `
SELECT id, tenant_id, project_id, name, backend, backend_ref, release_class,
       version, rotated_at, last_released_at, metadata, created_at
FROM secret_refs
WHERE tenant_id = $1
  AND name = $2
  AND ((project_id IS NULL AND $3::uuid IS NULL) OR project_id = $3::uuid)
LIMIT 1
`

func (p *PostgresStore) LookupRef(ctx context.Context, tenantID, projectID, name string) (SecretRef, error) {
	row := p.pool.QueryRow(ctx, lookupRefSQL, tenantID, name, nullableUUID(projectID))
	r, err := scanRef(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return SecretRef{}, ErrSecretNotFound
	}
	return r, err
}

const listRefsSQL = `
SELECT id, tenant_id, project_id, name, backend, backend_ref, release_class,
       version, rotated_at, last_released_at, metadata, created_at
FROM secret_refs WHERE tenant_id = $1
ORDER BY created_at DESC
`

func (p *PostgresStore) ListRefs(ctx context.Context, tenantID string) ([]SecretRef, error) {
	rows, err := p.pool.Query(ctx, listRefsSQL, tenantID)
	if err != nil {
		return nil, fmt.Errorf("secrets: list refs: %w", err)
	}
	defer rows.Close()
	out := make([]SecretRef, 0)
	for rows.Next() {
		r, err := scanRef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

const updateRefSQL = `
UPDATE secret_refs
SET backend = $2,
    backend_ref = $3,
    release_class = $4,
    version = $5,
    rotated_at = $6,
    last_released_at = $7,
    metadata = $8
WHERE id = $1
RETURNING id, tenant_id, project_id, name, backend, backend_ref, release_class,
          version, rotated_at, last_released_at, metadata, created_at
`

// UpdateRef runs fn against the current row, then persists. Wrapped
// in a serializable transaction so concurrent Rotate calls don't lose
// each other's writes.
func (p *PostgresStore) UpdateRef(ctx context.Context, id string, fn func(*SecretRef)) (SecretRef, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return SecretRef{}, fmt.Errorf("secrets: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row := tx.QueryRow(ctx, selectRefByIDSQL, id)
	current, err := scanRef(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return SecretRef{}, ErrSecretNotFound
	}
	if err != nil {
		return SecretRef{}, err
	}
	fn(&current)
	meta, err := json.Marshal(current.Metadata)
	if err != nil {
		return SecretRef{}, fmt.Errorf("secrets: marshal metadata: %w", err)
	}
	updated, err := scanRef(tx.QueryRow(ctx, updateRefSQL,
		current.ID,
		string(current.Backend),
		current.BackendRef,
		string(current.ReleaseClass),
		current.Version,
		nullableTime(current.RotatedAt),
		nullableTime(current.LastReleasedAt),
		meta,
	))
	if err != nil {
		return SecretRef{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SecretRef{}, fmt.Errorf("secrets: commit: %w", err)
	}
	return updated, nil
}

const insertReleaseSQL = `
INSERT INTO secret_releases
    (secret_ref_id, tenant_id, execution_id, workspace_id, policy_decision_id,
     released_to, released_at, expires_at, redaction_proof)
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, secret_ref_id, tenant_id, execution_id, workspace_id,
          policy_decision_id, released_to, released_at, expires_at,
          redaction_proof
`

const stampLastReleasedSQL = `
UPDATE secret_refs SET last_released_at = $2 WHERE id = $1
`

func (p *PostgresStore) RecordRelease(ctx context.Context, rel ReleaseRecord) (ReleaseRecord, error) {
	if rel.SecretRefID == "" || rel.TenantID == "" {
		return ReleaseRecord{}, fmt.Errorf("secrets: release requires secret_ref_id and tenant_id")
	}
	if rel.PolicyDecisionID == "" {
		return ReleaseRecord{}, ErrPolicyDecisionRequired
	}
	if !validReleaseTo(rel.ReleasedTo) {
		return ReleaseRecord{}, ErrInvalidReleaseTo
	}
	if rel.ReleasedAt.IsZero() {
		rel.ReleasedAt = time.Now().UTC()
	}
	if rel.RedactionProof == "" {
		rel.RedactionProof = "sha256:redacted"
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return ReleaseRecord{}, fmt.Errorf("secrets: begin release: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row := tx.QueryRow(ctx, insertReleaseSQL,
		rel.SecretRefID,
		rel.TenantID,
		nullableUUID(rel.ExecutionID),
		nullableString(rel.WorkspaceID),
		rel.PolicyDecisionID,
		rel.ReleasedTo,
		rel.ReleasedAt,
		rel.ExpiresAt,
		rel.RedactionProof,
	)
	stored, err := scanRelease(row)
	if err != nil {
		return ReleaseRecord{}, fmt.Errorf("secrets: insert release: %w", err)
	}
	if _, err := tx.Exec(ctx, stampLastReleasedSQL, rel.SecretRefID, rel.ReleasedAt); err != nil {
		return ReleaseRecord{}, fmt.Errorf("secrets: stamp last_released_at: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ReleaseRecord{}, fmt.Errorf("secrets: commit release: %w", err)
	}
	return stored, nil
}

const listReleasesSQL = `
SELECT id, secret_ref_id, tenant_id, execution_id, workspace_id,
       policy_decision_id, released_to, released_at, expires_at,
       redaction_proof
FROM secret_releases WHERE secret_ref_id = $1
ORDER BY released_at DESC
`

func (p *PostgresStore) ListReleases(ctx context.Context, refID string) ([]ReleaseRecord, error) {
	rows, err := p.pool.Query(ctx, listReleasesSQL, refID)
	if err != nil {
		return nil, fmt.Errorf("secrets: list releases: %w", err)
	}
	defer rows.Close()
	out := make([]ReleaseRecord, 0)
	for rows.Next() {
		rel, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rel)
	}
	return out, rows.Err()
}

// row covers both pgx.Row and pgx.Rows for shared scanning.
type row interface {
	Scan(dest ...any) error
}

func scanRef(r row) (SecretRef, error) {
	var (
		ref           SecretRef
		projectID     *string
		backendStr    string
		releaseClass  string
		rotatedAt     *time.Time
		lastReleased  *time.Time
		metaJSON      []byte
	)
	if err := r.Scan(
		&ref.ID,
		&ref.TenantID,
		&projectID,
		&ref.Name,
		&backendStr,
		&ref.BackendRef,
		&releaseClass,
		&ref.Version,
		&rotatedAt,
		&lastReleased,
		&metaJSON,
		&ref.CreatedAt,
	); err != nil {
		return SecretRef{}, err
	}
	if projectID != nil {
		ref.ProjectID = *projectID
	}
	ref.Backend = Backend(backendStr)
	ref.ReleaseClass = ReleaseClass(releaseClass)
	ref.RotatedAt = rotatedAt
	ref.LastReleasedAt = lastReleased
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &ref.Metadata)
	}
	return ref, nil
}

func scanRelease(r row) (ReleaseRecord, error) {
	var (
		rel         ReleaseRecord
		executionID *string
		workspaceID *string
	)
	if err := r.Scan(
		&rel.ID,
		&rel.SecretRefID,
		&rel.TenantID,
		&executionID,
		&workspaceID,
		&rel.PolicyDecisionID,
		&rel.ReleasedTo,
		&rel.ReleasedAt,
		&rel.ExpiresAt,
		&rel.RedactionProof,
	); err != nil {
		return ReleaseRecord{}, err
	}
	if executionID != nil {
		rel.ExecutionID = *executionID
	}
	if workspaceID != nil {
		rel.WorkspaceID = *workspaceID
	}
	return rel, nil
}

// nullableUUID returns *string for UUID columns so pgx encodes NULL
// for empty inputs and a typed uuid for non-empty inputs.
func nullableUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return *t
}
