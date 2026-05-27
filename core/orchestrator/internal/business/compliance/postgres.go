package compliance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

// PostgresBackend persists compliance state to Postgres. Tables live in
// migrations/00047_compliance.sql. Every write is single-row so we do
// not coordinate transactions across tables — the idempotency keys on
// charges and the natural uniqueness on enrolment tuples cover the
// concurrency contract.
type PostgresBackend struct {
	pool *pgxpool.Pool
}

// NewPostgresBackend wires the backend to an existing pgxpool. The
// caller MUST have applied migrations/00047_compliance.sql first.
func NewPostgresBackend(pool *pgxpool.Pool) *PostgresBackend {
	return &PostgresBackend{pool: pool}
}

const pgUniqueViolation = "23505"

func (b *PostgresBackend) Enroll(ctx context.Context, row EnrolledProject) error {
	_, err := b.pool.Exec(ctx, `
        INSERT INTO compliance_enrollments
            (id, tenant_id, project_id, framework_key, enrolled_at,
             last_verdict, next_charge_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		row.ID, row.TenantID, row.ProjectID, row.FrameworkKey,
		row.EnrolledAt, string(row.LastVerdict), row.NextChargeAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyEnrolled
		}
		return fmt.Errorf("compliance: insert enrollment: %w", err)
	}
	return nil
}

func (b *PostgresBackend) GetEnrollment(ctx context.Context, id string) (EnrolledProject, error) {
	row := b.pool.QueryRow(ctx, `
        SELECT id, tenant_id, project_id, framework_key,
               enrolled_at, last_evaluated_at, last_verdict, next_charge_at
        FROM compliance_enrollments WHERE id = $1`, id)
	return scanEnrollment(row)
}

func (b *PostgresBackend) GetEnrollmentByTuple(ctx context.Context, tenant, projectID, framework string) (EnrolledProject, error) {
	row := b.pool.QueryRow(ctx, `
        SELECT id, tenant_id, project_id, framework_key,
               enrolled_at, last_evaluated_at, last_verdict, next_charge_at
        FROM compliance_enrollments
        WHERE tenant_id = $1 AND project_id = $2 AND framework_key = $3`,
		tenant, projectID, framework,
	)
	return scanEnrollment(row)
}

func (b *PostgresBackend) ListEnrollments(ctx context.Context, tenant, projectID string) ([]EnrolledProject, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if projectID == "" {
		rows, err = b.pool.Query(ctx, `
            SELECT id, tenant_id, project_id, framework_key,
                   enrolled_at, last_evaluated_at, last_verdict, next_charge_at
            FROM compliance_enrollments
            WHERE tenant_id = $1
            ORDER BY enrolled_at DESC`, tenant)
	} else {
		rows, err = b.pool.Query(ctx, `
            SELECT id, tenant_id, project_id, framework_key,
                   enrolled_at, last_evaluated_at, last_verdict, next_charge_at
            FROM compliance_enrollments
            WHERE tenant_id = $1 AND project_id = $2
            ORDER BY enrolled_at DESC`, tenant, projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnrolledProject{}
	for rows.Next() {
		e, err := scanEnrollment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (b *PostgresBackend) ListAllEnrollments(ctx context.Context) ([]EnrolledProject, error) {
	rows, err := b.pool.Query(ctx, `
        SELECT id, tenant_id, project_id, framework_key,
               enrolled_at, last_evaluated_at, last_verdict, next_charge_at
        FROM compliance_enrollments ORDER BY enrolled_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnrolledProject{}
	for rows.Next() {
		e, err := scanEnrollment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (b *PostgresBackend) MarkEvaluated(ctx context.Context, id string, at time.Time, verdict VerdictKind) error {
	_, err := b.pool.Exec(ctx, `
        UPDATE compliance_enrollments
        SET last_evaluated_at = $2, last_verdict = $3
        WHERE id = $1`, id, at, string(verdict))
	return err
}

func (b *PostgresBackend) DeleteEnrollment(ctx context.Context, id string) error {
	if _, err := b.pool.Exec(ctx, `DELETE FROM compliance_results WHERE enrollment_id = $1`, id); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `DELETE FROM compliance_enrollments WHERE id = $1`, id)
	return err
}

func (b *PostgresBackend) SaveResults(ctx context.Context, enrollmentID string, results []ControlResult) error {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM compliance_results WHERE enrollment_id = $1`, enrollmentID); err != nil {
		return err
	}
	for _, r := range results {
		_, err := tx.Exec(ctx, `
            INSERT INTO compliance_results
                (id, enrollment_id, control_key, framework_key,
                 status, severity, evidence, path, evaluated_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			r.ID, enrollmentID, r.ControlKey, r.FrameworkKey,
			string(r.Status), string(r.Severity), r.Evidence, r.Path, r.EvaluatedAt,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (b *PostgresBackend) ListResults(ctx context.Context, enrollmentID string) ([]ControlResult, error) {
	rows, err := b.pool.Query(ctx, `
        SELECT id, enrollment_id, control_key, framework_key,
               status, severity, evidence, path, evaluated_at
        FROM compliance_results
        WHERE enrollment_id = $1
        ORDER BY evaluated_at DESC, severity ASC`, enrollmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ControlResult{}
	for rows.Next() {
		var (
			r           ControlResult
			status, sev string
			path        *string
		)
		if err := rows.Scan(&r.ID, &r.EnrollmentID, &r.ControlKey, &r.FrameworkKey,
			&status, &sev, &r.Evidence, &path, &r.EvaluatedAt); err != nil {
			return nil, err
		}
		r.Status = ControlStatus(status)
		r.Severity = severityFromString(sev)
		if path != nil {
			r.Path = *path
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (b *PostgresBackend) RecordCharge(ctx context.Context, c Charge) error {
	_, err := b.pool.Exec(ctx, `
        INSERT INTO compliance_charges
            (id, enrollment_id, tenant_id, framework_key, period,
             amount_usd, charged_at, idempotency_key)
        VALUES ($1, $2, $3, $4, $5, $6::numeric, $7, $8)`,
		c.ID, c.EnrollmentID, c.TenantID, c.FrameworkKey, c.Period,
		c.AmountUSD.String(), c.ChargedAt, c.IdempotencyKey,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyEnrolled
		}
		return err
	}
	return nil
}

func (b *PostgresBackend) HasCharge(ctx context.Context, idempotencyKey string) (bool, error) {
	var exists bool
	row := b.pool.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM compliance_charges WHERE idempotency_key = $1)`,
		idempotencyKey,
	)
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

var _ Backend = (*PostgresBackend)(nil)

// scanEnrollment maps one row into an EnrolledProject. Works for both
// pgx.Row and pgx.Rows because both expose Scan with the same shape.
type scanner interface {
	Scan(dest ...any) error
}

func scanEnrollment(s scanner) (EnrolledProject, error) {
	var (
		row         EnrolledProject
		verdict     string
		evaluatedAt *time.Time
	)
	if err := s.Scan(&row.ID, &row.TenantID, &row.ProjectID, &row.FrameworkKey,
		&row.EnrolledAt, &evaluatedAt, &verdict, &row.NextChargeAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EnrolledProject{}, ErrNotFound
		}
		return EnrolledProject{}, err
	}
	row.LastVerdict = VerdictKind(verdict)
	row.LastEvaluatedAt = evaluatedAt
	return row, nil
}

// severityFromString round-trips the persisted severity column into
// the typed domain.Severity. Unknown values pass through verbatim so
// future severity additions don't drop data on read.
func severityFromString(s string) domain.Severity {
	return domain.Severity(s)
}

// isUniqueViolation classifies the pg error code so callers can map it
// to ErrAlreadyEnrolled without leaking pgx into the resolver.
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == pgUniqueViolation
	}
	return false
}

// keep fmt import referenced — wrapping unknown DB errors is the
// canonical Postgres backend style across the repo.
var _ = fmt.Errorf
