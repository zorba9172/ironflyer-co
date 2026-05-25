// Package auth — password reset.
//
// Flow:
//
//  1. requestPasswordReset(email) ALWAYS returns ok:true so a probe
//     can't enumerate accounts. If the email matches a real user we
//     mint a 32-byte random token, store its SHA-256 hash with a 1h
//     TTL, and email the reset link.
//  2. resetPassword(token, newPassword) validates the token (not
//     expired, not previously used), rotates the bcrypt hash, marks
//     the row used, revokes every session for that user (forces re-
//     login from every device), and issues a fresh session.
//
// Throttling lives in the resolver: 5 requests/hour per IP AND
// 3 requests/hour per email; both backed by the existing Redis-backed
// rate limiter so multi-pod deployments share the budget.
package auth

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PasswordResetTTL is the default lifetime of a password-reset token.
const PasswordResetTTL = time.Hour

// ErrPasswordResetNotFound is returned for unknown / used / expired
// tokens. The caller MUST surface a generic "invalid or expired reset
// link" — never distinguish the three cases.
var ErrPasswordResetNotFound = errors.New("password reset token not found")

// PasswordResetRecord is the row callers consume after Consume.
type PasswordResetRecord struct {
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// PasswordResetStore persists reset tokens. Memory + Postgres back it.
type PasswordResetStore interface {
	Insert(ctx context.Context, tokenHash, userID string, expiresAt time.Time) error
	Consume(ctx context.Context, tokenHash string) (PasswordResetRecord, error)
}

// MemoryPasswordResetStore is the dev-mode in-process implementation.
type MemoryPasswordResetStore struct {
	mu     sync.Mutex
	byHash map[string]*passwordResetRow
}

type passwordResetRow struct {
	rec    PasswordResetRecord
	usedAt *time.Time
}

// NewMemoryPasswordResetStore constructs an empty in-memory store.
func NewMemoryPasswordResetStore() *MemoryPasswordResetStore {
	return &MemoryPasswordResetStore{byHash: make(map[string]*passwordResetRow)}
}

// Insert persists a fresh reset row.
func (s *MemoryPasswordResetStore) Insert(_ context.Context, tokenHash, userID string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byHash[tokenHash] = &passwordResetRow{
		rec: PasswordResetRecord{
			UserID:    userID,
			ExpiresAt: expiresAt,
			CreatedAt: time.Now().UTC(),
		},
	}
	return nil
}

// Consume marks the row used and returns it on success.
func (s *MemoryPasswordResetStore) Consume(_ context.Context, tokenHash string) (PasswordResetRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.byHash[tokenHash]
	if !ok || row.usedAt != nil {
		return PasswordResetRecord{}, ErrPasswordResetNotFound
	}
	if time.Now().UTC().After(row.rec.ExpiresAt) {
		return PasswordResetRecord{}, ErrPasswordResetNotFound
	}
	now := time.Now().UTC()
	row.usedAt = &now
	return row.rec, nil
}

// PostgresPasswordResetStore is the production backend.
type PostgresPasswordResetStore struct{ pool *pgxpool.Pool }

// NewPostgresPasswordResetStore wires the Postgres-backed store.
func NewPostgresPasswordResetStore(pool *pgxpool.Pool) *PostgresPasswordResetStore {
	return &PostgresPasswordResetStore{pool: pool}
}

// Insert persists a fresh reset row.
func (s *PostgresPasswordResetStore) Insert(ctx context.Context, tokenHash, userID string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO password_resets (token_hash, user_id, expires_at)
        VALUES ($1, $2, $3)`, tokenHash, userID, expiresAt)
	return err
}

// Consume stamps used_at and returns the record on success.
func (s *PostgresPasswordResetStore) Consume(ctx context.Context, tokenHash string) (PasswordResetRecord, error) {
	row := s.pool.QueryRow(ctx, `
        UPDATE password_resets
           SET used_at = now()
         WHERE token_hash = $1 AND used_at IS NULL
        RETURNING user_id, expires_at, created_at`, tokenHash)
	var rec PasswordResetRecord
	if err := row.Scan(&rec.UserID, &rec.ExpiresAt, &rec.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PasswordResetRecord{}, ErrPasswordResetNotFound
		}
		return PasswordResetRecord{}, err
	}
	if time.Now().UTC().After(rec.ExpiresAt) {
		return PasswordResetRecord{}, ErrPasswordResetNotFound
	}
	return rec, nil
}

// PasswordRotator is the surface the resolver depends on to actually
// update the stored bcrypt hash. Both Memory and Postgres user stores
// implement it via SetPasswordHash below.
type PasswordRotator interface {
	SetPasswordHash(ctx context.Context, userID, newHash string) error
}

// SetPasswordHash rotates the bcrypt hash on a Postgres-backed row.
func (s *PostgresUserStore) SetPasswordHash(ctx context.Context, userID, newHash string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`, newHash, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// SetPasswordHash rotates the hash on an in-memory user.
func (s *MemoryUserStore) SetPasswordHash(_ context.Context, userID, newHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[userID]; !ok {
		return ErrUserNotFound
	}
	s.hashes[userID] = newHash
	return nil
}
