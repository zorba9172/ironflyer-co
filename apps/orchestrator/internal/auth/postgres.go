package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const bootstrapSQL = `
CREATE TABLE IF NOT EXISTS users (
    id            UUID        PRIMARY KEY,
    email         TEXT        NOT NULL UNIQUE,
    name          TEXT        NOT NULL DEFAULT '',
    password_hash TEXT        NOT NULL,
    plan          TEXT        NOT NULL DEFAULT 'free',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(LOWER(email));
`

// BootstrapPostgres creates the users table if needed and seeds the demo
// account (demo@ironflyer.dev / demo1234) so existing flows keep working.
//
// Deprecated: schema and demo seed now live in
// apps/orchestrator/migrations/{00002_init_auth,00014_init_auth_demo_seed}.sql.
// Add new schema as numbered goose files; this function is kept only as a
// fallback for callers outside the goose runner.
func BootstrapPostgres(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, bootstrapSQL); err != nil {
		return err
	}
	store := NewPostgresUserStore(pool)
	_, _, err := store.GetByEmail(ctx, "demo@ironflyer.dev")
	if errors.Is(err, ErrUserNotFound) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("demo1234"), bcrypt.DefaultCost)
		// Force the well-known id "demo" so existing budget/project flows
		// continue to address the same user across restarts.
		// Stable demo UUID so cross-restart references remain valid.
		_, err := pool.Exec(ctx, `
            INSERT INTO users (id, email, name, password_hash, plan)
            VALUES ('00000000-0000-0000-0000-000000000001',
                    'demo@ironflyer.dev', 'Demo User', $1, 'pro')
            ON CONFLICT (email) DO NOTHING`, string(hash))
		return err
	}
	return nil
}

// PostgresUserStore persists users in the `users` table.
type PostgresUserStore struct{ pool *pgxpool.Pool }

func NewPostgresUserStore(pool *pgxpool.Pool) *PostgresUserStore {
	return &PostgresUserStore{pool: pool}
}

func (s *PostgresUserStore) Create(ctx context.Context, email, name, hash string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := s.pool.Exec(ctx, `
        INSERT INTO users (id, email, name, password_hash, plan, created_at)
        VALUES ($1, $2, $3, $4, 'free', $5)`,
		id, email, name, hash, now)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrUserExists
		}
		return User{}, err
	}
	return User{ID: id, Email: email, Name: name, Plan: "free", CreatedAt: now}, nil
}

func (s *PostgresUserStore) GetByEmail(ctx context.Context, email string) (User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	// COALESCE on org_id keeps the query compatible with rows written
	// before BootstrapSAMLPostgres ran (no column / null value).
	// COALESCE on telemetry_opt_out keeps the query compatible with
	// rows written before the 00015 migration added the column.
	row := s.pool.QueryRow(ctx,
		`SELECT id, email, name, password_hash, plan,
                COALESCE(org_id, ''), COALESCE(telemetry_opt_out, false), created_at,
                email_verified_at,
                COALESCE(roles, ARRAY[]::TEXT[])
		 FROM users WHERE LOWER(email) = $1`, email)
	var u User
	var hash string
	var verifiedAt *time.Time
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &hash, &u.Plan, &u.OrgID, &u.TelemetryOptOut, &u.CreatedAt, &verifiedAt, &u.Roles); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, "", ErrUserNotFound
		}
		return User{}, "", err
	}
	u.EmailVerifiedAt = verifiedAt
	return u, hash, nil
}

func (s *PostgresUserStore) GetByID(ctx context.Context, id string) (User, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, email, name, plan, COALESCE(org_id, ''),
                COALESCE(telemetry_opt_out, false), created_at,
                email_verified_at,
                COALESCE(roles, ARRAY[]::TEXT[])
         FROM users WHERE id = $1`, id)
	var u User
	var verifiedAt *time.Time
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Plan, &u.OrgID, &u.TelemetryOptOut, &u.CreatedAt, &verifiedAt, &u.Roles); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	u.EmailVerifiedAt = verifiedAt
	return u, nil
}

// GetByIDs batch-loads users in a single `WHERE id = ANY($1)` query so
// the GraphQL dataloader collapses an N-row "owner email" projection
// into one round-trip. Missing ids are omitted from the result map.
func (s *PostgresUserStore) GetByIDs(ctx context.Context, ids []string) (map[string]User, error) {
	out := make(map[string]User, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, email, name, plan, COALESCE(org_id, ''),
                COALESCE(telemetry_opt_out, false), created_at,
                email_verified_at,
                COALESCE(roles, ARRAY[]::TEXT[])
         FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u User
		var verifiedAt *time.Time
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Plan, &u.OrgID, &u.TelemetryOptOut, &u.CreatedAt, &verifiedAt, &u.Roles); err != nil {
			return nil, err
		}
		u.EmailVerifiedAt = verifiedAt
		out[u.ID] = u
	}
	return out, rows.Err()
}

// SetOrg assigns an org membership to a user. Used by the admin endpoints
// that turn an existing account into a tenant member.
func (s *PostgresUserStore) SetOrg(ctx context.Context, id, org string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET org_id = $1 WHERE id = $2`,
		strings.ToLower(strings.TrimSpace(org)), id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *PostgresUserStore) SetPlan(ctx context.Context, id, plan string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET plan = $1 WHERE id = $2`, plan, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// SetTelemetryOptOut persists the per-user opt-out boolean. Backed by
// the `telemetry_opt_out` column introduced in migration 00015. Returns
// ErrUserNotFound when the row is missing.
func (s *PostgresUserStore) SetTelemetryOptOut(ctx context.Context, id string, opt bool) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET telemetry_opt_out = $1 WHERE id = $2`, opt, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// Delete removes a user row. Idempotent: missing rows return nil.
func (s *PostgresUserStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

// SetRoles overwrites the user's role set. Passing nil / empty slice
// clears the column (the user keeps existing only at the Plan level).
// Roles are normalised to lowercase + trimmed before persistence so
// the canonical row matches the constants in role.go. Returns
// ErrUserNotFound when no row was updated so the caller can route a
// 404. Implements the auth.RoleSetter interface.
func (s *PostgresUserStore) SetRoles(ctx context.Context, id string, roles []string) error {
	clean := normaliseRoleSet(roles)
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET roles = $1 WHERE id = $2`, clean, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// normaliseRoleSet trims + lowercases + deduplicates roles. Returned
// slice is always non-nil (an empty TEXT[] survives the round-trip
// cleanly).
func normaliseRoleSet(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, r := range in {
		n := normaliseRole(r)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func isUniqueViolation(err error) bool {
	// pgx wraps the SQLSTATE; checking the string is the simplest portable
	// check that doesn't drag in pgconn explicitly here.
	return err != nil && strings.Contains(err.Error(), "SQLSTATE 23505")
}

var _ UserStore = (*PostgresUserStore)(nil)
