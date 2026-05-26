// Package eas wraps the Expo Application Services REST API
// (https://api.expo.dev/v2/) for the orchestrator's mobile path:
// retrieving builds, downloading artifacts, dispatching App Store /
// Google Play submissions, and publishing OTA updates.
//
// The public Client surface is intentionally narrow — only the
// endpoints the orchestrator + MobileBuildGate + ledger flows reach
// for. Background polling lives in poller.go.
package eas

import (
	"errors"
	"time"
)

// ErrEASTokenMissing is returned by Client methods when the orchestrator
// was booted without an EAS bearer token — both the project secret
// EAS_TOKEN and the env-var fallback are absent. The mobile resolvers
// surface this as a typed GraphQL NOT_CONFIGURED so the operator gets
// a clear next step.
var ErrEASTokenMissing = errors.New("eas: EAS_TOKEN not configured (set project secret EAS_TOKEN or the EAS_TOKEN env var)")

// BuildStatus mirrors the EAS REST status vocabulary. Treat unknown
// values as non-terminal so a future EAS release doesn't lock builds
// in the poller queue.
type BuildStatus string

const (
	BuildStatusNew        BuildStatus = "new"
	BuildStatusInQueue    BuildStatus = "in-queue"
	BuildStatusInProgress BuildStatus = "in-progress"
	BuildStatusFinished   BuildStatus = "finished"
	BuildStatusErrored    BuildStatus = "errored"
	BuildStatusCanceled   BuildStatus = "canceled"
)

// Terminal reports whether a build has reached a final state and
// should be removed from the poller queue.
func (s BuildStatus) Terminal() bool {
	switch s {
	case BuildStatusFinished, BuildStatusErrored, BuildStatusCanceled:
		return true
	}
	return false
}

// Succeeded reports whether a terminal build was successful.
func (s BuildStatus) Succeeded() bool {
	return s == BuildStatusFinished
}

// SubmissionStatus mirrors the EAS submission lifecycle.
type SubmissionStatus string

const (
	SubmissionStatusAwaitingBuild SubmissionStatus = "awaiting-build"
	SubmissionStatusInQueue       SubmissionStatus = "in-queue"
	SubmissionStatusInProgress    SubmissionStatus = "in-progress"
	SubmissionStatusFinished      SubmissionStatus = "finished"
	SubmissionStatusErrored       SubmissionStatus = "errored"
	SubmissionStatusCanceled      SubmissionStatus = "canceled"
)

// Terminal returns true for a submission that has stopped progressing.
func (s SubmissionStatus) Terminal() bool {
	switch s {
	case SubmissionStatusFinished, SubmissionStatusErrored, SubmissionStatusCanceled:
		return true
	}
	return false
}

// Build is the flattened EAS build record the orchestrator consumes.
// The dotted-key fields (ArtifactURL / ArtifactSize) come from the EAS
// API's nested `artifacts` object — the client.go decoder maps the raw
// nested payload into this flat shape.
type Build struct {
	ID              string      `json:"id"`
	Status          BuildStatus `json:"status"`
	Platform        string      `json:"platform"` // "ios" | "android"
	Profile         string      `json:"buildProfile"`
	Distribution    string      `json:"distribution"`
	ArtifactURL     string      `json:"artifactUrl,omitempty"`
	ArtifactSize    int64       `json:"artifactSize,omitempty"`
	LogURL          string      `json:"logUrl,omitempty"`
	AppVersion      string      `json:"appVersion"`
	AppBuildVersion string      `json:"appBuildVersion"`
	SDKVersion      string      `json:"sdkVersion"`
	Channel         string      `json:"channel,omitempty"`
	CreatedAt       time.Time   `json:"createdAt"`
	UpdatedAt       time.Time   `json:"updatedAt"`
	CompletedAt     *time.Time  `json:"completedAt,omitempty"`
	Error           *BuildError `json:"error,omitempty"`
	ProjectID       string      `json:"projectId"`
	Initiator       string      `json:"initiator,omitempty"`
}

// BuildError carries EAS's structured failure object so the operator
// dashboard can surface the actionable message + code instead of the
// raw stack trace.
type BuildError struct {
	ErrorCode string `json:"errorCode,omitempty"`
	Message   string `json:"message,omitempty"`
	DocsURL   string `json:"docsUrl,omitempty"`
}

// Submission is the flattened EAS submission record.
type Submission struct {
	ID          string           `json:"id"`
	Status      SubmissionStatus `json:"status"`
	Platform    string           `json:"platform"` // "ios" | "android"
	Target      string           `json:"target"`   // "ios-app-store" | "android-google-play"
	BuildID     string           `json:"buildId,omitempty"`
	ArchiveURL  string           `json:"archiveUrl,omitempty"`
	LogURL      string           `json:"logUrl,omitempty"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	CompletedAt *time.Time       `json:"completedAt,omitempty"`
	Error       *BuildError      `json:"error,omitempty"`
	ProjectID   string           `json:"projectId"`
}

// Update is the flattened EAS OTA update record returned by
// PublishUpdate. Only the operator-facing fields are surfaced — the
// raw manifest payload stays inside EAS.
type Update struct {
	ID             string    `json:"id"`
	Branch         string    `json:"branch"`
	Channel        string    `json:"channel"`
	RuntimeVersion string    `json:"runtimeVersion"`
	Message        string    `json:"message,omitempty"`
	ManifestURL    string    `json:"manifestUrl,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	Platform       string    `json:"platform,omitempty"`
	GroupID        string    `json:"group,omitempty"`
}

// ListBuildsOpts narrows the projects/{id}/builds listing.
type ListBuildsOpts struct {
	Platform string      // "ios" | "android" | "" (any)
	Status   BuildStatus // empty = any
	Limit    int         // 0 = EAS default
	Offset   int
	Channel  string
}

// SubmissionRequest is the body we POST to /v2/projects/{id}/submissions.
// Exactly one of BuildID / ArchiveURL must be set; CreateSubmission
// enforces that invariant before hitting the wire.
type SubmissionRequest struct {
	ProjectID  string
	Platform   string // "ios" | "android"
	BuildID    string // submit an existing EAS build artifact
	ArchiveURL string // OR submit an arbitrary archive URL

	IOS     *IOSSubmitConfig
	Android *AndroidSubmitConfig
}

// IOSSubmitConfig carries the App Store Connect knobs EAS forwards to
// fastlane's deliver. AppleID + ASCAppID are required; AppleTeamID is
// only required when the Apple account has multiple teams.
type IOSSubmitConfig struct {
	AppleID     string `json:"appleId"`
	ASCAppID    string `json:"ascAppId"`
	AppleTeamID string `json:"appleTeamId,omitempty"`
	// SKU is the App Store Connect product SKU — required only for
	// brand-new apps that have never been submitted before.
	SKU string `json:"sku,omitempty"`
	// CompanyName feeds the export-compliance form.
	CompanyName string `json:"companyName,omitempty"`
}

// AndroidSubmitConfig carries the Google Play release knobs.
// ServiceAccountKey is the raw bytes of the
// google-play-service-account.json that EAS forwards through to the
// Play Developer API; CreateSubmission base64-encodes it before
// shipping. Track must be one of production|beta|alpha|internal.
type AndroidSubmitConfig struct {
	ServiceAccountKey       []byte `json:"-"`
	Track                   string `json:"track"`
	ReleaseStatus           string `json:"releaseStatus,omitempty"` // draft|inProgress|halted|completed
	ChangesNotSentForReview bool   `json:"changesNotSentForReview,omitempty"`
}

// PublishUpdateRequest is the body for the channel-scoped OTA publish.
type PublishUpdateRequest struct {
	Branch         string         // e.g. "main"
	Message        string         // operator-readable changelog line
	RuntimeVersion string         // matches expo-updates' runtimeVersion
	ManifestExtra  map[string]any // arbitrary key/value forwarded to the manifest
}
