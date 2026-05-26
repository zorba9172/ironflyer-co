package devicecloud

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrProviderNotConfigured is returned by the Manager when a caller
// targets a Provider that has no client registered (typically because
// the user hasn't entered API credentials yet).
var ErrProviderNotConfigured = errors.New("devicecloud: provider not configured")

// ErrAWSDeviceFarmNotConfigured is the stable error returned by the
// AWS Device Farm stub while the aws-sdk-go-v2 dependency is opt-in.
// Resolvers translate this into a typed "not configured" GraphQL
// extension instead of a generic 500 so the UI can keep the provider
// chip disabled until credentials + the SDK are wired.
var ErrAWSDeviceFarmNotConfigured = errors.New("devicecloud: AWS Device Farm not configured (aws-sdk-go-v2 missing)")

// StartSessionRequest is the input envelope every provider's
// StartSession method consumes. SessionLength is the upper bound the
// orchestrator wants for the allocation — BrowserStack caps the App
// Live tier at 30 minutes per session, so values above 30m are clamped
// inside the client.
type StartSessionRequest struct {
	AppURL        string
	DeviceID      string
	SessionLength time.Duration
}

// ProviderClient is the multi-vendor surface the Manager fans out to.
// Implementations are responsible for translating their REST quirks
// (BrowserStack basic-auth uploads, AWS Device Farm signed requests)
// into the shared Device/Session shape so the resolver layer only
// talks to one interface.
type ProviderClient interface {
	Name() Provider
	ListDevices(ctx context.Context, platform string) ([]Device, error)
	UploadApp(ctx context.Context, artifactReader io.Reader, fileName string) (appURL string, err error)
	StartSession(ctx context.Context, req StartSessionRequest) (*Session, error)
	EndSession(ctx context.Context, sessionID string) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
}
