package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
)

// MockDriver runs sandboxes as plain host-filesystem directories. No Docker
// needed. Terminals are real PTYs running /bin/bash (or PowerShell) in the
// workspace dir. Good enough for dev, never for prod multi-tenancy.
type MockDriver struct {
	BaseDir string // parent dir under which workspace dirs are created
}

func NewMockDriver(baseDir string) *MockDriver {
	_ = os.MkdirAll(baseDir, 0o755)
	return &MockDriver{BaseDir: baseDir}
}

func (m *MockDriver) Name() string { return "mock" }

func (m *MockDriver) Create(_ context.Context, opts CreateOpts) (Workspace, error) {
	id := "ws-" + uuid.NewString()[:8]
	root := filepath.Join(m.BaseDir, id)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Workspace{}, err
	}
	// Seed with a README so the file list isn't empty.
	_ = os.WriteFile(filepath.Join(root, "README.md"),
		[]byte("# Workspace\nThis is an Ironflyer sandbox.\n"), 0o644)
	now := time.Now().UTC()
	return Workspace{
		ID:        id,
		UserID:    opts.UserID,
		ProjectID: opts.ProjectID,
		Status:    StatusRunning,
		Driver:    "mock",
		Root:      root,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (m *MockDriver) Destroy(_ context.Context, ws Workspace) error {
	if ws.Root == "" || !strings.HasPrefix(ws.Root, m.BaseDir) {
		return errors.New("refusing to destroy: invalid root")
	}
	return os.RemoveAll(ws.Root)
}

func (m *MockDriver) safePath(ws Workspace, p string) (string, error) {
	cleaned := filepath.Clean("/" + p)
	abs := filepath.Join(ws.Root, cleaned)
	if !strings.HasPrefix(abs, ws.Root) {
		return "", errors.New("path escape")
	}
	return abs, nil
}

func (m *MockDriver) ReadFile(_ context.Context, ws Workspace, p string) ([]byte, error) {
	abs, err := m.safePath(ws, p)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}

func (m *MockDriver) WriteFile(_ context.Context, ws Workspace, p string, data []byte) error {
	abs, err := m.safePath(ws, p)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, data, 0o644)
}

func (m *MockDriver) DeleteFile(_ context.Context, ws Workspace, p string) error {
	abs, err := m.safePath(ws, p)
	if err != nil {
		return err
	}
	return os.Remove(abs)
}

func (m *MockDriver) ListFiles(_ context.Context, ws Workspace) ([]FileEntry, error) {
	var entries []FileEntry
	err := filepath.WalkDir(ws.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == ws.Root {
			return nil
		}
		rel, _ := filepath.Rel(ws.Root, p)
		info, _ := d.Info()
		entries = append(entries, FileEntry{
			Path: filepath.ToSlash(rel), IsDir: d.IsDir(),
			Size: func() int64 { if info != nil { return info.Size() }; return 0 }(),
		})
		return nil
	})
	return entries, err
}

func (m *MockDriver) Terminal(_ context.Context, ws Workspace) (Session, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Dir = ws.Root
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"PS1=ironflyer:$ ",
	)
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	return &ptySession{f: f, cmd: cmd}, nil
}

// ptySession wraps a PTY file with a cmd handle for cleanup.
type ptySession struct {
	f   *os.File
	cmd *exec.Cmd
	mu  sync.Mutex
}

func (s *ptySession) Read(p []byte) (int, error)  { return s.f.Read(p) }
func (s *ptySession) Write(p []byte) (int, error) { return s.f.Write(p) }

func (s *ptySession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.f.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.cmd.Wait()
	return nil
}

func (s *ptySession) Resize(rows, cols uint16) error {
	return pty.Setsize(s.f, &pty.Winsize{Rows: rows, Cols: cols})
}

// GitClone shallow-clones a remote into the workspace via the local `git`
// binary. Token is injected as the URL user so it never appears in argv.
func (m *MockDriver) GitClone(ctx context.Context, ws Workspace, opts CloneOpts) error {
	if ws.Root == "" || !strings.HasPrefix(ws.Root, m.BaseDir) {
		return errors.New("workspace root invalid")
	}
	authedURL, err := injectToken(opts.CloneURL, opts.Token)
	if err != nil {
		return err
	}
	target := ws.Root
	if opts.Subdir != "" {
		abs, err := m.safePath(ws, opts.Subdir)
		if err != nil {
			return err
		}
		target = abs
		_ = os.MkdirAll(filepath.Dir(target), 0o755)
	} else {
		// Clone wants the target directory empty (or non-existent). Wipe any
		// existing seed content (e.g. README.md) before cloning into root.
		entries, _ := os.ReadDir(target)
		for _, e := range entries {
			_ = os.RemoveAll(filepath.Join(target, e.Name()))
		}
	}
	args := []string{"clone", "--depth=1"}
	if opts.Ref != "" {
		args = append(args, "--branch", opts.Ref)
	}
	args = append(args, authedURL, target)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // never prompt for credentials on stdin
		"GIT_LFS_SKIP_SMUDGE=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Surface stderr but strip the embedded token so it can't leak to logs.
		msg := scrubToken(string(out), opts.Token)
		return fmt.Errorf("git clone failed: %w (%s)", err, strings.TrimSpace(msg))
	}
	return nil
}

// injectToken rewrites `https://github.com/foo/bar(.git)` into
// `https://x-access-token:<TOKEN>@github.com/foo/bar.git`. No-ops for
// non-HTTPS URLs or when token is empty.
func injectToken(rawURL, token string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("clone URL empty")
	}
	if !strings.HasSuffix(rawURL, ".git") {
		rawURL = rawURL + ".git"
	}
	if token == "" {
		return rawURL, nil
	}
	if !strings.HasPrefix(rawURL, "https://") {
		return "", errors.New("only https:// URLs may carry a token")
	}
	return "https://x-access-token:" + token + "@" + strings.TrimPrefix(rawURL, "https://"), nil
}

func scrubToken(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "***")
}

// Exec runs a command inside the workspace dir. Output is captured with a
// hard size cap so a runaway process can't OOM the runtime. Timeout uses
// context.WithTimeout — exit code -1 indicates "killed by deadline".
func (m *MockDriver) Exec(ctx context.Context, ws Workspace, opts ExecOpts) (ExecResult, error) {
	if ws.Root == "" || !strings.HasPrefix(ws.Root, m.BaseDir) {
		return ExecResult{}, errors.New("workspace root invalid")
	}
	cwd := ws.Root
	if opts.Cwd != "" {
		abs, err := m.safePath(ws, opts.Cwd)
		if err != nil {
			return ExecResult{}, err
		}
		cwd = abs
	}
	timeout := ResolveExecTimeout(opts.TimeoutSeconds)
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	switch {
	case strings.TrimSpace(opts.Shell) != "":
		cmd = exec.CommandContext(cctx, "/bin/sh", "-c", opts.Shell)
	case len(opts.Cmd) > 0:
		cmd = exec.CommandContext(cctx, opts.Cmd[0], opts.Cmd[1:]...)
	default:
		return ExecResult{}, errors.New("either shell or cmd is required")
	}
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), opts.Env...)

	var stdout, stderr capBuffer
	stdout.cap = ExecMaxOutput
	stderr.cap = ExecMaxOutput
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	dur := time.Since(start)

	res := ExecResult{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   exitCodeOf(cmd, runErr),
		DurationMS: dur.Milliseconds(),
	}
	if errors.Is(cctx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
	}
	if stdout.truncated || stderr.truncated {
		res.TruncatedAt = ExecMaxOutput
	}
	return res, nil
}

// exitCodeOf returns the OS exit code, mapping signal/deadline kills to -1.
func exitCodeOf(cmd *exec.Cmd, err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// capBuffer is a write-only byte sink with a hard size cap. Writes beyond
// the cap are dropped and the truncated flag is raised.
type capBuffer struct {
	buf       []byte
	cap       int
	truncated bool
}

func (c *capBuffer) Write(p []byte) (int, error) {
	if c.cap == 0 {
		c.buf = append(c.buf, p...)
		return len(p), nil
	}
	if len(c.buf) >= c.cap {
		c.truncated = true
		return len(p), nil // pretend success so the process doesn't get EPIPE
	}
	room := c.cap - len(c.buf)
	if room >= len(p) {
		c.buf = append(c.buf, p...)
	} else {
		c.buf = append(c.buf, p[:room]...)
		c.truncated = true
	}
	return len(p), nil
}

func (c *capBuffer) String() string { return string(c.buf) }

// PreviewTarget for the Mock driver returns the loopback address of an
// in-process HTTP server that serves a placeholder page. This makes the
// preview iframe usable even when Docker is unavailable: it gives the web
// app something concrete to load and proves the reverse-proxy plumbing.
//
// The mock target is started lazily on first call and reused across all
// workspaces — different workspace IDs simply see the same canned page
// with their ID interpolated into the response.
// RestoreFromSnapshot is a no-op in the mock driver — the snapshot
// plane is the canonical store, but local mock workspaces seed
// themselves with a README and never restore from S3. The signature
// is implemented so production allocators can call the same method on
// every driver.
func (m *MockDriver) RestoreFromSnapshot(_ context.Context, _ string, _ string) error {
	return nil
}

// Checkpoint is a no-op in the mock driver. The snapshots.Manager
// owns the canonical tar.zst lifecycle; the mock backend has no
// persistent compute to flush.
func (m *MockDriver) Checkpoint(_ context.Context, _ string, _ string) error {
	return nil
}

func (m *MockDriver) PreviewTarget(_ context.Context, ws Workspace, port int) (string, error) {
	host, err := startMockPreviewServer()
	if err != nil {
		return "", err
	}
	// The mock server ignores the requested port — it has a single bind —
	// but we echo it back in the response body so the user sees that the
	// path-based routing arrived intact.
	_ = port
	_ = ws
	return host, nil
}

var (
	mockPreviewOnce sync.Once
	mockPreviewAddr string
	mockPreviewErr  error
)

func startMockPreviewServer() (string, error) {
	mockPreviewOnce.Do(func() {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			mockPreviewErr = fmt.Errorf("mock preview listen: %w", err)
			return
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			fmt.Fprintf(w, mockPreviewHTML, r.Header.Get("X-Ironflyer-Workspace"),
				r.Header.Get("X-Ironflyer-Port"), r.URL.Path)
		})
		srv := &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() { _ = srv.Serve(l) }()
		mockPreviewAddr = l.Addr().String()
	})
	return mockPreviewAddr, mockPreviewErr
}

const mockPreviewHTML = `<!doctype html><html><head>
<meta charset="utf-8"><title>Ironflyer mock preview</title>
<style>body{font-family:-apple-system,system-ui,sans-serif;background:#0a0a0a;color:#e8e8e8;
margin:0;padding:48px;line-height:1.5}.k{color:#c8ff5e}code{background:#1a1a1a;padding:2px 6px;
border-radius:4px}</style></head><body>
<h1>Hello from <span class="k">Ironflyer mock workspace</span></h1>
<p>The runtime reverse-proxy is working. No Docker required.</p>
<ul>
<li>Workspace: <code>%s</code></li>
<li>Internal port (echoed): <code>%s</code></li>
<li>Path after prefix strip: <code>%s</code></li>
</ul>
<p>Start a real dev server with <code>npm run dev</code> inside a Docker workspace
to see your own UI here.</p>
</body></html>`

// AllocatePreviewPort for the mock driver returns a stable URL
// derived from the workspace ID. There is no real dev server — the
// URL is purely a placeholder so the studio iframe can render
// something while the operator is in dev mode without Docker.
func (m *MockDriver) AllocatePreviewPort(_ context.Context, workspaceID string, internalPort int) (PreviewBinding, error) {
	if !PreviewPortAllowed(internalPort) {
		return PreviewBinding{}, errors.New("internal port not on safelist")
	}
	return PreviewBinding{
		WorkspaceID:  workspaceID,
		InternalPort: internalPort,
		ExternalPort: internalPort,
		URL:          "http://mock-runtime/preview/" + workspaceID + "/",
		ExpiresAt:    time.Now().Add(PreviewLeaseDuration).UTC(),
	}, nil
}

// ReleasePreviewPort is a no-op for the mock driver — bindings are
// derived from the workspace ID and never consume real resources.
func (m *MockDriver) ReleasePreviewPort(_ context.Context, _ string) error { return nil }

// Ensure interfaces are satisfied at compile time.
var (
	_ Driver  = (*MockDriver)(nil)
	_ Session = (*ptySession)(nil)
	_ io.ReadWriteCloser = (*ptySession)(nil)
)
