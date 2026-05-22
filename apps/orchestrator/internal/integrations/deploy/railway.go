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

// RailwayClient is a thin wrapper over Railway's GraphQL API + the `railway`
// CLI. The GraphQL API handles project/environment/service creation; actual
// build uploads use `railway up` which compresses and streams the project
// directory to Railway's build farm.
type RailwayClient struct {
	Token      string
	APIURL     string // defaults to https://backboard.railway.app/graphql/v2
	HTTP       *http.Client
	RailwayCLI string // override for tests; falls back to `railway` on PATH
}

func NewRailway(token string) *RailwayClient {
	return &RailwayClient{
		Token:  strings.TrimSpace(token),
		APIURL: "https://backboard.railway.app/graphql/v2",
		HTTP:   &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *RailwayClient) Enabled() bool { return c != nil && c.Token != "" }

// RailwayApp captures the IDs needed to drive a deploy.
type RailwayApp struct {
	ProjectID     string `json:"projectId"`
	EnvironmentID string `json:"environmentId"`
	ServiceID     string `json:"serviceId,omitempty"`
	Name          string `json:"name"`
}

// CreateApp creates a fresh project named `name` and returns the IDs. If a
// project with that name already exists, Railway's GraphQL will reject the
// mutation — callers handle that error by looking up the existing IDs.
func (c *RailwayClient) CreateApp(ctx context.Context, name string) (RailwayApp, error) {
	if !c.Enabled() {
		return RailwayApp{}, errRailwayNoToken
	}
	query := `mutation Create($name: String!) {
		projectCreate(input: { name: $name, defaultEnvironmentName: "production" }) {
			id
			environments { edges { node { id name } } }
		}
	}`
	var resp struct {
		Data struct {
			ProjectCreate struct {
				ID           string `json:"id"`
				Environments struct {
					Edges []struct {
						Node struct{ ID, Name string } `json:"node"`
					} `json:"edges"`
				} `json:"environments"`
			} `json:"projectCreate"`
		} `json:"data"`
		Errors []struct{ Message string } `json:"errors"`
	}
	if err := c.gql(ctx, query, map[string]any{"name": name}, &resp); err != nil {
		return RailwayApp{}, err
	}
	if len(resp.Errors) > 0 {
		return RailwayApp{}, fmt.Errorf("railway: %s", resp.Errors[0].Message)
	}
	app := RailwayApp{ProjectID: resp.Data.ProjectCreate.ID, Name: name}
	for _, e := range resp.Data.ProjectCreate.Environments.Edges {
		if e.Node.Name == "production" || app.EnvironmentID == "" {
			app.EnvironmentID = e.Node.ID
		}
	}
	return app, nil
}

// GetStatus reads the latest deploy state for a service. Returns "missing"
// when the service has no deployments yet.
func (c *RailwayClient) GetStatus(ctx context.Context, serviceID string) (string, error) {
	if !c.Enabled() {
		return "", errRailwayNoToken
	}
	if serviceID == "" {
		return "missing", nil
	}
	query := `query($id: String!) {
		service(id: $id) {
			deployments(first: 1) { edges { node { status staticUrl } } }
		}
	}`
	var resp struct {
		Data struct {
			Service struct {
				Deployments struct {
					Edges []struct {
						Node struct{ Status, StaticUrl string } `json:"node"`
					} `json:"edges"`
				} `json:"deployments"`
			} `json:"service"`
		} `json:"data"`
		Errors []struct{ Message string } `json:"errors"`
	}
	if err := c.gql(ctx, query, map[string]any{"id": serviceID}, &resp); err != nil {
		return "", err
	}
	if len(resp.Errors) > 0 {
		return "", fmt.Errorf("railway: %s", resp.Errors[0].Message)
	}
	if len(resp.Data.Service.Deployments.Edges) == 0 {
		return "pending", nil
	}
	return resp.Data.Service.Deployments.Edges[0].Node.Status, nil
}

// GetURL returns the public URL Railway assigned to a deployment, or "" if
// it isn't ready yet.
func (c *RailwayClient) GetURL(ctx context.Context, serviceID string) (string, error) {
	if !c.Enabled() {
		return "", errRailwayNoToken
	}
	query := `query($id: String!) {
		service(id: $id) {
			deployments(first: 1) { edges { node { staticUrl } } }
		}
	}`
	var resp struct {
		Data struct {
			Service struct {
				Deployments struct {
					Edges []struct {
						Node struct{ StaticUrl string } `json:"node"`
					} `json:"edges"`
				} `json:"deployments"`
			} `json:"service"`
		} `json:"data"`
	}
	if err := c.gql(ctx, query, map[string]any{"id": serviceID}, &resp); err != nil {
		return "", err
	}
	for _, e := range resp.Data.Service.Deployments.Edges {
		if e.Node.StaticUrl != "" {
			return "https://" + e.Node.StaticUrl, nil
		}
	}
	return "", nil
}

// DeployFromDir streams the project at `dir` to Railway via the `railway`
// CLI. Output lines are forwarded to onLine so the orchestrator can fan
// them out as SSE events.
func (c *RailwayClient) DeployFromDir(ctx context.Context, app RailwayApp, dir string, env map[string]string, onLine func(string)) error {
	if !c.Enabled() {
		return errRailwayNoToken
	}
	bin := c.RailwayCLI
	if bin == "" {
		bin = "railway"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("railway CLI not installed — `npm i -g @railway/cli` or see https://docs.railway.app/develop/cli")
	}
	args := []string{"up", "--detach"}
	if app.ServiceID != "" {
		args = append(args, "--service", app.ServiceID)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"RAILWAY_TOKEN="+c.Token,
		"RAILWAY_PROJECT_ID="+app.ProjectID,
		"RAILWAY_ENVIRONMENT_ID="+app.EnvironmentID,
	)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	streamLines(stdout, onLine)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("railway up failed: %w", err)
	}
	return nil
}

func (c *RailwayClient) gql(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.APIURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	bts, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("railway %d: %s", resp.StatusCode, strings.TrimSpace(string(bts)))
	}
	return json.Unmarshal(bts, out)
}

var errRailwayNoToken = errors.New("RAILWAY_TOKEN is not configured")
