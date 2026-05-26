package eas

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// StoreSubmitOpts is the operator-friendly wrapper around
// SubmissionRequest. The orchestrator's mobileSubmitToStore resolver
// reaches for this so it can pass through one flat struct instead of
// building the per-platform config inline.
type StoreSubmitOpts struct {
	// Platform is "ios" or "android".
	Platform string

	// iOS-only ----------------------------------------------------
	IOSAppleID     string
	IOSASCAppID    string
	IOSAppleTeamID string
	IOSSKU         string
	IOSCompanyName string

	// Android-only ------------------------------------------------
	// AndroidServiceAccountKey is the raw bytes of the Google Play
	// service-account JSON the operator uploaded via the project
	// secret store. EAS expects it base64-encoded; the client
	// performs the encode inline.
	AndroidServiceAccountKey []byte
	// AndroidTrack is production|beta|alpha|internal. Defaults to
	// "internal" when empty.
	AndroidTrack string
	// AndroidReleaseStatus optionally pins draft|inProgress|halted|completed.
	AndroidReleaseStatus string

	// ProjectID is the EAS project the build belongs to.
	ProjectID string
}

// SubmitBuildToStore wraps CreateSubmission with sane defaults and
// per-platform config wiring. The buildID points at an existing EAS
// build artifact; for an arbitrary archive URL, use CreateSubmission
// directly.
func (c *Client) SubmitBuildToStore(ctx context.Context, buildID string, opts StoreSubmitOpts) (*Submission, error) {
	if strings.TrimSpace(buildID) == "" {
		return nil, errors.New("eas: SubmitBuildToStore: empty buildID")
	}
	platform := strings.ToLower(strings.TrimSpace(opts.Platform))
	if platform != "ios" && platform != "android" {
		return nil, fmt.Errorf("eas: SubmitBuildToStore: invalid platform %q (want ios|android)", opts.Platform)
	}
	req := SubmissionRequest{
		ProjectID: opts.ProjectID,
		Platform:  platform,
		BuildID:   buildID,
	}
	switch platform {
	case "ios":
		if strings.TrimSpace(opts.IOSAppleID) == "" || strings.TrimSpace(opts.IOSASCAppID) == "" {
			return nil, errors.New("eas: SubmitBuildToStore: iOS requires IOSAppleID and IOSASCAppID")
		}
		req.IOS = &IOSSubmitConfig{
			AppleID:     opts.IOSAppleID,
			ASCAppID:    opts.IOSASCAppID,
			AppleTeamID: opts.IOSAppleTeamID,
			SKU:         opts.IOSSKU,
			CompanyName: opts.IOSCompanyName,
		}
	case "android":
		if len(opts.AndroidServiceAccountKey) == 0 {
			return nil, errors.New("eas: SubmitBuildToStore: android requires AndroidServiceAccountKey")
		}
		track := strings.ToLower(strings.TrimSpace(opts.AndroidTrack))
		if track == "" {
			track = "internal"
		}
		switch track {
		case "production", "beta", "alpha", "internal":
			// ok
		default:
			return nil, fmt.Errorf("eas: SubmitBuildToStore: invalid android track %q (want production|beta|alpha|internal)", opts.AndroidTrack)
		}
		req.Android = &AndroidSubmitConfig{
			ServiceAccountKey: opts.AndroidServiceAccountKey,
			Track:             track,
			ReleaseStatus:     opts.AndroidReleaseStatus,
		}
	}
	return c.CreateSubmission(ctx, req)
}
