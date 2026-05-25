package resolver

// Helpers consumed by auth.resolver.go. Live in their own file so
// gqlgen's "regenerate" pass does not strip them when the resolver
// file is rewritten.

import (
	"context"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/auth"
	"ironflyer/apps/orchestrator/internal/graph/model"
)

func toModelUser(u auth.User) *model.User {
	out := &model.User{
		ID:              u.ID,
		Email:           u.Email,
		CreatedAt:       u.CreatedAt,
		TelemetryOptOut: u.TelemetryOptOut,
	}
	if u.Name != "" {
		n := u.Name
		out.Name = &n
	}
	if u.Plan != "" {
		p := u.Plan
		out.Plan = &p
	}
	if u.OrgID != "" {
		o := u.OrgID
		out.OrgID = &o
	}
	if u.EmailVerifiedAt != nil {
		t := *u.EmailVerifiedAt
		out.EmailVerifiedAt = &t
	}
	return out
}

// issueAndPersistSession mints a fresh JWT with a unique jti,
// persists the session row (when r.Sessions is wired) so it can be
// listed / revoked, and returns the canonical Session envelope.
func (r *Resolver) issueAndPersistSession(ctx context.Context, u auth.User) (*model.Session, error) {
	if r.Auth == nil {
		return nil, gqlNotConfigured("auth")
	}
	jti := auth.NewJTI()
	token, _, err := r.Auth.IssueTokenWithJTI(u, jti, 0)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	exp := now.Add(r.Auth.TTL())
	if r.Sessions != nil {
		_ = r.Sessions.Insert(ctx, auth.Session{
			JTI:        jti,
			UserID:     u.ID,
			IssuedAt:   now,
			ExpiresAt:  exp,
			LastSeenAt: now,
		})
	}
	mu := toModelUser(u)
	jtiCopy := jti
	expCopy := exp
	current := true
	return &model.Session{
		User:      *mu,
		Token:     token,
		ExpiresAt: &expCopy,
		Jti:       &jtiCopy,
		Current:   &current,
	}, nil
}

// displayName returns the user's display name, falling back to the
// email's local-part so transactional templates never render an empty
// salutation.
func displayName(u auth.User) string {
	if strings.TrimSpace(u.Name) != "" {
		return u.Name
	}
	if i := strings.IndexByte(u.Email, '@'); i > 0 {
		return u.Email[:i]
	}
	return u.Email
}

// verifyURL builds the externally-visible URL the verification email
// embeds. Falls back to localhost when the web base URL is not wired.
func (r *Resolver) verifyURL(token string) string {
	base := strings.TrimRight(r.WebBaseURL, "/")
	if base == "" {
		base = "http://localhost:3000"
	}
	return base + "/auth/verify?token=" + token
}

// resetURL builds the externally-visible URL the password-reset email
// embeds.
func (r *Resolver) resetURL(token string) string {
	base := strings.TrimRight(r.WebBaseURL, "/")
	if base == "" {
		base = "http://localhost:3000"
	}
	return base + "/auth/reset?token=" + token
}
