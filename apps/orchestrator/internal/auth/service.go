package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Service is the entry point the HTTP layer talks to.
type Service struct {
	store      UserStore
	signingKey []byte
	issuer     string
	ttl        time.Duration
}

func NewService(store UserStore, signingKey []byte, issuer string, ttl time.Duration) *Service {
	if ttl == 0 {
		ttl = 7 * 24 * time.Hour
	}
	if issuer == "" {
		issuer = "ironflyer"
	}
	return &Service{store: store, signingKey: signingKey, issuer: issuer, ttl: ttl}
}

type SignupInput struct {
	Email, Name, Password string
}

func (s *Service) Signup(ctx context.Context, in SignupInput) (User, string, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !strings.Contains(in.Email, "@") {
		return User{}, "", errors.New("invalid email")
	}
	if len(in.Password) < 8 {
		return User{}, "", errors.New("password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, "", err
	}
	u, err := s.store.Create(ctx, in.Email, in.Name, string(hash))
	if err != nil {
		return User{}, "", err
	}
	token, err := s.issueToken(u)
	if err != nil {
		return User{}, "", err
	}
	return u, token, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (User, string, error) {
	u, hash, err := s.store.GetByEmail(ctx, email)
	if err != nil {
		// Mask user-existence to avoid enumeration.
		if errors.Is(err, ErrUserNotFound) {
			return User{}, "", ErrBadPassword
		}
		return User{}, "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return User{}, "", ErrBadPassword
	}
	token, err := s.issueToken(u)
	if err != nil {
		return User{}, "", err
	}
	return u, token, nil
}

func (s *Service) Verify(ctx context.Context, tokenStr string) (User, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return s.signingKey, nil
	})
	if err != nil {
		return User{}, err
	}
	c, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return User{}, errors.New("invalid token")
	}
	return s.store.GetByID(ctx, c.Subject)
}

func (s *Service) GetByID(ctx context.Context, id string) (User, error) {
	return s.store.GetByID(ctx, id)
}

// GetByIDs batches a user lookup by id. The graph/loaders package
// invokes this through the wider auth.UserStore contract so resolvers
// can use the dataloader-backed *Loaders.UserByID without each call
// site having to know which backend is live.
func (s *Service) GetByIDs(ctx context.Context, ids []string) (map[string]User, error) {
	return s.store.GetByIDs(ctx, ids)
}

// UserStore returns the underlying persistence layer. Reserved for
// per-request infrastructure (dataloaders, admin tooling) that needs
// direct access to batch APIs the Service does not yet wrap.
func (s *Service) UserStore() UserStore { return s.store }

func (s *Service) SetPlan(ctx context.Context, id, plan string) error {
	return s.store.SetPlan(ctx, id, plan)
}

// SetTelemetryOptOut persists the per-user telemetry opt-out flag.
// Resolvers call this when the user flips the privacy toggle.
func (s *Service) SetTelemetryOptOut(ctx context.Context, id string, opt bool) error {
	return s.store.SetTelemetryOptOut(ctx, id, opt)
}

// Delete removes a user record. Tokens issued before deletion remain
// signature-valid until they expire; verification still succeeds because
// our middleware doesn't re-query the user store, but every subsequent
// request that resolves the user identity will fail to load the row.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}

// IssueToken signs a JWT for an already-validated user. Used by OAuth login
// flows where the user identity was established externally.
func (s *Service) IssueToken(u User) (string, error) {
	return s.issueToken(u)
}

// EnsureUserByID returns an existing user. Wrapper kept for symmetry with
// EnsureUserByEmail.
func (s *Service) EnsureUserByID(ctx context.Context, id string) (User, error) {
	return s.store.GetByID(ctx, id)
}

// LookupByEmail is a read-only convenience for non-auth call sites
// (collab invites, admin tooling) that need to map an email to a user.
// Returns ErrUserNotFound when no row matches. The password hash is
// deliberately dropped — only the auth package itself ever reads it.
func (s *Service) LookupByEmail(ctx context.Context, email string) (User, bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return User{}, false, errors.New("email required")
	}
	u, _, err := s.store.GetByEmail(ctx, email)
	if err != nil {
		return User{}, false, err
	}
	return u, true, nil
}

// EnsureUserByEmail finds or creates a user identified by email — used by
// OAuth login. The created user has a PasswordlessHash, so password login
// won't work on the account (only the OAuth provider).
func (s *Service) EnsureUserByEmail(ctx context.Context, email, name string) (User, bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return User{}, false, errors.New("email required")
	}
	if u, _, err := s.store.GetByEmail(ctx, email); err == nil {
		return u, false, nil
	} else if !errors.Is(err, ErrUserNotFound) {
		return User{}, false, err
	}
	u, err := s.store.Create(ctx, email, name, PasswordlessHash)
	if err != nil {
		return User{}, false, err
	}
	return u, true, nil
}

type claims struct {
	Email string `json:"email"`
	Plan  string `json:"plan,omitempty"`
	jwt.RegisteredClaims
}

func (s *Service) issueToken(u User) (string, error) {
	tok, _, err := s.IssueTokenWithJTI(u, NewJTI(), 0)
	return tok, err
}

// IssueTokenWithJTI mints a JWT stamped with the supplied jti claim so
// the sessions table can key off it for revocation. When jti is empty
// a fresh one is generated. When ttl is zero the service's default ttl
// is used.
func (s *Service) IssueTokenWithJTI(u User, jti string, ttl time.Duration) (string, string, error) {
	if jti == "" {
		jti = NewJTI()
	}
	if ttl <= 0 {
		ttl = s.ttl
	}
	now := time.Now()
	c := claims{
		Email: u.Email, Plan: u.Plan,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        jti,
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.signingKey)
	return tok, jti, err
}

// SigningKey exposes the HS256 key so the sessions middleware can
// re-parse a JWT to extract its jti claim without re-running the full
// Service.Verify lookup chain.
func (s *Service) SigningKey() []byte { return s.signingKey }

// TTL exposes the default token lifetime so the sessions writer can
// compute the row's expires_at without reparsing the JWT.
func (s *Service) TTL() time.Duration { return s.ttl }

// compareBcryptHash is exposed inside the auth package so the email-
// change flow can re-verify a user's password without dragging the
// resolver into the bcrypt package directly.
func compareBcryptHash(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
