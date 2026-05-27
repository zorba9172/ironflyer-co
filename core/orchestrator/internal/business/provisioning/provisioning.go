// Package provisioning is Ironflyer's downstream-rail revenue layer:
// when an Ironflyer-shipped app needs a third-party rail (Stripe Connect
// for payments, Cloudflare/NameSilo for a custom domain, Resend/Postmark
// for transactional email, hosting for compute), Ironflyer is the
// *issuing party*. Every transaction on those rails earns Ironflyer a
// configurable revenue-share cut for the life of the relationship — the
// Shopify-on-Stripe model: a small slice of every payment routed
// through the platform, compounding forever.
//
// Boundaries with neighbours:
//
//   - `wallet` is the *inbound* prepaid credit a tenant tops up to pay
//     for Ironflyer executions. `provisioning` is the *outbound* set of
//     third-party rails the tenant's shipped app rides on, and the
//     residual cut Ironflyer earns there.
//   - `budget.Vault` snapshots margin on Ironflyer's own executions.
//     `provisioning` is a separate ledger of *forever* revenue lines
//     that survive past any single execution.
//   - The package owns its own connectors (Stripe Connect application
//     fees, domain reseller markup, email partner share, hosting
//     mark-up). Connectors share the `Connector` interface so wireup
//     stays one constructor + one registry — the same shape `wallet`
//     uses for `Topper` / `TopperRegistry`.
//
// Defaults asserted by this package (documented here so reviewers don't
// have to spelunk):
//
//   - All money is `decimal.Decimal` USD. Floats are never permitted on
//     any rail input or output, even when a third-party API returns
//     cents as a number — we convert immediately at the boundary.
//   - `RevenuePolicy.SharePct` is a *fraction*, not a percentage: 0.015
//     means 1.5%. `MinFeeUSD` is a per-event floor; events that compute
//     under the floor still take the floor (because the issuing party
//     pays a fixed-cost per call to the underlying rail).
//   - Stripe Connect uses Standard accounts (merchant owns customers,
//     refunds, disputes) onboarded via AccountLinks. We charge
//     `application_fee_amount` on every charge for the cut. Direct
//     charges are *not* in scope yet — Standard keeps the merchant on
//     the hook for compliance, which is the right tradeoff at the
//     Ironflyer issuer tier.
//   - Connector implementations besides Stripe Connect ship as
//     skeletons: Cloudflare Registrar and Resend partner APIs both
//     require partner-program approval beyond standard creds, so the
//     stubs document the wire shape and return ErrConnectorDisabled
//     until a real key lands. This is intentional — shipping a fake
//     connector that silently records phantom revenue would break the
//     ledger invariant the rest of the platform depends on.
package provisioning

import (
	"time"

	"github.com/shopspring/decimal"
)

// Resource kinds. Lowercase, hyphenated; stored verbatim in the
// `provisioned_resources.kind` column and used as registry keys, so
// renames are migration-class changes.
const (
	KindStripeConnect    = "stripe-connect"
	KindCloudflareDomain = "cloudflare-domain"
	KindResendEmail      = "resend-email"
	KindHosting          = "hosting"
)

// Status values for a ProvisionedResource. The CHECK constraint in
// migrations/00047_provisioning.sql enumerates the same set; new
// statuses MUST land in both places.
const (
	StatusPending   = "pending"
	StatusActive    = "active"
	StatusSuspended = "suspended"
	StatusClosed    = "closed"
)

// BillingCadence values for RevenuePolicy. per-transaction lands one
// RevenueEvent per underlying transaction (Stripe charge, domain
// renewal, email send batch); monthly-aggregate lands a single
// rolled-up event at month-end.
const (
	CadencePerTransaction   = "per-transaction"
	CadenceMonthlyAggregate = "monthly-aggregate"
)

// ProvisionedResource is one third-party rail Ironflyer issued on
// behalf of a tenant + project. ExternalID is the rail-side identity
// (Stripe acct_*, Cloudflare zone id, Resend domain id, hosting tenant
// id). The pair (TenantID, ProjectID, Kind) is intentionally NOT
// unique — a single project may run two Stripe Connect accounts (US
// + EU entities, sandbox + live) and the reseller flow needs every
// row addressable independently.
type ProvisionedResource struct {
	ID         string
	TenantID   string
	ProjectID  string
	Kind       string
	ExternalID string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// RevenuePolicy is the per-kind cut Ironflyer takes on a rail. Stored
// in `revenue_policies` and applied at RevenueEvent time so historical
// events keep the rate they were charged under (changing the policy
// does NOT retroactively restate older events).
type RevenuePolicy struct {
	Kind           string
	SharePct       decimal.Decimal
	MinFeeUSD      decimal.Decimal
	BillingCadence string
}

// RevenueEvent is one Ironflyer-cut line. GrossAmountUSD is what flowed
// through the rail; IronflyerCutUSD is the slice we take after policy
// application. ExternalRef is the rail's natural dedupe key (Stripe
// charge id, application_fee id, …) so a webhook redelivery folds onto
// the same row. LedgerEntryID points back to the platform ledger row
// where the cut landed as revenue.
type RevenueEvent struct {
	ID              string
	ResourceID      string
	OccurredAt      time.Time
	GrossAmountUSD  decimal.Decimal
	IronflyerCutUSD decimal.Decimal
	ExternalRef     string
	LedgerEntryID   string
}

// Vault is the package-level facade callers use. It bundles the
// persistence Service and the ConnectorRegistry so wireup hands a
// single value to the resolver layer instead of two parallel deps.
// Mirrors how the wallet package exposes (Service, TopperRegistry)
// pair-wise — same shape, same lifecycle.
type Vault struct {
	Service    Service
	Connectors *ConnectorRegistry
	Policies   PolicyStore
}

// NewVault is the standard constructor. Either argument may be nil;
// resolvers nil-guard and return NOT_CONFIGURED so dev environments
// without rails configured still boot.
func NewVault(svc Service, registry *ConnectorRegistry, policies PolicyStore) *Vault {
	return &Vault{Service: svc, Connectors: registry, Policies: policies}
}

// Enabled is true when the Vault has at least a Service AND one
// enabled Connector — i.e. the package can do real work. Used by the
// resolver `availableConnectors` query and the wireup log line.
func (v *Vault) Enabled() bool {
	return v != nil && v.Service != nil && v.Connectors != nil && v.Connectors.AnyEnabled()
}

// PolicyStore is the per-kind RevenuePolicy lookup. In dev the default
// in-memory implementation seeds sane fractions; in prod the Postgres
// backend reads from the `revenue_policies` table so operators can
// retune without a redeploy.
type PolicyStore interface {
	// Get returns the active policy for kind. Returns ErrPolicyMissing
	// when no policy is configured — caller decides whether to skip the
	// cut (safer) or error (stricter); the Stripe Connect path errors
	// because charging Connect without a recorded cut would leave money
	// on the table silently.
	Get(kind string) (RevenuePolicy, error)
}
