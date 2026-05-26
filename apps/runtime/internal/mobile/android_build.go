package mobile

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ironflyer/apps/runtime/internal/sandbox"
)

// androidGradleEnv assembles the env vars consumed by app/build.gradle.kts
// (the starter template reads these via System.getenv(...) inside the
// signingConfigs block). For debug builds we still pass everything that's
// populated so prebuilt JKS users don't have to special-case the env.
//
// EXPECTED signingConfigs.release in app/build.gradle.kts:
//
//	signingConfigs {
//	  create("release") {
//	    val ksPath = System.getenv("KEYSTORE_PATH")
//	    if (!ksPath.isNullOrEmpty()) {
//	      storeFile = file(ksPath)
//	      storePassword = System.getenv("STORE_PASSWORD")
//	      keyAlias = System.getenv("KEY_ALIAS")
//	      keyPassword = System.getenv("KEY_PASSWORD")
//	    }
//	  }
//	}
func androidGradleEnv(req BuildRequest) []string {
	var env []string
	if req.Signing.AndroidStorePass != "" {
		env = append(env, "STORE_PASSWORD="+req.Signing.AndroidStorePass)
	}
	if req.Signing.AndroidKeyAlias != "" {
		env = append(env, "KEY_ALIAS="+req.Signing.AndroidKeyAlias)
	}
	if req.Signing.AndroidKeyPass != "" {
		env = append(env, "KEY_PASSWORD="+req.Signing.AndroidKeyPass)
	}
	if req.Signing.AndroidKeystoreB64 != "" {
		env = append(env, "KEYSTORE_PATH=/tmp/upload.keystore")
	}
	if req.AppID != "" {
		env = append(env, "APP_ID="+req.AppID)
	}
	if req.Version != "" {
		env = append(env, "APP_VERSION="+req.Version)
	}
	return env
}

// buildAndroidNative drives a plain Android Studio project at the workspace
// root (no react-native or expo wrapper — the project IS the gradle root,
// so the assemble task lives at :app and the APK lands at
// app/build/outputs/... rather than android/app/...). For release the
// keystore bytes from req.Signing.AndroidKeystoreB64 are dropped onto
// disk at /tmp/upload.keystore and surfaced via KEYSTORE_PATH.
func buildAndroidNative(ctx context.Context, mgr *sandbox.Manager, ws sandbox.Workspace, req BuildRequest) (BuildResult, error) {
	start := time.Now()
	drv := mgr.Driver()
	release := strings.EqualFold(req.Profile, "production")

	// Materialize the keystore for release builds. We use a single shell
	// pipeline so the base64 payload never lands in argv (and therefore
	// never appears in /proc/<pid>/cmdline of any process snapshot).
	if release && req.Signing.AndroidKeystoreB64 != "" {
		write, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
			Shell: "umask 077 && printf %s " + shellQuote(req.Signing.AndroidKeystoreB64) +
				" | base64 -d > /tmp/upload.keystore",
			TimeoutSeconds: 30,
		})
		if err != nil {
			return BuildResult{}, fmt.Errorf("write keystore: %w", err)
		}
		if write.ExitCode != 0 {
			return BuildResult{
				ExitCode:   write.ExitCode,
				DurationMS: time.Since(start).Milliseconds(),
				LogTail:    tail(write.Stdout, write.Stderr),
			}, fmt.Errorf("write keystore exited %d", write.ExitCode)
		}
	}

	task := ":app:assembleDebug"
	artifact := "app/build/outputs/apk/debug/app-debug.apk"
	if release {
		task = ":app:assembleRelease"
		artifact = "app/build/outputs/apk/release/app-release.apk"
	}

	build, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          "./gradlew " + task + " -x lint --no-daemon --console=plain",
		TimeoutSeconds: 600,
		Env:            androidGradleEnv(req),
	})
	if err != nil {
		return BuildResult{}, fmt.Errorf("gradle: %w", err)
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
