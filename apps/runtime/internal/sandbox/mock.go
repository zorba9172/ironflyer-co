package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
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

// Ensure interfaces are satisfied at compile time.
var (
	_ Driver  = (*MockDriver)(nil)
	_ Session = (*ptySession)(nil)
	_ io.ReadWriteCloser = (*ptySession)(nil)
)
