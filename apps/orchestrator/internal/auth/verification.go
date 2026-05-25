// Package auth — email verification on signup.
//
// Flow:
//
//  1. On signup, IssueVerification mints a 32-byte random token, stores
//     only the SHA-256 hash in email_verifications (kind=signup, TTL 48h),
//     and emails the user a link to /auth/verify?token=<plain>.
//  2. The web client posts the token back via the `verifyEmail` mutation.
//     ConsumeVerification validates the hash, marks the row used, and
//     flips users.email_verified_at = now().
//  3. requireVerifiedEmail(ctx) gates paid-plan / deploy / custom-domain
//     mutations so an unverified user can still sign in + browse but
//     can't escalate to anything billable.
//
// Both Memory and Postgres backends implement VerificationStore so the
// dev box exercises the path without a Postgres dependency. The plain
// token is returned to the caller exactly once so the email sender can
// embed it in the link; only the hash ever reaches storage.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VerificationKind discriminates the email-verifications.kind column so
// the same table backs both the signup flow and the email-change flow.
type VerificationKind string

const (
	// VerificationSignup is the post-signup verification flow.
	VerificationSignup VerificationKind = "signup"
	// VerificationEmailChange proves ownership of a NEW email address
	// during the email-change flow. Confirming the token flips the
	// user's email + revokes every session for safety.
	VerificationEmailChange VerificationKind = "change"
)

// ErrVerificationNotFound is returned when a token does not exist OR
// has already been used. The caller MUST NOT distinguish the two —
// either way the right answer to the user is "invalid or expired link".
var ErrVerificationNotFound = errors.New("verification token not found")

// ErrVerificationExpired is returned when a token exists but its TTL
// has elapsed. Treated as the same client-facing error as NotFound; the
// distinction matters only for metrics / audit.
var ErrVerificationExpired = errors.New("verification token expired")

// VerificationRecord is the row shape callers consume after a successful
// consume. NewEmail is only populated for change-flow rows.
type VerificationRecord struct {
	UserID    string
	Kind      VerificationKind
	NewEmail  string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// VerificationStore persists email-verification tokens. Memory + Postgres
// both implement it so the dev box can exercise the flow without a DB.
type VerificationStore interface {
	// Insert stores the SHA-256 hash of the token. The plaintext is
	// returned to the issuer exactly once and emailed; storage never
	// sees the original.
	Insert(ctx context.Context, tokenHash, userID string, kind VerificationKind, newEmail string, expiresAt time.Time) error
	// Consume looks up a token by hash, refuses expired / used rows, and
	// stamps used_at on success. Returns the record so the resolver can
	// route the right post-consume action.
	Consume(ctx context.Context, tokenHash string) (VerificationRecord, error)
	// LatestForUser returns the most recently issued NON-consumed row
	// for the user so the throttler can enforce a per-user resend
	// minimum-interval cap.
	LatestForUser(ctx context.Context, userID string, kind VerificationKind) (VerificationRecord, error)
}

// VerificationTTL is the default lifetime of a signup verification token.
const VerificationTTL = 48 * time.Hour

// NewVerificationToken returns a freshly minted 32-byte random token in
// hex form (64 chars) along with its SHA-256 hash. The plaintext is for
// the email; only the hash should reach storage.
func NewVerificationToken() (plain, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	plain = hex.EncodeToString(buf)
	hash = hashToken(plain)
	return plain, hash, nil
}

// hashToken hashes a verification / reset / session token with SHA-256.
// Reused across the verification + password_reset modules so the wire
// format is uniform.
func hashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// HashTokenForResolver is the exported variant of hashToken so the
// resolver layer can recompute the storage hash from a plaintext
// token without re-implementing the algorithm. Kept as a thin alias
// (rather than dropping `hashToken`'s lowercase form) so package-
// internal callers stay terse.
func HashTokenForResolver(plain string) string { return hashToken(plain) }

// MarkEmailVerified flips users.email_verified_at to now(). Both store
// backends expose it via the EmailVerifier interface so the resolver
// doesn't fan out to multiple typed call sites.
type EmailVerifier interface {
	MarkEmailVerified(ctx context.Context, userID string, at time.Time) error
}

// requireVerifiedEmail is the gate every paid / deploy / custom-domain
// resolver should call before doing real work. Returns nil when the
// authenticated user has email_verified_at set; safeerror.NotVerified()
// otherwise. The actual safeerror wiring lives in the safeerror package
// (see safeerror.NotVerified) so the helper here only handles the
// context lookup + nil-check.
func RequireVerifiedEmail(u User) error {
	if u.EmailVerifiedAt == nil || u.EmailVerifiedAt.IsZero() {
		return ErrEmailNotVerified
	}
	return nil
}

// ErrEmailNotVerified is the sentinel resolvers translate into
// safeerror.NotVerified for the GraphQL client.
var ErrEmailNotVerified = errors.New("email not verified")

// MemoryVerificationStore is the dev-mode implementation. Bounded by
// caller behaviour (no GC); fine for the dev box.
type MemoryVerificationStore struct {
	mu      sync.Mutex
	byHash  map[string]*verificationRow
	byUser  map[string][]*verificationRow // user_id -> rows (any kind)
}

type verificationRow struct {
	tokenHash string
	rec       VerificationRecord
	usedAt    *time.Time
}

// NewMemoryVerificationStore constructs an empty in-memory store.
func NewMemoryVerificationStore() *MemoryVerificationStore {
	return &MemoryVerificationStore{
		byHash: make(map[string]*verificationRow),
		byUser: make(map[string][]*verificationRow),
	}
}

// Insert stores a fresh verification row.
func (s *MemoryVerificationStore) Insert(_ context.Context, tokenHash, userID string, kind VerificationKind, newEmail string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := &verificationRow{
		tokenHash: tokenHash,
		rec: VerificationRecord{
			UserID:    userID,
			Kind:      kind,
			NewEmail:  strings.ToLower(strings.TrimSpace(newEmail)),
			ExpiresAt: expiresAt,
			CreatedAt: time.Now().UTC(),
		},
	}
	s.byHash[tokenHash] = row
	s.byUser[userID] = append(s.byUser[userID], row)
	return nil
}

// Consume marks a row used and returns the record on success.
func (s *MemoryVerificationStore) Consume(_ context.Context, tokenHash string) (VerificationRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.byHash[tokenHash]
	if !ok {
		return VerificationRecord{}, ErrVerificationNotFound
	}
	if row.usedAt != nil {
		return VerificationRecord{}, ErrVerificationNotFound
	}
	if time.Now().UTC().After(row.rec.ExpiresAt) {
		return VerificationRecord{}, ErrVerificationExpired
	}
	now := time.Now().UTC()
	row.usedAt = &now
	return row.rec, nil
}

// LatestForUser returns the newest NON-consumed row of the requested kind.
func (s *MemoryVerificationStore) LatestForUser(_ context.Context, userID string, kind VerificationKind) (VerificationRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := s.byUser[userID]
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if r.usedAt != nil {
			continue
		}
		if r.rec.Kind != kind {
			continue
		}
		return r.rec, nil
	}
	return VerificationRecord{}, ErrVerificationNotFound
}

// PostgresVerificationStore is the production backend.
type PostgresVerificationStore struct{ pool *pgxpool.Pool }

// NewPostgresVerificationStore wires the Postgres-backed store.
func NewPostgresVerificationStore(pool *pgxpool.Pool) *PostgresVerificationStore {
	return &PostgresVerificationStore{pool: pool}
}

// Insert persists a fresh verification row.
func (s *PostgresVerificationStore) Insert(ctx context.Context, tokenHash, userID string, kind VerificationKind, newEmail string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO email_verifications (token_hash, user_id, kind, new_email, expires_at)
        VALUES ($1, $2, $3, $4, $5)`,
		tokenHash, userID, string(kind), strings.ToLower(strings.TrimSpace(newEmail)), expiresAt)
	return err
}

// Consume stamps used_at and returns the record on success.
func (s *PostgresVerificationStore) Consume(ctx context.Context, tokenHash string) (VerificationRecord, error) {
	row := s.pool.QueryRow(ctx, `
        UPDATE email_verifications
           SET used_at = now()
         WHERE token_hash = $1 AND used_at IS NULL
        RETURNING user_id, kind, new_email, expires_at, created_at`, tokenHash)
	var rec VerificationRecord
	var kind string
	if err := row.Scan(&rec.UserID, &kind, &rec.NewEmail, &rec.ExpiresAt, &rec.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VerificationRecord{}, ErrVerificationNotFound
		}
		return VerificationRecord{}, err
	}
	rec.Kind = VerificationKind(kind)
	if time.Now().UTC().After(rec.ExpiresAt) {
		return VerificationRecord{}, ErrVerificationExpired
	}
	return rec, nil
}

// LatestForUser returns the newest pending row of the given kind.
func (s *PostgresVerificationStore) LatestForUser(ctx context.Context, userID string, kind VerificationKind) (VerificationRecord, error) {
	row := s.pool.QueryRow(ctx, `
        SELECT user_id, kind, new_email, expires_at, created_at
          FROM email_verifications
         WHERE user_id = $1 AND kind = $2 AND used_at IS NULL
         ORDER BY created_at DESC
         LIMIT 1`, userID, string(kind))
	var rec VerificationRecord
	var k string
	if err := row.Scan(&rec.UserID, &k, &rec.NewEmail, &rec.ExpiresAt, &rec.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VerificationRecord{}, ErrVerificationNotFound
		}
		return VerificationRecord{}, err
	}
	rec.Kind = VerificationKind(k)
	return rec, nil
}

// MarkEmailVerified on the Postgres user store. Postgres pool is shared
// with the existing PostgresUserStore — this helper is a thin Exec.
func (s *PostgresUserStore) MarkEmailVerified(ctx context.Context, userID string, at time.Time) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET email_verified_at = $1 WHERE id = $2`, at, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// SetEmail flips the user's primary email. Returns ErrUserExists if the
// new address is already in use (unique constraint trips). Used by the
// confirmEmailChange flow.
func (s *PostgresUserStore) SetEmail(ctx context.Context, userID, newEmail string) error {
	newEmail = strings.ToLower(strings.TrimSpace(newEmail))
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET email = $1, email_verified_at = now() WHERE id = $2`,
		newEmail, userID)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrUserExists
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// MarkEmailVerified on the memory store sets the in-memory User row.
func (s *MemoryUserStore) MarkEmailVerified(_ context.Context, userID string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byID[userID]
	if !ok {
		return ErrUserNotFound
	}
	t := at
	u.EmailVerifiedAt = &t
	s.byID[userID] = u
	return nil
}

// SetEmail flips the in-memory user's email. Trips ErrUserExists when
// the new address collides with another user.
func (s *MemoryUserStore) SetEmail(_ context.Context, userID, newEmail string) error {
	newEmail = strings.ToLower(strings.TrimSpace(newEmail))
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.byEmail[newEmail]; ok && existing != userID {
		return ErrUserExists
	}
	u, ok := s.byID[userID]
	if !ok {
		return ErrUserNotFound
	}
	delete(s.byEmail, u.Email)
	u.Email = newEmail
	now := time.Now().UTC()
	u.EmailVerifiedAt = &now
	s.byID[userID] = u
	s.byEmail[newEmail] = userID
	return nil
}

// EnsureVerificationID is a tiny convenience used by tests / admin
// tooling — guarantees a row exists for the user with the given kind.
// Returns the plain token on success.
//
// (Kept here rather than the service so a future admin command can use
// it without going through the rate-limited resend mutation.)
func EnsureVerificationID(ctx context.Context, store VerificationStore, userID string, kind VerificationKind, newEmail string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = VerificationTTL
	}
	plain, hash, err := NewVerificationToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().UTC().Add(ttl)
	if err := store.Insert(ctx, hash, userID, kind, newEmail, expires); err != nil {
		return "", err
	}
	return plain, nil
}

// VerificationID is the public ID helper that bumps a fresh UUID for
// the verification's auxiliary ID field. Kept for symmetry with the
// other auth modules that derive their primary key from uuid.NewString.
func VerificationID() string { return uuid.NewString() }
