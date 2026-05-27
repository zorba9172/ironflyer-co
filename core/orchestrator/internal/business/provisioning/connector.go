package provisioning

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ProvisionOptions is the per-Provision input the resolver passes
// through to the connector. Free-form Metadata lets each connector
// pick up rail-specific fields (Stripe Connect needs business_type
// + country; Cloudflare needs the desired domain string; Resend
// needs the sending domain). Connectors validate their own required
// keys and return a typed error on miss.
type ProvisionOptions struct {
	// ReturnURL is where the rail should send the merchant after
	// onboarding completes (Stripe AccountLinks success_url, etc.).
	// Optional — connectors fall back to a sensible default.
	ReturnURL string
	// RefreshURL is where the rail should send the merchant when the
	// onboarding link expires. Optional.
	RefreshURL string
	// Metadata carries connector-specific knobs. Stored verbatim on the
	// rail when supported (Stripe metadata, etc.) so partner-side
	// queries can correlate back to Ironflyer.
	Metadata map[string]string
}

// Connector is the rail-side contract — the analogue of wallet.Topper.
// One Connector instance maps to one rail (Stripe Connect, Cloudflare
// Registrar, Resend, hosting), implements the four lifecycle calls,
// and owns its own credentials + HTTP client. The package never holds
// a typed pointer to a specific connector; resolvers and reconcilers
// take the interface.
type Connector interface {
	// Name is the stable kind identifier. Lowercase, hyphenated.
	// Matches one of provisioning.Kind* constants — the registry uses
	// Name to route ByKind lookups.
	Name() string

	// Enabled reports whether this connector has the credentials it
	// needs. Disabled connectors stay in the registry but never reach
	// the user — Active() filters them out so resolvers can offer the
	// connector chips list directly.
	Enabled() bool

	// Label is the human-readable name surfaced on the
	// `availableConnectors` GraphQL query — "Stripe payments" rather
	// than "stripe-connect" — so the dashboard can render a clean
	// chip without a client-side string table.
	Label() string

	// Provision creates a new rail-side resource for (tenant, project)
	// and returns the ProvisionedResource shape. The connector calls
	// the rail's onboarding API, gets back an external id, and
	// returns the unsaved struct — wireup then persists via
	// Service.Provision so the rail call and the DB write are kept
	// loosely coupled (a rail outage doesn't leak DB rows).
	Provision(ctx context.Context, tenant, project string, opts ProvisionOptions) (ProvisionedResource, error)

	// RecordRevenue is the cron-driven pull path: ask the rail for any
	// revenue activity on the resource since the last sweep and return
	// the resulting RevenueEvent list. Connectors that drive revenue
	// exclusively via webhooks (Resend, hosting) may return an empty
	// slice — the reconciler still calls them for healthcheck.
	RecordRevenue(ctx context.Context, resource ProvisionedResource) ([]RevenueEvent, error)

	// HandleWebhook is the push path: signature-verifies the inbound
	// vendor webhook body, decodes the event, and returns a single
	// RevenueEvent when the event is revenue-relevant (nil otherwise).
	// Wireup then persists the event via Service.RecordRevenue —
	// idempotent against ExternalRef so a redelivery is a no-op.
	HandleWebhook(ctx context.Context, rawBody []byte, signatureHeader string) (*RevenueEvent, error)

	// Suspend tells the rail to stop accepting new activity on the
	// resource (Stripe Connect: capabilities disabled; Resend: domain
	// paused). Reversible via a future Reactivate call. Idempotent.
	Suspend(ctx context.Context, resource ProvisionedResource) error
}

// ConnectorInfo is the resolver-facing projection of a Connector —
// just the bits the `availableConnectors` query exposes. SharePct is
// surfaced so the UI can render "1.5% per charge" next to the
// connector chip without a second round-trip.
type ConnectorInfo struct {
	Name     string
	Label    string
	Enabled  bool
	SharePct string
}

// ConnectorRegistry holds every Connector wired at boot. Mirrors
// wallet.TopperRegistry — primary connectors keyed by Kind, ByKind
// lookup returns ErrConnectorDisabled when the requested kind is
// missing or has Enabled() == false.
type ConnectorRegistry struct {
	mu         sync.RWMutex
	connectors map[string]Connector
}

// NewConnectorRegistry builds an empty registry. Callers Register
// individual Connector implementations after construction so wireup
// can pass nil for unconfigured rails without exploding.
func NewConnectorRegistry() *ConnectorRegistry {
	return &ConnectorRegistry{connectors: map[string]Connector{}}
}

// Register adds a Connector to the registry. A nil connector or a
// connector whose Name() collides with an existing entry is silently
// skipped — wireup can call Register unconditionally with a typed nil
// (typed-nil interfaces would panic on the Enabled() call later, but
// the nil check here catches the literal-nil case).
func (r *ConnectorRegistry) Register(c Connector) {
	if r == nil || c == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.connectors[c.Name()]; exists {
		return
	}
	r.connectors[c.Name()] = c
}

// ByKind returns the Connector for the requested kind. Case-insensitive
// on the kind string — UI-supplied values arrive lowercased but a
// curl from operator tooling may not. Returns ErrConnectorDisabled
// when the kind is unknown or its connector is disabled.
func (r *ConnectorRegistry) ByKind(kind string) (Connector, error) {
	if r == nil {
		return nil, ErrConnectorDisabled
	}
	n := strings.ToLower(strings.TrimSpace(kind))
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.connectors[n]
	if !ok || c == nil || !c.Enabled() {
		return nil, fmt.Errorf("%w: %s", ErrConnectorDisabled, n)
	}
	return c, nil
}

// Active returns the enabled connectors in deterministic order
// (lexicographic by Name). Resolvers fan this list into ConnectorInfo
// chips on the project dashboard.
func (r *ConnectorRegistry) Active() []Connector {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Connector, 0, len(r.connectors))
	for _, c := range r.connectors {
		if c == nil || !c.Enabled() {
			continue
		}
		out = append(out, c)
	}
	// Sort by Name so the UI ordering is stable across boots — map
	// iteration would otherwise jitter the connector chip order.
	sortConnectorsByName(out)
	return out
}

// AnyEnabled is the Vault.Enabled() helper — true when at least one
// connector is configured + enabled.
func (r *ConnectorRegistry) AnyEnabled() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.connectors {
		if c != nil && c.Enabled() {
			return true
		}
	}
	return false
}

// sortConnectorsByName is an in-place insertion sort. The Active()
// list is bounded by Kind* constants (single-digit length), so a
// stdlib sort.Slice would be more code than it saves.
func sortConnectorsByName(cs []Connector) {
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0 && cs[j-1].Name() > cs[j].Name(); j-- {
			cs[j-1], cs[j] = cs[j], cs[j-1]
		}
	}
}
