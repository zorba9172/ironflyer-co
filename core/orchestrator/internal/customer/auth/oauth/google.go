package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	googleAuthorize = "https://accounts.google.com/o/oauth2/v2/auth"
	googleToken     = "https://oauth2.googleapis.com/token"
	googleUserInfo  = "https://www.googleapis.com/oauth2/v3/userinfo"
)

// GoogleProvider implements Provider for Google sign-in.
type GoogleProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	client       *http.Client
}

// NewGoogleProvider returns a Provider when ClientID is configured.
func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	if strings.TrimSpace(clientID) == "" {
		return nil
	}
	return &GoogleProvider{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		client:       &http.Client{Timeout: 5 * time.Second},
	}
}

// Name returns the URL slug Google is mounted under.
func (g *GoogleProvider) Name() string { return "google" }

// AuthorizeURL composes the Google authorize endpoint URL.
func (g *GoogleProvider) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", g.ClientID)
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	if g.RedirectURL != "" {
		q.Set("redirect_uri", g.RedirectURL)
	}
	q.Set("access_type", "online")
	q.Set("prompt", "select_account")
	return googleAuthorize + "?" + q.Encode()
}

// Exchange swaps a code for an access token, then reads the userinfo
// endpoint (Google verifies the access_token server-side, so we do not
// need to validate the id_token JWT signature ourselves).
func (g *GoogleProvider) Exchange(ctx context.Context, code string) (UserProfile, error) {
	accessToken, err := g.exchangeCode(ctx, code)
	if err != nil {
		return UserProfile{}, err
	}
	return g.fetchUserInfo(ctx, accessToken)
}

func (g *GoogleProvider) exchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	if g.RedirectURL != "" {
		form.Set("redirect_uri", g.RedirectURL)
	}
	form.Set("grant_type", "authorization_code")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleToken, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
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
		return "", fmt.Errorf("google token: status %d: %s", resp.StatusCode, truncate(string(body), 256))
	}
	var parsed struct {
		AccessToken      string `json:"access_token"`
		IDToken          string `json:"id_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("google token: parse: %w", err)
	}
	if parsed.Error != "" {
		return "", fmt.Errorf("google token: %s: %s", parsed.Error, parsed.ErrorDescription)
	}
	if parsed.AccessToken == "" {
		return "", errors.New("google token: empty access_token")
	}
	return parsed.AccessToken, nil
}

type googleUserInfoResp struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
}

func (g *GoogleProvider) fetchUserInfo(ctx context.Context, accessToken string) (UserProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfo, nil)
	if err != nil {
		return UserProfile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return UserProfile{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UserProfile{}, fmt.Errorf("google userinfo: status %d", resp.StatusCode)
	}
	var u googleUserInfoResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256*1024)).Decode(&u); err != nil {
		return UserProfile{}, fmt.Errorf("google userinfo: parse: %w", err)
	}
	name := strings.TrimSpace(u.Name)
	if name == "" {
		name = strings.TrimSpace(u.GivenName)
	}
	return UserProfile{
		Email:          strings.ToLower(strings.TrimSpace(u.Email)),
		Name:           name,
		ProviderUserID: u.Sub,
		EmailVerified:  u.EmailVerified,
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
