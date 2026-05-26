// Package devicecloud is the Pro-tier multi-provider gateway to real
// mobile hardware. Free-tier projects ship with the existing Android
// emulator + Mac-pool simulator path; devicecloud layers BrowserStack
// App Live (~3000 physical Pixel/iPhone/Galaxy devices) and AWS Device
// Farm (batched test grids) behind a single ProviderClient interface so
// the orchestrator can fan a session out to whichever provider the user
// has credentials for.
//
// Per-user isolation is preserved by the Manager: every session start
// and end is wallet-anchored through ledger.RecordDeviceCloudMinutes,
// and projectID + workspaceID flow into the ledger metadata so the
// dashboards can split spend per project just like emulator minutes.
package devicecloud

import "time"

// Provider is the canonical identifier of a device-cloud vendor.
type Provider string

const (
	// ProviderBrowserStack is BrowserStack App Live — interactive
	// real-device sessions plus an embeddable browser viewer.
	ProviderBrowserStack Provider = "browserstack"
	// ProviderAWSDeviceFarm is the AWS Device Farm CI grid — batched
	// test runs, no interactive viewer. Cheaper, but not session-based.
	ProviderAWSDeviceFarm Provider = "aws-device-farm"
)

// Device is a single device offered by a provider. Real == true means
// physical hardware; emulator/simulator entries report Real = false so
// the UI can label them honestly.
type Device struct {
	ID           string   `json:"id"`
	Provider     Provider `json:"provider"`
	Platform     string   `json:"platform"` // android | ios
	OSVersion    string   `json:"osVersion"`
	Model        string   `json:"model"`
	Manufacturer string   `json:"manufacturer,omitempty"`
	Real         bool     `json:"real"`
}

// Session is the live or completed handle to a device-cloud allocation.
// BillableMinutesUsed updates as the manager polls the provider; the
// counter is what the Pro-tier cost panel mirrors live.
type Session struct {
	ID                  string    `json:"id"`
	Provider            Provider  `json:"provider"`
	DeviceID            string    `json:"deviceId"`
	AppURL              string    `json:"appUrl,omitempty"`
	SessionURL          string    `json:"sessionUrl,omitempty"`
	Status              string    `json:"status"` // creating | running | ended
	StartedAt           time.Time `json:"startedAt"`
	ExpiresAt           time.Time `json:"expiresAt"`
	BillableMinutesUsed float64   `json:"billableMinutesUsed"`
}
