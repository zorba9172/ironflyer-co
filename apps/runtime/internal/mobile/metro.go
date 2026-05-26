package mobile

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/apps/runtime/internal/sandbox"
)

// Metro lifecycle ports. We pin them so the proxy + safelist + Expo CLI
// invocation all agree. 19000 is the HTTP/WS dev server; 19001 is the
// manifest server Expo Go resolves exp:// against; 19002 is the inspector
// WebSocket used by React Native Debugger and the Hermes inspector.
const (
	MetroPortDefault     = 19000
	MetroManifestPort    = 19001
	MetroInspectorPort   = 19002
	metroStartupBudget   = 25 * time.Second
	metroPollInterval    = 500 * time.Millisecond
	openTunnelTimeoutSec = 30
)

// MetroSession describes one running Metro instance bound to a workspace.
// LANBaseURL is the container-internal URL the runtime preview proxy
// dials; PublicBaseURL is the proxy-fronted URL the cockpit iframe uses;
// TunnelURL (when non-empty) is the public-internet tunnel that lets
// Expo Go on a physical phone reach the dev server from outside the LAN.
type MetroSession struct {
	WorkspaceID   string    `json:"workspaceId"`
	MetroPort     int       `json:"metroPort"`
	ManifestPort  int       `json:"manifestPort"`
	InspectorPort int       `json:"inspectorPort"`
	LANBaseURL    string    `json:"lanBaseUrl"`
	PublicBaseURL string    `json:"publicBaseUrl"`
	TunnelURL     string    `json:"tunnelUrl,omitempty"`
	ExpoGoURL     string    `json:"expoGoUrl,omitempty"`
	QRPayload     string    `json:"qrPayload,omitempty"`
	StartedAt     time.Time `json:"startedAt"`
}

// Tunnel tracks an outbound tunnel we opened for Expo Go. The PID file
// lives inside the workspace so we can SIGTERM the process on Stop. The
// URL is the public exp:// (or https://) endpoint Expo Go scans.
type Tunnel struct {
	URL     string
	Backend string // "expo-ngrok" | "serveo"
}

// MetroProxy owns the per-workspace Metro lifecycle that the runtime
// preview pipeline can't handle on its own: HMR over WebSocket, an
// internet-reachable tunnel for physical-device Expo Go, and the
// exp:// payload the frontend renders as a QR.
//
// The proxy is intentionally additive — it sits next to the existing
// sandbox preview allocator rather than replacing it. The HTTP layer
// composes a proxy by reading PreviewBinding (for the LAN target) and
// MetroSession (for the tunnel + QR).
type MetroProxy struct {
	sandboxMgr *sandbox.Manager
	logger     zerolog.Logger

	mu       sync.RWMutex
	sessions map[string]*MetroSession // workspaceID → session
	tunnels  map[string]*Tunnel       // workspaceID → tunnel
}

// NewMetroProxy constructs a MetroProxy bound to the runtime's sandbox
// manager. The logger receives a "component=metro-proxy" field on every
// emit.
func NewMetroProxy(s *sandbox.Manager, logger zerolog.Logger) *MetroProxy {
	return &MetroProxy{
		sandboxMgr: s,
		logger:     logger.With().Str("component", "metro-proxy").Logger(),
		sessions:   map[string]*MetroSession{},
		tunnels:    map[string]*Tunnel{},
	}
}

// Get returns the currently-recorded MetroSession for workspaceID.
func (m *MetroProxy) Get(workspaceID string) (*MetroSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[workspaceID]
	if !ok {
		return nil, false
	}
	// Return a copy so callers don't mutate the cached pointer.
	cp := *s
	return &cp, true
}

// Start boots Metro inside the workspace, allocates a preview port for
// the LAN URL through the existing sandbox manager, opens a
// best-effort tunnel for off-LAN devices, and records the resulting
// MetroSession. Idempotent: a second Start for a workspace that already
// has a live session returns the existing record.
func (m *MetroProxy) Start(ctx context.Context, ws sandbox.Workspace) (*MetroSession, error) {
	if existing, ok := m.Get(ws.ID); ok {
		return existing, nil
	}

	// 1. Reserve the preview port on the safelist.
	binding, err := m.sandboxMgr.AllocatePreview(ctx, ws.ID, MetroPortDefault)
	if err != nil {
		return nil, fmt.Errorf("metro: allocate preview port: %w", err)
	}

	// 2. Spawn `expo start` in the background. We don't use --tunnel here
	// because we manage the tunnel ourselves below — that gives us
	// uniform observability across the two tunnel backends.
	drv := m.sandboxMgr.Driver()
	spawn := `set -e
mkdir -p /tmp/ironflyer-mobile
: > /tmp/ironflyer-mobile/metro.out
: > /tmp/ironflyer-mobile/metro.pid
nohup sh -c 'npx --no-install expo start --port ` + fmt.Sprintf("%d", MetroPortDefault) + ` --offline=false --non-interactive >/tmp/ironflyer-mobile/metro.out 2>&1' &
echo $! > /tmp/ironflyer-mobile/metro.pid
exit 0`
	if _, err := drv.Exec(ctx, ws, sandbox.ExecOpts{Shell: spawn, TimeoutSeconds: 30}); err != nil {
		_ = m.sandboxMgr.ReleasePreview(ctx, ws.ID)
		return nil, fmt.Errorf("metro: spawn: %w", err)
	}

	// 3. Poll the Metro status endpoint until it returns
	// `packager-status:running` or we time out. Failure to confirm is a
	// hard error — without a running packager there's nothing to proxy.
	if err := m.waitForMetro(ctx, ws); err != nil {
		m.logger.Warn().Err(err).Str("workspace", ws.ID).Msg("metro readiness probe failed")
		_ = m.sandboxMgr.ReleasePreview(ctx, ws.ID)
		_ = m.killMetro(ctx, ws)
		return nil, err
	}

	lan := "http://127.0.0.1:" + fmt.Sprintf("%d", MetroPortDefault)
	if t := strings.TrimSpace(binding.URL); t != "" {
		// PreviewBinding.URL is the proxy-fronted URL, not the in-container
		// loopback. We still surface the loopback as LANBaseURL because
		// that's what Expo Go on the same WiFi hits via the LAN tunnel.
		lan = "http://" + fakeLANHost(binding) + ":" + fmt.Sprintf("%d", binding.InternalPort)
	}

	// 4. Best-effort tunnel for off-LAN devices.
	tun, terr := m.openTunnel(ctx, ws, MetroPortDefault)
	if terr != nil {
		m.logger.Warn().Err(terr).Str("workspace", ws.ID).Msg("metro tunnel open failed; physical-device Expo Go will need LAN")
	}

	session := &MetroSession{
		WorkspaceID:   ws.ID,
		MetroPort:     MetroPortDefault,
		ManifestPort:  MetroManifestPort,
		InspectorPort: MetroInspectorPort,
		LANBaseURL:    lan,
		PublicBaseURL: binding.URL,
		StartedAt:     time.Now().UTC(),
	}
	if tun != nil {
		session.TunnelURL = tun.URL
	}
	session.ExpoGoURL = buildExpoGoURL(session.TunnelURL, session.LANBaseURL)
	session.QRPayload = session.ExpoGoURL

	m.mu.Lock()
	m.sessions[ws.ID] = session
	if tun != nil {
		m.tunnels[ws.ID] = tun
	}
	m.mu.Unlock()

	m.logger.Info().
		Str("workspace", ws.ID).
		Int("metroPort", session.MetroPort).
		Str("publicBaseUrl", session.PublicBaseURL).
		Str("tunnelUrl", session.TunnelURL).
		Msg("metro session started")

	return session, nil
}

// Stop kills the Metro process, releases the preview ports, and closes
// the tunnel. Best-effort across all three legs: a failure in any one
// step is logged but does not block the others.
func (m *MetroProxy) Stop(ctx context.Context, workspaceID string) error {
	ws, err := m.sandboxMgr.Get(workspaceID)
	if err != nil {
		return err
	}

	if err := m.killMetro(ctx, ws); err != nil {
		m.logger.Warn().Err(err).Str("workspace", workspaceID).Msg("metro kill")
	}
	if err := m.sandboxMgr.ReleasePreview(ctx, workspaceID); err != nil {
		m.logger.Warn().Err(err).Str("workspace", workspaceID).Msg("metro preview release")
	}
	if err := m.closeTunnel(ctx, ws); err != nil {
		m.logger.Warn().Err(err).Str("workspace", workspaceID).Msg("metro tunnel close")
	}

	m.mu.Lock()
	delete(m.sessions, workspaceID)
	delete(m.tunnels, workspaceID)
	m.mu.Unlock()

	m.logger.Info().Str("workspace", workspaceID).Msg("metro session stopped")
	return nil
}

// waitForMetro polls the Metro /status endpoint until it answers with
// `packager-status:running` or the budget runs out.
func (m *MetroProxy) waitForMetro(ctx context.Context, ws sandbox.Workspace) error {
	drv := m.sandboxMgr.Driver()
	deadline := time.Now().Add(metroStartupBudget)
	for time.Now().Before(deadline) {
		res, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
			Shell:          fmt.Sprintf("curl -fsS --max-time 2 http://127.0.0.1:%d/status 2>/dev/null || true", MetroPortDefault),
			TimeoutSeconds: 5,
		})
		if err == nil && strings.Contains(res.Stdout, "packager-status:running") {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(metroPollInterval):
		}
	}
	return fmt.Errorf("metro did not become ready within %s", metroStartupBudget)
}

// killMetro best-effort kills the recorded Metro PID. Idempotent.
func (m *MetroProxy) killMetro(ctx context.Context, ws sandbox.Workspace) error {
	drv := m.sandboxMgr.Driver()
	_, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell: `set +e
PID=$(cat /tmp/ironflyer-mobile/metro.pid 2>/dev/null)
if [ -n "$PID" ]; then kill -TERM "$PID" 2>/dev/null || true; fi
rm -f /tmp/ironflyer-mobile/metro.pid
exit 0`,
		TimeoutSeconds: 15,
	})
	return err
}

// openTunnel attempts the two backends in order and returns the first
// that produces a public URL. A non-nil error is informational — callers
// treat an empty Tunnel as "no tunnel" without failing.
func (m *MetroProxy) openTunnel(ctx context.Context, ws sandbox.Workspace, port int) (*Tunnel, error) {
	if url := m.tryExpoNgrok(ctx, ws, port); url != "" {
		return &Tunnel{URL: url, Backend: "expo-ngrok"}, nil
	}
	if url := m.tryServeo(ctx, ws, port); url != "" {
		return &Tunnel{URL: url, Backend: "serveo"}, nil
	}
	return nil, fmt.Errorf("no tunnel backend produced a URL")
}

// tryExpoNgrok shells out to Expo's vendored ngrok binary. This is the
// preferred path because it is Expo's own offering — Expo Go's QR
// scanner recognises the *.exp.direct format automatically.
func (m *MetroProxy) tryExpoNgrok(ctx context.Context, ws sandbox.Workspace, port int) string {
	drv := m.sandboxMgr.Driver()
	script := `set +e
mkdir -p /tmp/ironflyer-mobile
: > /tmp/ironflyer-mobile/tunnel.out
: > /tmp/ironflyer-mobile/tunnel.pid
( nohup sh -c 'npx --yes @expo/ngrok http ` + fmt.Sprintf("%d", port) + ` --log=stdout >/tmp/ironflyer-mobile/tunnel.out 2>&1' & echo $! > /tmp/ironflyer-mobile/tunnel.pid ) 2>/dev/null
for i in $(seq 1 ` + fmt.Sprintf("%d", openTunnelTimeoutSec) + `); do
  if grep -qE 'https://[a-zA-Z0-9.-]+\.(exp\.direct|ngrok-free\.app|ngrok\.io)' /tmp/ironflyer-mobile/tunnel.out 2>/dev/null; then break; fi
  sleep 1
done
grep -oE 'https://[a-zA-Z0-9.-]+\.(exp\.direct|ngrok-free\.app|ngrok\.io)' /tmp/ironflyer-mobile/tunnel.out 2>/dev/null | head -n1
exit 0`
	res, err := drv.Exec(ctx, ws, sandbox.ExecOpts{Shell: script, TimeoutSeconds: openTunnelTimeoutSec + 10})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

// tryServeo falls back to the free serveo.net SSH-based reverse tunnel.
// Best-effort: if SSH or serveo is unavailable in the workspace image we
// return an empty string and the session ships without a tunnel.
func (m *MetroProxy) tryServeo(ctx context.Context, ws sandbox.Workspace, port int) string {
	drv := m.sandboxMgr.Driver()
	script := `set +e
mkdir -p /tmp/ironflyer-mobile
: > /tmp/ironflyer-mobile/tunnel.out
: > /tmp/ironflyer-mobile/tunnel.pid
( nohup ssh -o StrictHostKeyChecking=no -o ServerAliveInterval=30 -R 0:localhost:` + fmt.Sprintf("%d", port) + ` nokey@serveo.net >/tmp/ironflyer-mobile/tunnel.out 2>&1 & echo $! > /tmp/ironflyer-mobile/tunnel.pid ) 2>/dev/null
for i in $(seq 1 ` + fmt.Sprintf("%d", openTunnelTimeoutSec) + `); do
  if grep -qE 'https://[a-zA-Z0-9.-]+\.serveo\.net' /tmp/ironflyer-mobile/tunnel.out 2>/dev/null; then break; fi
  sleep 1
done
grep -oE 'https://[a-zA-Z0-9.-]+\.serveo\.net' /tmp/ironflyer-mobile/tunnel.out 2>/dev/null | head -n1
exit 0`
	res, err := drv.Exec(ctx, ws, sandbox.ExecOpts{Shell: script, TimeoutSeconds: openTunnelTimeoutSec + 10})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

// closeTunnel SIGTERMs the recorded tunnel PID, if any.
func (m *MetroProxy) closeTunnel(ctx context.Context, ws sandbox.Workspace) error {
	drv := m.sandboxMgr.Driver()
	_, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell: `set +e
PID=$(cat /tmp/ironflyer-mobile/tunnel.pid 2>/dev/null)
if [ -n "$PID" ]; then kill -TERM "$PID" 2>/dev/null || true; fi
rm -f /tmp/ironflyer-mobile/tunnel.pid
exit 0`,
		TimeoutSeconds: 10,
	})
	return err
}

// buildExpoGoURL produces the exp:// (or proxied https) string Expo Go
// scans from a QR. Prefer the tunnel — physical devices off the LAN
// can't reach a container IP — and fall back to the LAN URL.
func buildExpoGoURL(tunnel, lan string) string {
	pick := strings.TrimSpace(tunnel)
	if pick == "" {
		pick = strings.TrimSpace(lan)
	}
	if pick == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(pick, "https://"):
		return "exp://" + strings.TrimPrefix(pick, "https://")
	case strings.HasPrefix(pick, "http://"):
		return "exp://" + strings.TrimPrefix(pick, "http://")
	default:
		return pick
	}
}

// fakeLANHost extracts the LAN host portion from a PreviewBinding URL.
// When the URL doesn't carry a host we fall back to the workspace ID so
// the caller still gets a deterministic string for diagnostics.
func fakeLANHost(b sandbox.PreviewBinding) string {
	u := strings.TrimPrefix(strings.TrimPrefix(b.URL, "http://"), "https://")
	if idx := strings.IndexByte(u, '/'); idx > 0 {
		u = u[:idx]
	}
	if idx := strings.IndexByte(u, ':'); idx > 0 {
		u = u[:idx]
	}
	if u == "" {
		return b.WorkspaceID
	}
	return u
}
