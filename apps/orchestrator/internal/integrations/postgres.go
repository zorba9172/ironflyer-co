package integrations

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const bootstrapSQL = `
CREATE TABLE IF NOT EXISTS user_integrations (
    user_id        UUID        NOT NULL,
    kind           TEXT        NOT NULL,
    access_token   TEXT        NOT NULL,
    refresh_token  TEXT        NULL,
    scope          TEXT        NULL,
    external_id    TEXT        NULL,
    external_login TEXT        NULL,
    expires_at     TIMESTAMPTZ NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, kind)
);
CREATE INDEX IF NOT EXISTS idx_user_integrations_kind ON user_integrations(kind);
`

func BootstrapPostgres(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, bootstrapSQL)
	return err
}

type PostgresTokenStore struct{ pool *pgxpool.Pool }

func NewPostgresTokenStore(pool *pgxpool.Pool) *PostgresTokenStore {
	return &PostgresTokenStore{pool: pool}
}

func (s *PostgresTokenStore) Put(ctx context.Context, t Token) (Token, error) {
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	_, err := s.pool.Exec(ctx, `
        INSERT INTO user_integrations
          (user_id, kind, access_token, refresh_token, scope,
           external_id, external_login, expires_at, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        ON CONFLICT (user_id, kind) DO UPDATE SET
          access_token  = EXCLUDED.access_token,
          refresh_token = EXCLUDED.refresh_token,
          scope         = EXCLUDED.scope,
          external_id   = EXCLUDED.external_id,
          external_login= EXCLUDED.external_login,
          expires_at    = EXCLUDED.expires_at,
          updated_at    = EXCLUDED.updated_at`,
		t.UserID, string(t.Kind), t.AccessToken, nullable(t.RefreshToken),
		nullable(t.Scope), nullable(t.ExternalID), nullable(t.ExternalLogin),
		t.ExpiresAt, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return Token{}, err
	}
	return t, nil
}

func (s *PostgresTokenStore) Get(ctx context.Context, userID string, kind Kind) (Token, error) {
	row := s.pool.QueryRow(ctx, `
        SELECT user_id, kind, access_token, COALESCE(refresh_token,''),
               COALESCE(scope,''), COALESCE(external_id,''),
               COALESCE(external_login,''), expires_at, created_at, updated_at
        FROM user_integrations WHERE user_id = $1 AND kind = $2`, userID, string(kind))
	var t Token
	var kindStr string
	if err := row.Scan(&t.UserID, &kindStr, &t.AccessToken, &t.RefreshToken,
		&t.Scope, &t.ExternalID, &t.ExternalLogin, &t.ExpiresAt,
		&t.CreatedAt, &t.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Token{}, ErrNotFound
		}
		return Token{}, err
	}
	t.Kind = Kind(kindStr)
	return t, nil
}

func (s *PostgresTokenStore) Delete(ctx context.Context, userID string, kind Kind) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM user_integrations WHERE user_id = $1 AND kind = $2`, userID, string(kind))
	return err
}

func (s *PostgresTokenStore) ListByUser(ctx context.Context, userID string) ([]Token, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT user_id, kind, access_token, COALESCE(refresh_token,''),
               COALESCE(scope,''), COALESCE(external_id,''),
               COALESCE(external_login,''), expires_at, created_at, updated_at
        FROM user_integrations WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Token
	for rows.Next() {
		var t Token
		var kindStr string
		if err := rows.Scan(&t.UserID, &kindStr, &t.AccessToken, &t.RefreshToken,
			&t.Scope, &t.ExternalID, &t.ExternalLogin, &t.ExpiresAt,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Kind = Kind(kindStr)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *PostgresTokenStore) FindByExternal(ctx context.Context, kind Kind, externalID string) (string, error) {
	if externalID == "" {
		return "", ErrNotFound
	}
	row := s.pool.QueryRow(ctx, `
        SELECT user_id::text FROM user_integrations
        WHERE kind = $1 AND external_id = $2
        LIMIT 1`, string(kind), externalID)
	var uid string
	if err := row.Scan(&uid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return uid, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var _ TokenStore = (*PostgresTokenStore)(nil)
