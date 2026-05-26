// Package oauth wires social sign-in (Google, GitHub) on top of the
// existing auth.Service. Each provider returns a UserProfile; the
// Handler maps it to a user via EnsureUserByEmail and ships the JWT
// back through a URL fragment so it never reaches access logs.
package oauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/customer/notify"
)

const (
	stateTTL          = 10 * time.Minute
	defaultFragmentOK = "#token=%s&expiresAt=%s"
)

// Provider abstracts a single OAuth identity provider.
type Provider interface {
	Name() string
	AuthorizeURL(state string) string
	Exchange(ctx context.Context, code string) (UserProfile, error)
}

// UserProfile is the normalised view the Handler operates on.
type UserProfile struct {
	Email          string
	Name           string
	ProviderUserID string
	EmailVerified  bool
}

// Handler turns the chi-routed /auth/{provider}/{start|callback} pair
// into a working OAuth flow against the provided providers.
type Handler struct {
	providers           map[string]Provider
	auth                *auth.Service
	sessions            auth.SessionStore
	stateSecret         []byte
	logger              zerolog.Logger
	defaultPostLoginURL string
	notifier            *notify.Dispatcher
}

// Config bundles Handler dependencies; main.go assembles this once.
type Config struct {
	Providers           []Provider
	Auth                *auth.Service
	Sessions            auth.SessionStore
	StateSecret         []byte
	Logger              zerolog.Logger
	DefaultPostLoginURL string
	// Notifier ships the welcome email when EnsureUserByEmail
	// returns isNew=true. Nil-safe; callers may omit it during
	// partial wiring.
	Notifier *notify.Dispatcher
}

// New constructs a Handler. Returns nil when no providers are
// configured so the router can skip mounting the routes entirely.
func New(cfg Config) *Handler {
	if len(cfg.Providers) == 0 || cfg.Auth == nil || len(cfg.StateSecret) == 0 {
		return nil
	}
	m := make(map[string]Provider, len(cfg.Providers))
	for _, p := range cfg.Providers {
		if p == nil {
			continue
		}
		m[p.Name()] = p
	}
	if len(m) == 0 {
		return nil
	}
	return &Handler{
		providers:           m,
		auth:                cfg.Auth,
		sessions:            cfg.Sessions,
		stateSecret:         cfg.StateSecret,
		logger:              cfg.Logger,
		defaultPostLoginURL: cfg.DefaultPostLoginURL,
		notifier:            cfg.Notifier,
	}
}

// Start redirects the browser to the provider's authorize URL,
// stamping a signed state token that carries the post-login redirect.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "provider")
	p, ok := h.providers[name]
	if !ok {
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}
	returnTo := h.redirectTarget(r)
	state, err := h.signState(returnTo, time.Now().Add(stateTTL))
	if err != nil {
		h.logger.Error().Err(err).Str("provider", name).Msg("oauth: sign state")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, p.AuthorizeURL(state), http.StatusFound)
}

// Callback verifies state, exchanges the code, ensures the user, and
// redirects to ${returnTo}#token=...&expiresAt=... so the token rides
// in a URL fragment (never reaches the Referer header or server logs).
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "provider")
	p, ok := h.providers[name]
	if !ok {
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Error(w, "oauth provider rejected the request: "+errParam, http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}
	returnTo, err := h.verifyState(state)
	if err != nil {
		h.logger.Warn().Err(err).Str("provider", name).Msg("oauth: state verify failed")
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	profile, err := p.Exchange(r.Context(), code)
	if err != nil {
		h.logger.Warn().Err(err).Str("provider", name).Msg("oauth: exchange failed")
		http.Error(w, "oauth exchange failed", http.StatusBadGateway)
		return
	}
	if !profile.EmailVerified {
		http.Error(w, "your "+name+" account email is not verified; verify it with the provider then try again", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(profile.Email) == "" {
		http.Error(w, "provider returned no email", http.StatusBadRequest)
		return
	}
	u, isNew, err := h.auth.EnsureUserByEmail(r.Context(), profile.Email, profile.Name)
	if err != nil {
		h.logger.Error().Err(err).Str("provider", name).Msg("oauth: ensure user")
		http.Error(w, "could not provision account", http.StatusInternalServerError)
		return
	}
	if isNew && h.notifier != nil {
		displayName := u.Name
		if strings.TrimSpace(displayName) == "" {
			if i := strings.IndexByte(u.Email, '@'); i > 0 {
				displayName = u.Email[:i]
			}
		}
		if derr := h.notifier.Dispatch(r.Context(), u.ID, u.Email, notify.KindWelcome, notify.WelcomePayload{
			Name:  displayName,
			Email: u.Email,
		}); derr != nil {
			h.logger.Warn().Err(derr).Str("provider", name).Str("user_id", u.ID).Msg("oauth: welcome email dispatch failed")
		}
	}
	jti := auth.NewJTI()
	token, _, err := h.auth.IssueTokenWithJTI(u, jti, 0)
	if err != nil {
		h.logger.Error().Err(err).Str("provider", name).Msg("oauth: issue token")
		http.Error(w, "could not issue session", http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	exp := now.Add(h.auth.TTL())
	if h.sessions != nil {
		if serr := h.sessions.Insert(r.Context(), auth.Session{
			JTI:        jti,
			UserID:     u.ID,
			IssuedAt:   now,
			ExpiresAt:  exp,
			LastSeenAt: now,
			IPAddress:  clientIP(r),
			UserAgent:  r.UserAgent(),
		}); serr != nil {
			h.logger.Warn().Err(serr).Str("user_id", u.ID).Msg("oauth: session insert (continuing)")
		}
	}
	redirect := buildRedirect(returnTo, token, exp)
	h.logger.Info().Str("provider", name).Str("user_id", u.ID).
		Str("email", u.Email).Msg("oauth: sign-in ok")
	http.Redirect(w, r, redirect, http.StatusFound)
}

func (h *Handler) redirectTarget(r *http.Request) string {
	candidate := strings.TrimSpace(r.URL.Query().Get("redirect"))
	if candidate == "" {
		candidate = strings.TrimSpace(r.URL.Query().Get("next"))
	}
	if candidate == "" {
		return h.defaultPostLoginURL
	}
	if !strings.HasPrefix(candidate, "/") || strings.HasPrefix(candidate, "//") {
		return h.defaultPostLoginURL
	}
	base := strings.TrimRight(h.defaultPostLoginURL, "/")
	if base == "" {
		return candidate
	}
	if u, err := url.Parse(base); err == nil {
		u.Path = candidate
		u.RawQuery = ""
		u.Fragment = ""
		return u.String()
	}
	return h.defaultPostLoginURL
}

func (h *Handler) signState(returnTo string, expiry time.Time) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(nonce) + "|" + returnTo + "|" + strconv.FormatInt(expiry.Unix(), 10)
	mac := hmac.New(sha256.New, h.stateSecret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + sig)), nil
}

func (h *Handler) verifyState(state string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "", fmt.Errorf("decode state: %w", err)
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 {
		return "", errors.New("state shape")
	}
	payload := strings.Join(parts[:3], "|")
	gotSig, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return "", fmt.Errorf("decode sig: %w", err)
	}
	mac := hmac.New(sha256.New, h.stateSecret)
	mac.Write([]byte(payload))
	if !hmac.Equal(gotSig, mac.Sum(nil)) {
		return "", errors.New("signature mismatch")
	}
	expUnix, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", fmt.Errorf("decode expiry: %w", err)
	}
	if time.Now().Unix() > expUnix {
		return "", errors.New("state expired")
	}
	return parts[1], nil
}

func buildRedirect(returnTo, token string, exp time.Time) string {
	frag := fmt.Sprintf(defaultFragmentOK,
		url.QueryEscape(token),
		url.QueryEscape(exp.UTC().Format(time.RFC3339)),
	)
	if idx := strings.IndexByte(returnTo, '#'); idx >= 0 {
		returnTo = returnTo[:idx]
	}
	return returnTo + frag
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i > 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	if v := r.Header.Get("X-Real-Ip"); v != "" {
		return v
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	return host
}
