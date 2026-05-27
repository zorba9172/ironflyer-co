package provisioning

import (
	"context"
	"errors"
	"net/http"
	"time"

	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// DomainResellerOpts wires the domain reseller connector. APIKey is
// the reseller-program credential; ResellerProvider switches the wire
// implementation between Cloudflare Registrar's partner API and
// NameSilo's wholesale API. Other resellers (Porkbun, OpenSRS, ...)
// can be added by branching ResellerProvider.
//
// Both Cloudflare Registrar and NameSilo require a partner-program
// approval that is not standard self-serve creds, so this connector
// ships as a skeleton — Provision returns ErrConnectorDisabled until
// APIKey is wired AND ResellerProvider is set to a supported value.
// Doing the real API call from a half-wired connector would silently
// record phantom revenue rows that break the ledger invariant.
type DomainResellerOpts struct {
	APIKey           string
	ResellerProvider string // "cloudflare" | "namesilo"
	WebhookSecret    string
	HTTPClient       *http.Client
	Policies         PolicyStore
}

// DomainReseller is the Connector for custom-domain provisioning.
// Standard cuts apply: 20% of renewal markup, $1 floor per event (see
// DefaultPolicies). Renewals are typically yearly so per-event
// throughput is low; cadence is monthly-aggregate.
type DomainReseller struct {
	opts DomainResellerOpts
	http *http.Client
}

// NewDomainReseller constructs the reseller connector. Returns a
// disabled connector when APIKey or ResellerProvider is empty.
func NewDomainReseller(opts DomainResellerOpts) *DomainReseller {
	if opts.HTTPClient == nil {
		opts.HTTPClient = httpclient.Standard(15 * time.Second)
	}
	return &DomainReseller{opts: opts, http: opts.HTTPClient}
}

// Name implements Connector.
func (d *DomainReseller) Name() string { return KindCloudflareDomain }

// Label implements Connector.
func (d *DomainReseller) Label() string { return "Custom domain (reseller)" }

// Enabled implements Connector. Both an APIKey AND a supported
// ResellerProvider must be set; either alone is treated as
// half-configured and rejected.
func (d *DomainReseller) Enabled() bool {
	if d == nil || d.opts.APIKey == "" {
		return false
	}
	switch d.opts.ResellerProvider {
	case "cloudflare", "namesilo":
		return true
	default:
		return false
	}
}

// Provision implements Connector. The skeleton routes through
// ResellerProvider to the underlying reseller's domain-registration
// API; until either branch is wired with partner-approved creds, the
// connector stays disabled and this method returns ErrConnectorDisabled.
//
// Expected Metadata keys (validated when the real call is implemented):
//   - "domain"       — fully-qualified name to register
//   - "term_years"   — initial registration term (default 1)
//   - "contact_*"    — ICANN-required contact set
func (d *DomainReseller) Provision(_ context.Context, _, _ string, _ ProvisionOptions) (ProvisionedResource, error) {
	if !d.Enabled() {
		return ProvisionedResource{}, ErrConnectorDisabled
	}
	// The Cloudflare Registrar partner endpoint POSTs to
	// /accounts/:id/registrar/domains with the registration body;
	// NameSilo uses /api/registerDomain?key=...&domain=... — both
	// are deferred until partner-program creds are wired in prod.
	return ProvisionedResource{}, errors.New("provisioning: domain reseller wire implementation pending partner-program credentials")
}

// RecordRevenue implements Connector. Domain renewals are billed
// monthly via the reseller invoice export — the cron pulls the export
// and emits one RevenueEvent per renewed domain. Returns an empty
// slice when no renewals landed since the last sweep.
func (d *DomainReseller) RecordRevenue(_ context.Context, _ ProvisionedResource) ([]RevenueEvent, error) {
	if !d.Enabled() {
		return nil, ErrConnectorDisabled
	}
	return nil, nil
}

// HandleWebhook implements Connector. Cloudflare publishes domain
// lifecycle webhooks; NameSilo does not. The skeleton returns
// (nil, nil) on all events until the Cloudflare branch is wired.
func (d *DomainReseller) HandleWebhook(_ context.Context, _ []byte, _ string) (*RevenueEvent, error) {
	if d.opts.WebhookSecret == "" {
		return nil, errors.New("provisioning: domain reseller webhook secret not configured")
	}
	return nil, nil
}

// Suspend implements Connector. Domain "suspension" without a transfer
// is the reseller's lock-domain flag — the API call is provider-specific
// and lands when the Provision branch lands.
func (d *DomainReseller) Suspend(_ context.Context, _ ProvisionedResource) error {
	if !d.Enabled() {
		return ErrConnectorDisabled
	}
	return nil
}

var _ Connector = (*DomainReseller)(nil)
