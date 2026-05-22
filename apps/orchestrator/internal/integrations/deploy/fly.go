package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// FlyClient is a slim wrapper over Fly.io's Machines REST API + the flyctl
// CLI. The REST API is used for app creation and status lookups (cheap,
// idempotent), while the actual image build + deploy is delegated to
// `flyctl deploy` because Fly's build pipeline is non-trivial to reproduce
// in-process. If flyctl isn't installed we surface a clear error that the
// gate routes to the UI as an actionable hint.
type FlyClient struct {
	Token   string
	BaseURL string // defaults to https://api.machines.dev
	HTTP    *http.Client
	// FlyctlPath lets tests/dev environments override the CLI. When empty we
	// look up `flyctl` on $PATH at call time.
	FlyctlPath string
}

// NewFly returns a client backed by FLY_API_TOKEN. If the token is empty
// the client still constructs but every API call will fail with a clear
// error — the gate uses Enabled() to decide whether to surface deploy as a
// real option in the UI.
func NewFly(token string) *FlyClient {
	return &FlyClient{
		Token:   strings.TrimSpace(token),
		BaseURL: "https://api.machines.dev",
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *FlyClient) Enabled() bool { return c != nil && c.Token != "" }

// CreateApp idempotently creates a new Fly app in the given org under the
// org's primary plan. Existing apps short-circuit with a nil error so the
// caller can re-deploy without manual cleanup.
func (c *FlyClient) CreateApp(ctx context.Context, name, orgSlug string) error {
	if !c.Enabled() {
		return errFlyNoToken
	}
	if orgSlug == "" {
		orgSlug = "personal"
	}
	body := map[string]string{"app_name": name, "org_slug": orgSlug}
	resp, err := c.do(ctx, http.MethodPost, "/v1/apps", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusOK:
		return nil
	case http.StatusUnprocessableEntity, http.StatusConflict:
		// Existing app — that's fine, we'll deploy onto it.
		return nil
	default:
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("fly create app: %d %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
}

// FlyDeployStatus is the slim shape we report back to the UI.
type FlyDeployStatus struct {
	AppName  string `json:"appName"`
	Status   string `json:"status"`
	Hostname string `json:"hostname,omitempty"`
}

// GetStatus returns the most recent machine status for an app. A missing
// app is surfaced as ("", "") rather than an error so deploy-then-poll
// flows don't fail on first read.
func (c *FlyClient) GetStatus(ctx context.Context, appName string) (FlyDeployStatus, error) {
	if !c.Enabled() {
		return FlyDeployStatus{}, errFlyNoToken
	}
	resp, err := c.do(ctx, http.MethodGet, "/v1/apps/"+appName+"/machines", nil)
	if err != nil {
		return FlyDeployStatus{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return FlyDeployStatus{AppName: appName, Status: "missing"}, nil
	}
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return FlyDeployStatus{}, fmt.Errorf("fly status: %d %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	var machines []struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&machines); err != nil {
		return FlyDeployStatus{}, err
	}
	state := "pending"
	for _, m := range machines {
		if m.State == "started" {
			state = "started"
			break
		}
		state = m.State
	}
	return FlyDeployStatus{
		AppName:  appName,
		Status:   state,
		Hostname: appName + ".fly.dev",
	}, nil
}

// GetURL is a convenience accessor — Fly hostnames are deterministic.
func (c *FlyClient) GetURL(appName string) string {
	return "https://" + appName + ".fly.dev"
}

// DeployFromDir shells out to `flyctl deploy --remote-only` against the
// given directory. The deploy is streamed line-by-line to the provided
// onLine callback so the HTTP layer can fan it out as SSE events.
//
// Env entries are passed via `--env KEY=VALUE` so the deployed VM picks
// them up without writing them into fly.toml.
//
// If flyctl isn't installed, the returned error includes an install hint
// that the UI can render verbatim.
func (c *FlyClient) DeployFromDir(ctx context.Context, appName, dir string, env map[string]string, onLine func(string)) error {
	if !c.Enabled() {
		return errFlyNoToken
	}
	bin := c.FlyctlPath
	if bin == "" {
		bin = "flyctl"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("flyctl not installed — `brew install flyctl` or see https://fly.io/docs/hands-on/install-flyctl/")
	}
	args := []string{"deploy", "--remote-only", "--app", appName}
	for k, v := range env {
		args = append(args, "--env", k+"="+v)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "FLY_API_TOKEN="+c.Token)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // collapse streams so order matches user expectation
	if err := cmd.Start(); err != nil {
		return err
	}
	streamLines(stdout, onLine)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("flyctl deploy failed: %w", err)
	}
	return nil
}

func (c *FlyClient) do(ctx context.Context, method, path string, in any) (*http.Response, error) {
	var body io.Reader
	if in != nil {
		bts, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(bts)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.HTTP.Do(req)
}

var errFlyNoToken = errors.New("FLY_API_TOKEN is not configured")

// streamLines reads r line-by-line and invokes onLine for each, including
// the trailing partial line. It returns on EOF and never blocks the caller.
func streamLines(r io.Reader, onLine func(string)) {
	if onLine == nil {
		_, _ = io.Copy(io.Discard, r)
		return
	}
	buf := make([]byte, 4096)
	var carry strings.Builder
	for {
		n, err := r.Read(buf)
		if n > 0 {
			carry.Write(buf[:n])
			s := carry.String()
			carry.Reset()
			for {
				idx := strings.IndexByte(s, '\n')
				if idx < 0 {
					carry.WriteString(s)
					break
				}
				onLine(strings.TrimRight(s[:idx], "\r"))
				s = s[idx+1:]
			}
		}
		if err != nil {
			if carry.Len() > 0 {
				onLine(carry.String())
			}
			return
		}
	}
}
