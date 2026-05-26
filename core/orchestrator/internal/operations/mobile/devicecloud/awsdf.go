package devicecloud

import (
	"context"
	"io"
)

// AWSDeviceFarmClient is a deliberate stub.
//
// AWS Device Farm is most valuable for batched CI test grids, not the
// interactive sessions that BrowserStack handles. Wiring aws-sdk-go-v2
// pulls in a multi-megabyte dependency tree we don't yet need, so this
// stub keeps the integration shape (ProviderClient interface) in place
// without paying the binary-size tax until a real customer asks for it.
//
// TODO(devicecloud-aws): when AWS Device Farm becomes a paying surface:
//   1. `go get github.com/aws/aws-sdk-go-v2/service/devicefarm` and the
//      shared config package.
//   2. Replace the body of each method with the matching SDK call
//      (CreateUpload + S3 PUT for UploadApp, ScheduleRun + GetRun for
//      sessions, StopRun for EndSession, GetRun for GetSession).
//   3. Add the credentials wiring in ResolveCredentials so the
//      aws.Config picks up project Secrets first, env second.
//   4. Cache devices through ListDevices like BrowserStack does — the
//      AWS catalogue is also slow-changing.
//
// Until then, every method returns ErrAWSDeviceFarmNotConfigured so
// resolvers can surface a typed "Pro tier add-on coming soon" chip.
type AWSDeviceFarmClient struct{}

// NewAWSDeviceFarmClient constructs the stub. Accepting credentials in
// the signature now lets the real implementation slot in without
// touching call sites in cmd/ wireup.
func NewAWSDeviceFarmClient(_, _, _ string) *AWSDeviceFarmClient {
	return &AWSDeviceFarmClient{}
}

// Name returns ProviderAWSDeviceFarm.
func (c *AWSDeviceFarmClient) Name() Provider { return ProviderAWSDeviceFarm }

// ListDevices returns ErrAWSDeviceFarmNotConfigured.
func (c *AWSDeviceFarmClient) ListDevices(_ context.Context, _ string) ([]Device, error) {
	return nil, ErrAWSDeviceFarmNotConfigured
}

// UploadApp returns ErrAWSDeviceFarmNotConfigured.
func (c *AWSDeviceFarmClient) UploadApp(_ context.Context, _ io.Reader, _ string) (string, error) {
	return "", ErrAWSDeviceFarmNotConfigured
}

// StartSession returns ErrAWSDeviceFarmNotConfigured.
func (c *AWSDeviceFarmClient) StartSession(_ context.Context, _ StartSessionRequest) (*Session, error) {
	return nil, ErrAWSDeviceFarmNotConfigured
}

// EndSession returns ErrAWSDeviceFarmNotConfigured.
func (c *AWSDeviceFarmClient) EndSession(_ context.Context, _ string) error {
	return ErrAWSDeviceFarmNotConfigured
}

// GetSession returns ErrAWSDeviceFarmNotConfigured.
func (c *AWSDeviceFarmClient) GetSession(_ context.Context, _ string) (*Session, error) {
	return nil, ErrAWSDeviceFarmNotConfigured
}
