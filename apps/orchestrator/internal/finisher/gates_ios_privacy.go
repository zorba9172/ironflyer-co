package finisher

import (
	"context"
	"regexp"
	"strings"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
)

// IOSPrivacyManifestGate enforces the PrivacyInfo.xcprivacy manifest
// Apple has required for App Store submission since May 2024. Without
// it App Store Connect rejects the upload at validation time — most
// builders don't enforce this until the developer hits the wall.
//
// Only fires when iOS is an effective target.
type IOSPrivacyManifestGate struct{}

func (IOSPrivacyManifestGate) Name() domain.GateName    { return domain.GateIOSPrivacyManifest }
func (IOSPrivacyManifestGate) RepairAgent() agents.Role { return agents.RoleMobileCoder }

func (IOSPrivacyManifestGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil || !projectIsMobile(env.Project) {
		return nil
	}
	m := env.Project.Spec.Stack.Mobile
	if !containsTarget(m.EffectiveTargets(), domain.MobileTargetIOS) {
		return nil
	}
	return checkPrivacyManifest(env.Project)
}

// privacyManifestRe matches "PrivacyInfo.xcprivacy" anywhere under
// ios/ or Resources/.
var privacyManifestRe = regexp.MustCompile(`(?i)(^|/)PrivacyInfo\.xcprivacy$`)

// thirdPartyPrivacyManifestSDKs maps a JS / native dependency
// identifier to the human-facing SDK name. Apple now requires every
// listed SDK to ship its own PrivacyInfo.xcprivacy and the host app
// has to declare matching NSPrivacyAccessedAPITypes / domains.
//
// Source: Apple's "Required reason API" + "Privacy manifests" guides.
// We keep this small and high-confidence — false positives here are
// expensive (loud warnings on a clean app).
var thirdPartyPrivacyManifestSDKs = map[string]string{
	"@sentry/react-native": "Sentry",
	"@sentry/browser":      "Sentry",
	"firebase":             "Firebase",
	"@react-native-firebase/app": "Firebase",
	"@stripe/stripe-react-native": "Stripe",
	"@stripe/stripe-js":           "Stripe",
	"react-native-mmkv":           "MMKV",
	"expo-secure-store":           "Expo SecureStore",
	"@amplitude/analytics-react-native": "Amplitude",
	"posthog-react-native":              "PostHog",
}

func checkPrivacyManifest(p *domain.Project) []domain.Issue {
	manifestPath, manifestBody, ok := fileBodyAny(p, func(path string) bool {
		return privacyManifestRe.MatchString(path)
	})
	if !ok {
		return []domain.Issue{{
			Gate: domain.GateIOSPrivacyManifest, Severity: domain.SeverityError,
			Message: "missing PrivacyInfo.xcprivacy — App Store rejects iOS builds without it since May 2024",
			Hint:    "add ios/<Target>/PrivacyInfo.xcprivacy with NSPrivacyTracking, NSPrivacyTrackingDomains, and NSPrivacyAccessedAPITypes",
		}}
	}
	var issues []domain.Issue

	// Binary plists are valid for App Store but our static gate can't
	// parse them; emit a warning so the operator knows the gate is
	// running blind on the file.
	if isBinaryPlist(manifestBody) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateIOSPrivacyManifest, Severity: domain.SeverityWarning,
			Message: "PrivacyInfo.xcprivacy is a binary plist — gate cannot validate keys statically",
			Hint:    "convert to XML plist for review-time verification (`plutil -convert xml1 PrivacyInfo.xcprivacy`)",
			Path:    manifestPath,
		})
		return issues
	}

	required := []string{
		"NSPrivacyTracking",
		"NSPrivacyTrackingDomains",
		"NSPrivacyAccessedAPITypes",
	}
	for _, key := range required {
		if !strings.Contains(manifestBody, "<key>"+key+"</key>") {
			issues = append(issues, domain.Issue{
				Gate: domain.GateIOSPrivacyManifest, Severity: domain.SeverityError,
				Message: "PrivacyInfo.xcprivacy missing required key " + key,
				Hint:    "Apple Privacy Manifest reference: developer.apple.com/documentation/bundleresources/privacy_manifest_files",
				Path:    manifestPath,
			})
		}
	}

	// NSPrivacyAccessedAPITypes must be an array with at least one
	// dictionary carrying an NSPrivacyAccessedAPIType from the Apple
	// enum. Missing entry while the app uses certain APIs is a hard
	// rejection at submit time.
	if !strings.Contains(manifestBody, "NSPrivacyAccessedAPIType") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateIOSPrivacyManifest, Severity: domain.SeverityError,
			Message: "NSPrivacyAccessedAPITypes is present but empty",
			Hint:    "declare each required-reason API you call (UserDefaults, FileTimestamp, SystemBootTime, DiskSpace, ActiveKeyboards)",
			Path:    manifestPath,
		})
	}

	// Third-party SDK cross-check.
	for dep, sdkName := range thirdPartyPrivacyManifestSDKs {
		if !projectDeclaresJSDep(p, dep) {
			continue
		}
		// If the host privacy manifest doesn't list any accessed-API
		// types, we can't tell whether the SDK is covered; emit a
		// SeverityWarning.
		if !strings.Contains(manifestBody, "<key>NSPrivacyAccessedAPITypes</key>") {
			issues = append(issues, domain.Issue{
				Gate: domain.GateIOSPrivacyManifest, Severity: domain.SeverityWarning,
				Message: sdkName + " requires NSPrivacyAccessedAPITypes coverage",
				Hint:    "consult the SDK's privacy manifest doc and mirror its required-reason API list into PrivacyInfo.xcprivacy",
				Path:    manifestPath,
			})
		}
	}
	return issues
}

// isBinaryPlist returns true when the file body looks like a binary
// plist (Apple's bplist00..bplist17 magic). XML plists start with "<?xml".
func isBinaryPlist(body string) bool {
	if len(body) < 8 {
		return false
	}
	return strings.HasPrefix(body, "bplist")
}

// projectDeclaresJSDep does a fast string scan over package.json for
// the dependency identifier. Returns false on any project missing
// package.json (Flutter / native).
func projectDeclaresJSDep(p *domain.Project, dep string) bool {
	body, ok := fileBody(p, "package.json")
	if !ok {
		return false
	}
	return strings.Contains(body, `"`+dep+`"`)
}
