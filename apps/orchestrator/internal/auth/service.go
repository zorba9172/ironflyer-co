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

func (s *Service) SetPlan(ctx context.Context, id, plan string) error {
	return s.store.SetPlan(ctx, id, plan)
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
	now := time.Now()
	c := claims{
		Email: u.Email, Plan: u.Plan,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.signingKey)
}
