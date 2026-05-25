// Package auth — stateful session registry.
//
// JWTs remain primarily stateless (signature + exp), but every freshly
// issued token also lands in the `sessions` table keyed by its `jti`
// claim. The verifier checks the row before letting a request through:
// revoked_at IS NULL or 401. To keep the hot path fast we cache the
// result in Redis (60s TTL); revocation pushes the jti to a Redis
// "revoked" set so other pods see the change instantly.
//
// Cache strategy:
//   - key "sess:<jti>" : "ok" | "revoked" (60s TTL)
//   - set "sess:revoked" : member set of recently-revoked jtis (no TTL;
//     trimmed by the periodic janitor that drops expired sessions)
//
// On a request:
//   1. SREM-check the revoked set; if present -> 401.
//   2. GET the cached state; on miss query Postgres and cache for 60s.
//
// Memory store mirrors the contract so the dev box still works without
// Redis or Postgres.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSessionNotFound is returned when the row does not exist.
var ErrSessionNotFound = errors.New("session not found")

// ErrSessionRevoked is returned when the row exists but revoked_at is set.
var ErrSessionRevoked = errors.New("session revoked")

// Session is the row shape callers consume.
type Session struct {
	JTI        string
	UserID     string
	IssuedAt   time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time
	IPAddress  string
	UserAgent  string
	RevokedAt  *time.Time
}

// SessionStore persists the per-jti session registry.
type SessionStore interface {
	Insert(ctx context.Context, s Session) error
	Get(ctx context.Context, jti string) (Session, error)
	List(ctx context.Context, userID string) ([]Session, error)
	Revoke(ctx context.Context, jti string) error
	RevokeAllForUser(ctx context.Context, userID string) error
	RevokeAllExcept(ctx context.Context, userID, keepJTI string) error
	Touch(ctx context.Context, jti string, ip, ua string, at time.Time) error
}

// MemorySessionStore is the dev-mode implementation.
type MemorySessionStore struct {
	mu   sync.Mutex
	rows map[string]*Session
}

// NewMemorySessionStore constructs an empty in-memory session registry.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{rows: make(map[string]*Session)}
}

// Insert persists a freshly issued session row.
func (s *MemorySessionStore) Insert(_ context.Context, sess Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := sess
	s.rows[sess.JTI] = &row
	return nil
}

// Get returns the session row by jti.
func (s *MemorySessionStore) Get(_ context.Context, jti string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[jti]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	return *row, nil
}

// List returns every NON-revoked, non-expired session for the user.
func (s *MemorySessionStore) List(_ context.Context, userID string) ([]Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	out := make([]Session, 0)
	for _, r := range s.rows {
		if r.UserID != userID {
			continue
		}
		if r.RevokedAt != nil {
			continue
		}
		if !r.ExpiresAt.IsZero() && now.After(r.ExpiresAt) {
			continue
		}
		out = append(out, *r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].LastSeenAt.After(out[j].LastSeenAt)
	})
	return out, nil
}

// Revoke stamps revoked_at on a single session.
func (s *MemorySessionStore) Revoke(_ context.Context, jti string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[jti]
	if !ok {
		return ErrSessionNotFound
	}
	now := time.Now().UTC()
	row.RevokedAt = &now
	return nil
}

// RevokeAllForUser stamps every active session for the user as revoked.
func (s *MemorySessionStore) RevokeAllForUser(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for _, r := range s.rows {
		if r.UserID == userID && r.RevokedAt == nil {
			t := now
			r.RevokedAt = &t
		}
	}
	return nil
}

// RevokeAllExcept revokes every session for the user except keepJTI.
func (s *MemorySessionStore) RevokeAllExcept(_ context.Context, userID, keepJTI string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for _, r := range s.rows {
		if r.UserID == userID && r.JTI != keepJTI && r.RevokedAt == nil {
			t := now
			r.RevokedAt = &t
		}
	}
	return nil
}

// Touch updates the last_seen_at + ip/ua columns on a row. Best-effort:
// callers should ignore the error.
func (s *MemorySessionStore) Touch(_ context.Context, jti, ip, ua string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[jti]
	if !ok {
		return ErrSessionNotFound
	}
	row.LastSeenAt = at
	if ip != "" {
		row.IPAddress = ip
	}
	if ua != "" {
		row.UserAgent = ua
	}
	return nil
}

// PostgresSessionStore is the production backend.
type PostgresSessionStore struct{ pool *pgxpool.Pool }

// NewPostgresSessionStore constructs the Postgres-backed store.
func NewPostgresSessionStore(pool *pgxpool.Pool) *PostgresSessionStore {
	return &PostgresSessionStore{pool: pool}
}

// Insert persists a fresh session row.
func (s *PostgresSessionStore) Insert(ctx context.Context, sess Session) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO sessions (jti, user_id, issued_at, expires_at, last_seen_at, ip_address, user_agent)
        VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		sess.JTI, sess.UserID, sess.IssuedAt, sess.ExpiresAt, sess.LastSeenAt, sess.IPAddress, sess.UserAgent)
	return err
}

// Get returns the session row by jti.
func (s *PostgresSessionStore) Get(ctx context.Context, jti string) (Session, error) {
	row := s.pool.QueryRow(ctx, `
        SELECT jti, user_id, issued_at, expires_at, last_seen_at, ip_address, user_agent, revoked_at
          FROM sessions WHERE jti = $1`, jti)
	var sess Session
	if err := row.Scan(&sess.JTI, &sess.UserID, &sess.IssuedAt, &sess.ExpiresAt,
		&sess.LastSeenAt, &sess.IPAddress, &sess.UserAgent, &sess.RevokedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}
	return sess, nil
}

// List returns every non-revoked, non-expired session for the user.
func (s *PostgresSessionStore) List(ctx context.Context, userID string) ([]Session, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT jti, user_id, issued_at, expires_at, last_seen_at, ip_address, user_agent, revoked_at
          FROM sessions
         WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
         ORDER BY last_seen_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Session, 0)
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.JTI, &sess.UserID, &sess.IssuedAt, &sess.ExpiresAt,
			&sess.LastSeenAt, &sess.IPAddress, &sess.UserAgent, &sess.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// Revoke stamps revoked_at = now() on a single session.
func (s *PostgresSessionStore) Revoke(ctx context.Context, jti string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE jti = $1 AND revoked_at IS NULL`, jti)
	return err
}

// RevokeAllForUser revokes every active session for the user.
func (s *PostgresSessionStore) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// RevokeAllExcept revokes every session for the user except keepJTI.
func (s *PostgresSessionStore) RevokeAllExcept(ctx context.Context, userID, keepJTI string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now()
          WHERE user_id = $1 AND jti <> $2 AND revoked_at IS NULL`, userID, keepJTI)
	return err
}

// Touch updates last_seen_at + ip/ua for an active session.
func (s *PostgresSessionStore) Touch(ctx context.Context, jti, ip, ua string, at time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET last_seen_at = $2, ip_address = COALESCE(NULLIF($3, ''), ip_address),
              user_agent = COALESCE(NULLIF($4, ''), user_agent)
          WHERE jti = $1`, jti, at, ip, ua)
	return err
}

// NewJTI returns a fresh random 128-bit identifier in hex form. Used by
// the signing path so every JWT carries a unique jti claim.
func NewJTI() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// SessionCache is the narrow contract the middleware uses to short-
// circuit the per-request DB hit. Backed by Redis in production via
// the redisbus client adapter (see RedisSessionCache below); a no-op
// implementation keeps the auth package compilable when no Redis is
// configured.
type SessionCache interface {
	// State returns "" when the cache has no opinion (caller must hit
	// the DB), "ok" when the jti is known-good, or "revoked" when the
	// jti was explicitly invalidated.
	State(ctx context.Context, jti string) string
	// Cache marks the jti as ok-or-revoked for the supplied TTL.
	Cache(ctx context.Context, jti, state string, ttl time.Duration)
	// MarkRevoked is the broadcast-revocation push so other pods see
	// the change instantly without waiting for the 60s TTL to expire.
	MarkRevoked(ctx context.Context, jti string)
}

// NoopSessionCache is the fallback. Every State call returns "" so the
// middleware always falls through to the DB.
type NoopSessionCache struct{}

// State always returns the empty string (cache miss).
func (NoopSessionCache) State(_ context.Context, _ string) string { return "" }

// Cache is a no-op.
func (NoopSessionCache) Cache(_ context.Context, _, _ string, _ time.Duration) {}

// MarkRevoked is a no-op.
func (NoopSessionCache) MarkRevoked(_ context.Context, _ string) {}
