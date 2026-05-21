package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DockerDriver shells out to the `docker` CLI to manage per-user containers.
// We avoid the heavyweight `docker/docker` Go SDK to keep go.mod small;
// the CLI is widely available on dev machines and the contract is stable.
//
// Each workspace = one container running code-server (default image
// `codercom/code-server:latest`) with a named volume mounted at /home/coder.
type DockerDriver struct {
	Image       string
	HostPortLow int
}

func NewDockerDriver(image string) *DockerDriver {
	return &DockerDriver{Image: image, HostPortLow: 18000}
}

func (d *DockerDriver) Name() string { return "docker" }

func (d *DockerDriver) Create(ctx context.Context, opts CreateOpts) (Workspace, error) {
	id := "ws-" + uuid.NewString()[:8]
	name := "ironflyer-" + id
	volume := "ironflyer-vol-" + id

	if out, err := d.run(ctx, "volume", "create", volume); err != nil {
		return Workspace{}, fmt.Errorf("create volume: %w (%s)", err, out)
	}
	// Pick a host port — naive: random by container ID. Improved later.
	hostPort := d.pickPort(id)
	args := []string{
		"run", "-d", "--name", name,
		"-v", volume + ":/home/coder",
		"-p", fmt.Sprintf("%d:8080", hostPort),
		"-e", "PASSWORD=ironflyer-dev",
		d.Image,
	}
	if out, err := d.run(ctx, args...); err != nil {
		return Workspace{}, fmt.Errorf("run: %w (%s)", err, out)
	}

	now := time.Now().UTC()
	return Workspace{
		ID:        id,
		UserID:    opts.UserID,
		ProjectID: opts.ProjectID,
		Status:    StatusRunning,
		Driver:    "docker",
		Root:      name, // container name
		IDEURL:    fmt.Sprintf("http://localhost:%d", hostPort),
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (d *DockerDriver) Destroy(ctx context.Context, ws Workspace) error {
	_, _ = d.run(ctx, "rm", "-f", ws.Root)
	_, _ = d.run(ctx, "volume", "rm", "ironflyer-vol-"+ws.ID)
	return nil
}

// File ops run inside the container via `docker exec`.
func (d *DockerDriver) ReadFile(ctx context.Context, ws Workspace, p string) ([]byte, error) {
	if err := validatePath(p); err != nil {
		return nil, err
	}
	return d.runRaw(ctx, "exec", ws.Root, "cat", "/home/coder/"+strings.TrimPrefix(p, "/"))
}

func (d *DockerDriver) WriteFile(ctx context.Context, ws Workspace, p string, data []byte) error {
	if err := validatePath(p); err != nil {
		return err
	}
	target := "/home/coder/" + strings.TrimPrefix(p, "/")
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", ws.Root,
		"sh", "-c", "mkdir -p \"$(dirname '"+shellEscape(target)+"')\" && cat > '"+shellEscape(target)+"'")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("write %s: %w (%s)", p, err, out)
	}
	return nil
}

func (d *DockerDriver) DeleteFile(ctx context.Context, ws Workspace, p string) error {
	if err := validatePath(p); err != nil {
		return err
	}
	_, err := d.run(ctx, "exec", ws.Root, "rm", "-f", "/home/coder/"+strings.TrimPrefix(p, "/"))
	return err
}

func (d *DockerDriver) ListFiles(ctx context.Context, ws Workspace) ([]FileEntry, error) {
	out, err := d.runRaw(ctx, "exec", ws.Root, "find", "/home/coder", "-maxdepth", "6",
		"-printf", "%y\t%s\t%P\n")
	if err != nil {
		return nil, err
	}
	var entries []FileEntry
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 || parts[2] == "" {
			continue
		}
		entries = append(entries, FileEntry{
			Path: parts[2], IsDir: parts[0] == "d",
		})
	}
	return entries, nil
}

func (d *DockerDriver) Terminal(ctx context.Context, ws Workspace) (Session, error) {
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", ws.Root, "bash")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &dockerSession{cmd: cmd, in: stdin, out: stdout}, nil
}

type dockerSession struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out io.ReadCloser
}

func (s *dockerSession) Read(p []byte) (int, error)  { return s.out.Read(p) }
func (s *dockerSession) Write(p []byte) (int, error) { return s.in.Write(p) }
func (s *dockerSession) Close() error {
	_ = s.in.Close()
	_ = s.out.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.cmd.Wait()
	return nil
}

// Docker exec is not a real PTY (without -t), so resize is a no-op. Use
// `docker exec -it` from a real TTY for true PTY semantics; that requires
// allocating a PTY on the host side, which the Mock driver already does.
func (s *dockerSession) Resize(_, _ uint16) error { return nil }

func (d *DockerDriver) run(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	return string(out), err
}

func (d *DockerDriver) runRaw(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "docker", args...).Output()
}

func (d *DockerDriver) pickPort(id string) int {
	h := 0
	for _, c := range id {
		h = (h*31 + int(c)) % 9000
	}
	return d.HostPortLow + h
}

func validatePath(p string) error {
	if strings.Contains(p, "..") {
		return errors.New("path escape")
	}
	if strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "/home/coder") {
		return errors.New("absolute path not allowed")
	}
	return nil
}

func shellEscape(s string) string {
	// Single-quoted, escape internal single quotes.
	return strings.ReplaceAll(s, "'", `'\''`)
}

// GitClone runs `git clone` inside the container at /home/coder. The token,
// if any, is embedded in the URL so it never reaches argv.
func (d *DockerDriver) GitClone(ctx context.Context, ws Workspace, opts CloneOpts) error {
	authed, err := injectToken(opts.CloneURL, opts.Token)
	if err != nil {
		return err
	}
	target := "/home/coder"
	if opts.Subdir != "" {
		if err := validatePath(opts.Subdir); err != nil {
			return err
		}
		target = "/home/coder/" + strings.TrimPrefix(opts.Subdir, "/")
	} else {
		// Empty /home/coder so clone has a clean target.
		_, _ = d.run(ctx, "exec", ws.Root, "sh", "-c", "rm -rf /home/coder/* /home/coder/.[!.]* /home/coder/..?* 2>/dev/null; true")
	}
	args := []string{"exec", "-e", "GIT_TERMINAL_PROMPT=0", "-e", "GIT_LFS_SKIP_SMUDGE=1",
		ws.Root, "git", "clone", "--depth=1"}
	if opts.Ref != "" {
		args = append(args, "--branch", opts.Ref)
	}
	args = append(args, authed, target)
	out, err := d.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("docker git clone failed: %w (%s)", err, scrubToken(out, opts.Token))
	}
	return nil
}

// Exec runs a command inside the container via `docker exec`. We always go
// through `sh -c` so the orchestrator can pass either a shell string or an
// argv (joined with whitespace). Output is captured with a size cap.
func (d *DockerDriver) Exec(ctx context.Context, ws Workspace, opts ExecOpts) (ExecResult, error) {
	cctx, cancel := context.WithTimeout(ctx, ResolveExecTimeout(opts.TimeoutSeconds))
	defer cancel()

	shellCmd := strings.TrimSpace(opts.Shell)
	if shellCmd == "" {
		if len(opts.Cmd) == 0 {
			return ExecResult{}, errors.New("either shell or cmd is required")
		}
		// argv → shell string. Each arg gets single-quoted to preserve spaces.
		parts := make([]string, 0, len(opts.Cmd))
		for _, a := range opts.Cmd {
			parts = append(parts, "'"+shellEscape(a)+"'")
		}
		shellCmd = strings.Join(parts, " ")
	}

	args := []string{"exec"}
	for _, e := range opts.Env {
		args = append(args, "-e", e)
	}
	if cwd := strings.TrimSpace(opts.Cwd); cwd != "" {
		if err := validatePath(cwd); err != nil {
			return ExecResult{}, err
		}
		args = append(args, "-w", "/home/coder/"+strings.TrimPrefix(cwd, "/"))
	} else {
		args = append(args, "-w", "/home/coder")
	}
	args = append(args, ws.Root, "sh", "-c", shellCmd)

	cmd := exec.CommandContext(cctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	dur := time.Since(start)

	res := ExecResult{
		Stdout:     capString(stdout.String(), ExecMaxOutput),
		Stderr:     capString(stderr.String(), ExecMaxOutput),
		ExitCode:   exitCodeOfRun(cmd, runErr),
		DurationMS: dur.Milliseconds(),
	}
	if errors.Is(cctx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
	}
	if len(stdout.String()) > ExecMaxOutput || len(stderr.String()) > ExecMaxOutput {
		res.TruncatedAt = ExecMaxOutput
	}
	return res, nil
}

func exitCodeOfRun(_ *exec.Cmd, err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

func capString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

var _ Driver = (*DockerDriver)(nil)
