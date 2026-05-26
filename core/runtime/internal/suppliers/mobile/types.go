// Package mobile owns per-workspace mobile build + dev-server lifecycles
// (Expo, React Native bare, Android native, iOS native, Flutter). The
// runtime service is a separate Go module from the orchestrator; the
// constants here MIRROR the orchestrator's domain types deliberately —
// do not introduce a cross-module import.
package mobile

import "time"

// MobileTarget is the OS the build artifact targets.
type MobileTarget string

const (
	TargetAndroid MobileTarget = "android"
	TargetIOS     MobileTarget = "ios"
)

// MobileKind is the project flavour the runtime drives. These mirror the
// orchestrator's domain constants intentionally — we duplicate rather
// than import to keep the runtime module self-contained.
type MobileKind string

const (
	KindExpo            MobileKind = "expo"
	KindReactNativeBare MobileKind = "react-native-bare"
	KindAndroidNative   MobileKind = "android-native"
	KindIOSNative       MobileKind = "ios-native"
	KindFlutter         MobileKind = "flutter"
)

// BuildRequest is the input to Manager.Build. Signing holds pre-resolved
// secret VALUES (the orchestrator resolves project secret references
// before dispatching to the runtime).
type BuildRequest struct {
	WorkspaceID string       `json:"workspaceId"`
	Kind        MobileKind   `json:"kind"`
	Target      MobileTarget `json:"target"`
	Profile     string       `json:"profile,omitempty"` // "development" | "preview" | "production"
	AppID       string       `json:"appId,omitempty"`
	Version     string       `json:"version,omitempty"`
	Signing     SigningRefs  `json:"signing,omitempty"`
}

// BuildResult is the outcome of a single mobile build run.
type BuildResult struct {
	ArtifactPath      string `json:"artifactPath,omitempty"`
	ArtifactSizeBytes int64  `json:"artifactSizeBytes,omitempty"`
	DurationMS        int64  `json:"durationMs"`
	ExitCode          int    `json:"exitCode"`
	LogTail           string `json:"logTail,omitempty"`
	ManifestSHA       string `json:"manifestSha,omitempty"`
}

// SigningRefs carries the pre-resolved signing material the runtime needs
// to produce a release build. Every field is the actual secret value —
// the orchestrator is responsible for resolving project-secret KEYS into
// VALUES before calling the runtime. The runtime never touches the
// orchestrator's secret store directly.
type SigningRefs struct {
	// Android — debug builds need none of these; release builds need all four.
	AndroidKeystoreB64 string `json:"androidKeystoreB64,omitempty"`
	AndroidKeyAlias    string `json:"androidKeyAlias,omitempty"`
	AndroidStorePass   string `json:"androidStorePass,omitempty"`
	AndroidKeyPass     string `json:"androidKeyPass,omitempty"`
	// iOS — required for any device build; simulator builds skip codesign.
	IOSProvisioningProfileB64 string `json:"iosProvisioningProfileB64,omitempty"`
	IOSCertificateP12B64      string `json:"iosCertificateP12B64,omitempty"`
	IOSCertificatePassword    string `json:"iosCertificatePassword,omitempty"`
	IOSTeamID                 string `json:"iosTeamId,omitempty"`
}

// EmulatorSession represents one running Android emulator inside a
// workspace, bridged out to the web frontend via a WebRTC relay.
type EmulatorSession struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	AVDName     string    `json:"avdName"`
	ADBPort     int       `json:"adbPort"`
	WebRTCURL   string    `json:"webrtcUrl"`
	StartedAt   time.Time `json:"startedAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// ExpoSession represents a running `npx expo start --tunnel` instance.
// The QR code is rendered CLIENT-SIDE by the frontend (qrcode.react),
// so we only ship the payload string the QR encodes.
type ExpoSession struct {
	WorkspaceID string    `json:"workspaceId"`
	MetroURL    string    `json:"metroUrl,omitempty"`
	LANURL      string    `json:"lanUrl,omitempty"`
	TunnelURL   string    `json:"tunnelUrl,omitempty"`
	QRPayload   string    `json:"qrPayload,omitempty"`
	StartedAt   time.Time `json:"startedAt"`
}
