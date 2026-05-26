package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	githubAuthorize = "https://github.com/login/oauth/authorize"
	githubToken     = "https://github.com/login/oauth/access_token"
	githubUserAPI   = "https://api.github.com/user"
	githubEmailsAPI = "https://api.github.com/user/emails"
	githubUA        = "Ironflyer-OAuth"
)

// GitHubProvider implements Provider for GitHub user OAuth.
type GitHubProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	client       *http.Client
}

// NewGitHubProvider returns a Provider when credentials look usable.
// Returns nil when ClientID is empty so the Handler skips registration.
func NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider {
	if strings.TrimSpace(clientID) == "" {
		return nil
	}
	return &GitHubProvider{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		client:       &http.Client{Timeout: 5 * time.Second},
	}
}

// Name returns the URL slug GitHub is mounted under.
func (g *GitHubProvider) Name() string { return "github" }

// AuthorizeURL composes the GitHub authorize endpoint URL.
func (g *GitHubProvider) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", g.ClientID)
	q.Set("scope", "read:user user:email")
	q.Set("state", state)
	if g.RedirectURL != "" {
		q.Set("redirect_uri", g.RedirectURL)
	}
	q.Set("allow_signup", "true")
	return githubAuthorize + "?" + q.Encode()
}

// Exchange swaps a code for an access token, then loads the user
// profile + a verified primary email.
func (g *GitHubProvider) Exchange(ctx context.Context, code string) (UserProfile, error) {
	tok, err := g.exchangeCode(ctx, code)
	if err != nil {
		return UserProfile{}, err
	}
	profile, err := g.fetchUser(ctx, tok)
	if err != nil {
		return UserProfile{}, err
	}
	if profile.Email == "" || !profile.EmailVerified {
		email, verified, eerr := g.fetchPrimaryEmail(ctx, tok)
		if eerr != nil {
			return UserProfile{}, eerr
		}
		profile.Email = email
		profile.EmailVerified = verified
	}
	if profile.Email == "" {
		return UserProfile{}, errors.New("github: no usable email on account")
	}
	return profile, nil
}

func (g *GitHubProvider) exchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("code", code)
	if g.RedirectURL != "" {
		form.Set("redirect_uri", g.RedirectURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubToken, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", githubUA)
	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github token: status %d", resp.StatusCode)
	}
	var parsed struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("github token: parse: %w", err)
	}
	if parsed.Error != "" {
		return "", fmt.Errorf("github token: %s: %s", parsed.Error, parsed.ErrorDescription)
	}
	if parsed.AccessToken == "" {
		return "", errors.New("github token: empty access_token")
	}
	return parsed.AccessToken, nil
}

type githubUser struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (g *GitHubProvider) fetchUser(ctx context.Context, accessToken string) (UserProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserAPI, nil)
	if err != nil {
		return UserProfile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", githubUA)
	resp, err := g.client.Do(req)
	if err != nil {
		return UserProfile{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UserProfile{}, fmt.Errorf("github user: status %d", resp.StatusCode)
	}
	var u githubUser
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256*1024)).Decode(&u); err != nil {
		return UserProfile{}, fmt.Errorf("github user: parse: %w", err)
	}
	name := strings.TrimSpace(u.Name)
	if name == "" {
		name = u.Login
	}
	return UserProfile{
		Email:          strings.ToLower(strings.TrimSpace(u.Email)),
		Name:           name,
		ProviderUserID: strconv.Itoa(u.ID),
	}, nil
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (g *GitHubProvider) fetchPrimaryEmail(ctx context.Context, accessToken string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubEmailsAPI, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", githubUA)
	resp, err := g.client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("github emails: status %d", resp.StatusCode)
	}
	var emails []githubEmail
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256*1024)).Decode(&emails); err != nil {
		return "", false, fmt.Errorf("github emails: parse: %w", err)
	}
	var fallback githubEmail
	for _, e := range emails {
		if e.Primary && e.Verified {
			return strings.ToLower(strings.TrimSpace(e.Email)), true, nil
		}
		if fallback.Email == "" && e.Verified {
			fallback = e
		}
	}
	if fallback.Email != "" {
		return strings.ToLower(strings.TrimSpace(fallback.Email)), fallback.Verified, nil
	}
	return "", false, errors.New("github emails: no verified address")
}
