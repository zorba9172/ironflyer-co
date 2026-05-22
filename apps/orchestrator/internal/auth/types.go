// Package auth handles user identity: signup, login, JWT issuance and
// verification, and the chi middleware that materialises the authenticated
// user on the request context.
package auth

import (
	"context"
	"errors"
	"time"
)

// User is the identity record. Password hashes never leave the auth package.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name,omitempty"`
	Plan      string    `json:"plan,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// UserStore is the persistence contract. Memory and Postgres back it.
type UserStore interface {
	Create(ctx context.Context, email, name, passwordHash string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, string /*hash*/, error)
	GetByID(ctx context.Context, id string) (User, error)
	SetPlan(ctx context.Context, id, plan string) error
	Delete(ctx context.Context, id string) error
}

// PasswordlessHash is a marker stored in the password_hash column for users
// created via external OAuth (no password login). Bcrypt will never produce
// this string, so password auth against it always fails.
const PasswordlessHash = "!oauth-only"

var (
	ErrUserExists   = errors.New("user already exists")
	ErrUserNotFound = errors.New("user not found")
	ErrBadPassword  = errors.New("invalid email or password")
)

// ctxKey is unexported so callers must use the package's helpers.
type ctxKey struct{}

var userCtxKey = ctxKey{}

// WithUser attaches a user to the context.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

// FromContext returns the authenticated user or false.
func FromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userCtxKey).(User)
	return u, ok
}
