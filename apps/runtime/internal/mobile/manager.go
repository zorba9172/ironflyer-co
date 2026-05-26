package mobile

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ironflyer/apps/runtime/internal/sandbox"
)

// EmulatorSessionTTL bounds how long an Android emulator stays advertised
// before the janitor reclaims it. The actual emulator process inside the
// workspace lives until the workspace is destroyed; this TTL is only
// metadata-level.
const EmulatorSessionTTL = 2 * time.Hour

// BuildStatus describes the lifecycle of an async build invocation.
type BuildStatus string

const (
	BuildStatusRunning BuildStatus = "running"
	BuildStatusDone    BuildStatus = "done"
	BuildStatusError   BuildStatus = "error"
)

// BuildRecord is the polling surface for /mobile/build/{buildId}.
type BuildRecord struct {
	BuildID    string       `json:"buildId"`
	Status     BuildStatus  `json:"status"`
	Request    BuildRequest `json:"request"`
	Result     BuildResult  `json:"result"`
	Error      string       `json:"error,omitempty"`
	StartedAt  time.Time    `json:"startedAt"`
	FinishedAt time.Time    `json:"finishedAt,omitempty"`
}

// Manager owns the per-workspace mobile lifecycle (builds, dev servers,
// emulators). All state lives in memory; restart wipes pending builds
// and live emulator sessions — that's intentional. The orchestrator is
// the source of truth for build history.
type Manager struct {
	sandbox *sandbox.Manager
	logger  zerolog.Logger
	metro   *MetroProxy

	mu        sync.RWMutex
	emulators map[string]EmulatorSession  // by workspaceID
	expo      map[string]ExpoSession      // by workspaceID
	builds    map[string]*BuildRecord     // by buildID
	byWS      map[string]map[string]bool  // workspaceID → set of buildIDs
}

// NewManager constructs a Manager bound to the runtime's sandbox.Manager.
func NewManager(sandboxMgr *sandbox.Manager, logger zerolog.Logger) *Manager {
	return &Manager{
		sandbox:   sandboxMgr,
		logger:    logger.With().Str("component", "mobile").Logger(),
		metro:     NewMetroProxy(sandboxMgr, logger),
		emulators: map[string]EmulatorSession{},
		expo:      map[string]ExpoSession{},
		builds:    map[string]*BuildRecord{},
		byWS:      map[string]map[string]bool{},
	}
}

// MetroProxy returns the Manager's metro hot-reload proxy. The HTTP layer
// uses this to register the `/mobile/metro/*` lifecycle + proxy routes.
func (m *Manager) MetroProxy() *MetroProxy { return m.metro }

// Build dispatches on (kind, target) to the matching driver. Returns the
// completed BuildResult and any driver error. Callers that need async
// behaviour use StartBuild, which wraps this in a goroutine.
func (m *Manager) Build(ctx context.Context, ws sandbox.Workspace, req BuildRequest) (BuildResult, error) {
	switch req.Kind {
	case KindExpo, KindReactNativeBare:
		switch req.Target {
		case TargetAndroid:
			return buildExpoAndroid(ctx, m.sandbox, ws, req)
		case TargetIOS:
			return buildExpoIOSEAS(ctx, m.sandbox, ws, req)
		}
	case KindAndroidNative:
		if req.Target != TargetAndroid {
			return BuildResult{}, fmt.Errorf("android-native only supports android target")
		}
		return buildAndroidNative(ctx, m.sandbox, ws, req)
	case KindIOSNative:
		if req.Target != TargetIOS {
			return BuildResult{}, fmt.Errorf("ios-native only supports ios target")
		}
		return buildIOSNative(ctx, m.sandbox, ws, req)
	case KindFlutter:
		switch req.Target {
		case TargetAndroid:
			return buildFlutterAndroid(ctx, m.sandbox, ws, req)
		case TargetIOS:
			return buildFlutterIOS(ctx, m.sandbox, ws, req)
		}
	}
	return BuildResult{}, fmt.Errorf("unsupported kind/target combination: %s/%s", req.Kind, req.Target)
}

// StartBuild kicks Build off in the background and returns a buildID the
// caller can poll. The build runs on a detached context with a 25-minute
// hard cap so a stuck Gradle daemon doesn't leak workspaces forever.
func (m *Manager) StartBuild(ws sandbox.Workspace, req BuildRequest) string {
	buildID := "bld-" + uuid.NewString()[:12]
	rec := &BuildRecord{
		BuildID:   buildID,
		Status:    BuildStatusRunning,
		Request:   req,
		StartedAt: time.Now().UTC(),
	}
	m.mu.Lock()
	m.builds[buildID] = rec
	if m.byWS[ws.ID] == nil {
		m.byWS[ws.ID] = map[string]bool{}
	}
	m.byWS[ws.ID][buildID] = true
	m.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
		defer cancel()
		res, err := m.Build(ctx, ws, req)
		m.mu.Lock()
		defer m.mu.Unlock()
		rec.Result = res
		rec.FinishedAt = time.Now().UTC()
		if err != nil {
			rec.Status = BuildStatusError
			rec.Error = err.Error()
			m.logger.Warn().Err(err).Str("workspace", ws.ID).Str("buildId", buildID).
				Str("kind", string(req.Kind)).Str("target", string(req.Target)).Msg("mobile build failed")
			return
		}
		rec.Status = BuildStatusDone
		m.logger.Info().Str("workspace", ws.ID).Str("buildId", buildID).
			Str("kind", string(req.Kind)).Str("target", string(req.Target)).
			Int64("durationMs", res.DurationMS).Int64("size", res.ArtifactSizeBytes).
			Msg("mobile build done")
	}()
	return buildID
}

// LookupBuild returns the recorded build for buildID. The bool reports
// whether the build exists.
func (m *Manager) LookupBuild(buildID string) (BuildRecord, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.builds[buildID]
	if !ok {
		return BuildRecord{}, false
	}
	return *rec, true
}

// BuildBelongsTo reports whether the buildID was created for this
// workspace. Used by HTTP handlers to enforce per-user isolation.
func (m *Manager) BuildBelongsTo(workspaceID, buildID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	set, ok := m.byWS[workspaceID]
	if !ok {
		return false
	}
	return set[buildID]
}

// StartExpo launches `expo start --tunnel` inside the workspace and
// records the session. Idempotent: a second call for a workspace that
// already has a live session returns the existing one.
func (m *Manager) StartExpo(ctx context.Context, ws sandbox.Workspace) (ExpoSession, error) {
	m.mu.RLock()
	if s, ok := m.expo[ws.ID]; ok {
		m.mu.RUnlock()
		return s, nil
	}
	m.mu.RUnlock()

	session, err := startExpoServer(ctx, m.sandbox, ws)
	if err != nil {
		return ExpoSession{}, err
	}
	m.mu.Lock()
	m.expo[ws.ID] = session
	m.mu.Unlock()
	m.logger.Info().Str("workspace", ws.ID).Str("tunnel", session.TunnelURL).
		Str("lan", session.LANURL).Msg("expo dev server started")
	return session, nil
}

// StopExpo SIGTERMs the Metro/Expo process and clears the in-memory
// session record. Best-effort: clearing the record is unconditional even
// if the kill failed, so a stale PID never wedges the lifecycle.
func (m *Manager) StopExpo(ctx context.Context, workspaceID string) error {
	ws, err := m.sandbox.Get(workspaceID)
	if err != nil {
		return err
	}
	stopErr := stopExpoServer(ctx, m.sandbox, ws)
	m.mu.Lock()
	delete(m.expo, workspaceID)
	m.mu.Unlock()
	m.logger.Info().Str("workspace", workspaceID).Msg("expo dev server stopped")
	return stopErr
}

// LookupExpo returns the recorded session for the workspace, if any.
func (m *Manager) LookupExpo(workspaceID string) (ExpoSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.expo[workspaceID]
	return s, ok
}

// StartAndroidEmulator launches `emulator -avd <avd>` headless inside the
// workspace and waits for adb to confirm boot. The WebRTC bridge is a
// separate service we proxy later — for now we publish the relative
// path the frontend should dial.
func (m *Manager) StartAndroidEmulator(ctx context.Context, ws sandbox.Workspace, avd string) (EmulatorSession, error) {
	avd = strings.TrimSpace(avd)
	if avd == "" {
		return EmulatorSession{}, errors.New("avd name required")
	}
	m.mu.RLock()
	if s, ok := m.emulators[ws.ID]; ok && s.AVDName == avd {
		m.mu.RUnlock()
		return s, nil
	}
	m.mu.RUnlock()

	drv := m.sandbox.Driver()
	spawn := `set -e
mkdir -p /tmp/ironflyer-mobile
: > /tmp/ironflyer-mobile/emulator.out
: > /tmp/ironflyer-mobile/emulator.pid
nohup sh -c '$ANDROID_HOME/emulator/emulator -avd ` + shellQuote(avd) + ` -no-window -no-audio -no-boot-anim -gpu swiftshader_indirect -accel auto >/tmp/ironflyer-mobile/emulator.out 2>&1' &
echo $! > /tmp/ironflyer-mobile/emulator.pid
# Wait up to 60s for adb to enumerate the device.
$ANDROID_HOME/platform-tools/adb start-server >/dev/null 2>&1 || true
for i in $(seq 1 60); do
  if $ANDROID_HOME/platform-tools/adb devices | grep -qE 'emulator-[0-9]+\s+device'; then
    break
  fi
  sleep 1
done
$ANDROID_HOME/platform-tools/adb devices`
	res, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          spawn,
		TimeoutSeconds: 90,
	})
	if err != nil {
		return EmulatorSession{}, fmt.Errorf("emulator start: %w", err)
	}
	if !strings.Contains(res.Stdout, "emulator-") {
		return EmulatorSession{}, fmt.Errorf("emulator did not register with adb: %s", strings.TrimSpace(res.Stderr))
	}

	now := time.Now().UTC()
	// Default to the legacy in-runtime placeholder. When the bridge
	// is wired up (IRONFLYER_BRIDGE_URL + IRONFLYER_BRIDGE_TOKEN set
	// in the runtime env), swap to the absolute scrcpy-bridge WS URL.
	webrtcURL := fmt.Sprintf("/runtime/v1/workspaces/%s/emulator/webrtc", ws.ID)
	if alloc := bridgeAllocatorFromEnv(); alloc != nil {
		if url, err := alloc.Allocate(ctx, ws.ID, "emulator-5554"); err == nil {
			webrtcURL = url
		} else {
			m.logger.Warn().Err(err).Str("workspace", ws.ID).
				Msg("scrcpy bridge allocate failed; falling back to placeholder URL")
		}
	}
	session := EmulatorSession{
		ID:          "emu-" + uuid.NewString()[:8],
		WorkspaceID: ws.ID,
		AVDName:     avd,
		ADBPort:     5554, // standard emulator-5554 device id; real port picked by ADB inside the workspace
		WebRTCURL:   webrtcURL,
		StartedAt:   now,
		ExpiresAt:   now.Add(EmulatorSessionTTL),
	}
	m.mu.Lock()
	m.emulators[ws.ID] = session
	m.mu.Unlock()
	m.logger.Info().Str("workspace", ws.ID).Str("avd", avd).Str("emulator", session.ID).
		Msg("android emulator started")
	return session, nil
}

// StopAndroidEmulator best-effort SIGKILLs the emulator via `adb emu
// kill`. We don't surface adb failures because the emulator may have
// died already; the workspace destroy path will reclaim everything
// anyway.
func (m *Manager) StopAndroidEmulator(ctx context.Context, workspaceID string) error {
	ws, err := m.sandbox.Get(workspaceID)
	if err != nil {
		return err
	}
	drv := m.sandbox.Driver()
	_, _ = drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell: `set +e
$ANDROID_HOME/platform-tools/adb -s emulator-5554 emu kill 2>/dev/null || true
PID=$(cat /tmp/ironflyer-mobile/emulator.pid 2>/dev/null)
if [ -n "$PID" ]; then kill -TERM "$PID" 2>/dev/null || true; fi
rm -f /tmp/ironflyer-mobile/emulator.pid
exit 0`,
		TimeoutSeconds: 30,
	})
	m.mu.Lock()
	delete(m.emulators, workspaceID)
	m.mu.Unlock()
	m.logger.Info().Str("workspace", workspaceID).Msg("android emulator stopped")
	return nil
}

// LookupEmulator returns the session if one is registered.
func (m *Manager) LookupEmulator(workspaceID string) (EmulatorSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.emulators[workspaceID]
	return s, ok
}
