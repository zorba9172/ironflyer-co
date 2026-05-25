// Package patch — staging store contracts + Postgres-backed
// implementation. The contract is in this file so callers can wire
// either the in-memory (staging.go's MemoryStagingStore) or the
// Postgres variant without conditional imports.
//
// Schema: migrations/00018_patch_stages.sql. The columns mirror the
// Go struct so reads are a single SELECT and writes are a single
// upsert.
package patch

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// StagingStore is the persistence contract for PatchStage records.
// Implementations must be safe for concurrent calls — the Engine
// shares one instance across goroutines.
type StagingStore interface {
	Put(st PatchStage) error
	Get(id string) (PatchStage, bool, error)
	List(projectID string) ([]PatchStage, error)
	Delete(id string) error
}

// PostgresStagingStore persists PatchStage rows in the orchestrator's
// Postgres instance. Schema lives in migrations/00018_patch_stages.sql.
// The patch_ids column is JSONB so the slice can grow without a join
// table — staging is a low-cardinality feature, not a workload.
type PostgresStagingStore struct {
	db *sql.DB
}

// NewPostgresStagingStore wraps a *sql.DB. The caller is responsible
// for running the 00018 migration before constructing this store.
func NewPostgresStagingStore(db *sql.DB) *PostgresStagingStore {
	return &PostgresStagingStore{db: db}
}

func (p *PostgresStagingStore) Put(st PatchStage) error {
	if p == nil || p.db == nil {
		return errors.New("postgres staging store not configured")
	}
	ids, err := json.Marshal(st.PatchIDs)
	if err != nil {
		return err
	}
	_, err = p.db.Exec(`
		INSERT INTO patch_stages (id, project_id, name, description, patch_ids, status,
		                          rejection_reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			name             = EXCLUDED.name,
			description      = EXCLUDED.description,
			patch_ids        = EXCLUDED.patch_ids,
			status           = EXCLUDED.status,
			rejection_reason = EXCLUDED.rejection_reason,
			updated_at       = EXCLUDED.updated_at
	`,
		st.ID, st.ProjectID, st.Name, st.Description, string(ids), string(st.Status),
		nullString(st.RejectionReason), st.CreatedAt.UTC(), st.UpdatedAt.UTC(),
	)
	return err
}

func (p *PostgresStagingStore) Get(id string) (PatchStage, bool, error) {
	if p == nil || p.db == nil {
		return PatchStage{}, false, errors.New("postgres staging store not configured")
	}
	row := p.db.QueryRow(`
		SELECT id, project_id, name, COALESCE(description,''), patch_ids,
		       status, COALESCE(rejection_reason,''), created_at, updated_at
		FROM patch_stages WHERE id = $1
	`, id)
	st, err := scanStage(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return PatchStage{}, false, nil
	}
	if err != nil {
		return PatchStage{}, false, err
	}
	return st, true, nil
}

func (p *PostgresStagingStore) List(projectID string) ([]PatchStage, error) {
	if p == nil || p.db == nil {
		return nil, errors.New("postgres staging store not configured")
	}
	rows, err := p.db.Query(`
		SELECT id, project_id, name, COALESCE(description,''), patch_ids,
		       status, COALESCE(rejection_reason,''), created_at, updated_at
		FROM patch_stages
		WHERE project_id = $1 OR $1 = ''
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PatchStage
	for rows.Next() {
		st, err := scanStage(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (p *PostgresStagingStore) Delete(id string) error {
	if p == nil || p.db == nil {
		return errors.New("postgres staging store not configured")
	}
	_, err := p.db.Exec(`DELETE FROM patch_stages WHERE id = $1`, id)
	return err
}

// scanRow is the narrow interface satisfied by both *sql.Row and
// *sql.Rows so scanStage can serve Get and List without duplication.
type scanRow func(dest ...any) error

func scanStage(scan scanRow) (PatchStage, error) {
	var (
		st         PatchStage
		idsJSON    string
		statusStr  string
		createdAt  time.Time
		updatedAt  time.Time
		rejection  string
		desc       string
	)
	if err := scan(&st.ID, &st.ProjectID, &st.Name, &desc, &idsJSON, &statusStr, &rejection, &createdAt, &updatedAt); err != nil {
		return PatchStage{}, err
	}
	st.Description = desc
	st.Status = StageStatus(statusStr)
	st.CreatedAt = createdAt
	st.UpdatedAt = updatedAt
	st.RejectionReason = rejection
	if idsJSON != "" {
		if err := json.Unmarshal([]byte(idsJSON), &st.PatchIDs); err != nil {
			return PatchStage{}, err
		}
	}
	return st, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
