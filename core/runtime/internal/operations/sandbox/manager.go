// Package sandbox abstracts per-user workspace lifecycles. Drivers:
//   - mock:   in-process filesystem, no containers (good for dev/tests)
//   - docker: shells out to `docker` CLI to spin up code-server per user
//
// The Manager is the only entry point the HTTP layer uses; drivers are
// behind an interface so we can swap to Firecracker microVMs later.
package sandbox

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"
)

type Status string

const (
	StatusCreating Status = "creating"
	StatusRunning  Status = "running"
	StatusStopped  Status = "stopped"
	StatusError    Status = "error"
)

// Workspace describes one user sandbox.
type Workspace struct {
	ID         string `json:"id"`
	UserID     string `json:"userId"`
	ProjectID  string `json:"projectId,omitempty"`
	Status     Status `json:"status"`
	Driver     string `json:"driver"`
	Root       string `json:"root"`               // path on host (mock) or container ID (docker)
	HostPath   string `json:"hostPath,omitempty"` // host-side workspace directory when known
	PreviewURL string `json:"previewUrl,omitempty"`
	IDEURL     string `json:"ideUrl,omitempty"` // code-server URL
	// IDEPassword is the random per-workspace credential code-server boots
	// with. Surfaced so the orchestrator can wrap it in a signed preview
	// token; never log this value or echo it in error messages.
	IDEPassword string    `json:"idePassword,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// IsAccessibleBy returns true when userID owns the workspace. Empty userID
// means "no auth configured" (dev) — accept everything.
func (w Workspace) IsAccessibleBy(userID string) bool {
	return userID == "" || w.UserID == userID
}

// CreateOpts is what the HTTP layer hands to the manager when spinning up.
type CreateOpts struct {
	UserID    string
	ProjectID string
}

// CloneOpts configures a git clone inside a workspace. The driver is
// responsible for sanitising the URL/token and for path containment.
type CloneOpts struct {
	CloneURL string // https URL — `.git` suffix optional
	Token    string // optional personal/oauth token; injected into URL
	Ref      string // branch or tag; empty = default branch
	Subdir   string // optional subdirectory within the workspace
}

// Driver is what each backend (mock / docker) implements.
type Driver interface {
	Name() string
	Create(ctx context.Context, opts CreateOpts) (Workspace, error)
	Destroy(ctx context.Context, ws Workspace) error

	// File ops are scoped to the workspace root. Paths are workspace-relative.
	ReadFile(ctx context.Context, ws Workspace, path string) ([]byte, error)
	WriteFile(ctx context.Context, ws Workspace, path string, data []byte) error
	DeleteFile(ctx context.Context, ws Workspace, path string) error
	ListFiles(ctx context.Context, ws Workspace) ([]FileEntry, error)

	// Terminal returns a bidirectional shell session.
	Terminal(ctx context.Context, ws Workspace) (Session, error)

	// GitClone shallow-clones a repository into the workspace.
	GitClone(ctx context.Context, ws Workspace, opts CloneOpts) error

	// Exec runs a one-shot command inside the workspace. Stdout/stderr are
	// captured separately and truncated to ExecMaxOutput bytes. Context
	// cancellation kills the process group.
	Exec(ctx context.Context, ws Workspace, opts ExecOpts) (ExecResult, error)

	// PreviewTarget returns a host:port (or a full base URL) the runtime
	// reverse-proxy can dial to reach the workspace's internal dev server.
	//
	// For the Docker driver this is the container's bridge-network IP plus
	// the requested internal port. For the Mock driver it can be a built-in
	// loopback server that serves a placeholder page. The returned value is
	// either of the form "host:port" or a fully-qualified base URL with the
	// scheme (e.g. "http://172.17.0.2:3000").
	PreviewTarget(ctx context.Context, ws Workspace, port int) (string, error)

	// RestoreFromSnapshot pulls a snapshot from snapshotURI and unpacks
	// it into workspaceDir. snapshotURI is the s3:// URI returned by
	// the snapshots.Manager (snapshots.Metadata.URI()); workspaceDir
	// is the driver-local path the sandbox should mount.
	//
	// Implementations may skip the network round-trip when they have a
	// node-local hot cache for workspaceDir. Errors must NOT be
	// fatal at the driver layer — the allocator will fall back to a
	// fresh workspace if RestoreFromSnapshot fails.
	RestoreFromSnapshot(ctx context.Context, snapshotURI, workspaceDir string) error

	// Checkpoint tars+compresses workspaceDir and uploads it to
	// destSnapshotURI (s3://bucket/key). Called at lifecycle gates
	// (after_patch / after_gate / before_idle_teardown / periodic).
	// The snapshots.Manager owns the URI scheme; the driver owns the
	// transport. Empty destSnapshotURI is a no-op.
	Checkpoint(ctx context.Context, workspaceDir string, destSnapshotURI string) error

	// AllocatePreviewPort assigns a port for the workspace's dev server
	// (Next.js 3000, Vite 5173, etc.) and returns a publicly-reachable
	// URL the studio iframe can load.
	//
	// Idempotent: repeat calls with the same workspaceID return the
	// same binding (and refresh its expiry). internalPort must come
	// from PreviewPortSafelist so untrusted blueprints can't expose
	// arbitrary container ports.
	AllocatePreviewPort(ctx context.Context, workspaceID string, internalPort int) (PreviewBinding, error)

	// ReleasePreviewPort frees the workspace's preview binding.
	// Best-effort: unknown workspace IDs return nil.
	ReleasePreviewPort(ctx context.Context, workspaceID string) error
}

// PreviewBinding describes a workspace's live-preview reservation so
// the orchestrator can hand the iframe a URL the moment the workspace
// exists — no waiting for a deploy step to finish.
type PreviewBinding struct {
	WorkspaceID  string    `json:"workspaceId"`
	InternalPort int       `json:"internalPort"`
	ExternalPort int       `json:"externalPort"`
	URL          string    `json:"url"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// PreviewPortSafelist enumerates the internal dev-server ports a
// workspace is permitted to publish via AllocatePreviewPort. Keeping
// this small avoids accidentally exposing SSH (22), metrics (9090),
// databases, or any port a malicious blueprint might bind. Edit with
// care.
var PreviewPortSafelist = map[int]bool{
	3000: true, // Next.js, Express defaults
	4200: true, // Angular CLI
	5173: true, // Vite
	8000: true, // Django, FastAPI, Python http.server
	8080: true, // Spring Boot, Go default
	8081: true, // alt http + legacy Metro packager port (RN < 0.74)
	// Expo / Metro family — needed for hot reload over the runtime proxy.
	// 19000: Metro dev server (HTTP + WS HMR endpoint at /message).
	// 19001: Manifest server (exp:// resolves to this).
	// 19002: Inspector WebSocket (React Native Debugger / Hermes).
	19000: true,
	19001: true,
	19002: true,
}

// PreviewPortAllowed reports whether port is on the safelist.
func PreviewPortAllowed(port int) bool {
	return PreviewPortSafelist[port]
}

// PreviewLeaseDuration is how long a freshly-allocated preview lease
// stays valid before the periodic janitor may reclaim it. Refreshed
// implicitly on every AllocatePreviewPort call (idempotency path).
const PreviewLeaseDuration = time.Hour

// ExecOpts describes a one-shot command run inside a workspace.
type ExecOpts struct {
	// Shell is the preferred way to run commands. When non-empty the driver
	// invokes `sh -c <Shell>` inside the workspace. Use this for pipelines,
	// chained commands, or anything where you'd reach for shell syntax.
	Shell string `json:"shell,omitempty"`
	// Cmd is an argv when you don't want a shell. Ignored when Shell is set.
	Cmd []string `json:"cmd,omitempty"`
	// Cwd is workspace-relative. Empty = workspace root.
	Cwd string `json:"cwd,omitempty"`
	// Env is extra `KEY=VAL` pairs appended to the driver's base env.
	Env []string `json:"env,omitempty"`
	// TimeoutSeconds caps the wall clock for the run. 0 → ExecDefaultTimeout.
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
}

// ExecResult is the (truncated) outcome of an Exec call.
type ExecResult struct {
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	ExitCode    int    `json:"exitCode"`
	DurationMS  int64  `json:"durationMs"`
	TimedOut    bool   `json:"timedOut,omitempty"`
	TruncatedAt int    `json:"truncatedAt,omitempty"` // bytes; 0 = not truncated
}

const (
	// ExecDefaultTimeout is what we use when ExecOpts.TimeoutSeconds is zero.
	ExecDefaultTimeout = 60 * time.Second
	// ExecMaxTimeout is the hard cap; longer requests are clamped silently.
	ExecMaxTimeout = 5 * time.Minute
	// ExecMaxOutput caps each of stdout/stderr individually.
	ExecMaxOutput = 1 << 20 // 1 MiB
)

// ResolveExecTimeout normalises ExecOpts.TimeoutSeconds against the defaults
// and the hard cap. Shared by drivers.
func ResolveExecTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return ExecDefaultTimeout
	}
	d := time.Duration(seconds) * time.Second
	if d > ExecMaxTimeout {
		return ExecMaxTimeout
	}
	return d
}

type FileEntry struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
}

// Session is a PTY-like full-duplex stream.
type Session interface {
	io.Reader
	io.Writer
	io.Closer
	Resize(rows, cols uint16) error
}

var ErrNotFound = errors.New("workspace not found")

// DetectedPort is a port the runtime believes the workspace bound when a
// dev server was started. The first time we see a particular port for a
// workspace we capture FirstSeen; LastSeen is bumped on every observation.
type DetectedPort struct {
	Port      int       `json:"port"`
	Source    string    `json:"source,omitempty"` // "exec-stdout" | "exec-stderr" | "manual"
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
}

// Manager keeps the workspace registry plus the chosen driver.
type Manager struct {
	mu     sync.RWMutex
	driver Driver
	byID   map[string]Workspace
	// byProject maps a ProjectID to its workspace ID. Drivers mint an opaque
	// workspace ID, but callers (the studio) address a workspace by the
	// project it belongs to, so we keep this secondary index for resolution.
	byProject map[string]string
	// ports tracks dev-server ports we've auto-detected per workspace, keyed
	// by workspace ID then by port number.
	ports map[string]map[int]DetectedPort
}

func NewManager(d Driver) *Manager {
	return &Manager{
		driver:    d,
		byID:      make(map[string]Workspace),
		byProject: make(map[string]string),
		ports:     make(map[string]map[int]DetectedPort),
	}
}

// RecordPort registers (or refreshes) a detected dev-server port for a
// workspace. Source describes how we learned about it ("exec-stdout", etc).
func (m *Manager) RecordPort(workspaceID string, port int, source string) {
	if workspaceID == "" || port <= 0 || port > 65535 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[workspaceID]; !ok {
		return
	}
	bucket := m.ports[workspaceID]
	if bucket == nil {
		bucket = make(map[int]DetectedPort)
		m.ports[workspaceID] = bucket
	}
	now := time.Now().UTC()
	if existing, ok := bucket[port]; ok {
		existing.LastSeen = now
		bucket[port] = existing
		return
	}
	bucket[port] = DetectedPort{
		Port: port, Source: source,
		FirstSeen: now, LastSeen: now,
	}
}

// Ports returns the detected dev-server ports for a workspace, sorted by
// first-seen time ascending.
func (m *Manager) Ports(workspaceID string) []DetectedPort {
	m.mu.RLock()
	defer m.mu.RUnlock()
	bucket := m.ports[workspaceID]
	if len(bucket) == 0 {
		return nil
	}
	out := make([]DetectedPort, 0, len(bucket))
	for _, p := range bucket {
		out = append(out, p)
	}
	// Simple insertion sort by FirstSeen — stable, no extra allocs.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].FirstSeen.After(out[j].FirstSeen); j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func (m *Manager) Driver() Driver { return m.driver }

func (m *Manager) Create(ctx context.Context, opts CreateOpts) (Workspace, error) {
	ws, err := m.driver.Create(ctx, opts)
	if err != nil {
		return Workspace{}, err
	}
	m.mu.Lock()
	m.byID[ws.ID] = ws
	if ws.ProjectID != "" {
		m.byProject[ws.ProjectID] = ws.ID
	}
	m.mu.Unlock()
	return ws, nil
}

func (m *Manager) Get(id string) (Workspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ws, ok := m.byID[id]
	if !ok {
		return Workspace{}, ErrNotFound
	}
	return ws, nil
}

// GetByProject resolves the live workspace for a project. Callers that address
// a workspace by its project (rather than the driver-minted workspace ID) use
// this. Returns false when no workspace has been provisioned for the project.
func (m *Manager) GetByProject(projectID string) (Workspace, bool) {
	if projectID == "" {
		return Workspace{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.byProject[projectID]
	if !ok {
		return Workspace{}, false
	}
	ws, ok := m.byID[id]
	return ws, ok
}

func (m *Manager) List() []Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Workspace, 0, len(m.byID))
	for _, w := range m.byID {
		out = append(out, w)
	}
	return out
}

func (m *Manager) Destroy(ctx context.Context, id string) error {
	m.mu.Lock()
	ws, ok := m.byID[id]
	if !ok {
		m.mu.Unlock()
		return ErrNotFound
	}
	delete(m.byID, id)
	delete(m.ports, id)
	if ws.ProjectID != "" && m.byProject[ws.ProjectID] == id {
		delete(m.byProject, ws.ProjectID)
	}
	m.mu.Unlock()
	return m.driver.Destroy(ctx, ws)
}

// AllocatePreview asks the driver to allocate a preview binding for
// the workspace and caches the URL on the workspace record so a
// subsequent GET /workspaces/{id} can surface it without a second
// driver round-trip.
func (m *Manager) AllocatePreview(ctx context.Context, workspaceID string, internalPort int) (PreviewBinding, error) {
	if _, err := m.Get(workspaceID); err != nil {
		return PreviewBinding{}, err
	}
	if !PreviewPortAllowed(internalPort) {
		return PreviewBinding{}, errors.New("internal port not on safelist")
	}
	binding, err := m.driver.AllocatePreviewPort(ctx, workspaceID, internalPort)
	if err != nil {
		return PreviewBinding{}, err
	}
	m.mu.Lock()
	if w, ok := m.byID[workspaceID]; ok {
		w.PreviewURL = binding.URL
		w.UpdatedAt = time.Now().UTC()
		m.byID[workspaceID] = w
	}
	m.mu.Unlock()
	return binding, nil
}

// ReleasePreview releases the workspace's preview binding via the
// driver and clears the cached URL on the workspace record.
func (m *Manager) ReleasePreview(ctx context.Context, workspaceID string) error {
	if _, err := m.Get(workspaceID); err != nil {
		return err
	}
	err := m.driver.ReleasePreviewPort(ctx, workspaceID)
	m.mu.Lock()
	if w, ok := m.byID[workspaceID]; ok {
		w.PreviewURL = ""
		w.UpdatedAt = time.Now().UTC()
		m.byID[workspaceID] = w
	}
	m.mu.Unlock()
	return err
}

// touch updates the UpdatedAt time after any successful operation.
func (m *Manager) touch(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w, ok := m.byID[id]; ok {
		w.UpdatedAt = time.Now().UTC()
		m.byID[id] = w
	}
}
