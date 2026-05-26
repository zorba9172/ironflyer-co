package mobile

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ironflyer/apps/runtime/internal/sandbox"
)

// buildFlutterAndroid runs `flutter build apk` at the workspace root.
// Debug and release share the same artifact directory layout — only the
// filename suffix changes. Signing is delegated to the standard Gradle
// signingConfigs path (same env vars as the android-native driver).
func buildFlutterAndroid(ctx context.Context, mgr *sandbox.Manager, ws sandbox.Workspace, req BuildRequest) (BuildResult, error) {
	start := time.Now()
	drv := mgr.Driver()
	release := strings.EqualFold(req.Profile, "production")

	// Same keystore materialisation pattern as android-native, gated on
	// release builds where signing material is mandatory.
	if release && req.Signing.AndroidKeystoreB64 != "" {
		write, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
			Shell: "umask 077 && printf %s " + shellQuote(req.Signing.AndroidKeystoreB64) +
				" | base64 -d > /tmp/upload.keystore",
			TimeoutSeconds: 30,
		})
		if err != nil || write.ExitCode != 0 {
			return BuildResult{}, fmt.Errorf("write keystore: %w", err)
		}
	}

	mode := "--debug"
	artifact := "build/app/outputs/flutter-apk/app-debug.apk"
	if release {
		mode = "--release"
		artifact = "build/app/outputs/flutter-apk/app-release.apk"
	}
	build, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          "flutter build apk " + mode,
		TimeoutSeconds: 900,
		Env:            androidGradleEnv(req),
	})
	if err != nil {
		return BuildResult{}, fmt.Errorf("flutter build: %w", err)
	}
	res := BuildResult{
		ArtifactPath: artifact,
		ExitCode:     build.ExitCode,
		DurationMS:   time.Since(start).Milliseconds(),
		LogTail:      tail(build.Stdout, build.Stderr),
	}
	if build.ExitCode != 0 {
		return res, fmt.Errorf("flutter build exited %d", build.ExitCode)
	}
	if size, ok := statSize(ctx, drv, ws, artifact); ok {
		res.ArtifactSizeBytes = size
	}
	if sha, ok := statSHA(ctx, drv, ws, artifact); ok {
		res.ManifestSHA = sha
	}
	return res, nil
}

// buildFlutterIOS runs `flutter build ios` and is gated behind the Mac
// pool (same as the iOS-native driver). Debug builds skip codesign so
// they can run on the simulator without the orchestrator resolving any
// certificate material.
func buildFlutterIOS(ctx context.Context, mgr *sandbox.Manager, ws sandbox.Workspace, req BuildRequest) (BuildResult, error) {
	if !macPoolEnabled() {
		return BuildResult{}, ErrMacPoolDisabled
	}
	start := time.Now()
	drv := mgr.Driver()
	release := strings.EqualFold(req.Profile, "production")
	if release {
		if err := importIOSSigning(ctx, drv, ws, req.Signing); err != nil {
			return BuildResult{}, err
		}
	}
	mode := "--debug --no-codesign"
	artifact := "build/ios/iphonesimulator/Runner.app"
	if release {
		mode = "--release"
		artifact = "build/ios/iphoneos/Runner.app"
	}
	build, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          "flutter build ios " + mode,
		TimeoutSeconds: 900,
		Env:            iosEnv(req),
	})
	if err != nil {
		return BuildResult{}, fmt.Errorf("flutter build ios: %w", err)
	}
	res := BuildResult{
		ArtifactPath: artifact,
		ExitCode:     build.ExitCode,
		DurationMS:   time.Since(start).Milliseconds(),
		LogTail:      tail(build.Stdout, build.Stderr),
	}
	if build.ExitCode != 0 {
		return res, fmt.Errorf("flutter build ios exited %d", build.ExitCode)
	}
	if size, ok := statSize(ctx, drv, ws, artifact); ok {
		res.ArtifactSizeBytes = size
	}
	if sha, ok := statSHA(ctx, drv, ws, artifact); ok {
		res.ManifestSHA = sha
	}
	return res, nil
}
