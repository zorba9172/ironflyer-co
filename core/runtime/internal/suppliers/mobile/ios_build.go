package mobile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"ironflyer/core/runtime/internal/operations/sandbox"
)

// ErrMacPoolDisabled is returned by every iOS-native / Flutter-iOS build
// driver when the runtime is not running on (or wired to) a Mac host.
// The orchestrator surfaces this as a "iOS Pro tier required" gate
// finding rather than a hard 5xx.
var ErrMacPoolDisabled = errors.New("ios builds require IRONFLYER_MAC_POOL_ENABLED=1")

// macPoolEnabled reports whether the runtime is configured to drive an
// xcodebuild-capable host. Host provisioning is documented in
// infra/Dockerfile.mobile-runtime-mac.md.
func macPoolEnabled() bool {
	return strings.TrimSpace(os.Getenv("IRONFLYER_MAC_POOL_ENABLED")) == "1"
}

// buildIOSNative drives an Xcode project at the workspace root. Debug
// builds target the iPhone Simulator (no codesign); production builds
// import the .p12 + .mobileprovision before invoking xcodebuild so the
// keychain has the identity available.
//
// We use `xcodegen` to materialise the .xcodeproj from project.yml — the
// starter template ships project.yml only, keeping the diff-friendly
// representation as source of truth.
func buildIOSNative(ctx context.Context, mgr *sandbox.Manager, ws sandbox.Workspace, req BuildRequest) (BuildResult, error) {
	if !macPoolEnabled() {
		return BuildResult{}, ErrMacPoolDisabled
	}
	start := time.Now()
	drv := mgr.Driver()
	release := strings.EqualFold(req.Profile, "production")

	if _, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          "command -v xcodegen >/dev/null 2>&1 && make generate || xcodegen generate",
		TimeoutSeconds: 120,
	}); err != nil {
		return BuildResult{}, fmt.Errorf("xcodegen: %w", err)
	}

	if release {
		if err := importIOSSigning(ctx, drv, ws, req.Signing); err != nil {
			return BuildResult{}, err
		}
	}

	cfg := "Debug"
	sdk := "iphonesimulator"
	dest := "generic/platform=iOS Simulator"
	artifact := "build/Build/Products/Debug-iphonesimulator/IronflyerStarter.app"
	if release {
		cfg = "Release"
		sdk = "iphoneos"
		dest = "generic/platform=iOS"
		artifact = "build/Build/Products/Release-iphoneos/IronflyerStarter.app"
	}
	cmd := fmt.Sprintf(
		"xcodebuild -project IronflyerStarter.xcodeproj -scheme IronflyerStarter "+
			"-configuration %s -sdk %s -destination %s "+
			"-derivedDataPath build CODE_SIGNING_ALLOWED=%s build",
		cfg, sdk, shellQuote(dest), boolStr(release),
	)
	build, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          cmd,
		TimeoutSeconds: 900,
		Env:            iosEnv(req),
	})
	if err != nil {
		return BuildResult{}, fmt.Errorf("xcodebuild: %w", err)
	}
	res := BuildResult{
		ArtifactPath: artifact,
		ExitCode:     build.ExitCode,
		DurationMS:   time.Since(start).Milliseconds(),
		LogTail:      tail(build.Stdout, build.Stderr),
	}
	if build.ExitCode != 0 {
		return res, fmt.Errorf("xcodebuild exited %d", build.ExitCode)
	}
	if size, ok := statSize(ctx, drv, ws, artifact); ok {
		res.ArtifactSizeBytes = size
	}
	if sha, ok := statSHA(ctx, drv, ws, artifact); ok {
		res.ManifestSHA = sha
	}
	return res, nil
}

// importIOSSigning drops the .p12 and .mobileprovision onto disk and
// imports them into the build user's keychain + provisioning profile
// directory. Idempotent: re-running is safe because `security import`
// is a no-op when the cert already exists.
func importIOSSigning(ctx context.Context, drv sandbox.Driver, ws sandbox.Workspace, refs SigningRefs) error {
	if refs.IOSCertificateP12B64 == "" || refs.IOSProvisioningProfileB64 == "" {
		return errors.New("ios release requires p12 + provisioning profile")
	}
	pass := strings.TrimSpace(refs.IOSCertificatePassword)
	if pass == "" {
		return errors.New("ios certificate password required")
	}
	cmd := `set -e
umask 077
mkdir -p "$HOME/Library/MobileDevice/Provisioning Profiles"
printf %s ` + shellQuote(refs.IOSCertificateP12B64) + ` | base64 -d > /tmp/ironflyer.p12
printf %s ` + shellQuote(refs.IOSProvisioningProfileB64) + ` | base64 -d > /tmp/ironflyer.mobileprovision
security import /tmp/ironflyer.p12 -k "$HOME/Library/Keychains/login.keychain-db" \
  -P ` + shellQuote(pass) + ` -T /usr/bin/codesign -T /usr/bin/security >/dev/null 2>&1 || true
# Provisioning profile filename must match its UUID; xcodebuild walks the dir.
UUID=$(security cms -D -i /tmp/ironflyer.mobileprovision | plutil -extract UUID xml1 -o - - | sed -n 's:.*<string>\(.*\)</string>.*:\1:p')
if [ -n "$UUID" ]; then
  cp /tmp/ironflyer.mobileprovision "$HOME/Library/MobileDevice/Provisioning Profiles/$UUID.mobileprovision"
fi
rm -f /tmp/ironflyer.p12 /tmp/ironflyer.mobileprovision
`
	res, err := drv.Exec(ctx, ws, sandbox.ExecOpts{
		Shell:          cmd,
		TimeoutSeconds: 60,
	})
	if err != nil {
		return fmt.Errorf("ios signing import: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("ios signing import exited %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// iosEnv collects xcodebuild-visible env values (team ID, app ID).
func iosEnv(req BuildRequest) []string {
	var env []string
	if req.Signing.IOSTeamID != "" {
		env = append(env, "DEVELOPMENT_TEAM="+req.Signing.IOSTeamID)
	}
	if req.AppID != "" {
		env = append(env, "PRODUCT_BUNDLE_IDENTIFIER="+req.AppID)
	}
	if req.Version != "" {
		env = append(env, "MARKETING_VERSION="+req.Version)
	}
	return env
}

func boolStr(release bool) string {
	if release {
		return "YES"
	}
	return "NO"
}
