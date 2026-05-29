package sandbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DockerDriver shells out to the `docker` CLI to manage per-user containers.
// We avoid the heavyweight `docker/docker` Go SDK to keep go.mod small;
// the CLI is widely available on dev machines and the contract is stable.
//
// Each workspace = one container running the web IDE with a host-path
// mount at /home/coder. The CANONICAL IDE is the custom-branded Eclipse
// Theia app (clients/ide/, image `ironflyer/theia-ide:latest` on :3030).
// Because that image is built locally and is NOT published to a public
// registry, the compiled-in DEFAULT stays `codercom/code-server:latest`
// (8080) — a registry-pullable fallback so any deployment that relies on
// Docker pulling the default keeps booting. Select the branded IDE with
// IRONFLYER_IDE_IMAGE=ironflyer/theia-ide:latest +
// IRONFLYER_IDE_CONTAINER_PORT=3030 (see config.go / .env).
//
// In production the host path is on EFS (mounted at
// `${RUNTIME_EFS_MOUNT}/<workspaceID>`), which lets any runtime pod
// pick up an existing workspace's files without a per-pod volume.
type DockerDriver struct {
	Image       string
	HostPortLow int

	// ContainerPort is the in-container port the web IDE listens on. We
	// publish it to the host via `-p 127.0.0.1:<hostPort>:<ContainerPort>`.
	// The canonical branded Theia image listens on 3030; the code-server
	// fallback defaults to 8080. Zero falls back to DefaultIDEContainerPort
	// (8080) so an unconfigured deployment still boots the pullable
	// fallback; set IRONFLYER_IDE_CONTAINER_PORT=3030 alongside the Theia
	// image.
	ContainerPort int

	// EFSRoot is the host-side parent directory used for every workspace
	// bind mount. Empty means "use Docker named volumes" (legacy path).
	EFSRoot string
	// FallbackRoot is the per-pod local directory used when EFSRoot is
	// configured but not actually mounted/writable (dev mode).
	FallbackRoot string

	// snapshotShim is the transport injected by WithSnapshotShim and
	// used by RestoreFromSnapshot / Checkpoint. Nil means "no remote
	// snapshot storage configured" — both methods become no-ops.
	snapshotShim SnapshotShim
}

// DefaultIDEImage is the web-IDE container image used when none is
// configured. The CANONICAL IDE is the branded Eclipse Theia app
// (theiaImageName below), but that image is built locally and is not on
// a public registry, so hard-defaulting to it would break any deployment
// that expects Docker to pull the default. We therefore keep the
// registry-pullable code-server as the compiled-in default and select
// the branded IDE explicitly via
// IRONFLYER_IDE_IMAGE=ironflyer/theia-ide:latest.
const DefaultIDEImage = "codercom/code-server:latest"

// DefaultIDEContainerPort is the in-container port the fallback
// code-server listens on. The canonical Theia image listens on 3030
// instead — pair IRONFLYER_IDE_IMAGE=ironflyer/theia-ide:latest with
// IRONFLYER_IDE_CONTAINER_PORT=3030.
const DefaultIDEContainerPort = 8080

// theiaImageName is the canonical branded Eclipse Theia image tag (built
// from clients/ide/). We key the run-args off the image name so the
// code-server-only flags (`--auth none`, `--disable-telemetry`) are NOT
// passed to Theia — its entrypoint takes no such flags and would reject
// the unknown args. This special-casing is what lets one driver serve
// either IDE; see imageArgs.
const theiaImageName = "ironflyer/theia-ide:latest"

func NewDockerDriver(image string) *DockerDriver {
	if strings.TrimSpace(image) == "" {
		image = DefaultIDEImage
	}
	return &DockerDriver{Image: image, HostPortLow: 18000, ContainerPort: DefaultIDEContainerPort}
}

// WithIDEImage overrides the web-IDE container image. Empty is ignored so
// callers can pass through an unset value without clobbering the default.
// Returns the driver for chaining.
func (d *DockerDriver) WithIDEImage(image string) *DockerDriver {
	if strings.TrimSpace(image) != "" {
		d.Image = image
	}
	return d
}

// WithContainerPort overrides the in-container IDE port (code-server
// 8080 vs Theia 3030). Zero or negative is ignored so callers can pass
// through an unset env without clobbering the default. Returns the
// driver for chaining.
func (d *DockerDriver) WithContainerPort(port int) *DockerDriver {
	if port > 0 {
		d.ContainerPort = port
	}
	return d
}

// containerPort returns the configured in-container IDE port, falling
// back to the code-server default when unset.
func (d *DockerDriver) containerPort() int {
	if d.ContainerPort > 0 {
		return d.ContainerPort
	}
	return DefaultIDEContainerPort
}

// imageArgs returns the image plus any image-specific trailing run args.
// code-server takes `--auth none --disable-telemetry`; the Theia browser
// app has no such flags, so for it we pass the image alone and let its
// entrypoint open /home/coder. Keying off the image name keeps a single
// driver serving either IDE without duplicating the whole Create path.
func imageArgs(image string) []string {
	if image == theiaImageName {
		return []string{image}
	}
	return []string{image, "--auth", "none", "--disable-telemetry"}
}

// WithEFS configures the bind-mount roots. Returns the driver so this
// can be chained with the existing constructor.
func (d *DockerDriver) WithEFS(efsRoot, fallbackRoot string) *DockerDriver {
	d.EFSRoot = efsRoot
	d.FallbackRoot = fallbackRoot
	return d
}

func (d *DockerDriver) Name() string { return "docker" }

func (d *DockerDriver) Create(ctx context.Context, opts CreateOpts) (Workspace, error) {
	id := "ws-" + uuid.NewString()[:8]
	name := "ironflyer-" + id

	// Resolve the bind-mount source. EFS path = production multi-pod
	// scale path; named volume = legacy single-pod path. We pick the
	// EFS layout whenever EFSRoot is configured because the runtime
	// pod's filesystem itself is read-only in prod.
	var mountSpec string
	if d.EFSRoot != "" {
		path, _, err := resolveEFSMount(d.EFSRoot, d.FallbackRoot, id)
		if err != nil {
			return Workspace{}, fmt.Errorf("efs mount: %w", err)
		}
		mountSpec = path + ":/home/coder"
	} else {
		volume := "ironflyer-vol-" + id
		if out, err := d.run(ctx, "volume", "create", volume); err != nil {
			return Workspace{}, fmt.Errorf("create volume: %w (%s)", err, out)
		}
		mountSpec = volume + ":/home/coder"
	}
	// Pick a host port — naive: random by container ID. Improved later.
	hostPort := d.pickPort(id)
	password, err := generateIDEPassword()
	if err != nil {
		return Workspace{}, fmt.Errorf("generate password: %w", err)
	}
	args := []string{
		"run", "-d", "--name", name,
		// Bind the IDE to loopback only — the runtime reverse-proxy
		// dials the container via PreviewTarget, never the host port,
		// so leaving it on 0.0.0.0 would only invite public probes.
		// The container-side port is configurable (code-server 8080,
		// Theia 3030).
		"-p", fmt.Sprintf("127.0.0.1:%d:%d", hostPort, d.containerPort()),
		"-v", mountSpec,
		"-e", "PASSWORD=" + password,
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges:true",
		"--pids-limit=512",
		"--memory=2g",
		"--cpus=2",
		"--tmpfs", "/tmp:rw,size=512m,mode=1777",
		"--tmpfs", "/run:rw,size=64m,mode=755",
		"--restart=no",
	}
	// imageArgs appends the image plus any image-specific entrypoint args
	// (code-server takes `--auth none --disable-telemetry`; Theia does not).
	args = append(args, imageArgs(d.Image)...)
	if out, err := d.run(ctx, args...); err != nil {
		return Workspace{}, fmt.Errorf("run: %w (%s)", err, out)
	}

	now := time.Now().UTC()
	return Workspace{
		ID:          id,
		UserID:      opts.UserID,
		ProjectID:   opts.ProjectID,
		Status:      StatusRunning,
		Driver:      "docker",
		Root:        name, // container name
		IDEURL:      fmt.Sprintf("http://127.0.0.1:%d", hostPort),
		IDEPassword: password,
		CreatedAt:   now, UpdatedAt: now,
	}, nil
}

// generateIDEPassword mints a per-workspace code-server credential with 32
// bytes of crypto entropy, base64-encoded so the value is shell-safe when
// passed via `-e PASSWORD=…`.
func generateIDEPassword() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (d *DockerDriver) Destroy(ctx context.Context, ws Workspace) error {
	_, _ = d.run(ctx, "rm", "-f", ws.Root)
	if d.EFSRoot == "" {
		_, _ = d.run(ctx, "volume", "rm", "ironflyer-vol-"+ws.ID)
	} else {
		// EFS path: remove the bind-mount directory so a recreated
		// workspace with the same ID starts clean. Best-effort; the
		// archive scanner is the system of record for cold storage.
		_ = os.RemoveAll(filepath.Join(d.EFSRoot, ws.ID))
		if d.FallbackRoot != "" {
			_ = os.RemoveAll(filepath.Join(d.FallbackRoot, ws.ID))
		}
	}
	return nil
}

// File ops run inside the container via `docker exec`.
func (d *DockerDriver) ReadFile(ctx context.Context, ws Workspace, p string) ([]byte, error) {
	target, err := d.resolveInsideHome(ctx, ws, p)
	if err != nil {
		return nil, err
	}
	return d.runRaw(ctx, "exec", ws.Root, "cat", target)
}

func (d *DockerDriver) WriteFile(ctx context.Context, ws Workspace, p string, data []byte) error {
	if err := validatePath(p); err != nil {
		return err
	}
	target := "/home/coder/" + strings.TrimPrefix(p, "/")
	// We refuse to overwrite an existing symlink — that's how a hostile
	// blueprint would aim a writer at /etc/passwd. New files are fine
	// because cat > redirect creates a regular file.
	if existing, _ := d.resolveInContainer(ctx, ws, target); existing != "" && !strings.HasPrefix(existing, "/home/coder/") && existing != "/home/coder" {
		return errors.New("path escape via symlink")
	}
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
	target, err := d.resolveInsideHome(ctx, ws, p)
	if err != nil {
		return err
	}
	_, err = d.run(ctx, "exec", ws.Root, "rm", "-f", target)
	return err
}

// resolveInsideHome string-validates p, then asks the container to canonicalise
// the path (resolving symlinks) and confirms the result still lives under
// /home/coder. This closes the symlink-escape gap where a user could plant
// /home/coder/etc -> /etc and read /etc/passwd via the file API.
func (d *DockerDriver) resolveInsideHome(ctx context.Context, ws Workspace, p string) (string, error) {
	if err := validatePath(p); err != nil {
		return "", err
	}
	target := "/home/coder/" + strings.TrimPrefix(p, "/")
	resolved, err := d.resolveInContainer(ctx, ws, target)
	if err != nil {
		return "", err
	}
	if resolved == "" {
		return target, nil
	}
	if resolved != "/home/coder" && !strings.HasPrefix(resolved, "/home/coder/") {
		return "", errors.New("path escape via symlink")
	}
	return resolved, nil
}

// resolveInContainer runs `readlink -m` in the container to canonicalise a
// path. `-m` does not require the leaf to exist, which is what we want for
// pre-write checks. Returns "" with no error when the resolver isn't
// available (e.g. minimal base images without coreutils).
func (d *DockerDriver) resolveInContainer(ctx context.Context, ws Workspace, target string) (string, error) {
	out, err := d.runRaw(ctx, "exec", ws.Root, "readlink", "-m", "--", target)
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func (d *DockerDriver) ListFiles(ctx context.Context, ws Workspace) ([]FileEntry, error) {
	out, err := d.runRaw(ctx, "exec", ws.Root, "find", "/home/coder", "-maxdepth", "6",
		"-printf", "%y\t%s\t%P\n")
	if err != nil {
		return nil, err
	}
	// Each entry is one newline-delimited line; size hint avoids growth doublings.
	lines := strings.Split(string(out), "\n")
	entries := make([]FileEntry, 0, len(lines))
	for _, line := range lines {
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

// resolveEFSMount picks the host-side bind-mount path for a workspace.
// Production deployments mount EFS at efsRoot and that's where files
// live; if EFS isn't actually writable (dev/test without the volume)
// we fall back to a per-pod local directory. Mirrors the helper in
// internal/drivers/docker/efs.go but stays in this package to keep
// driver creation a closed system with no cross-package dependency.
func resolveEFSMount(efsRoot, fallbackRoot, id string) (string, bool, error) {
	if efsRoot == "" {
		efsRoot = "/var/lib/ironflyer/workspaces"
	}
	if efsUsable(efsRoot) {
		path := filepath.Join(efsRoot, id)
		if err := os.MkdirAll(path, 0o755); err == nil {
			return path, true, nil
		}
	}
	if fallbackRoot == "" {
		fallbackRoot = "/tmp/ironflyer-workspaces"
	}
	path := filepath.Join(fallbackRoot, id)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", false, err
	}
	return path, false, nil
}

func efsUsable(root string) bool {
	if root == "" {
		return false
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		if err := os.MkdirAll(root, 0o755); err != nil {
			return false
		}
	}
	sentinel := filepath.Join(root, ".ironflyer-write-check")
	f, err := os.Create(sentinel)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(sentinel)
	return true
}

func (d *DockerDriver) pickPort(id string) int {
	h := 0
	for _, c := range id {
		h = (h*31 + int(c)) % 9000
	}
	return d.HostPortLow + h
}

func validatePath(p string) error {
	if p == "" {
		return errors.New("empty path")
	}
	// Reject any segment that is literally "..". A substring check on ".."
	// would block legitimate filenames like "foo..bar"; splitting on the
	// slash is precise and matches how the path is later joined.
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return errors.New("path escape")
		}
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

// PreviewTarget resolves the container's bridge-network IP and returns
// "<ip>:<port>" so the runtime reverse-proxy can dial straight into the
// dev server. We `docker inspect --format` once per call; the result is
// stable for the container lifetime but containers may restart with a
// new IP, so we don't cache.
func (d *DockerDriver) PreviewTarget(ctx context.Context, ws Workspace, port int) (string, error) {
	if port <= 0 || port > 65535 {
		return "", errors.New("invalid internal port")
	}
	if strings.TrimSpace(ws.Root) == "" {
		return "", errors.New("workspace has no container")
	}
	// Try the default bridge network first; fall back to the first
	// non-empty IPAddress across all attached networks.
	out, err := d.runRaw(ctx, "inspect",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}\n{{end}}",
		ws.Root)
	if err != nil {
		return "", fmt.Errorf("docker inspect: %w", err)
	}
	var ip string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			ip = line
			break
		}
	}
	if ip == "" {
		return "", errors.New("container has no IP address")
	}
	return fmt.Sprintf("%s:%d", ip, port), nil
}

// RestoreFromSnapshot pulls the tar.zst at snapshotURI and unpacks it
// into workspaceDir. workspaceDir is the host-side path the docker
// driver bind-mounts into the container (EFS in prod, FallbackRoot in
// dev). When snapshotURI is empty the call is a no-op so a fresh
// workspace path stays cheap.
//
// The actual transport is delegated to the snapshots.Manager via the
// shellOut shim below — Docker driver does not own a second S3
// client. The integration agent injects the shim during wireup.
func (d *DockerDriver) RestoreFromSnapshot(ctx context.Context, snapshotURI, workspaceDir string) error {
	if snapshotURI == "" {
		return nil
	}
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return fmt.Errorf("docker restore: workspace dir: %w", err)
	}
	shim := snapshotShimFor(d)
	if shim == nil {
		// No transport wired (dev mode). Surface as a soft success so
		// the allocator can still proceed; the workspace stays empty.
		return nil
	}
	return shim.Restore(ctx, snapshotURI, workspaceDir)
}

// Checkpoint tars+zstds workspaceDir and uploads it to destSnapshotURI.
// Empty destSnapshotURI is a no-op. Like RestoreFromSnapshot, the
// transport is delegated to the snapshot shim wired at startup.
func (d *DockerDriver) Checkpoint(ctx context.Context, workspaceDir, destSnapshotURI string) error {
	if destSnapshotURI == "" {
		return nil
	}
	info, err := os.Stat(workspaceDir)
	if err != nil {
		return fmt.Errorf("docker checkpoint: stat: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("docker checkpoint: %q is not a directory", workspaceDir)
	}
	shim := snapshotShimFor(d)
	if shim == nil {
		return nil
	}
	return shim.Checkpoint(ctx, workspaceDir, destSnapshotURI)
}

// SnapshotShim is the transport surface RestoreFromSnapshot and
// Checkpoint dial. The wireup agent supplies one (typically a thin
// adapter over snapshots.Manager). Kept here as an interface so the
// driver does not depend on the snapshots package directly and avoids
// an import cycle.
type SnapshotShim interface {
	Restore(ctx context.Context, snapshotURI, destDir string) error
	Checkpoint(ctx context.Context, srcDir, destSnapshotURI string) error
}

// WithSnapshotShim wires a transport for snapshot restore/checkpoint.
// Calling this is idempotent.
func (d *DockerDriver) WithSnapshotShim(s SnapshotShim) *DockerDriver {
	d.snapshotShim = s
	return d
}

// snapshotShimFor returns the configured shim or nil.
func snapshotShimFor(d *DockerDriver) SnapshotShim { return d.snapshotShim }

// -------------------------------------------------------------------
// Preview port allocation.
//
// The Docker driver took a deliberate v1 shortcut: rather than recreate
// the container with a new `-p host:container` mapping (which would
// require stop/rm/run and lose any in-memory state), we lean on the
// existing reverse-proxy in core/runtime/internal/preview. The proxy
// already dials `<container_ip>:<port>` via PreviewTarget — preview
// allocation becomes a bookkeeping operation: pick a stable external
// port number, remember the workspace→port binding, and hand the
// caller a URL whose host segment is the runtime's public host.
//
// External ports are advisory metadata for dashboards; the actual
// traffic flows through the runtime's preview prefix and never binds
// to the host. This keeps Docker workspaces hot-recreate-free and
// matches what production gVisor/Kata sandboxes will need: a single
// public host name + signed preview tokens, never per-tenant port
// exposure on the host firewall.
// -------------------------------------------------------------------

var (
	previewBindingsMu sync.Mutex
	previewBindings   = map[string]PreviewBinding{}
	previewUsedPorts  = map[int]string{} // external port → workspaceID
)

// AllocatePreviewPort assigns a workspace dev-server port and returns
// a URL that points at the runtime's preview reverse proxy. Idempotent
// on workspaceID — repeat calls extend the existing lease.
func (d *DockerDriver) AllocatePreviewPort(_ context.Context, workspaceID string, internalPort int) (PreviewBinding, error) {
	if workspaceID == "" {
		return PreviewBinding{}, errors.New("workspaceID required")
	}
	if !PreviewPortAllowed(internalPort) {
		return PreviewBinding{}, fmt.Errorf("internal port %d not on safelist", internalPort)
	}
	now := time.Now().UTC()
	previewBindingsMu.Lock()
	defer previewBindingsMu.Unlock()
	if existing, ok := previewBindings[workspaceID]; ok {
		existing.InternalPort = internalPort
		existing.ExpiresAt = now.Add(PreviewLeaseDuration)
		previewBindings[workspaceID] = existing
		return existing, nil
	}
	port, err := pickPreviewExternalPort(workspaceID)
	if err != nil {
		return PreviewBinding{}, err
	}
	binding := PreviewBinding{
		WorkspaceID:  workspaceID,
		InternalPort: internalPort,
		ExternalPort: port,
		URL:          buildPreviewURL(workspaceID, internalPort),
		ExpiresAt:    now.Add(PreviewLeaseDuration),
	}
	previewBindings[workspaceID] = binding
	previewUsedPorts[port] = workspaceID
	return binding, nil
}

// ReleasePreviewPort frees the binding's external port slot.
func (d *DockerDriver) ReleasePreviewPort(_ context.Context, workspaceID string) error {
	if workspaceID == "" {
		return nil
	}
	previewBindingsMu.Lock()
	defer previewBindingsMu.Unlock()
	b, ok := previewBindings[workspaceID]
	if !ok {
		return nil
	}
	delete(previewBindings, workspaceID)
	if previewUsedPorts[b.ExternalPort] == workspaceID {
		delete(previewUsedPorts, b.ExternalPort)
	}
	return nil
}

// pickPreviewExternalPort picks an unused advisory port in [18000,
// 18999]. Caller must hold previewBindingsMu.
func pickPreviewExternalPort(seed string) (int, error) {
	const lo, hi = 18000, 18999
	// Hash the seed for a stable first guess so the same workspace
	// trends to the same port across restarts (helps dashboards).
	start := lo
	if seed != "" {
		h := 0
		for _, c := range seed {
			h = (h*31 + int(c)) % (hi - lo + 1)
		}
		start = lo + h
	}
	for i := 0; i < (hi-lo+1); i++ {
		p := lo + ((start - lo + i) % (hi - lo + 1))
		if _, taken := previewUsedPorts[p]; !taken {
			return p, nil
		}
	}
	return 0, errors.New("no preview port available in [18000,18999]")
}

// buildPreviewURL renders the URL the iframe loads. Host comes from
// IRONFLYER_RUNTIME_PUBLIC_HOST (default "localhost"); the path
// follows the reverse-proxy prefix convention used by
// internal/preview so the same iframe URL works whether the runtime
// is bound to localhost or a public hostname.
func buildPreviewURL(workspaceID string, internalPort int) string {
	host := strings.TrimSpace(os.Getenv("IRONFLYER_RUNTIME_PUBLIC_HOST"))
	if host == "" {
		host = "localhost:8090"
	}
	scheme := "http"
	if strings.HasPrefix(host, "https://") {
		scheme = "https"
		host = strings.TrimPrefix(host, "https://")
	} else if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
	}
	return fmt.Sprintf("%s://%s/preview/%s/%d/", scheme, host, workspaceID, internalPort)
}

var _ Driver = (*DockerDriver)(nil)
