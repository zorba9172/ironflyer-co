package devicecloud

import (
	"os"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

// Credentials carries the four secrets the device-cloud providers need.
// Empty fields mean "not configured" — the Manager refuses to register
// a provider whose credentials are empty so the cockpit can render a
// disabled chip instead of a runtime 401.
type Credentials struct {
	BrowserStackUsername  string
	BrowserStackAccessKey string
	AWSAccessKeyID        string
	AWSSecretAccessKey    string
}

// ResolveCredentials returns the device-cloud credentials for a
// project. Project Secrets win over environment variables so a tenant
// can override the platform-wide BrowserStack account with their own.
// Per-user isolation: ResolveCredentials never reads from another
// project — Project.Secrets is the only project-scoped surface.
func ResolveCredentials(p *domain.Project) Credentials {
	var creds Credentials
	if p != nil {
		creds.BrowserStackUsername = strings.TrimSpace(p.Secrets["BROWSERSTACK_USERNAME"])
		creds.BrowserStackAccessKey = strings.TrimSpace(p.Secrets["BROWSERSTACK_ACCESS_KEY"])
		creds.AWSAccessKeyID = strings.TrimSpace(p.Secrets["AWS_ACCESS_KEY_ID"])
		creds.AWSSecretAccessKey = strings.TrimSpace(p.Secrets["AWS_SECRET_ACCESS_KEY"])
	}
	if creds.BrowserStackUsername == "" {
		creds.BrowserStackUsername = strings.TrimSpace(os.Getenv("BROWSERSTACK_USERNAME"))
	}
	if creds.BrowserStackAccessKey == "" {
		creds.BrowserStackAccessKey = strings.TrimSpace(os.Getenv("BROWSERSTACK_ACCESS_KEY"))
	}
	if creds.AWSAccessKeyID == "" {
		creds.AWSAccessKeyID = strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	}
	if creds.AWSSecretAccessKey == "" {
		creds.AWSSecretAccessKey = strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	}
	return creds
}

// HasBrowserStack reports whether the BrowserStack provider is wirable.
func (c Credentials) HasBrowserStack() bool {
	return c.BrowserStackUsername != "" && c.BrowserStackAccessKey != ""
}

// HasAWSDeviceFarm reports whether the AWS Device Farm stub should be
// registered. The stub never makes real calls but we still gate
// registration on credentials so the UI chip stays honest.
func (c Credentials) HasAWSDeviceFarm() bool {
	return c.AWSAccessKeyID != "" && c.AWSSecretAccessKey != ""
}
