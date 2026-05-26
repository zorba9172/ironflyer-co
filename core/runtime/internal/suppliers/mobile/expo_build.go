package mobile

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"ironflyer/core/runtime/internal/operations/sandbox"
)

// buildExpoAndroid handles the Expo / managed-workflow Android build.
// We prebuild into android/ first (no install — node_modules is already
// present from the workspace's auto-build) and then drive Gradle. The
// debug APK is emitted at the standard RN path; we surface it so the
// orchestrator can upload to artifact storage.
func buildExpoAndroid(ctx context.Context, mgr *sandbox.Manager, ws sandbox.Workspace, req BuildRequest) (BuildResult, error) {
	start := time.Now()
	drv := mgr.Driver()

	prebuild, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          "npx --no-install expo prebuild --platform android --no-install --non-interactive",
		TimeoutSeconds: 600,
	})
	if err != nil {
		return BuildResult{}, fmt.Errorf("expo prebuild: %w", err)
	}
	if prebuild.ExitCode != 0 {
		return BuildResult{
			ExitCode:   prebuild.ExitCode,
			DurationMS: time.Since(start).Milliseconds(),
			LogTail:    tail(prebuild.Stdout, prebuild.Stderr),
		}, fmt.Errorf("expo prebuild exited %d", prebuild.ExitCode)
	}

	gradleTask := ":app:assembleDebug"
	if strings.EqualFold(req.Profile, "production") {
		gradleTask = ":app:assembleRelease"
	}
	build, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          fmt.Sprintf("cd android && ./gradlew %s -x lint --no-daemon --console=plain", gradleTask),
		TimeoutSeconds: 600,
		Env:            androidGradleEnv(req),
	})
	if err != nil {
		return BuildResult{}, fmt.Errorf("gradle assemble: %w", err)
	}

	artifact := "android/app/build/outputs/apk/debug/app-debug.apk"
	if strings.EqualFold(req.Profile, "production") {
		artifact = "android/app/build/outputs/apk/release/app-release.apk"
	}
	res := BuildResult{
		ArtifactPath: artifact,
		ExitCode:     build.ExitCode,
		DurationMS:   time.Since(start).Milliseconds(),
		LogTail:      tail(build.Stdout, build.Stderr),
	}
	if build.ExitCode != 0 {
		return res, fmt.Errorf("gradle exited %d", build.ExitCode)
	}
	if size, ok := statSize(ctx, drv, ws, artifact); ok {
		res.ArtifactSizeBytes = size
	}
	if sha, ok := statSHA(ctx, drv, ws, artifact); ok {
		res.ManifestSHA = sha
	}
	return res, nil
}

var easBuildIDRe = regexp.MustCompile(`Build ID:\s*([0-9a-fA-F-]{8,})`)

// buildExpoIOSEAS uses EAS cloud builds — no Mac required. We dispatch
// the build to Expo's servers and return the build ID; the orchestrator
// polls EAS for the artifact URL once the cloud build completes.
func buildExpoIOSEAS(ctx context.Context, mgr *sandbox.Manager, ws sandbox.Workspace, req BuildRequest) (BuildResult, error) {
	start := time.Now()
	drv := mgr.Driver()
	profile := strings.TrimSpace(req.Profile)
	if profile == "" {
		profile = "preview"
	}
	cmd := fmt.Sprintf(
		"eas build --platform ios --profile=%s --non-interactive --no-wait --json",
		shellQuote(profile),
	)
	res, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          cmd,
		TimeoutSeconds: 600,
	})
	if err != nil {
		return BuildResult{}, fmt.Errorf("eas build: %w", err)
	}
	br := BuildResult{
		ExitCode:   res.ExitCode,
		DurationMS: time.Since(start).Milliseconds(),
		LogTail:    tail(res.Stdout, res.Stderr),
	}
	if res.ExitCode != 0 {
		return br, fmt.Errorf("eas build exited %d", res.ExitCode)
	}
	if m := easBuildIDRe.FindStringSubmatch(res.Stdout + "\n" + res.Stderr); len(m) == 2 {
		br.ArtifactPath = "eas://" + m[1]
	} else {
		// JSON output: best-effort grep for "id" key. We avoid a json
		// decode here because EAS may emit either a single object or an
		// array depending on version. The orchestrator does the
		// authoritative poll.
		br.ArtifactPath = "eas://pending"
	}
	return br, nil
}

// Listening URL parsing for `expo start`. Stdout looks like:
//
//	› Metro waiting on exp://192.168.1.10:19000
//	› Tunnel ready. › exp://abc123.exp.direct
var (
	expoLANRe    = regexp.MustCompile(`exp://([0-9.]+:\d+)`)
	expoTunnelRe = regexp.MustCompile(`exp://([a-zA-Z0-9.-]+\.exp\.direct(?::\d+)?)`)
	expoMetroRe  = regexp.MustCompile(`http://(localhost|127\.0\.0\.1):(\d+)`)
)

// parseExpoOutput extracts the URLs Expo prints on startup. Safe to call
// with partial output; missing fields stay empty.
func parseExpoOutput(out string) (metroURL, lanURL, tunnelURL string) {
	if m := expoMetroRe.FindStringSubmatch(out); len(m) == 3 {
		metroURL = "http://" + m[1] + ":" + m[2]
	}
	if m := expoLANRe.FindStringSubmatch(out); len(m) == 2 {
		lanURL = "exp://" + m[1]
	}
	if m := expoTunnelRe.FindStringSubmatch(out); len(m) == 2 {
		tunnelURL = "exp://" + m[1]
	}
	return
}

// startExpoServer launches `expo start` in the background and waits long
// enough to capture the printed URLs. We don't keep the process attached
// — the Driver.Exec contract runs once and returns; the actual long-lived
// Metro process keeps running inside the workspace until SIGTERM'd via
// the stop endpoint.
func startExpoServer(ctx context.Context, mgr *sandbox.Manager, ws sandbox.Workspace) (ExpoSession, error) {
	drv := mgr.Driver()
	// Spawn in background with nohup, capture the first ~30s of output to
	// /tmp/expo.out so we can read URLs after the process is daemonised.
	spawn := `set -e
mkdir -p /tmp/ironflyer-mobile
: > /tmp/ironflyer-mobile/expo.out
: > /tmp/ironflyer-mobile/expo.pid
nohup sh -c 'npx --no-install expo start --port 19000 --tunnel --non-interactive >/tmp/ironflyer-mobile/expo.out 2>&1' &
echo $! > /tmp/ironflyer-mobile/expo.pid
# Give Metro + ngrok up to 45s to advertise URLs.
for i in $(seq 1 45); do
  if grep -qE 'exp://[a-zA-Z0-9.]+\.exp\.direct' /tmp/ironflyer-mobile/expo.out; then break; fi
  sleep 1
done
cat /tmp/ironflyer-mobile/expo.out`
	res, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          spawn,
		TimeoutSeconds: 60,
	})
	if err != nil {
		return ExpoSession{}, fmt.Errorf("expo start: %w", err)
	}
	metro, lan, tunnel := parseExpoOutput(res.Stdout)
	if metro == "" && lan == "" && tunnel == "" {
		return ExpoSession{}, fmt.Errorf("expo start did not advertise URLs: %s", strings.TrimSpace(res.Stderr))
	}
	return ExpoSession{
		WorkspaceID: ws.ID,
		MetroURL:    metro,
		LANURL:      lan,
		TunnelURL:   tunnel,
		QRPayload:   buildExpoQRPayload(tunnel, lan),
		StartedAt:   time.Now().UTC(),
	}, nil
}

// stopExpoServer SIGTERMs the recorded PID. Best-effort: if the PID file
// is missing we still return nil because the caller has already cleared
// the in-memory session.
func stopExpoServer(ctx context.Context, mgr *sandbox.Manager, ws sandbox.Workspace) error {
	drv := mgr.Driver()
	_, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell: `set +e
PID=$(cat /tmp/ironflyer-mobile/expo.pid 2>/dev/null)
if [ -n "$PID" ]; then kill -TERM "$PID" 2>/dev/null || true; fi
rm -f /tmp/ironflyer-mobile/expo.pid
exit 0`,
		TimeoutSeconds: 15,
	})
	if err != nil {
		return errors.New("stop expo: " + err.Error())
	}
	return nil
}

// shellQuote single-quotes a value safely for /bin/sh -c.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// tail returns a bounded log tail by joining stdout + stderr and
// trimming to the last 8 KiB.
func tail(stdout, stderr string) string {
	combined := stdout
	if stderr != "" {
		if combined != "" {
			combined += "\n"
		}
		combined += stderr
	}
	const max = 8 << 10
	if len(combined) > max {
		combined = combined[len(combined)-max:]
	}
	return combined
}

// statSize reports the file size at workspace-relative path; missing
// files return ok=false rather than an error so the build can still
// surface a partial result.
func statSize(ctx context.Context, drv sandbox.Driver, ws sandbox.Workspace, p string) (int64, bool) {
	res, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          "stat -c %s " + shellQuote(p) + " 2>/dev/null || wc -c < " + shellQuote(p) + " 2>/dev/null || echo 0",
		TimeoutSeconds: 10,
	})
	if err != nil || res.ExitCode != 0 {
		return 0, false
	}
	var size int64
	_, scanErr := fmt.Sscanf(strings.TrimSpace(res.Stdout), "%d", &size)
	if scanErr != nil || size <= 0 {
		return 0, false
	}
	return size, true
}

// statSHA returns the sha256 of the artifact (used for ManifestSHA so the
// orchestrator can dedupe identical builds).
func statSHA(ctx context.Context, drv sandbox.Driver, ws sandbox.Workspace, p string) (string, bool) {
	res, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          "sha256sum " + shellQuote(p) + " 2>/dev/null | awk '{print $1}'",
		TimeoutSeconds: 30,
	})
	if err != nil || res.ExitCode != 0 {
		return "", false
	}
	sha := strings.TrimSpace(res.Stdout)
	if sha == "" {
		return "", false
	}
	return sha, true
}

// silenceUnused keeps `path` in scope when future build drivers compose
// artifact paths via path.Join — keeps the import in one place.
var _ = path.Join
