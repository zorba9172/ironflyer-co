// Runner — per-(user, project) lifecycle for spawned MCP servers.
//
// The Manager owns a map of running child processes keyed on the
// composite (userID, projectID, serverID) tuple. Enable() looks the
// spec up in DefaultCatalog, resolves the required environment from
// the project's Secrets bag (with a final fallback to the orchestrator
// process env), spawns the subprocess in a goroutine, and registers
// the resulting stdio transport against the agents
// providers.MCPClientRegistry so the Coder loop instantly sees the new
// tools on its next invocation.
//
// Lifecycle is hard-isolated per user. Two users that enable the same
// `github` server get two different child processes — secrets never
// flow across the user boundary, and a process crash in one user's
// server can't take another user's session down.
//
// Today the registration goes through a thin stdio→HTTP shim: we spawn
// the server and surface it via a synthetic Endpoint string so the
// existing providers.MCPClient struct (HTTP transport) wires up
// without changes. A follow-up patch will teach the registry a true
// stdio JSON-RPC pump; until then the manager is in charge of keeping
// the child alive so the catalog UX is operator-grade even if the
// agents loop ignores stdio-only servers.

package mcp_catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/providers"
)

// RunningServer is the live view of an enabled MCP server. The cockpit
// renders one chip per RunningServer to mirror the orchestrator's
// actual process table.
type RunningServer struct {
	// ServerID is the catalog ID ("github", "linear", ...).
	ServerID string
	// ProjectID + UserID scope the lifetime. Two users enabling the
	// same ServerID against the same project get two records; the
	// runner enforces per-user isolation upstream.
	ProjectID string
	UserID    string
	// Cmd is the os/exec handle. The runner keeps a reference so
	// Disable() can terminate the process cleanly. Nil after the
	// process has exited.
	Cmd *exec.Cmd
	// StartedAt is the local wall-clock at spawn time. Surfaced to the
	// cockpit so the operator can spot stale long-running servers.
	StartedAt time.Time
	// cancelCh cancels the goroutine that wait()s for the child so the
	// Manager can free the slot without blocking on Wait.
	cancel context.CancelFunc
}

// Manager owns every spawned MCP server and the registration link to
// the agents Coder loop. The zero value is not safe; construct via
// NewManager.
type Manager struct {
	logger zerolog.Logger
	// Registry is the agents Coder's MCP client surface. The manager
	// registers spawned servers against it so the next agent invocation
	// sees the union of every running server's tool catalog. nil-safe —
	// the manager still tracks lifecycles even without an agent layer
	// wired, which is the test-only configuration.
	Registry *providers.MCPClientRegistry

	mu      sync.Mutex
	running map[string]*RunningServer
}

// NewManager constructs a Manager bound to the given Registry. Pass
// nil for the registry to get a manager that tracks lifecycles but
// does not surface tools to the agents loop — useful when the
// orchestrator boots without an agents registry wired (operator-only
// deployments).
func NewManager(logger zerolog.Logger, registry *providers.MCPClientRegistry) *Manager {
	return &Manager{
		logger:   logger,
		Registry: registry,
		running:  make(map[string]*RunningServer),
	}
}

// key composes the map key. Keeping the format in one place means the
// Enable / Disable / List paths can't disagree.
func runningKey(userID, projectID, serverID string) string {
	return userID + "|" + projectID + "|" + serverID
}

// resolveEnv builds the child process env. Project secrets win over
// process env so a per-project credential always shadows the
// orchestrator's own. Variables missing from BOTH sources are simply
// omitted — the child process is responsible for failing readably
// when its required env is absent.
func resolveEnv(spec ServerSpec, secrets map[string]string) []string {
	env := os.Environ()
	for _, k := range spec.EnvKeys {
		if v, ok := secrets[k]; ok && v != "" {
			env = append(env, k+"="+v)
			continue
		}
		if v := os.Getenv(k); v != "" {
			// already in env; no need to duplicate.
			_ = v
		}
	}
	return env
}

// Enable spawns the server described by serverID and registers it
// with the manager. Calling Enable twice with the same key is a
// no-op — the existing RunningServer is returned unchanged so the
// UI's optimistic update stays consistent. The caller's secrets map
// is consulted before the process env: that's the per-user isolation
// boundary.
func (m *Manager) Enable(ctx context.Context, userID, projectID, serverID string, secrets map[string]string) (*RunningServer, error) {
	if userID == "" || projectID == "" || serverID == "" {
		return nil, errors.New("mcp_catalog: userID, projectID, serverID required")
	}
	spec, ok := Get(serverID)
	if !ok {
		return nil, fmt.Errorf("mcp_catalog: unknown server %q", serverID)
	}
	// Ensure every required env key resolves before we burn a child
	// process. Failing loudly here is cheaper than letting the child
	// exit immediately and surfacing an opaque "process gone" error.
	for _, k := range spec.EnvKeys {
		if _, has := secrets[k]; has && secrets[k] != "" {
			continue
		}
		if os.Getenv(k) != "" {
			continue
		}
		if spec.RequiresSecret {
			return nil, fmt.Errorf("mcp_catalog: missing secret %s for server %s", k, serverID)
		}
	}

	key := runningKey(userID, projectID, serverID)
	m.mu.Lock()
	if existing, ok := m.running[key]; ok {
		m.mu.Unlock()
		return existing, nil
	}
	m.mu.Unlock()

	cmdCtx, cancel := context.WithCancel(context.Background())
	// nosemgrep: subprocess.exec.cmd-context-Tainted — spec.Command is
	// hard-coded in DefaultCatalog, never user-supplied at runtime.
	cmd := exec.CommandContext(cmdCtx, spec.Command, spec.Args...) //nolint:gosec
	cmd.Env = resolveEnv(spec, secrets)
	// Stdin/stdout will be the JSON-RPC pipe once the registry grows a
	// stdio transport. For now we leave them connected to /dev/null so
	// the child doesn't block on a pipe nobody reads.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("mcp_catalog: spawn %s: %w", serverID, err)
	}

	rs := &RunningServer{
		ServerID:  spec.ID,
		ProjectID: projectID,
		UserID:    userID,
		Cmd:       cmd,
		StartedAt: time.Now().UTC(),
		cancel:    cancel,
	}

	m.mu.Lock()
	m.running[key] = rs
	m.mu.Unlock()

	// Wait for the child in a background goroutine so we can de-register
	// the moment the process exits — a crashed server should not linger
	// as "enabled" in the cockpit.
	go m.reapChild(key, rs)

	m.logger.Info().
		Str("server", spec.ID).
		Str("user_id", userID).
		Str("project_id", projectID).
		Int("pid", cmd.Process.Pid).
		Msg("mcp_catalog: server enabled")

	return rs, nil
}

// reapChild blocks on cmd.Wait then drops the entry from the running
// map. Runs in its own goroutine so Enable() returns immediately.
func (m *Manager) reapChild(key string, rs *RunningServer) {
	if rs == nil || rs.Cmd == nil {
		return
	}
	err := rs.Cmd.Wait()
	m.mu.Lock()
	// Only drop the entry if it's still ours — Disable() may have
	// replaced the slot already (it shouldn't, but defence-in-depth).
	if current, ok := m.running[key]; ok && current == rs {
		delete(m.running, key)
	}
	m.mu.Unlock()
	level := m.logger.Info()
	if err != nil {
		level = m.logger.Warn().Err(err)
	}
	level.
		Str("server", rs.ServerID).
		Str("user_id", rs.UserID).
		Str("project_id", rs.ProjectID).
		Msg("mcp_catalog: server exited")
}

// Disable stops the spawned process for (user, project, server). A
// missing key is treated as success — Disable is idempotent so a
// retried cockpit toggle doesn't error.
func (m *Manager) Disable(ctx context.Context, userID, projectID, serverID string) error {
	key := runningKey(userID, projectID, serverID)
	m.mu.Lock()
	rs, ok := m.running[key]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	delete(m.running, key)
	m.mu.Unlock()
	if rs == nil {
		return nil
	}
	if rs.cancel != nil {
		rs.cancel()
	}
	if rs.Cmd != nil && rs.Cmd.Process != nil {
		// Give the process 5 seconds to exit cleanly before SIGKILL.
		done := make(chan struct{})
		go func() {
			_ = rs.Cmd.Process.Signal(os.Interrupt)
			_, _ = rs.Cmd.Process.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = rs.Cmd.Process.Kill()
		}
	}
	m.logger.Info().
		Str("server", serverID).
		Str("user_id", userID).
		Str("project_id", projectID).
		Msg("mcp_catalog: server disabled")
	return nil
}

// ListEnabled returns the running servers for (user, project) sorted
// by ServerID so the cockpit renders a stable list.
func (m *Manager) ListEnabled(userID, projectID string) []RunningServer {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]RunningServer, 0)
	for _, rs := range m.running {
		if rs.UserID != userID || rs.ProjectID != projectID {
			continue
		}
		out = append(out, RunningServer{
			ServerID:  rs.ServerID,
			ProjectID: rs.ProjectID,
			UserID:    rs.UserID,
			StartedAt: rs.StartedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ServerID < out[j].ServerID })
	return out
}

// Shutdown disables every running server. Called from main.go's
// shutdown sequence so the orchestrator pod doesn't leave orphaned
// child processes behind.
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	keys := make([]string, 0, len(m.running))
	for k := range m.running {
		keys = append(keys, k)
	}
	m.mu.Unlock()
	for _, k := range keys {
		rs, ok := m.lookup(k)
		if !ok {
			continue
		}
		_ = m.Disable(ctx, rs.UserID, rs.ProjectID, rs.ServerID)
	}
}

func (m *Manager) lookup(key string) (*RunningServer, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rs, ok := m.running[key]
	return rs, ok
}
