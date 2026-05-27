package provisioning

import (
	"context"
	"errors"
	"net/http"
	"time"

	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// EmailPartnerOpts wires the transactional-email connector. Provider
// switches the wire implementation between Resend's partner program
// and Postmark's reseller API. Both require a partner-program
// agreement on top of standard API creds, so the connector ships as a
// skeleton — Enabled() is false until APIKey + Provider are both set.
type EmailPartnerOpts struct {
	APIKey        string
	Provider      string // "resend" | "postmark"
	WebhookSecret string
	HTTPClient    *http.Client
	Policies      PolicyStore
}

// EmailPartner is the Connector for transactional-email rails. The cut
// model is 10% of the merchant's invoice (see DefaultPolicies); volume
// is reconciled monthly from the partner's reporting API.
type EmailPartner struct {
	opts EmailPartnerOpts
	http *http.Client
}

// NewEmailPartner constructs the email-partner connector. Returns a
// disabled connector when APIKey or Provider is empty.
func NewEmailPartner(opts EmailPartnerOpts) *EmailPartner {
	if opts.HTTPClient == nil {
		opts.HTTPClient = httpclient.Standard(15 * time.Second)
	}
	return &EmailPartner{opts: opts, http: opts.HTTPClient}
}

// Name implements Connector.
func (e *EmailPartner) Name() string { return KindResendEmail }

// Label implements Connector.
func (e *EmailPartner) Label() string { return "Transactional email (partner)" }

// Enabled implements Connector. Requires APIKey AND a supported
// Provider — half-configured deployments stay disabled.
func (e *EmailPartner) Enabled() bool {
	if e == nil || e.opts.APIKey == "" {
		return false
	}
	switch e.opts.Provider {
	case "resend", "postmark":
		return true
	default:
		return false
	}
}

// Provision implements Connector. Skeleton — returns ErrConnectorDisabled
// until partner-program creds are wired. Expected Metadata keys
// (validated when the real call is implemented):
//   - "sending_domain"    — the merchant's verified sending domain
//   - "monthly_volume"    — projected monthly send count (for tier)
func (e *EmailPartner) Provision(_ context.Context, _, _ string, _ ProvisionOptions) (ProvisionedResource, error) {
	if !e.Enabled() {
		return ProvisionedResource{}, ErrConnectorDisabled
	}
	return ProvisionedResource{}, errors.New("provisioning: email partner wire implementation pending partner-program credentials")
}

// RecordRevenue implements Connector. Email volume is reported via
// the partner's monthly invoice API; the cron sweeps it and emits one
// RevenueEvent per merchant per month.
func (e *EmailPartner) RecordRevenue(_ context.Context, _ ProvisionedResource) ([]RevenueEvent, error) {
	if !e.Enabled() {
		return nil, ErrConnectorDisabled
	}
	return nil, nil
}

// HandleWebhook implements Connector. Resend webhooks fire on
// delivery / bounce / complaint events (not revenue per se), so the
// connector ignores them by returning (nil, nil) until partner-program
// revenue webhooks are wired.
func (e *EmailPartner) HandleWebhook(_ context.Context, _ []byte, _ string) (*RevenueEvent, error) {
	if e.opts.WebhookSecret == "" {
		return nil, errors.New("provisioning: email partner webhook secret not configured")
	}
	return nil, nil
}

// Suspend implements Connector. Pauses the merchant's sending domain
// via the partner API — concrete call lands with the Provision branch.
func (e *EmailPartner) Suspend(_ context.Context, _ ProvisionedResource) error {
	if !e.Enabled() {
		return ErrConnectorDisabled
	}
	return nil
}

var _ Connector = (*EmailPartner)(nil)
