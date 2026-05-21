// Package integrations holds per-user, per-provider OAuth links — first
// citizen is GitHub. Tokens are persisted via a TokenStore (Memory or
// Postgres) so they survive restarts and can be revoked from a single place.
package integrations

import (
	"context"
	"errors"
	"time"
)

type Kind string

const (
	KindGitHub Kind = "github"
)

// Token is a per-user OAuth grant. AccessToken/RefreshToken are stored
// verbatim — in prod these should be encrypted at rest.
type Token struct {
	UserID       string    `json:"userId"`
	Kind         Kind      `json:"kind"`
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ExternalID   string    `json:"externalId,omitempty"` // e.g. GitHub user id
	ExternalLogin string   `json:"externalLogin,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
}

// TokenStore is the persistence contract. Implementations exist for memory
// (dev) and Postgres (prod).
type TokenStore interface {
	Put(ctx context.Context, t Token) (Token, error)
	Get(ctx context.Context, userID string, kind Kind) (Token, error)
	Delete(ctx context.Context, userID string, kind Kind) error
	ListByUser(ctx context.Context, userID string) ([]Token, error)
	// FindByExternal returns the userID linked to the given external identity
	// (kind + externalID). ErrNotFound when nothing matches. Used to route
	// OAuth-login callbacks to an existing account.
	FindByExternal(ctx context.Context, kind Kind, externalID string) (string, error)
}

var ErrNotFound = errors.New("integration not connected")
