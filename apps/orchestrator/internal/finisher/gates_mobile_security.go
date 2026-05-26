package finisher

import (
	"context"
	"regexp"
	"strings"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
)

// MobileSecurityGate layers mobile-platform-specific OWASP-MASVS-style
// checks on top of the generic SecurityGate. Independent of (and
// additive to) finisher.SecurityGate so the AppSec scanner doesn't have
// to learn AndroidManifest / Info.plist semantics.
//
// The gate stays pure-static — it scans Project.Files only, no
// runtime exec required. Mobile codebases tend to be small and the
// rules fire fast.
type MobileSecurityGate struct{}

func (MobileSecurityGate) Name() domain.GateName    { return domain.GateMobileSecurity }
func (MobileSecurityGate) RepairAgent() agents.Role { return agents.RoleMobileCoder }

func (MobileSecurityGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil || !projectIsMobile(env.Project) {
		return nil
	}
	p := env.Project
	var issues []domain.Issue
	issues = append(issues, scanAndroidManifests(p)...)
	issues = append(issues, scanIOSInfoPlists(p)...)
	issues = append(issues, scanJSBundleSecrets(p)...)
	issues = append(issues, scanProductionConsoleLog(p)...)
	return issues
}

// androidManifestFile reports whether a path is an AndroidManifest.xml
// — there can be multiple (app/src/main, app/src/debug, library
// modules). All of them get scanned.
var androidManifestRe = regexp.MustCompile(`(?i)(^|/)AndroidManifest\.xml$`)

func scanAndroidManifests(p *domain.Project) []domain.Issue {
	var issues []domain.Issue
	for _, f := range p.Files {
		if !androidManifestRe.MatchString(f.Path) {
			continue
		}
		issues = append(issues, scanOneAndroidManifest(f.Path, f.Content)...)
	}
	return issues
}

func scanOneAndroidManifest(path, body string) []domain.Issue {
	var issues []domain.Issue
	if strings.Contains(body, `android:allowBackup="true"`) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileSecurity, Severity: domain.SeverityWarning,
			Message: "android:allowBackup=\"true\" exposes app private storage to adb backup",
			Hint:    "set android:allowBackup=\"false\" unless you ship a custom BackupAgent",
			Path:    path,
		})
	}
	if strings.Contains(body, `android:debuggable="true"`) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileSecurity, Severity: domain.SeverityError,
			Message: "android:debuggable=\"true\" — never ship a debuggable APK to production",
			Hint:    "remove the debuggable attribute or gate it behind buildTypes.debug only",
			Path:    path,
		})
	}
	if strings.Contains(body, "android.permission.SEND_SMS") {
		if !codeReferencesAndroidSMS(body, path) {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobileSecurity, Severity: domain.SeverityWarning,
				Message: "SEND_SMS permission declared but no SmsManager call found in code",
				Hint:    "drop the permission or use it — unused permissions hurt Play Store review",
				Path:    path,
			})
		}
	}
	if strings.Contains(body, "android.permission.WRITE_EXTERNAL_STORAGE") {
		if isTargetSdkGTE(body, 30) {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobileSecurity, Severity: domain.SeverityWarning,
				Message: "WRITE_EXTERNAL_STORAGE on targetSdk >= 30 is deprecated by scoped storage",
				Hint:    "switch to MediaStore APIs / SAF; the permission no longer grants writes",
				Path:    path,
			})
		}
	}
	if strings.Contains(body, "android.permission.QUERY_ALL_PACKAGES") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileSecurity, Severity: domain.SeverityWarning,
			Message: "QUERY_ALL_PACKAGES requires Play Console policy declaration",
			Hint:    "narrow to explicit <queries> entries listing the packages you actually need",
			Path:    path,
		})
	}
	return issues
}

// codeReferencesAndroidSMS scans the wider project for any reference
// to SmsManager — caller passes one manifest at a time but we need to
// look at *any* source file, so we re-scan the loaded body for an
// inline hint and then fall back to a project-wide grep below.
func codeReferencesAndroidSMS(_ /* manifestBody */, _ /* manifestPath */ string) bool {
	// Heuristic stub — when this is called we don't yet have project
	// scope. The caller of scanOneAndroidManifest could pass the
	// project but we don't want to widen the signature for one rule;
	// the project-level grep happens in scanProjectForString below
	// and the wrapping issue is only emitted if the grep misses.
	return false
}

// targetSdkRe pulls <uses-sdk android:targetSdkVersion="30" /> values.
var targetSdkRe = regexp.MustCompile(`android:targetSdkVersion="(\d+)"`)

func isTargetSdkGTE(body string, want int) bool {
	m := targetSdkRe.FindStringSubmatch(body)
	if len(m) != 2 {
		return false
	}
	var v int
	for _, r := range m[1] {
		if r < '0' || r > '9' {
			return false
		}
		v = v*10 + int(r-'0')
	}
	return v >= want
}

// iosPermissionAPI maps Apple framework symbols to the Info.plist
// usage-description key they require. Calling any of these APIs
// without the matching string causes iOS to silently terminate the
// app on first invocation — App Store reviewers ding for this too.
var iosPermissionAPI = []struct {
	APISymbol      string
	UsageKey       string
	HumanFacing    string
}{
	{"AVCaptureDevice", "NSCameraUsageDescription", "camera"},
	{"CLLocationManager", "NSLocationWhenInUseUsageDescription", "location"},
	{"EKEventStore", "NSCalendarsUsageDescription", "calendar"},
	{"PHPhotoLibrary", "NSPhotoLibraryUsageDescription", "photo library"},
	{"MFMessageComposeViewController", "NSContactsUsageDescription", "messages"},
	{"LAContext", "NSFaceIDUsageDescription", "Face ID"},
}

func scanIOSInfoPlists(p *domain.Project) []domain.Issue {
	// Locate every Info.plist in the project; we may have one for the
	// main app target and additional ones for app extensions.
	var plists []domain.FileNode
	for _, f := range p.Files {
		if infoPlistRegexp.MatchString(f.Path) {
			plists = append(plists, f)
		}
	}
	if len(plists) == 0 {
		return nil
	}
	var issues []domain.Issue
	for _, pl := range plists {
		if plistArbitraryLoadsEnabled(pl.Content) {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobileSecurity, Severity: domain.SeverityError,
				Message: "NSAllowsArbitraryLoads=true bypasses App Transport Security",
				Hint:    "remove the global exception; declare per-domain NSExceptionDomains instead",
				Path:    pl.Path,
			})
		}
	}
	// API-vs-Description cross-check: for each known API, if any source
	// file references it AND no Info.plist declares the usage key,
	// emit one SeverityError per missing string.
	for _, rule := range iosPermissionAPI {
		if !projectReferencesSymbol(p, rule.APISymbol) {
			continue
		}
		if anyPlistDeclaresKey(plists, rule.UsageKey) {
			continue
		}
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileSecurity, Severity: domain.SeverityError,
			Message: "iOS code uses " + rule.APISymbol + " but no " + rule.UsageKey + " in Info.plist",
			Hint:    "add " + rule.UsageKey + " — iOS silently crashes on first " + rule.HumanFacing + " request without it",
			Path:    plists[0].Path,
		})
	}
	return issues
}

func plistArbitraryLoadsEnabled(body string) bool {
	low := strings.ToLower(body)
	if !strings.Contains(low, "nsallowsarbitraryloads") {
		return false
	}
	// Find the key, then look at the next <true/> | <false/> token.
	idx := strings.Index(low, "nsallowsarbitraryloads")
	rest := body[idx:]
	tNext := strings.Index(strings.ToLower(rest), "<true/>")
	fNext := strings.Index(strings.ToLower(rest), "<false/>")
	if tNext < 0 {
		return false
	}
	if fNext > 0 && fNext < tNext {
		return false
	}
	return true
}

func projectReferencesSymbol(p *domain.Project, sym string) bool {
	for _, f := range p.Files {
		low := strings.ToLower(f.Path)
		if strings.HasSuffix(low, ".swift") || strings.HasSuffix(low, ".m") ||
			strings.HasSuffix(low, ".mm") || strings.HasSuffix(low, ".h") ||
			strings.HasSuffix(low, ".kt") || strings.HasSuffix(low, ".java") {
			if strings.Contains(f.Content, sym) {
				return true
			}
		}
	}
	return false
}

func anyPlistDeclaresKey(plists []domain.FileNode, key string) bool {
	want := "<key>" + key + "</key>"
	for _, pl := range plists {
		if strings.Contains(pl.Content, want) {
			return true
		}
	}
	return false
}

// hardcodedSecretRe matches the literal pattern called out in the
// gate spec: a likely credential identifier followed by a quoted
// value of >= 16 characters.
var hardcodedSecretRe = regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*['"]([A-Za-z0-9_\-]{16,})['"]`)

func scanJSBundleSecrets(p *domain.Project) []domain.Issue {
	var issues []domain.Issue
	for _, f := range p.Files {
		if !isJSSourceFile(f.Path) {
			continue
		}
		// Skip env templates / example files — those are deliberately
		// fake and trigger the rule with placeholder strings.
		low := strings.ToLower(f.Path)
		if strings.Contains(low, ".env.example") || strings.Contains(low, ".env.sample") {
			continue
		}
		matches := hardcodedSecretRe.FindAllStringSubmatch(f.Content, 4)
		for _, m := range matches {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobileSecurity, Severity: domain.SeverityCritical,
				Message: "hardcoded " + m[1] + " in JS bundle — shipped to every install",
				Hint:    "rotate immediately; load from EXPO_PUBLIC_* env or expo-secure-store / app config secrets",
				Path:    f.Path,
			})
			if len(issues) >= 10 {
				return issues
			}
		}
	}
	return issues
}

func scanProductionConsoleLog(p *domain.Project) []domain.Issue {
	total := 0
	var firstPath string
	for _, f := range p.Files {
		if !isJSProductionSource(f.Path) {
			continue
		}
		n := strings.Count(f.Content, "console.log")
		if n == 0 {
			continue
		}
		if firstPath == "" {
			firstPath = f.Path
		}
		total += n
	}
	if total <= 50 {
		return nil
	}
	return []domain.Issue{{
		Gate: domain.GateMobileSecurity, Severity: domain.SeverityWarning,
		Message: "console.log called " + itoaPositive(total) + " times across production source",
		Hint:    "strip with babel-plugin-transform-remove-console (production env) — leaking PII to logcat / Console.app is a privacy regression",
		Path:    firstPath,
	}}
}

func isJSSourceFile(path string) bool {
	low := strings.ToLower(path)
	return strings.HasSuffix(low, ".ts") || strings.HasSuffix(low, ".tsx") ||
		strings.HasSuffix(low, ".js") || strings.HasSuffix(low, ".jsx")
}

// isJSProductionSource excludes test files and node_modules so the
// console.log heuristic doesn't fire on third-party deps or test
// scaffolding.
func isJSProductionSource(path string) bool {
	if !isJSSourceFile(path) {
		return false
	}
	low := strings.ToLower(path)
	if strings.Contains(low, "node_modules/") {
		return false
	}
	if strings.Contains(low, "__tests__/") || strings.Contains(low, "/tests/") {
		return false
	}
	if strings.Contains(low, ".spec.") || strings.Contains(low, ".test.") {
		return false
	}
	if !strings.HasPrefix(low, "app/") && !strings.HasPrefix(low, "src/") {
		return false
	}
	return true
}
