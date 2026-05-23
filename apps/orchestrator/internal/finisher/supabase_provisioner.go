// Package finisher — Supabase provisioner. Implements DBProvisioner
// against the public Supabase Management API. Designed for the case
// where Ironflyer is operated by a team that holds a Supabase
// Personal Access Token plus an organization id, and wants every
// project the finisher spins up to get its own real Postgres without
// the user wiring credentials manually.
//
// Cost discipline: this provisioner can create *billable* Supabase
// projects. The orchestrator's main.go is the only place that
// instantiates it — keep it gated behind explicit env vars so a
// misconfigured dev environment never accidentally bills a card.
//
// Lifecycle (happy path):
//   1. POST /v1/projects  → returns { id (ref), status="COMING_UP" }
//   2. poll GET /v1/projects/{ref} until status == "ACTIVE_HEALTHY"
//   3. construct DSN: postgresql://postgres:<dbPass>@db.<ref>.supabase.co:5432/postgres
//
// Idempotency: the upstream DBProvisioner caller (Engine.ensureDatabase)
// skips provisioning when the project's Secrets already contain
// DATABASE_URL, so we only ever land here on a project's first run. We
// do NOT need to remember projects across orchestrator restarts.

package finisher

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/domain"
)

// SupabaseProvisioner provisions a fresh Supabase project per Ironflyer
// project. Pass a Personal Access Token from the Supabase dashboard and
// the organization id the new project should belong to. Region defaults
// to "us-east-1" if empty.
type SupabaseProvisioner struct {
	// AccessToken is a Supabase Personal Access Token with project-create
	// scope (or service account token with equivalent permissions).
	AccessToken string
	// OrganizationID is the slug/uuid the new project will belong to.
	OrganizationID string
	// Region is a Supabase region code, e.g. "us-east-1", "eu-west-1".
	// Defaults to "us-east-1" when empty.
	Region string
	// HTTP is the client used for all calls. Leave nil to get a sane
	// default with a 30s timeout per request. Provide a custom one in
	// tests or to inject retry middleware.
	HTTP *http.Client
	// PollTimeout is the maximum wall-clock time we'll wait for the new
	// project to reach ACTIVE_HEALTHY. Default 6 minutes — Supabase
	// typically takes 90–180s but DNS/cert propagation can extend that.
	PollTimeout time.Duration
	// PollInterval is the gap between status checks. Default 10s.
	PollInterval time.Duration
	// BaseURL overrides the API host for tests / staging. Empty uses
	// https://api.supabase.com.
	BaseURL string
}

const (
	supabaseDefaultBaseURL  = "https://api.supabase.com"
	supabaseDefaultRegion   = "us-east-1"
	supabaseDefaultPollTime = 6 * time.Minute
	supabaseDefaultPollGap  = 10 * time.Second
)

func (s *SupabaseProvisioner) http() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (s *SupabaseProvisioner) baseURL() string {
	if s.BaseURL != "" {
		return strings.TrimRight(s.BaseURL, "/")
	}
	return supabaseDefaultBaseURL
}

func (s *SupabaseProvisioner) region() string {
	if s.Region != "" {
		return s.Region
	}
	return supabaseDefaultRegion
}

func (s *SupabaseProvisioner) pollTimeout() time.Duration {
	if s.PollTimeout > 0 {
		return s.PollTimeout
	}
	return supabaseDefaultPollTime
}

func (s *SupabaseProvisioner) pollInterval() time.Duration {
	if s.PollInterval > 0 {
		return s.PollInterval
	}
	return supabaseDefaultPollGap
}

// Provision creates a Supabase project, waits for it to come up, and
// returns a DBProvision pointing at the new database.
func (s *SupabaseProvisioner) Provision(ctx context.Context, projectID string, _ []domain.EntityDef) (DBProvision, error) {
	if s == nil {
		return DBProvision{}, errors.New("nil SupabaseProvisioner")
	}
	if s.AccessToken == "" {
		return DBProvision{}, errors.New("supabase: AccessToken required")
	}
	if s.OrganizationID == "" {
		return DBProvision{}, errors.New("supabase: OrganizationID required")
	}

	// Strong DB password — long enough that brute force is not in the
	// threat model. Hex keeps it safe to embed in URLs without encoding.
	dbPass, err := randHex(24)
	if err != nil {
		return DBProvision{}, fmt.Errorf("supabase: generate db password: %w", err)
	}

	name := "ironflyer-" + sanitiseProjectName(projectID)
	createBody := map[string]any{
		"name":            name,
		"organization_id": s.OrganizationID,
		"region":          s.region(),
		"plan":            "free",
		"db_pass":         dbPass,
	}
	var created struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Region         string `json:"region"`
		Status         string `json:"status"`
		OrganizationID string `json:"organization_id"`
	}
	if err := s.do(ctx, http.MethodPost, "/v1/projects", createBody, &created); err != nil {
		return DBProvision{}, fmt.Errorf("supabase create project: %w", err)
	}
	if created.ID == "" {
		return DBProvision{}, errors.New("supabase create: empty project ref in response")
	}

	// Poll until ACTIVE_HEALTHY (or the bounded timeout expires).
	deadline := time.Now().Add(s.pollTimeout())
	gap := s.pollInterval()
	for {
		if err := ctx.Err(); err != nil {
			return DBProvision{}, err
		}
		var status struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := s.do(ctx, http.MethodGet, "/v1/projects/"+created.ID, nil, &status); err != nil {
			return DBProvision{}, fmt.Errorf("supabase poll status: %w", err)
		}
		if status.Status == "ACTIVE_HEALTHY" || status.Status == "ACTIVE" {
			break
		}
		if time.Now().After(deadline) {
			return DBProvision{}, fmt.Errorf("supabase: project %s not healthy after %s (last status %q)", created.ID, s.pollTimeout(), status.Status)
		}
		select {
		case <-ctx.Done():
			return DBProvision{}, ctx.Err()
		case <-time.After(gap):
		}
	}

	dsn := fmt.Sprintf(
		"postgresql://postgres:%s@db.%s.supabase.co:5432/postgres",
		dbPass, created.ID,
	)
	return DBProvision{
		DSN:       dsn,
		Provider:  "supabase",
		PublicURL: fmt.Sprintf("https://supabase.com/dashboard/project/%s", created.ID),
	}, nil
}

// do issues a JSON request with the configured access token. A response
// body is read fully and decoded into out (when non-nil); a non-2xx
// status is returned as an error with a trimmed body for context.
func (s *SupabaseProvisioner) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL()+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.AccessToken)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.http().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("supabase %s %s: %d: %s", method, path, resp.StatusCode, trimForErr(raw))
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func randHex(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// sanitiseProjectName turns an arbitrary project id into something
// Supabase accepts as a project name (alnum + dashes, 3..40 chars).
// Conservative: drop non-alnum, lowercase, trim to 32 chars.
func sanitiseProjectName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-':
			b.WriteRune(r)
		}
		if b.Len() >= 32 {
			break
		}
	}
	if b.Len() < 3 {
		return "project-" + b.String()
	}
	return b.String()
}

func trimForErr(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}
