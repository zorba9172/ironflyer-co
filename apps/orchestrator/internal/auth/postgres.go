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
	row := s.pool.QueryRow(ctx,
		`SELECT id, email, name, password_hash, plan, created_at
		 FROM users WHERE LOWER(email) = $1`, email)
	var u User
	var hash string
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &hash, &u.Plan, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, "", ErrUserNotFound
		}
		return User{}, "", err
	}
	return u, hash, nil
}

func (s *PostgresUserStore) GetByID(ctx context.Context, id string) (User, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, email, name, plan, created_at FROM users WHERE id = $1`, id)
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Plan, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	return u, nil
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

// Delete removes a user row. Idempotent: missing rows return nil.
func (s *PostgresUserStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

func isUniqueViolation(err error) bool {
	// pgx wraps the SQLSTATE; checking the string is the simplest portable
	// check that doesn't drag in pgconn explicitly here.
	return err != nil && strings.Contains(err.Error(), "SQLSTATE 23505")
}

var _ UserStore = (*PostgresUserStore)(nil)
