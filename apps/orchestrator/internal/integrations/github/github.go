// Package github wraps go-github + oauth2 with the orchestrator's idea of
// users + token persistence. Endpoints are wired in httpapi.
package github

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	gh "github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
	ghoauth "golang.org/x/oauth2/github"

	"ironflyer/apps/orchestrator/internal/integrations"
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string // defaults to read:user + repo if empty
}

// FlowMode marks which side of the OAuth dance the user is doing.
type FlowMode string

const (
	// FlowLink connects a GitHub account to the already-authenticated user.
	FlowLink FlowMode = "link"
	// FlowLogin signs the visitor in (creating an account if needed)
	// using only their GitHub identity. There is no Ironflyer JWT yet.
	FlowLogin FlowMode = "login"
)

type Service struct {
	cfg   Config
	store integrations.TokenStore
	mu    sync.Mutex
	pending map[string]pendingState
}

type pendingState struct {
	Mode      FlowMode
	UserID    string // empty for FlowLogin
	CreatedAt time.Time
}

func New(cfg Config, store integrations.TokenStore) *Service {
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"read:user", "user:email", "repo"}
	}
	return &Service{cfg: cfg, store: store, pending: make(map[string]pendingState)}
}

func (s *Service) Enabled() bool {
	return s.cfg.ClientID != "" && s.cfg.ClientSecret != ""
}

func (s *Service) oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.cfg.ClientID,
		ClientSecret: s.cfg.ClientSecret,
		RedirectURL:  s.cfg.RedirectURL,
		Scopes:       s.cfg.Scopes,
		Endpoint:     ghoauth.Endpoint,
	}
}

// AuthURL issues a state for either flow.
func (s *Service) AuthURL(mode FlowMode, userID string) (url string, state string, err error) {
	if !s.Enabled() {
		return "", "", errors.New("github integration disabled (set GITHUB_CLIENT_ID/SECRET)")
	}
	if mode == FlowLink && userID == "" {
		return "", "", errors.New("link flow requires userID")
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	state = hex.EncodeToString(buf)
	s.mu.Lock()
	s.pending[state] = pendingState{Mode: mode, UserID: userID, CreatedAt: time.Now().UTC()}
	s.gcLocked()
	s.mu.Unlock()
	return s.oauthConfig().AuthCodeURL(state, oauth2.AccessTypeOnline), state, nil
}

func (s *Service) gcLocked() {
	cutoff := time.Now().UTC().Add(-10 * time.Minute)
	for k, v := range s.pending {
		if v.CreatedAt.Before(cutoff) {
			delete(s.pending, k)
		}
	}
}

// ExchangeResult carries the outcome of a callback exchange so the HTTP
// layer can branch on flow.
type ExchangeResult struct {
	Mode         FlowMode
	UserID       string // for link flow
	AccessToken  string
	RefreshToken string
	Scope        string
	ExpiresAt    *time.Time
	GitHubID     string
	GitHubLogin  string
	GitHubEmail  string
	GitHubName   string
}

// Exchange validates state, exchanges the code, fetches the GitHub identity,
// and returns the data needed to either link or log in. The caller decides
// what to do with the user (create / find / link).
func (s *Service) Exchange(ctx context.Context, state, code string) (ExchangeResult, error) {
	s.mu.Lock()
	p, ok := s.pending[state]
	if ok {
		delete(s.pending, state)
	}
	s.mu.Unlock()
	if !ok {
		return ExchangeResult{}, errors.New("unknown or expired state")
	}
	tok, err := s.oauthConfig().Exchange(ctx, code)
	if err != nil {
		return ExchangeResult{}, fmt.Errorf("exchange: %w", err)
	}
	client := gh.NewClient(s.oauthConfig().Client(ctx, tok))
	me, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return ExchangeResult{}, fmt.Errorf("fetch user: %w", err)
	}
	res := ExchangeResult{
		Mode:         p.Mode,
		UserID:       p.UserID,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Scope:        scopeString(tok),
		GitHubID:     strconv.FormatInt(me.GetID(), 10),
		GitHubLogin:  me.GetLogin(),
		GitHubEmail:  me.GetEmail(),
		GitHubName:   me.GetName(),
	}
	if !tok.Expiry.IsZero() {
		exp := tok.Expiry
		res.ExpiresAt = &exp
	}
	return res, nil
}

// PersistToken upserts the integration token for a given userID.
func (s *Service) PersistToken(ctx context.Context, userID string, res ExchangeResult) error {
	t := integrations.Token{
		UserID:        userID,
		Kind:          integrations.KindGitHub,
		AccessToken:   res.AccessToken,
		RefreshToken:  res.RefreshToken,
		Scope:         res.Scope,
		ExternalID:    res.GitHubID,
		ExternalLogin: res.GitHubLogin,
		ExpiresAt:     res.ExpiresAt,
	}
	_, err := s.store.Put(ctx, t)
	return err
}

// FindUserIDByExternalID scans the token store for a user already linked to
// the given GitHub ID. Returns "" if nobody is linked. (The TokenStore
// interface only offers per-user lookup; we keep this scan to memory by
// inspecting any per-user calls — for Postgres we lean on a future
// FindByExternalID method.)
//
// This default impl is best-effort: callers should rely on the FindLinked
// helper which the HTTP layer wires via the user repository.
type ExternalLookup func(ctx context.Context, externalID string) (string /*userID*/, error)

// Status reports whether a user has an active GitHub connection.
type Status struct {
	Connected bool   `json:"connected"`
	Login     string `json:"login,omitempty"`
	Scope     string `json:"scope,omitempty"`
}

func (s *Service) Status(ctx context.Context, userID string) (Status, error) {
	t, err := s.store.Get(ctx, userID, integrations.KindGitHub)
	if err != nil {
		if errors.Is(err, integrations.ErrNotFound) {
			return Status{Connected: false}, nil
		}
		return Status{}, err
	}
	return Status{Connected: true, Login: t.ExternalLogin, Scope: t.Scope}, nil
}

func (s *Service) Disconnect(ctx context.Context, userID string) error {
	return s.store.Delete(ctx, userID, integrations.KindGitHub)
}

// TokenFor returns the stored access token for a user, or ErrNotFound.
func (s *Service) TokenFor(ctx context.Context, userID string) (string, error) {
	t, err := s.store.Get(ctx, userID, integrations.KindGitHub)
	if err != nil {
		return "", err
	}
	return t.AccessToken, nil
}

// Repo is the slim shape the UI needs.
type Repo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"fullName"`
	Description   string `json:"description,omitempty"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"defaultBranch"`
	HTMLURL       string `json:"htmlUrl"`
	CloneURL      string `json:"cloneUrl"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
}

func (s *Service) ListRepos(ctx context.Context, userID string) ([]Repo, error) {
	t, err := s.store.Get(ctx, userID, integrations.KindGitHub)
	if err != nil {
		return nil, err
	}
	client := gh.NewClient(s.oauthConfig().Client(ctx, &oauth2.Token{AccessToken: t.AccessToken}))
	repos, _, err := client.Repositories.ListByAuthenticatedUser(ctx, &gh.RepositoryListByAuthenticatedUserOptions{
		Sort:        "updated",
		Affiliation: "owner,collaborator,organization_member",
		ListOptions: gh.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, err
	}
	out := make([]Repo, 0, len(repos))
	for _, r := range repos {
		out = append(out, Repo{
			ID:            r.GetID(),
			Name:          r.GetName(),
			FullName:      r.GetFullName(),
			Description:   r.GetDescription(),
			Private:       r.GetPrivate(),
			DefaultBranch: r.GetDefaultBranch(),
			HTMLURL:       r.GetHTMLURL(),
			CloneURL:      r.GetCloneURL(),
			UpdatedAt:     r.GetUpdatedAt().Format(time.RFC3339),
		})
	}
	return out, nil
}

func scopeString(t *oauth2.Token) string {
	if t == nil {
		return ""
	}
	if v, ok := t.Extra("scope").(string); ok {
		return v
	}
	return ""
}
