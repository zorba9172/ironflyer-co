// Package auth — superuser bootstrap.
//
// When IRONFLYER_SUPERUSER_EMAIL and IRONFLYER_SUPERUSER_PASSWORD are
// both set, main.go calls EnsureSuperuser at boot. The helper is
// idempotent: it looks the user up by email, creates them via the
// existing Signup helper (bcrypt path) when absent, then guarantees the
// platform_operator role and a verified email. The password is never
// logged.
package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// EnsureSuperuser provisions or upgrades a privileged operator account.
// Returns the resolved user. Pass an EmailVerifier (the user store
// implements it on both backends) so the EmailVerifiedAt stamp can be
// applied; pass a RoleSetter to promote the role. Both args are
// required for the operation to succeed — pass nils explicitly when
// running in a mode that doesn't support roles (the helper will return
// an error explaining what's missing).
func EnsureSuperuser(ctx context.Context, svc *Service, store UserStore, verifier EmailVerifier, roles RoleSetter, email, password string, logger zerolog.Logger) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return User{}, errors.New("superuser email + password required")
	}
	if svc == nil || store == nil {
		return User{}, errors.New("auth service + user store required")
	}
	if roles == nil {
		return User{}, errors.New("user store does not support roles (RoleSetter)")
	}
	if verifier == nil {
		return User{}, errors.New("user store does not support email verification (EmailVerifier)")
	}

	u, _, err := store.GetByEmail(ctx, email)
	created := false
	switch {
	case err == nil:
		// already exists
	case errors.Is(err, ErrUserNotFound):
		u, _, err = svc.Signup(ctx, SignupInput{Email: email, Name: "Superuser", Password: password})
		if err != nil {
			return User{}, err
		}
		created = true
	default:
		return User{}, err
	}

	if !u.HasRole(RolePlatformOperator) {
		next := append([]string{}, u.Roles...)
		next = append(next, RolePlatformOperator)
		if err := roles.SetRoles(ctx, u.ID, next); err != nil {
			return User{}, err
		}
	}

	if u.EmailVerifiedAt == nil || u.EmailVerifiedAt.IsZero() {
		if err := verifier.MarkEmailVerified(ctx, u.ID, time.Now().UTC()); err != nil {
			return User{}, err
		}
	}

	if refreshed, rerr := store.GetByID(ctx, u.ID); rerr == nil {
		u = refreshed
	}

	logger.Info().
		Str("user_id", u.ID).
		Str("email", u.Email).
		Bool("created", created).
		Strs("roles", u.Roles).
		Msg("superuser provisioned")

	return u, nil
}
