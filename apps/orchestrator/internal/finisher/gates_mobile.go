package finisher

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/runtime"
)

// appIDRegexp compiles domain.AppIDPattern once at package load. Re-used by
// the MobileBuildGate every iteration.
var appIDRegexp = regexp.MustCompile(domain.AppIDPattern)

// semverRegexp matches a permissive subset of semver: MAJOR.MINOR.PATCH with
// non-negative integers. We deliberately don't accept pre-release suffixes —
// store metadata uploads reject them anyway.
var semverRegexp = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// xcodeprojPbxRegexp matches "<anything>.xcodeproj/project.pbxproj" so the
// gate can confirm an Xcode project exists without knowing the project name.
var xcodeprojPbxRegexp = regexp.MustCompile(`(?i)(^|/)[^/]+\.xcodeproj/project\.pbxproj$`)

// infoPlistRegexp matches an iOS Info.plist at any depth.
var infoPlistRegexp = regexp.MustCompile(`(?i)(^|/)Info\.plist$`)

// MobileBuildGate enforces manifest validity AND a working debug build for
// every mobile target declared on the project's StackDecision. Skips
// cleanly on web-only projects.
type MobileBuildGate struct{}

func (MobileBuildGate) Name() domain.GateName    { return domain.GateMobileBuild }
func (MobileBuildGate) RepairAgent() agents.Role { return agents.RoleMobileCoder }

func (MobileBuildGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return nil
	}
	p := env.Project
	if !p.Spec.Stack.IsMobile() {
		// Web-only — gate stays dark.
		return nil
	}
	m := p.Spec.Stack.Mobile

	var issues []domain.Issue

	// 1. Static manifest validation (always runs, no runtime needed).
	issues = append(issues, validateMobileManifest(p, m)...)

	// 2. NeedsMacHost gate-time refusal (degraded warning only).
	macPool := os.Getenv("IRONFLYER_MAC_POOL_ENABLED") == "1"
	if m.NeedsMacHost() && !macPool {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityWarning,
			Message: "iOS build requires the Mac pool (Pro tier) — gate degraded",
			Hint:    "upgrade to the iOS Pro tier or switch to MobileKindExpo to build iOS in EAS cloud",
		})
	}

	// 3. Runtime build execution — only when a workspace is bound.
	if env.HasRuntime() {
		issues = append(issues, runMobileBuilds(ctx, env, m, macPool)...)
	}

	return issues
}

// validateMobileManifest runs all the file-and-config checks that don't
// need a runtime: AppID, version, per-Kind required files, manifest parse,
// signing secret presence, bundle identifier conflicts.
func validateMobileManifest(p *domain.Project, m domain.MobileStack) []domain.Issue {
	var issues []domain.Issue

	// AppID is mandatory and must be reverse-DNS.
	if strings.TrimSpace(m.AppID) == "" {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "mobile.appId is empty",
			Hint:    "set a reverse-DNS bundle identifier (e.g. com.acme.flightplan)",
		})
	} else if !appIDRegexp.MatchString(m.AppID) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "mobile.appId '" + m.AppID + "' is not a valid reverse-DNS identifier",
			Hint:    "two or more dot-separated segments, each starting with a letter; letters/digits/underscore only",
		})
	}

	if v := strings.TrimSpace(m.Version); v != "" && !semverRegexp.MatchString(v) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "mobile.version '" + v + "' is not a simple semver (MAJOR.MINOR.PATCH)",
		})
	}

	// Kind-specific required files.
	switch m.Kind {
	case domain.MobileKindExpo:
		issues = append(issues, validateExpoManifest(p, m)...)
	case domain.MobileKindReactNativeBare:
		issues = append(issues, validateReactNativeBareManifest(p, m)...)
	case domain.MobileKindAndroidNative:
		issues = append(issues, validateAndroidNativeManifest(p, m)...)
	case domain.MobileKindIOSNative:
		issues = append(issues, validateIOSNativeManifest(p, m)...)
	case domain.MobileKindFlutter:
		issues = append(issues, validateFlutterManifest(p, m)...)
	}

	// Signing secret presence.
	issues = append(issues, validateMobileSigning(p, m)...)

	return issues
}

// validateExpoManifest checks for app.json / app.config.{js,ts} and
// validates the JSON manifest's top-level expo.{name,slug,version}.
func validateExpoManifest(p *domain.Project, m domain.MobileStack) []domain.Issue {
	var issues []domain.Issue
	appJSONRaw, hasAppJSON := fileBody(p, "app.json")
	hasAppConfig := hasFile(p, "app.config.js") || hasFile(p, "app.config.ts")
	if !hasAppJSON && !hasAppConfig {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "expo project is missing app.json (or app.config.{js,ts})",
			Hint:    "create app.json with at minimum {\"expo\":{\"name\":...,\"slug\":...,\"version\":...}}",
		}}
	}
	if hasAppJSON {
		var doc struct {
			Expo struct {
				Name    string `json:"name"`
				Slug    string `json:"slug"`
				Version string `json:"version"`
				Android struct {
					Package string `json:"package"`
				} `json:"android"`
				IOS struct {
					BundleIdentifier string `json:"bundleIdentifier"`
				} `json:"ios"`
			} `json:"expo"`
		}
		if err := json.Unmarshal([]byte(appJSONRaw), &doc); err != nil {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
				Message: "app.json is not valid JSON: " + err.Error(),
				Path:    "app.json",
			})
			return issues
		}
		if strings.TrimSpace(doc.Expo.Name) == "" {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
				Message: "app.json: expo.name is empty", Path: "app.json",
			})
		}
		if strings.TrimSpace(doc.Expo.Slug) == "" {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
				Message: "app.json: expo.slug is empty", Path: "app.json",
			})
		}
		if strings.TrimSpace(doc.Expo.Version) == "" {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
				Message: "app.json: expo.version is empty", Path: "app.json",
			})
		}
		// Bundle identifier conflict.
		if m.AppID != "" {
			if pkg := strings.TrimSpace(doc.Expo.Android.Package); pkg != "" && pkg != m.AppID {
				issues = append(issues, domain.Issue{
					Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
					Message: "app.json expo.android.package '" + pkg + "' conflicts with mobile.appId '" + m.AppID + "'",
					Path:    "app.json",
				})
			}
			if bid := strings.TrimSpace(doc.Expo.IOS.BundleIdentifier); bid != "" && bid != m.AppID {
				issues = append(issues, domain.Issue{
					Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
					Message: "app.json expo.ios.bundleIdentifier '" + bid + "' conflicts with mobile.appId '" + m.AppID + "'",
					Path:    "app.json",
				})
			}
		}
	}
	return issues
}

// validateReactNativeBareManifest requires both an Android Gradle file and
// an iOS pbxproj at the expected paths.
func validateReactNativeBareManifest(p *domain.Project, m domain.MobileStack) []domain.Issue {
	var issues []domain.Issue
	if !hasFile(p, "android/build.gradle") && !hasFile(p, "android/build.gradle.kts") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "react-native-bare: missing android/build.gradle(.kts)",
			Hint:    "run `npx react-native init` then commit the android/ scaffold",
		})
	}
	if !anyFileMatches(p, xcodeprojPbxRegexp, "ios/") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "react-native-bare: missing ios/*.xcodeproj/project.pbxproj",
			Hint:    "run `npx react-native init` then commit the ios/ scaffold",
		})
	}
	// Bundle identifier conflict against AndroidManifest.xml applicationId.
	issues = append(issues, androidManifestConflictChecks(p, m, "android/app/src/main/AndroidManifest.xml")...)
	return issues
}

// validateAndroidNativeManifest requires app/build.gradle(.kts) plus the
// AndroidManifest at the canonical path.
func validateAndroidNativeManifest(p *domain.Project, m domain.MobileStack) []domain.Issue {
	var issues []domain.Issue
	if !hasFile(p, "app/build.gradle") && !hasFile(p, "app/build.gradle.kts") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "android-native: missing app/build.gradle(.kts)",
		})
	}
	if !hasFile(p, "app/src/main/AndroidManifest.xml") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "android-native: missing app/src/main/AndroidManifest.xml",
		})
	}
	// applicationId conflict check against build.gradle.
	issues = append(issues, androidGradleAppIDConflictChecks(p, m)...)
	issues = append(issues, androidManifestConflictChecks(p, m, "app/src/main/AndroidManifest.xml")...)
	return issues
}

// validateIOSNativeManifest requires an xcodeproj and an Info.plist.
func validateIOSNativeManifest(p *domain.Project, m domain.MobileStack) []domain.Issue {
	var issues []domain.Issue
	if !anyFileMatches(p, xcodeprojPbxRegexp, "") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "ios-native: missing *.xcodeproj/project.pbxproj",
		})
	}
	if !anyFileMatches(p, infoPlistRegexp, "") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "ios-native: missing Info.plist",
		})
	}
	issues = append(issues, infoPlistConflictChecks(p, m)...)
	return issues
}

// validateFlutterManifest requires pubspec.yaml and verifies it parses
// well enough to extract `name:` / `version:` at the top level.
func validateFlutterManifest(p *domain.Project, m domain.MobileStack) []domain.Issue {
	body, ok := fileBody(p, "pubspec.yaml")
	if !ok {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "flutter project: missing pubspec.yaml",
			Hint:    "run `flutter create` to scaffold the pubspec",
		}}
	}
	name, ver, parseErr := scanPubspecKeys(body)
	if parseErr != "" {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "pubspec.yaml parse: " + parseErr,
			Path:    "pubspec.yaml",
		}}
	}
	var issues []domain.Issue
	if name == "" {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "pubspec.yaml: missing `name:`", Path: "pubspec.yaml",
		})
	}
	if ver != "" && !semverRegexp.MatchString(strings.SplitN(ver, "+", 2)[0]) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityWarning,
			Message: "pubspec.yaml: version '" + ver + "' is not simple semver",
			Path:    "pubspec.yaml",
		})
	}
	_ = m
	return issues
}

// validateMobileSigning verifies named secrets exist in Project.Secrets and
// flags missing release-build signing material.
func validateMobileSigning(p *domain.Project, m domain.MobileStack) []domain.Issue {
	if m.Signing == nil {
		return nil
	}
	s := m.Signing
	releaseProfile := m.EAS != nil && strings.EqualFold(strings.TrimSpace(m.EAS.Profile), "production")

	checks := []struct {
		key, label string
	}{
		{s.AndroidKeystoreSecret, "android keystore"},
		{s.AndroidStorePasswordSecret, "android store password"},
		{s.AndroidKeyPasswordSecret, "android key password"},
		{s.IOSProvisioningProfileSecret, "ios provisioning profile"},
		{s.IOSCertificateP12Secret, "ios certificate p12"},
		{s.IOSCertificatePasswordSecret, "ios certificate password"},
	}
	var issues []domain.Issue
	for _, c := range checks {
		if strings.TrimSpace(c.key) == "" {
			continue
		}
		if _, ok := p.Secrets[c.key]; !ok {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
				Message: "signing secret '" + c.key + "' (" + c.label + ") is referenced but not present in Project.Secrets",
				Hint:    "upload the secret via the project's vault before triggering a release build",
			})
		}
	}

	if releaseProfile {
		targets := m.EffectiveTargets()
		wantAndroid := containsTarget(targets, domain.MobileTargetAndroid)
		wantIOS := containsTarget(targets, domain.MobileTargetIOS)
		if wantAndroid {
			if strings.TrimSpace(s.AndroidKeystoreSecret) == "" || strings.TrimSpace(s.AndroidKeyAlias) == "" {
				issues = append(issues, domain.Issue{
					Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
					Message: "EAS production profile requires android keystore + key alias",
					Hint:    "set mobile.signing.androidKeystoreSecret and androidKeyAlias",
				})
			}
		}
		if wantIOS {
			if strings.TrimSpace(s.IOSProvisioningProfileSecret) == "" ||
				strings.TrimSpace(s.IOSCertificateP12Secret) == "" {
				issues = append(issues, domain.Issue{
					Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
					Message: "EAS production profile requires iOS provisioning profile + p12 certificate",
					Hint:    "set mobile.signing.iosProvisioningProfileSecret and iosCertificateP12Secret",
				})
			}
		}
	}
	return issues
}

// androidManifestConflictChecks scans AndroidManifest.xml for an explicit
// package="..." attribute and warns when it disagrees with Mobile.AppID.
func androidManifestConflictChecks(p *domain.Project, m domain.MobileStack, manifestPath string) []domain.Issue {
	if m.AppID == "" {
		return nil
	}
	body, ok := fileBody(p, manifestPath)
	if !ok {
		return nil
	}
	if pkg := scanXMLAttr(body, "package"); pkg != "" && pkg != m.AppID {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "AndroidManifest.xml package='" + pkg + "' conflicts with mobile.appId '" + m.AppID + "'",
			Path:    manifestPath,
		}}
	}
	return nil
}

// androidGradleAppIDConflictChecks scans app/build.gradle(.kts) for an
// applicationId line and flags drift from Mobile.AppID.
func androidGradleAppIDConflictChecks(p *domain.Project, m domain.MobileStack) []domain.Issue {
	if m.AppID == "" {
		return nil
	}
	for _, path := range []string{"app/build.gradle", "app/build.gradle.kts"} {
		body, ok := fileBody(p, path)
		if !ok {
			continue
		}
		if appID := scanGradleApplicationID(body); appID != "" && appID != m.AppID {
			return []domain.Issue{{
				Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
				Message: "applicationId '" + appID + "' in " + path + " conflicts with mobile.appId '" + m.AppID + "'",
				Path:    path,
			}}
		}
	}
	return nil
}

// infoPlistConflictChecks scans an Info.plist for the
// CFBundleIdentifier string value (when not a template like
// $(PRODUCT_BUNDLE_IDENTIFIER)) and flags drift from Mobile.AppID.
func infoPlistConflictChecks(p *domain.Project, m domain.MobileStack) []domain.Issue {
	if m.AppID == "" {
		return nil
	}
	// Walk every file matching infoPlistRegexp.
	for _, f := range p.Files {
		if !infoPlistRegexp.MatchString(f.Path) {
			continue
		}
		bid := scanPlistCFBundleIdentifier(f.Content)
		if bid == "" || strings.Contains(bid, "$(") {
			continue
		}
		if bid != m.AppID {
			return []domain.Issue{{
				Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
				Message: "CFBundleIdentifier '" + bid + "' in " + f.Path + " conflicts with mobile.appId '" + m.AppID + "'",
				Path:    f.Path,
			}}
		}
	}
	return nil
}

// runMobileBuilds executes the platform-appropriate debug build for each
// effective target. iOS targets that require a Mac without a Mac pool emit
// a SeverityInfo "deferred to EAS cloud" issue and don't run.
func runMobileBuilds(ctx context.Context, env *GateEnv, m domain.MobileStack, macPool bool) []domain.Issue {
	var issues []domain.Issue
	for _, t := range m.EffectiveTargets() {
		switch t {
		case domain.MobileTargetAndroid:
			issues = append(issues, runMobileAndroidBuild(ctx, env, m)...)
		case domain.MobileTargetIOS:
			if !macPool {
				issues = append(issues, domain.Issue{
					Gate: domain.GateMobileBuild, Severity: domain.SeverityInfo,
					Message: "iOS build deferred to EAS cloud (no Mac pool available locally)",
					Hint:    "set IRONFLYER_MAC_POOL_ENABLED=1 once a Mac runner is online, or rely on EAS cloud builds",
				})
				continue
			}
			issues = append(issues, runMobileIOSBuild(ctx, env, m)...)
		}
	}
	return issues
}

// runMobileAndroidBuild picks the right android command for the project's
// Kind, executes it with a 10-minute timeout, then size-checks the
// resulting APK.
func runMobileAndroidBuild(ctx context.Context, env *GateEnv, m domain.MobileStack) []domain.Issue {
	cmd, label, apkPath, ok := androidBuildCommand(m)
	if !ok {
		return nil
	}
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 600,
	})
	if err != nil {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "android build exec: " + err.Error(), Hint: label,
		}}
	}
	if res.TimedOut {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityCritical,
			Message: "android build timed out after 600s", Hint: label,
		}}
	}
	if res.ExitCode != 0 {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "android build failed (exit " + itoaPositive(res.ExitCode) + ")",
			Hint:    label + " — " + tail(res.Stderr+res.Stdout, 600),
		}}
	}
	// APK existence + size check.
	return append(checkAndroidAPKOutput(ctx, env, apkPath),
		releaseAPKSigningCheck(ctx, env, m, apkPath)...)
}

// runMobileIOSBuild picks the right iOS command for the project's Kind and
// runs it under the Mac pool (caller has already verified IRONFLYER_MAC_POOL_ENABLED).
func runMobileIOSBuild(ctx context.Context, env *GateEnv, m domain.MobileStack) []domain.Issue {
	cmd, label, appBundlePrefix, ok := iosBuildCommand(m)
	if !ok {
		return nil
	}
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 600,
	})
	if err != nil {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "ios build exec: " + err.Error(), Hint: label,
		}}
	}
	if res.TimedOut {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityCritical,
			Message: "ios build timed out after 600s", Hint: label,
		}}
	}
	if res.ExitCode != 0 {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "ios build failed (exit " + itoaPositive(res.ExitCode) + ")",
			Hint:    label + " — " + tail(res.Stderr+res.Stdout, 600),
		}}
	}
	return checkIOSAppBundleOutput(ctx, env, appBundlePrefix)
}

// androidBuildCommand returns (shellCmd, label, expectedAPKPath, ok) for
// the project's mobile Kind. Returns ok=false when there's nothing to build.
func androidBuildCommand(m domain.MobileStack) (string, string, string, bool) {
	switch m.Kind {
	case domain.MobileKindExpo:
		return "npx expo prebuild --platform android --no-install && cd android && ./gradlew assembleDebug -x lint",
			"expo android debug",
			"android/app/build/outputs/apk/debug/app-debug.apk",
			true
	case domain.MobileKindReactNativeBare:
		return "cd android && ./gradlew assembleDebug -x lint",
			"react-native-bare android debug",
			"android/app/build/outputs/apk/debug/app-debug.apk",
			true
	case domain.MobileKindAndroidNative:
		return "./gradlew :app:assembleDebug -x lint",
			"android-native debug",
			"app/build/outputs/apk/debug/app-debug.apk",
			true
	case domain.MobileKindFlutter:
		return "flutter build apk --debug",
			"flutter android debug",
			"build/app/outputs/flutter-apk/app-debug.apk",
			true
	}
	return "", "", "", false
}

// iosBuildCommand returns (shellCmd, label, appBundleSearchPrefix, ok).
// Caller must guarantee the Mac pool is available.
func iosBuildCommand(m domain.MobileStack) (string, string, string, bool) {
	switch m.Kind {
	case domain.MobileKindExpo:
		profile := "preview"
		if m.EAS != nil && strings.TrimSpace(m.EAS.Profile) != "" {
			profile = strings.TrimSpace(m.EAS.Profile)
		}
		return "eas build --platform ios --profile=" + shellQuote(profile) + " --local --non-interactive",
			"eas ios " + profile,
			"ios/build/Build/Products/Debug-iphonesimulator/",
			true
	case domain.MobileKindReactNativeBare:
		return "xcodebuild -workspace ios/*.xcworkspace -scheme app -configuration Debug -sdk iphonesimulator build",
			"xcodebuild ios debug",
			"ios/build/Build/Products/Debug-iphonesimulator/",
			true
	case domain.MobileKindIOSNative:
		return "xcodebuild -project *.xcodeproj -scheme app -configuration Debug -sdk iphonesimulator build",
			"ios-native debug",
			"build/Debug-iphonesimulator/",
			true
	case domain.MobileKindFlutter:
		return "flutter build ios --debug --no-codesign",
			"flutter ios debug",
			"build/ios/iphonesimulator/Runner.app",
			true
	}
	return "", "", "", false
}

// checkAndroidAPKOutput confirms the APK exists and warns when it weighs
// more than 200 MiB.
func checkAndroidAPKOutput(ctx context.Context, env *GateEnv, apkPath string) []domain.Issue {
	cmd := "test -f " + shellQuote(apkPath) + " && stat -c %s " + shellQuote(apkPath) + " 2>/dev/null || stat -f %z " + shellQuote(apkPath)
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 30,
	})
	if err != nil || res.TimedOut || res.ExitCode != 0 {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "android build claimed success but APK missing at " + apkPath,
			Hint:    "verify gradle output paths and that assembleDebug actually ran",
		}}
	}
	sz := parseSizeBytes(res.Stdout)
	if sz <= 0 {
		return nil
	}
	const limit = 200 * 1024 * 1024
	if sz > limit {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityWarning,
			Message: "android APK is " + formatMiB(sz) + " — exceeds 200 MiB threshold",
			Hint:    "split asset packs, enable R8 shrinking, or trim debug symbols",
			Path:    apkPath,
		}}
	}
	return nil
}

// checkIOSAppBundleOutput confirms a .app bundle landed under the expected
// build directory. The prefix may be a directory or a fully-qualified .app
// path (e.g. Flutter's build/ios/iphonesimulator/Runner.app).
func checkIOSAppBundleOutput(ctx context.Context, env *GateEnv, prefix string) []domain.Issue {
	var cmd string
	if strings.HasSuffix(prefix, ".app") || strings.HasSuffix(prefix, ".app/") {
		cmd = "test -d " + shellQuote(strings.TrimSuffix(prefix, "/"))
	} else {
		// Look for at least one .app under the prefix directory.
		cmd = "ls -d " + shellQuote(strings.TrimSuffix(prefix, "/")) + "/*.app >/dev/null 2>&1"
	}
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 30,
	})
	if err != nil || res.TimedOut || res.ExitCode != 0 {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityError,
			Message: "ios build claimed success but no .app bundle found under " + prefix,
			Hint:    "verify the xcodebuild scheme/configuration produced a Debug-iphonesimulator build",
		}}
	}
	return nil
}

// releaseAPKSigningCheck verifies an APK targeted at the production EAS
// profile carries a non-empty CERT.SF entry — a quick proxy for "this APK
// is signed". Only runs when EAS.Profile == "production".
func releaseAPKSigningCheck(ctx context.Context, env *GateEnv, m domain.MobileStack, apkPath string) []domain.Issue {
	if m.EAS == nil || !strings.EqualFold(strings.TrimSpace(m.EAS.Profile), "production") {
		return nil
	}
	cmd := "unzip -p " + shellQuote(apkPath) + " META-INF/CERT.SF 2>/dev/null || true"
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 30,
	})
	if err != nil || res.TimedOut {
		// Best-effort — don't fail the gate on a degraded helper.
		return nil
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return []domain.Issue{{
			Gate: domain.GateMobileBuild, Severity: domain.SeverityWarning,
			Message: "release APK appears unsigned — META-INF/CERT.SF is empty",
			Hint:    "verify mobile.signing.androidKeystoreSecret is wired and gradle signingConfigs picked it up",
			Path:    apkPath,
		}}
	}
	return nil
}

// ---- helpers ----

// anyFileMatches reports whether any file in the project matches re; when
// pathPrefix is non-empty the file path must also start with it (case
// insensitive). Lets us say "any *.xcodeproj/project.pbxproj under ios/".
func anyFileMatches(p *domain.Project, re *regexp.Regexp, pathPrefix string) bool {
	prefix := strings.ToLower(pathPrefix)
	for _, f := range p.Files {
		lp := strings.ToLower(strings.TrimPrefix(f.Path, "/"))
		if prefix != "" && !strings.HasPrefix(lp, prefix) {
			continue
		}
		if re.MatchString(f.Path) {
			return true
		}
	}
	return false
}

// containsTarget is a small util because importing slices.Contains adds no
// value when the slices are 1–2 elements.
func containsTarget(ts []domain.MobileTarget, want domain.MobileTarget) bool {
	for _, t := range ts {
		if t == want {
			return true
		}
	}
	return false
}

// scanPubspecKeys extracts the top-level `name:` and `version:` from a
// pubspec.yaml WITHOUT pulling in a yaml dependency. Only top-level keys
// matter for the manifest gate. Returns (name, version, parseErr) — parseErr
// is non-empty when the file is structurally broken (e.g. tab-indented).
func scanPubspecKeys(body string) (string, string, string) {
	var name, version string
	for i, raw := range strings.Split(body, "\n") {
		line := raw
		// Strip comments.
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(line, "\t") {
			return "", "", "tabs are illegal in YAML (line " + itoaPositive(i+1) + ")"
		}
		// Only top-level keys (no leading spaces).
		if line[0] == ' ' {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		val = strings.Trim(val, `"'`)
		switch key {
		case "name":
			name = val
		case "version":
			version = val
		}
	}
	return name, version, ""
}

// scanXMLAttr returns the first occurrence of attr="value" anywhere in body.
// Sufficient for AndroidManifest.xml package="..." extraction without
// dragging in encoding/xml.
func scanXMLAttr(body, attr string) string {
	needle := attr + "=\""
	idx := strings.Index(body, needle)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(needle):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// scanGradleApplicationID returns the first `applicationId "..."` (Groovy)
// or `applicationId = "..."` (Kotlin DSL) value, ignoring commented lines.
func scanGradleApplicationID(body string) string {
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if !strings.HasPrefix(line, "applicationId") {
			continue
		}
		// Trim the keyword, then any "=" sign, then surrounding quotes.
		rest := strings.TrimSpace(strings.TrimPrefix(line, "applicationId"))
		rest = strings.TrimPrefix(rest, "=")
		rest = strings.TrimSpace(rest)
		if len(rest) >= 2 && (rest[0] == '"' || rest[0] == '\'') {
			q := rest[0]
			end := strings.IndexByte(rest[1:], q)
			if end >= 0 {
				return rest[1 : 1+end]
			}
		}
	}
	return ""
}

// scanPlistCFBundleIdentifier returns the <string> value following a
// <key>CFBundleIdentifier</key> entry in an Info.plist. Returns "" when not
// present. Good enough for the conflict check without parsing XML/plist.
func scanPlistCFBundleIdentifier(body string) string {
	const key = "<key>CFBundleIdentifier</key>"
	idx := strings.Index(body, key)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(key):]
	open := strings.Index(rest, "<string>")
	if open < 0 {
		return ""
	}
	rest = rest[open+len("<string>"):]
	close := strings.Index(rest, "</string>")
	if close < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:close])
}

// parseSizeBytes reads the integer prefix of a stat-style output (just a
// decimal number, possibly with trailing whitespace).
func parseSizeBytes(s string) int64 {
	s = strings.TrimSpace(s)
	var out int64
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		out = out*10 + int64(r-'0')
	}
	return out
}

// formatMiB renders a byte count as e.g. "212.4 MiB".
func formatMiB(n int64) string {
	const mib = 1024 * 1024
	whole := n / mib
	frac := (n % mib) * 10 / mib
	return itoaPositive(int(whole)) + "." + itoaPositive(int(frac)) + " MiB"
}
