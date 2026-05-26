package finisher

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
)

// PushCredentialsGate validates that a mobile project which actually uses
// push notifications has the credentials needed to deliver them. Apple
// requires an APNs .p8 key (or a .p12 cert) keyed by Key ID + Team ID +
// Bundle ID; Android+Firebase needs a service-account JSON plus a
// google-services.json whose package_name matches the build.gradle
// applicationId. Most "vibe-coded" mobile apps ship without these and
// then quietly fail to push in production — this gate refuses to let
// that happen.
type PushCredentialsGate struct{}

func (PushCredentialsGate) Name() domain.GateName    { return domain.GateMobilePushCredentials }
func (PushCredentialsGate) RepairAgent() agents.Role { return agents.RoleMobileDeployer }

// pushUsageMarkers is the list of identifiers we scan the project for to
// decide whether the gate is active. ANY hit activates the gate; ZERO
// hits keep it dark (the project doesn't ship push, so there are no
// credentials to check).
var pushUsageMarkers = []string{
	"expo-notifications",
	"@react-native-firebase/messaging",
	"react-native-push-notification",
	"UNUserNotificationCenter",
	"FirebaseMessaging",
	"firebase_messaging",
}

// p8HeaderRe matches the canonical PEM PKCS#8 header that an APNs .p8
// key starts with. We never decode the key value — that would risk
// leaking it through error messages or telemetry — we just sanity-check
// the format.
var p8HeaderRe = regexp.MustCompile(`(?m)^-----BEGIN PRIVATE KEY-----`)

// notificationChannelRe / requestPermissionsRe / setChannelRe are static
// signal scans for Android 8+ channel registration and iOS permission
// requests. False negatives are tolerable (we emit a warning, not an
// error) — the goal is to catch the common "forgot to call it" mistakes.
var (
	requestPermissionsRe = regexp.MustCompile(`Notifications\.requestPermissionsAsync\s*\(`)
	setChannelRe         = regexp.MustCompile(`Notifications\.setNotificationChannelAsync\s*\(`)
	notificationChannelRe = regexp.MustCompile(`new\s+NotificationChannel\s*\(`)
)

func (PushCredentialsGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return nil
	}
	p := env.Project
	if !p.Spec.Stack.IsMobile() {
		return nil
	}
	m := p.Spec.Stack.Mobile

	usage := detectPushUsage(p)
	if !usage.any() {
		// Project doesn't import any push library — gate stays dark.
		return nil
	}

	targets := m.EffectiveTargets()
	wantIOS := containsTarget(targets, domain.MobileTargetIOS)
	wantAndroid := containsTarget(targets, domain.MobileTargetAndroid)

	var issues []domain.Issue
	if wantIOS {
		issues = append(issues, checkAPNCredentials(p, usage)...)
	}
	if wantAndroid {
		issues = append(issues, checkFCMCredentials(p, usage)...)
	}
	// Cross-platform notification UX checks (permission ask, channel
	// declaration). These don't depend on which target is selected — the
	// code-level mistake exists either way.
	issues = append(issues, checkNotificationUX(p, usage)...)
	return issues
}

// pushUsage records which import markers were detected and which file
// each one was sighted in. The bools steer the per-platform credential
// checks (e.g. only require google-services.json when @react-native-firebase
// is in use; Expo can fetch it via EAS).
type pushUsage struct {
	expoNotifications      bool
	rnFirebaseMessaging    bool
	rnPushNotification     bool
	iosNative              bool
	androidNativeFirebase  bool
	flutterFirebase        bool
}

func (u pushUsage) any() bool {
	return u.expoNotifications || u.rnFirebaseMessaging || u.rnPushNotification ||
		u.iosNative || u.androidNativeFirebase || u.flutterFirebase
}

// detectPushUsage runs a pure file-and-artifact scan across the project.
// No runtime needed — the rule explicitly says this gate must work
// without a workspace bound.
func detectPushUsage(p *domain.Project) pushUsage {
	u := pushUsage{}
	for _, f := range p.Files {
		c := f.Content
		if c == "" {
			continue
		}
		// Cheap substring tests — every marker is a unique enough string
		// that fast paths beat compiling six regexes.
		if strings.Contains(c, "expo-notifications") {
			u.expoNotifications = true
		}
		if strings.Contains(c, "@react-native-firebase/messaging") {
			u.rnFirebaseMessaging = true
		}
		if strings.Contains(c, "react-native-push-notification") {
			u.rnPushNotification = true
		}
		if strings.Contains(c, "UNUserNotificationCenter") {
			u.iosNative = true
		}
		if strings.Contains(c, "FirebaseMessaging") {
			u.androidNativeFirebase = true
		}
		if strings.Contains(c, "firebase_messaging") {
			u.flutterFirebase = true
		}
	}
	return u
}

// checkAPNCredentials validates iOS APNs setup: a .p8 (or full key-id +
// team-id + bundle-id triple) in Project.Secrets, plus the
// aps-environment entitlement reference in *.entitlements.
func checkAPNCredentials(p *domain.Project, _ pushUsage) []domain.Issue {
	var issues []domain.Issue

	hasP8 := false
	if v, ok := p.Secrets["APN_KEY_P8"]; ok && strings.TrimSpace(v) != "" {
		hasP8 = true
		// Format sanity-check only — never decode or echo the value.
		if !p8HeaderRe.MatchString(v) {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobilePushCredentials, Severity: domain.SeverityError,
				Message: "APN_KEY_P8 does not start with '-----BEGIN PRIVATE KEY-----'",
				Hint:    "upload the raw .p8 contents from App Store Connect, not the base64-wrapped form",
			})
		}
	}
	hasKeyTriple := strings.TrimSpace(p.Secrets["APN_KEY_ID"]) != "" &&
		strings.TrimSpace(p.Secrets["APN_TEAM_ID"]) != "" &&
		strings.TrimSpace(p.Secrets["APN_BUNDLE_ID"]) != ""
	if !hasP8 && !hasKeyTriple {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobilePushCredentials, Severity: domain.SeverityError,
			Message: "APN credentials missing: provide APN_KEY_P8 OR (APN_KEY_ID + APN_TEAM_ID + APN_BUNDLE_ID) in Project.Secrets",
			Hint:    "App Store Connect → Keys → APNs Authentication Key generates a .p8 plus the Key ID and Team ID",
		})
	}

	// aps-environment entitlement reference must live in the *.entitlements
	// file. Info.plist alone is insufficient — Apple checks the
	// entitlement, not the info dictionary.
	hasEntitlement := false
	for _, f := range p.Files {
		if !strings.HasSuffix(strings.ToLower(f.Path), ".entitlements") {
			continue
		}
		if strings.Contains(f.Content, "aps-environment") {
			hasEntitlement = true
			break
		}
	}
	if !hasEntitlement {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobilePushCredentials, Severity: domain.SeverityError,
			Message: "iOS push enabled but no *.entitlements file declares aps-environment",
			Hint:    "add `<key>aps-environment</key><string>development</string>` (or `production`) to the project's .entitlements",
		})
	}

	return issues
}

// checkFCMCredentials validates Android FCM setup: a service-account JSON
// secret, google-services.json on disk (required for bare RN-Firebase;
// SeverityWarning otherwise), and an applicationId in build.gradle that
// matches the package_name inside google-services.json.
func checkFCMCredentials(p *domain.Project, u pushUsage) []domain.Issue {
	var issues []domain.Issue

	svcRaw, hasSvc := p.Secrets["FCM_SERVICE_ACCOUNT_KEY"]
	svcRaw = strings.TrimSpace(svcRaw)
	if !hasSvc || svcRaw == "" {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobilePushCredentials, Severity: domain.SeverityError,
			Message: "FCM_SERVICE_ACCOUNT_KEY missing from Project.Secrets",
			Hint:    "Firebase Console → Project Settings → Service Accounts → Generate new private key → paste the JSON",
		})
	}

	gsBody, hasGS := fileBody(p, "android/app/google-services.json")
	if !hasGS {
		// react-native-firebase needs the file on disk; Expo can fetch
		// it dynamically via EAS so we degrade to a warning there.
		sev := domain.SeverityWarning
		hint := "Expo can fetch google-services.json from EAS at build time, but committing it makes the build reproducible"
		if u.rnFirebaseMessaging || u.androidNativeFirebase {
			sev = domain.SeverityError
			hint = "download google-services.json from the Firebase Console and commit it to android/app/"
		}
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobilePushCredentials, Severity: sev,
			Message: "android/app/google-services.json is missing",
			Hint:    hint,
		})
	}

	// applicationId vs google-services.json package_name consistency.
	if hasGS {
		pkgName := scanGoogleServicesPackageName(gsBody)
		appID := firstAndroidApplicationID(p)
		if pkgName != "" && appID != "" && pkgName != appID {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobilePushCredentials, Severity: domain.SeverityError,
				Message: "android/app/google-services.json package_name '" + pkgName + "' does not match build.gradle applicationId '" + appID + "'",
				Hint:    "regenerate google-services.json for the correct Firebase app, or align the applicationId",
				Path:    "android/app/google-services.json",
			})
		}
	}

	// Project ID from the service account JSON exists as a hint for the
	// repair agent — we don't enforce a cross-config check here because
	// the project_id field is informational on the client side.
	if hasSvc && svcRaw != "" {
		var svcDoc struct {
			ProjectID string `json:"project_id"`
		}
		_ = json.Unmarshal([]byte(svcRaw), &svcDoc)
		if strings.TrimSpace(svcDoc.ProjectID) == "" {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobilePushCredentials, Severity: domain.SeverityWarning,
				Message: "FCM_SERVICE_ACCOUNT_KEY parses but has no 'project_id' field",
				Hint:    "make sure the secret holds the raw JSON downloaded from Firebase, not a wrapper or env-var form",
			})
		}
	}

	return issues
}

// checkNotificationUX scans for the two foot-guns that silently drop
// notifications even when credentials are correct: missing iOS permission
// request (expo-notifications) and missing Android NotificationChannel
// (Android 8+).
func checkNotificationUX(p *domain.Project, u pushUsage) []domain.Issue {
	var issues []domain.Issue
	if u.expoNotifications {
		askFound := false
		for _, f := range p.Files {
			if !looksLikeJSSource(f.Path) {
				continue
			}
			if requestPermissionsRe.MatchString(f.Content) {
				askFound = true
				break
			}
		}
		if !askFound {
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobilePushCredentials, Severity: domain.SeverityWarning,
				Message: "expo-notifications is used but Notifications.requestPermissionsAsync is never called",
				Hint:    "iOS silently refuses to deliver push until the user has granted permission via this API",
			})
		}
	}

	// Android 8+ channel declaration.
	channelFound := false
	for _, f := range p.Files {
		if u.expoNotifications {
			if looksLikeJSSource(f.Path) && setChannelRe.MatchString(f.Content) {
				channelFound = true
				break
			}
		}
		if u.rnFirebaseMessaging || u.androidNativeFirebase {
			if looksLikeAndroidSource(f.Path) && notificationChannelRe.MatchString(f.Content) {
				channelFound = true
				break
			}
		}
	}
	if !channelFound && (u.expoNotifications || u.rnFirebaseMessaging || u.androidNativeFirebase) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobilePushCredentials, Severity: domain.SeverityWarning,
			Message: "Android 8+ silently drops notifications without a registered NotificationChannel",
			Hint:    "Expo: call Notifications.setNotificationChannelAsync; bare: instantiate `new NotificationChannel(...)` in Kotlin/Java",
		})
	}

	return issues
}

// scanGoogleServicesPackageName extracts client.client_info.android_client_info.package_name
// from a google-services.json body without depending on a full JSON parse
// (the file may contain extra fields we don't model).
func scanGoogleServicesPackageName(body string) string {
	var doc struct {
		Client []struct {
			ClientInfo struct {
				AndroidClientInfo struct {
					PackageName string `json:"package_name"`
				} `json:"android_client_info"`
			} `json:"client_info"`
		} `json:"client"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return ""
	}
	for _, c := range doc.Client {
		if pn := strings.TrimSpace(c.ClientInfo.AndroidClientInfo.PackageName); pn != "" {
			return pn
		}
	}
	return ""
}

// firstAndroidApplicationID resolves the applicationId for the project,
// preferring app/build.gradle(.kts) and falling back to
// android/app/build.gradle(.kts) when the project follows the React
// Native layout.
func firstAndroidApplicationID(p *domain.Project) string {
	for _, path := range []string{
		"app/build.gradle", "app/build.gradle.kts",
		"android/app/build.gradle", "android/app/build.gradle.kts",
	} {
		if body, ok := fileBody(p, path); ok {
			if id := scanGradleApplicationID(body); id != "" {
				return id
			}
		}
	}
	return ""
}

// looksLikeJSSource is a fast extension check for sources we want to scan
// for JS/TS API calls. Lower-cased compare keeps it cheap.
func looksLikeJSSource(path string) bool {
	lp := strings.ToLower(path)
	return strings.HasSuffix(lp, ".js") || strings.HasSuffix(lp, ".jsx") ||
		strings.HasSuffix(lp, ".ts") || strings.HasSuffix(lp, ".tsx") ||
		strings.HasSuffix(lp, ".mjs") || strings.HasSuffix(lp, ".cjs")
}

// looksLikeAndroidSource checks Kotlin / Java extensions for the bare
// channel-registration scan.
func looksLikeAndroidSource(path string) bool {
	lp := strings.ToLower(path)
	return strings.HasSuffix(lp, ".kt") || strings.HasSuffix(lp, ".java")
}
