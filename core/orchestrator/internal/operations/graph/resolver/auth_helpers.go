package resolver

// Helpers consumed by auth.resolver.go. Live in their own file so
// gqlgen's "regenerate" pass does not strip them when the resolver
// file is rewritten.

import (
	"context"
	"strings"
	"time"

	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/customer/notify"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
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

// maybeDispatchNewDeviceLogin fires KindNewDeviceLogin when the
// per-request (IP, UA) pair does not match the user's most recent
// existing session row. The Sessions store has not yet recorded the
// current login (Insert lands inside issueAndPersistSession), so a
// "first ever sign-in" cleanly skips the alert. Heuristic: differ on
// IP OR on the first 30 chars of UA (browser + OS family).
//
// Fire-and-forget — payload-level idempotency (sha256(IP|UA|YYYYMMDD))
// guarantees that repeated logins on the same device the same day
// don't spam, and we never block the resolver response on dispatch.
func (r *Resolver) maybeDispatchNewDeviceLogin(ctx context.Context, u auth.User) {
	if r == nil || r.Notifier == nil {
		return
	}
	info := auth.RequestInfoFromContext(ctx)
	if info.IPAddress == "" && info.UserAgent == "" {
		return
	}
	if r.Sessions == nil {
		return
	}
	prior, err := r.Sessions.List(ctx, u.ID)
	if err != nil || len(prior) == 0 {
		return
	}
	last := prior[0]
	if !isNewDevice(last.IPAddress, last.UserAgent, info.IPAddress, info.UserAgent) {
		return
	}
	payload := notify.NewDeviceLoginPayload{
		Name:       displayName(u),
		IPAddress:  info.IPAddress,
		UserAgent:  info.UserAgent,
		LoggedInAt: time.Now().UTC(),
	}
	if err := r.Notifier.Dispatch(ctx, u.ID, u.Email, notify.KindNewDeviceLogin, payload); err != nil {
		r.Logger.Warn().Err(err).Str("user_id", u.ID).Msg("auth: new-device login dispatch failed")
	}
}

// isNewDevice reports whether the (ip, ua) pair represents a device
// distinct from the prior recorded session. Logic:
//
//   - any UA prefix mismatch on the first 30 chars (covers browser +
//     OS family) flags new device.
//   - any IP mismatch flags new device too — mobile networks rotate
//     IPs frequently, but the user-facing notice ("from a new IP")
//     remains accurate and the same-day dedupe key keeps spam down.
func isNewDevice(priorIP, priorUA, ip, ua string) bool {
	if strings.TrimSpace(ip) == "" && strings.TrimSpace(ua) == "" {
		return false
	}
	if priorIP != ip {
		return true
	}
	return uaPrefix(priorUA) != uaPrefix(ua)
}

func uaPrefix(ua string) string {
	if len(ua) <= 30 {
		return ua
	}
	return ua[:30]
}

// dispatchMFAEnabled fires KindMFAEnabled after an MFA enrollment is
// successfully confirmed. Fire-and-forget — errors log but never block
// the resolver response. Exposed as a Resolver method so the
// confirmMfaEnrollment resolver can call it in one line once the
// resolver lands, without needing to know about the dispatcher layout.
func (r *Resolver) dispatchMFAEnabled(ctx context.Context, u auth.User, enrolledAt time.Time) {
	if r == nil || r.Notifier == nil {
		return
	}
	if enrolledAt.IsZero() {
		enrolledAt = time.Now().UTC()
	}
	if err := r.Notifier.Dispatch(ctx, u.ID, u.Email, notify.KindMFAEnabled, notify.MFAEnabledPayload{
		Name:       displayName(u),
		EnrolledAt: enrolledAt,
	}); err != nil {
		r.Logger.Warn().Err(err).Str("user_id", u.ID).Msg("auth: mfa-enabled dispatch failed")
	}
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
// embeds. Points at the Next.js page actually shipped under
// clients/web/app/login/reset/page.tsx.
func (r *Resolver) resetURL(token string) string {
	base := strings.TrimRight(r.WebBaseURL, "/")
	if base == "" {
		base = "http://localhost:3000"
	}
	return base + "/login/reset?token=" + token
}
