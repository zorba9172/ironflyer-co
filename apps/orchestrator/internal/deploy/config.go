package deploy

import "time"

// Config is the operator-tunable knob set for the deploy plane.
// Defaults match the V22 plan ("approvals expire in 30 minutes by
// default", "Vercel API talks to https://api.vercel.com"); the
// integration agent overrides via env in cmd/orchestrator/main.go.
type Config struct {
	// DefaultApprovalTTL is how long a pending approval row stays
	// pending before Service.Expire / RequestApproval flips it to
	// `expired`. Resolver lets callers override per-request via the
	// expiresInMinutes argument.
	DefaultApprovalTTL time.Duration

	// VercelAPIBase is the Vercel REST root. Override in tests / for
	// the EU API host. Empty falls back to https://api.vercel.com.
	VercelAPIBase string

	// SecretNameVercelToken is the SecretResolver key the Vercel
	// adapter looks up per tenant for the deploy API token. Defaults
	// to "VERCEL_TOKEN" — the same name the V22 secrets plane
	// catalogs production-deploy tokens under.
	SecretNameVercelToken string

	// AutoExpireSweep is how often a background sweeper would walk
	// the deploy_approvals table to flip expired rows. Zero disables
	// the sweep; the integration agent decides whether to spawn one.
	AutoExpireSweep time.Duration

	// ManagedDomainBase is the suffix used for instant Ironflyer
	// subdomains, e.g. project-slug.ironflyer.app.
	ManagedDomainBase string

	// EdgeDNSTarget is the DNS target custom domains should point at
	// when the active provider is the built-in Ironflyer edge.
	EdgeDNSTarget string

	// DefaultDomainProvider/Registrar select the provider adapters for
	// domain routing and purchase flows when GraphQL inputs omit them.
	DefaultDomainProvider string
	DefaultRegistrar      string

	// CloudflareAccountID enables the Cloudflare Registrar API adapter
	// when present together with a CLOUDFLARE_API_TOKEN secret.
	CloudflareAccountID string

	// Domain purchase is a real financial action. Keep it explicitly
	// disabled unless an operator turns it on and caps spend.
	DomainPurchaseEnabled           bool
	MaxDomainPurchaseUSD            float64
	DomainPurchasePriceTolerancePct float64
	RequireDomainRegistrantContact  bool
}

// DefaultConfig returns Config with the V22 baseline values.
func DefaultConfig() Config {
	return Config{
		DefaultApprovalTTL:    30 * time.Minute,
		VercelAPIBase:         "https://api.vercel.com",
		SecretNameVercelToken: "VERCEL_TOKEN",
		AutoExpireSweep:       0,
		ManagedDomainBase:     "ironflyer.app",
		EdgeDNSTarget:         "edge.ironflyer.app",
		DefaultDomainProvider: "ironflyer",
		DefaultRegistrar:      "manual",
		MaxDomainPurchaseUSD:  75,

		// A 10% tolerance covers registrar rounding/tax drift while
		// still preventing surprise charges.
		DomainPurchasePriceTolerancePct: 10,
	}
}
