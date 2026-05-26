// Package appetize is a thin REST client + meter for Appetize.io, the
// browser-embeddable iOS / Android simulator service we use on the
// Free tier (no Mac pool required). The API surface is documented at
// https://docs.appetize.io/.
//
// Two surfaces:
//   - Client       — pure REST wrapper (upload, get, delete, embed URL)
//   - MeteredClient — wraps Client to emit Appetize-minute ledger entries
//
// Both stay nil-safe at call sites; the orchestrator boots fine
// without an APPETIZE_TOKEN, the resolver simply errors typed.
package appetize

import "time"

// App is the JSON shape Appetize returns from POST /v1/apps and
// GET /v1/apps/{publicKey}. We retain only the fields the orchestrator
// actually consumes — the upstream response includes a much larger
// blob (build metadata, app analytics) we explicitly ignore.
type App struct {
	PublicKey  string    `json:"publicKey"`
	PrivateKey string    `json:"privateKey,omitempty"`
	AppURL     string    `json:"appURL"`
	ManageURL  string    `json:"manageURL,omitempty"`
	Platform   string    `json:"platform"`
	Created    time.Time `json:"created"`
	Updated    time.Time `json:"updated"`
	// Versions captures the historic versionCode list Appetize returns
	// for Android uploads. iOS uploads return a similar `version` field;
	// we keep both under one Go field for convenience.
	Versions []string `json:"versionCode,omitempty"`
}

// UploadRequest is the multipart body sent to POST /v1/apps. The caller
// provides the artifact as an io.Reader so we can stream large IPAs /
// APKs without buffering them in memory; FileName is the form-data
// "filename" attribute (Appetize requires the extension to match the
// declared platform).
type UploadRequest struct {
	ArtifactReader interface {
		Read(p []byte) (n int, err error)
	}
	FileName string
	// Platform is one of "ios" | "android". Appetize rejects upload
	// requests where Platform disagrees with the artifact extension.
	Platform  string
	BuildName string
	Note      string
}

// EmbedOptions is the subset of Appetize embed query parameters
// Ironflyer surfaces. The full list lives at
// https://docs.appetize.io/core-features/playing-apps/embed-options.
type EmbedOptions struct {
	// Device — e.g. "iphone15pro", "iphone15", "ipadpro13", "pixel8".
	Device string
	// OSVersion — e.g. "18" for iOS 18 or "14" for Android 14.
	OSVersion string
	// DeviceColor — e.g. "black", "silver", "gold".
	DeviceColor string
	// Scale — render scale percent, 25..200. Zero falls back to 75.
	Scale int
	// Locale — IETF language tag, e.g. "en_US".
	Locale string
	// Orientation — "portrait" | "landscape".
	Orientation string
	// Centered — surface the device frame centred in the iframe.
	Centered bool
	// AutoPlay — boot the simulator immediately on iframe load.
	AutoPlay bool
	// RecordSession — capture every user input + visible frame to the
	// Appetize session recording (Pro feature; ignored otherwise).
	RecordSession bool
}
