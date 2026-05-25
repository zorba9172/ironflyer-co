// Package auth — email change flow.
//
// Two-step, with strong safety bias:
//
//  1. requestEmailChange(newEmail, currentPassword)
//     - Verifies the current password (no drive-by hijack of a forgotten
//       laptop). On mismatch we return safeerror.BadInput so the client
//     - Mints a token, stores it in email_verifications with
//       kind='change' + new_email = the requested address, TTL 48h.
//     - Sends the verification email to the NEW address (NOT the
//       current one — proving the user actually owns the new mailbox is
//       the entire point of the flow).
//  2. confirmEmailChange(token)
//     - Validates the token; on success flips users.email to the new
//       value, stamps email_verified_at = now(), and revokes every
//       session for that user so the next request from any device
//       requires a fresh sign-in.
//
// The "send to NEW address only" + "revoke all sessions on confirm"
// pair is the canonical commercial pattern. Send-to-old would only
// enable account recovery, and recovery is handled by the password
// reset flow — kept separate so the threat models don't mix.
package auth

import (
	"context"
	"errors"
	"strings"
	"time"
)

// EmailChangeTTL is the lifetime of a change-verification token. Longer
// than the reset flow (1h) because the user may need to switch to the
// new mailbox to receive the message.
const EmailChangeTTL = 48 * time.Hour

// EmailChangeInput is the resolver-facing input. Kept here so the
// resolver doesn't reach into the GraphQL model types directly for the
// password-confirmation contract.
type EmailChangeInput struct {
	NewEmail        string
	CurrentPassword string
}

// EmailChangeRequest is the bookkeeping returned from RequestEmailChange.
// The PlainToken is the value that goes into the email URL; the caller
// must NOT log it.
type EmailChangeRequest struct {
	PlainToken string
	NewEmail   string
	ExpiresAt  time.Time
}

// ErrEmailChangeInvalidPassword is returned when the current-password
// check fails. The resolver translates this into safeerror.BadInput so
// the client can show the field-level error.
var ErrEmailChangeInvalidPassword = errors.New("current password mismatch")

// RequestEmailChange validates the supplied password against the stored
// hash and, on success, issues a fresh change-verification token.
//
// The caller is responsible for emailing the resulting PlainToken to
// the NEW address. The store-side row carries kind='change' + the
// new_email so confirmEmailChange knows what to flip the user to.
func RequestEmailChange(ctx context.Context, users UserStore, vstore VerificationStore, userID, newEmail, currentPassword string, ttl time.Duration) (EmailChangeRequest, error) {
	newEmail = strings.ToLower(strings.TrimSpace(newEmail))
	if newEmail == "" || !strings.Contains(newEmail, "@") {
		return EmailChangeRequest{}, errors.New("invalid email")
	}
	u, err := users.GetByID(ctx, userID)
	if err != nil {
		return EmailChangeRequest{}, err
	}
	// Verify the current password — we re-fetch by email to pull the
	// hash (UserStore.GetByID intentionally omits the hash).
	_, hash, err := users.GetByEmail(ctx, u.Email)
	if err != nil {
		return EmailChangeRequest{}, err
	}
	if err := compareBcryptHash(hash, currentPassword); err != nil {
		return EmailChangeRequest{}, ErrEmailChangeInvalidPassword
	}
	if ttl <= 0 {
		ttl = EmailChangeTTL
	}
	plain, hsh, err := NewVerificationToken()
	if err != nil {
		return EmailChangeRequest{}, err
	}
	expires := time.Now().UTC().Add(ttl)
	if err := vstore.Insert(ctx, hsh, userID, VerificationEmailChange, newEmail, expires); err != nil {
		return EmailChangeRequest{}, err
	}
	return EmailChangeRequest{PlainToken: plain, NewEmail: newEmail, ExpiresAt: expires}, nil
}
