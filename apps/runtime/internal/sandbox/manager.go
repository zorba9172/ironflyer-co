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
	ID         string    `json:"id"`
	UserID     string    `json:"userId"`
	ProjectID  string    `json:"projectId,omitempty"`
	Status     Status    `json:"status"`
	Driver     string    `json:"driver"`
	Root       string    `json:"root"`              // path on host (mock) or container ID (docker)
	PreviewURL string    `json:"previewUrl,omitempty"`
	IDEURL     string    `json:"ideUrl,omitempty"`  // code-server URL
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
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
}

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
	Stdout      string  `json:"stdout"`
	Stderr      string  `json:"stderr"`
	ExitCode    int     `json:"exitCode"`
	DurationMS  int64   `json:"durationMs"`
	TimedOut    bool    `json:"timedOut,omitempty"`
	TruncatedAt int     `json:"truncatedAt,omitempty"` // bytes; 0 = not truncated
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

// Manager keeps the workspace registry plus the chosen driver.
type Manager struct {
	mu     sync.RWMutex
	driver Driver
	byID   map[string]Workspace
}

func NewManager(d Driver) *Manager {
	return &Manager{driver: d, byID: make(map[string]Workspace)}
}

func (m *Manager) Driver() Driver { return m.driver }

func (m *Manager) Create(ctx context.Context, opts CreateOpts) (Workspace, error) {
	ws, err := m.driver.Create(ctx, opts)
	if err != nil {
		return Workspace{}, err
	}
	m.mu.Lock()
	m.byID[ws.ID] = ws
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
	m.mu.Unlock()
	return m.driver.Destroy(ctx, ws)
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
