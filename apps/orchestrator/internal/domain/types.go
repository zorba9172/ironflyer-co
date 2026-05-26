// Package domain holds the core Ironflyer types shared across packages.
package domain

import (
	"encoding/json"
	"strings"
	"time"
)

type GateName string

const (
	GateSpec        GateName = "spec"
	GateUX          GateName = "ux"
	GateArch        GateName = "arch"
	GateCode        GateName = "code"
	GateLint        GateName = "lint"
	GateTest        GateName = "test"
	GateSecurity    GateName = "security"
	GateBudget      GateName = "budget"
	GateDeploy      GateName = "deploy"
	// GateMobileBuild runs when the project's StackDecision.Mobile.Kind is
	// non-empty. It validates the mobile manifest (Expo app.json, Android
	// build.gradle, iOS Info.plist), signing setup, bundle identifier
	// uniqueness, target-platform alignment, and — when a runtime is
	// available — drives a real `eas build --local` / `gradle assembleDebug`
	// / `xcodebuild build` to confirm the project actually compiles for the
	// declared target. ProfitGuard meters this gate; large native builds are
	// expensive and must respect the wallet contract before they spin up.
	GateMobileBuild GateName = "mobile_build"
	// GateMobileExpoDoctor runs `expo-doctor` (or its static fallback)
	// for Expo / React Native bare projects so issues like
	// mismatched-native-deps or missing Metro config land in the
	// gate verdict before the build gate even tries to compile.
	GateMobileExpoDoctor GateName = "mobile_expo_doctor"
	// GateMobileSize enforces APK/AAB/IPA size budgets against the
	// last mobile build artifact. Oversized binaries hurt install
	// conversion (Android) or get rejected outright (iOS App Store).
	GateMobileSize GateName = "mobile_size"
	// GateMobileSecurity layers mobile-specific OWASP-MASVS-style
	// checks (Android manifest hygiene, iOS Info.plist permission
	// strings, hardcoded API keys in the JS bundle) on top of the
	// generic SecurityGate.
	GateMobileSecurity GateName = "mobile_security"
	// GateIOSPrivacyManifest enforces the PrivacyInfo.xcprivacy
	// manifest Apple requires for App Store submission since May
	// 2024. Without it App Store Connect rejects the upload.
	GateIOSPrivacyManifest GateName = "ios_privacy_manifest"
	// GateMobilePushCredentials fires when the project uses push
	// notifications (expo-notifications, @react-native-firebase/messaging,
	// UNUserNotificationCenter, FirebaseMessaging, or firebase_messaging
	// from Flutter) and validates that the credentials needed to ship
	// push to a real device exist: APN .p8 + key ID + team ID for iOS;
	// FCM service account + google-services.json + applicationId
	// consistency for Android. Without this gate, push silently fails
	// in production — competitors don't enforce it.
	GateMobilePushCredentials GateName = "mobile_push_credentials"
	// GateLighthouse audits a deployed web project against Google's
	// PageSpeed Insights API (Lighthouse on Google infra) for
	// Performance, Accessibility, SEO, and Best Practices. Runs after
	// Deploy on projects with a public preview URL; stays dark on
	// mobile-only or backend-only stacks. Performance and Accessibility
	// failures are blocking (Coder repair); SEO and Best Practices warn.
	// This gate is the brand-quality enforcer that separates Ironflyer's
	// "production discipline" pitch from competitors who ship without
	// measured live-runtime quality.
	GateLighthouse GateName = "lighthouse"
	// GateMobileBundleAnalyzer reads the JS bundle produced by the
	// MobileBuildGate (or its source-map output) and flags packages
	// over a per-package size budget. Common offenders (moment.js,
	// full lodash, full icon libraries) trigger an opinionated Hint
	// pointing at a lighter alternative so the repair Coder doesn't
	// have to re-derive the well-known fixes. Pure analysis — never
	// fails the build when react-native-bundle-visualizer isn't
	// installed; emits a SeverityInfo nudge instead.
	GateMobileBundleAnalyzer GateName = "mobile_bundle_analyzer"
)

func AllGates() []GateName {
	return []GateName{
		GateSpec, GateUX, GateArch,
		GateCode, GateLint, GateTest,
		GateSecurity, GateBudget,
		GateMobileBuild,
		GateMobileExpoDoctor, GateMobileSize,
		GateMobileSecurity, GateIOSPrivacyManifest,
		GateMobilePushCredentials,
		GateMobileBundleAnalyzer,
		GateDeploy,
		GateLighthouse,
	}
}

type GateStatus string

const (
	GateStatusPending  GateStatus = "pending"
	GateStatusRunning  GateStatus = "running"
	GateStatusPassed   GateStatus = "passed"
	GateStatusFailed   GateStatus = "failed"
	GateStatusBlocked  GateStatus = "blocked"
	GateStatusRepaired GateStatus = "repaired"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

type Issue struct {
	Gate     GateName `json:"gate"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Hint     string   `json:"hint,omitempty"`
	Path     string   `json:"path,omitempty"`
}

type GateState struct {
	Name     GateName   `json:"name"`
	Status   GateStatus `json:"status"`
	Issues   []Issue    `json:"issues,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Project struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	// OwnerID is the user that owns this project. Empty means "public" —
	// every authenticated user can read it (used for the seed demo project).
	OwnerID string                 `json:"ownerId,omitempty"`
	// Federated is true when the owner has opted this project into their
	// own personal memory-federation pool. Federation NEVER crosses users
	// — only the same OwnerID's other federated projects can read this
	// project's memory. See internal/memory + the /me/memory-federation
	// endpoints.
	Federated bool `json:"federated,omitempty"`
	Spec    ProductSpec            `json:"spec"`
	Files   []FileNode             `json:"files"`
	// Artifacts holds typed, structured documents produced by the finisher
	// pipeline (plan, stack, screen_map, design_tokens, …). Stored as raw
	// JSON so callers can evolve the inner shape without a schema lock.
	// Prefer GetArtifact / SetArtifact over direct map access so nil-safe
	// behaviour is preserved.
	Artifacts map[string]json.RawMessage `json:"artifacts,omitempty"`
	Gates   map[GateName]GateState `json:"gates"`
	Events  []Event                `json:"events"`
	GitHub  *GitHubLink            `json:"github,omitempty"`
	// Secrets holds provisioned per-project credentials — DATABASE_URL,
	// Stripe keys, Supabase service-role tokens, etc. Never serialised to
	// JSON so it cannot leak through API responses; callers that need a
	// value must read it through the store and inject it explicitly (the
	// runtime sandbox is the typical sink).
	Secrets map[string]string `json:"-"`
	// VisualTargets is the pixel-perfect contract: the user uploads one
	// or more reference screenshots (Figma export, Lovable iteration,
	// hand-drawn mockup) and the UXGate refuses to pass until the live
	// preview matches within tolerance. Empty slice = no visual contract
	// (project ships on the regular gate set).
	VisualTargets []VisualTarget `json:"visualTargets,omitempty"`
	// Subprojects models multi-service / monorepo layouts. When empty,
	// the project is single-service (the existing default). When non-
	// empty, each Subproject represents a deployable unit; the finisher
	// can apply scaffolders + gates per-subproject in a later iteration.
	Subprojects []Subproject `json:"subprojects,omitempty"`
	CreatedAt time.Time            `json:"createdAt"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

// IsAccessibleBy returns true when userID owns the project or it is public.
func (p Project) IsAccessibleBy(userID string) bool {
	return p.OwnerID == "" || p.OwnerID == userID
}

// Subproject is one service within a monorepo Project. The parent
// Project owns the overall spec + gates; each Subproject owns its
// own slice of files + its own scaffolder choice + its own
// language stack. Inspired by Nx / Turbo / pnpm workspaces.
type Subproject struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Path      string        `json:"path"`           // e.g. "apps/api", "services/worker"
	Stack     StackDecision `json:"stack"`          // frontend/backend/storage/auth declared per service
	Role      string        `json:"role,omitempty"` // "frontend" | "backend" | "worker" | "mobile" | "ml" | ...
	CreatedAt time.Time     `json:"createdAt"`
}

// SubprojectByPath returns the Subproject whose Path matches the
// file's directory prefix (longest match wins). Returns nil when
// no subproject claims the file — that's the implicit "root"
// subproject.
func (p Project) SubprojectByPath(filePath string) *Subproject {
	clean := strings.TrimPrefix(filePath, "/")
	var best *Subproject
	bestLen := -1
	for i := range p.Subprojects {
		sp := &p.Subprojects[i]
		base := strings.Trim(sp.Path, "/")
		if base == "" {
			continue
		}
		if clean == base || strings.HasPrefix(clean, base+"/") {
			if len(base) > bestLen {
				best = sp
				bestLen = len(base)
			}
		}
	}
	return best
}

type ProductSpec struct {
	Idea         string       `json:"idea"`
	UserStories  []UserStory  `json:"userStories"`
	DataModel    []EntityDef  `json:"dataModel"`
	Stack        StackDecision `json:"stack"`
}

type UserStory struct {
	ID          string   `json:"id"`
	As          string   `json:"as"`
	IWant       string   `json:"iWant"`
	SoThat      string   `json:"soThat"`
	Acceptance  []string `json:"acceptance"`
}

type EntityDef struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

type StackDecision struct {
	Frontend string `json:"frontend"`
	Backend  string `json:"backend"`
	Storage  string `json:"storage"`
	Auth     string `json:"auth"`
	// Mobile is OPTIONAL. When zero-valued (Kind == ""), the project is
	// web-only and the MobileBuildGate stays dark. When set, every gate
	// that knows about mobile (Arch, MobileBuild, Deploy) applies the
	// mobile contract to the relevant subproject(s).
	Mobile MobileStack `json:"mobile,omitempty"`
}

// MobileKind enumerates the mobile stacks Ironflyer supports natively.
// Adding a new value is a coordinated change: domain → finisher gates →
// runtime mobile driver → starter template → web stack picker.
type MobileKind string

const (
	// MobileKindNone is the zero value — project is web-only, no mobile
	// gate runs. Equivalent to an empty MobileStack.
	MobileKindNone MobileKind = ""
	// MobileKindExpo is the recommended path: Expo Router + EAS Build,
	// no Mac required for Android, EAS handles iOS signing in the cloud.
	// Validated against template "react-native-expo".
	MobileKindExpo MobileKind = "expo"
	// MobileKindReactNativeBare is the ejected/bare React Native flow:
	// the project owns its own ios/ and android/ directories. Native
	// builds run via Gradle (Android) and xcodebuild (iOS Pro tier).
	MobileKindReactNativeBare MobileKind = "react-native-bare"
	// MobileKindAndroidNative is pure Kotlin + Jetpack Compose, built
	// in a Linux container with the Android SDK + Gradle. No iOS path.
	MobileKindAndroidNative MobileKind = "android-native"
	// MobileKindIOSNative is pure Swift + SwiftUI, built on an actual
	// Mac host (Scaleway / MacStadium / AWS mac instances) via the
	// iOS Pro tier. Refuses to run without a wired mac pool.
	MobileKindIOSNative MobileKind = "ios-native"
	// MobileKindFlutter is Flutter + Dart, Android builds in Linux,
	// iOS builds require the same Mac path as MobileKindIOSNative.
	MobileKindFlutter MobileKind = "flutter"
)

// MobileTarget is one platform a mobile build must produce a binary for.
type MobileTarget string

const (
	MobileTargetAndroid MobileTarget = "android"
	MobileTargetIOS     MobileTarget = "ios"
)

// MobileStack declares everything the finisher + runtime need to drive a
// mobile build without re-deriving config from raw files every gate
// iteration. It is persisted as part of ProductSpec.Stack so a project
// resumes cleanly across orchestrator restarts.
type MobileStack struct {
	// Kind selects the build pipeline. Empty = web-only project.
	Kind MobileKind `json:"kind,omitempty"`
	// Targets enumerates the platforms the project commits to producing
	// builds for. Subset of {android, ios}. Empty defaults to both when
	// Kind is set, except MobileKindAndroidNative (android only) and
	// MobileKindIOSNative (ios only).
	Targets []MobileTarget `json:"targets,omitempty"`
	// AppID is the reverse-DNS bundle identifier (e.g.
	// "com.acme.flightplan"). Validated against AppIDPattern. Mandatory
	// when Kind != MobileKindNone.
	AppID string `json:"appId,omitempty"`
	// DisplayName is the human-readable app name shown on the home
	// screen (e.g. "Flight Plan"). Defaults to Project.Name when empty.
	DisplayName string `json:"displayName,omitempty"`
	// Version is the user-facing semver (e.g. "1.0.0"). Defaults to
	// "0.1.0" on first build.
	Version string `json:"version,omitempty"`
	// MinAndroidSDK is the android `minSdkVersion`. 0 → 24 (Android 7,
	// the same floor Expo SDK 53 enforces).
	MinAndroidSDK int `json:"minAndroidSdk,omitempty"`
	// MinIOSVersion is the iOS deployment target. Empty → "15.1" (the
	// same floor React Native 0.74+ ships with).
	MinIOSVersion string `json:"minIosVersion,omitempty"`
	// EAS holds Expo-specific configuration: project ID, build profiles
	// (development / preview / production), submit credentials hint.
	// Only meaningful when Kind == MobileKindExpo or MobileKindReactNativeBare.
	EAS *EASConfig `json:"eas,omitempty"`
	// Signing holds the keystore + provisioning-profile metadata. The
	// finisher gate validates this exists; actual private-key material
	// is stored in Project.Secrets (never serialised to JSON) and
	// injected into the build sandbox at exec time.
	Signing *MobileSigning `json:"signing,omitempty"`
}

// EASConfig is the Expo Application Services profile selection.
type EASConfig struct {
	// ProjectID is the Expo Project ID (UUID) issued by `eas init`.
	ProjectID string `json:"projectId,omitempty"`
	// Profile selects the EAS build profile (development | preview |
	// production). Defaults to "preview" — internal-distribution builds
	// without store submission.
	Profile string `json:"profile,omitempty"`
	// Channel is the EAS Update channel (e.g. "main", "staging"). Empty
	// disables OTA updates.
	Channel string `json:"channel,omitempty"`
	// Owner is the Expo account/organisation slug. Empty = the EAS
	// token's default owner.
	Owner string `json:"owner,omitempty"`
}

// MobileSigning is the build-time signing configuration. The gate verifies
// the secret keys named here EXIST in Project.Secrets — actual values are
// never serialised through the API surface.
type MobileSigning struct {
	// AndroidKeystoreSecret is the Project.Secrets key that holds the
	// base64-encoded keystore file. Empty disables Android release
	// builds (debug builds still work).
	AndroidKeystoreSecret string `json:"androidKeystoreSecret,omitempty"`
	// AndroidKeyAlias is the key alias inside the keystore.
	AndroidKeyAlias string `json:"androidKeyAlias,omitempty"`
	// AndroidStorePasswordSecret is the Project.Secrets key holding the
	// store password.
	AndroidStorePasswordSecret string `json:"androidStorePasswordSecret,omitempty"`
	// AndroidKeyPasswordSecret is the Project.Secrets key holding the
	// per-key password (often the same as the store password).
	AndroidKeyPasswordSecret string `json:"androidKeyPasswordSecret,omitempty"`
	// IOSProvisioningProfileSecret is the Project.Secrets key holding
	// the base64-encoded .mobileprovision file. Empty disables iOS
	// release builds.
	IOSProvisioningProfileSecret string `json:"iosProvisioningProfileSecret,omitempty"`
	// IOSCertificateP12Secret is the Project.Secrets key holding the
	// base64-encoded .p12 distribution certificate.
	IOSCertificateP12Secret string `json:"iosCertificateP12Secret,omitempty"`
	// IOSCertificatePasswordSecret is the Project.Secrets key holding
	// the .p12 password.
	IOSCertificatePasswordSecret string `json:"iosCertificatePasswordSecret,omitempty"`
	// IOSTeamID is the Apple Developer team identifier (10-char
	// alphanumeric). Stored in the clear — it is not a secret on its
	// own.
	IOSTeamID string `json:"iosTeamId,omitempty"`
}

// AppIDPattern is the reverse-DNS validator for MobileStack.AppID.
// Two-or-more dot-separated segments, each starting with a letter,
// containing only letters, digits, and underscores. This matches the
// intersection of the Android `applicationId` rule and the iOS bundle
// identifier rule — anything that passes here is legal on both stores.
const AppIDPattern = `^[a-zA-Z][a-zA-Z0-9_]*(?:\.[a-zA-Z][a-zA-Z0-9_]*)+$`

// IsMobile reports whether this StackDecision opts into mobile builds.
// Equivalent to `s.Mobile.Kind != MobileKindNone` but reads cleaner at
// call sites that don't want to import the constant.
func (s StackDecision) IsMobile() bool {
	return s.Mobile.Kind != MobileKindNone
}

// EffectiveTargets returns the platforms the gate should enforce a
// passing build on, applying Kind-specific defaults when Targets is
// empty. Returns nil for web-only stacks.
func (m MobileStack) EffectiveTargets() []MobileTarget {
	if m.Kind == MobileKindNone {
		return nil
	}
	if len(m.Targets) > 0 {
		return m.Targets
	}
	switch m.Kind {
	case MobileKindAndroidNative:
		return []MobileTarget{MobileTargetAndroid}
	case MobileKindIOSNative:
		return []MobileTarget{MobileTargetIOS}
	default:
		return []MobileTarget{MobileTargetAndroid, MobileTargetIOS}
	}
}

// NeedsMacHost reports whether the project's iOS target forces the
// mobile build gate onto the Pro tier (Mac pool). Used by ProfitGuard
// to attach the macOS hourly cost to the reservation BEFORE the gate
// actually allocates a workspace.
func (m MobileStack) NeedsMacHost() bool {
	for _, t := range m.EffectiveTargets() {
		if t == MobileTargetIOS {
			// Expo + EAS can build iOS in the EAS cloud — no Mac in our
			// pool needed. Every other kind that touches iOS does need
			// a Mac.
			if m.Kind == MobileKindExpo {
				return false
			}
			return true
		}
	}
	return false
}

// VisualTarget is one reference screenshot the user wants the live
// preview to match. The UXGate fetches a screenshot of the running app
// at RouteHint + viewport, diffs it against ImagePNGBase64, and refuses
// to pass when the difference exceeds Tolerance.
type VisualTarget struct {
	ID              string `json:"id"`
	Name            string `json:"name,omitempty"`
	// RouteHint is the path the runtime should screenshot (e.g. "/",
	// "/pricing", "/app/dashboard"). Empty = "/".
	RouteHint       string `json:"routeHint,omitempty"`
	ViewportW       int    `json:"viewportW"`        // e.g. 1280
	ViewportH       int    `json:"viewportH"`        // e.g. 800
	// ImagePNGBase64 is the target screenshot, base64-encoded PNG bytes
	// (no data: prefix). The orchestrator decodes lazily — keep it small
	// (<= 2 MiB after encoding).
	ImagePNGBase64  string `json:"imagePngBase64"`
	// Tolerance is the fraction of pixels (0..1) that may differ before
	// the gate fails. Default 0.02 = 2% — generous enough that anti-
	// aliasing flicker won't fire false positives.
	Tolerance       float64 `json:"tolerance,omitempty"`
}

// GitHubLink binds a project to a remote GitHub repo so the coder/deploy
// gates can clone, push, and open PRs against it.
type GitHubLink struct {
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	FullName      string `json:"fullName"`
	DefaultBranch string `json:"defaultBranch"`
	HTMLURL       string `json:"htmlUrl"`
}

type FileNode struct {
	Path    string `json:"path"`
	Type    string `json:"type"`
	Size    int    `json:"size,omitempty"`
	Content string `json:"content,omitempty"`
}

type Event struct {
	ID        string    `json:"id"`
	Step      string    `json:"step"`
	Agent     string    `json:"agent,omitempty"`
	Gate      GateName  `json:"gate,omitempty"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// AcceptanceCriterion is the structured form of a single user-story
// acceptance line. The Spec gate extracts these from UserStory.Acceptance
// and walks the file tree to mark each one as `Validated` when SOMETHING in
// the workspace appears to address it (route, handler, component, …).
// `StoryID` ties the criterion back to its UserStory.ID.
type AcceptanceCriterion struct {
	ID          string `json:"id"`
	StoryID     string `json:"storyId,omitempty"`
	Description string `json:"description"`
	// Validated is set true when a Spec-gate sweep has matched at least
	// one piece of code/spec text to the criterion. Unvalidated criteria
	// fail the Spec gate; validated-without-test criteria warn.
	Validated bool `json:"validated"`
	// HasAutomatedTest is true when a test file appears to assert this
	// criterion. When all criteria are Validated but at least one lacks
	// an automated test, the Spec gate downgrades to a warn verdict.
	HasAutomatedTest bool `json:"hasAutomatedTest,omitempty"`
	// EvidencePath is a single file path that satisfied the criterion
	// (empty when not Validated). Used by the dashboard to deep-link the
	// user to the supporting code.
	EvidencePath string `json:"evidencePath,omitempty"`
}
