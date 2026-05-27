// Package gqlhardening collects the production hardening pieces the
// orchestrator's GraphQL surface needs in front of every operation:
// depth + complexity caps, persisted query allowlist, introspection
// gate, CSRF guard for browser sessions, WS origin allowlist, rate
// limiter wired into the abuse engine, and a redacted error presenter.
//
// Wiring contract (integration agent assembles this in api.go — DO NOT
// modify here):
//
//   - main builds an abuse.Engine + a persisted.Store.
//   - api.go mounts middleware on POST /graphql in this order:
//     CSRF → PersistedQueries → Depth → Complexity → Introspection
//     → RateLimiter → handler.
//   - The depth, complexity, and introspection guards register as
//     gqlgen Extensions on the handler; CSRF, persisted-query, and
//     rate-limit run as chi http.Handler middleware in front of it.
//
// Env vars (loaded by Load):
//
//	IRONFLYER_GQL_MAX_DEPTH         — default 10
//	IRONFLYER_GQL_COMPLEXITY_LIMIT  — default 1000
//	IRONFLYER_GQL_PROD              — default false; true enables
//	                                  persisted-query allowlist + introspection gate + redaction
//	IRONFLYER_GQL_WS_ORIGINS        — comma-separated allowlist
//	IRONFLYER_GQL_CSRF_HEADER       — default "X-Ironflyer-CSRF"
//	IRONFLYER_GQL_CSRF_COOKIE       — default "ironflyer_csrf"
//	IRONFLYER_GQL_RL_BASE_BURST     — default 60 (per-second burst per tenant+op)
package gqlhardening

import (
	"ironflyer/core/orchestrator/internal/pkg/env"
)

// Config bundles the env-driven knobs for the hardening layer. Pass
// the zero-value Config to LoadDefaults to get production-safe values
// without touching env at the call-site.
type Config struct {
	MaxDepth        int
	ComplexityLimit int
	ProdMode        bool

	// APQLocked, when true, locks the Automatic Persisted Query cache
	// to the startup-seeded registry — clients may only execute
	// pre-registered query hashes. Raw queries and unknown hashes
	// return PERSISTED_QUERY_NOT_REGISTERED. See DEPLOY.md §5
	// (GRAPHQL_APQ_LOCKED).
	APQLocked bool

	// APQRegistryDir is the on-disk source for the locked-mode
	// persisted-query registry. The directory is walked at startup and
	// every .graphql / .gql file is parsed; each named operation is
	// hashed (sha256 of the canonical query text) and pre-loaded into
	// the APQ cache. Empty means "use the wired-in client gql dir"
	// (clients/web/src/lib/gql/operations).
	APQRegistryDir string

	WSOrigins  []string
	CSRFHeader string
	CSRFCookie string
	// CSRFCookieDomain, when non-empty, scopes the CSRF cookie so it
	// is visible to all subdomains under a registrable domain (e.g.
	// ".ironflyer.ai" lets app.* read the cookie set by api.* during
	// the double-submit handshake). Leave empty to keep the cookie
	// host-only on the origin that set it.
	CSRFCookieDomain string

	// Base rate limit knobs. The composed key is
	// "<tenant>:<operation>:<abuse_tier>" and the abuse-tier multiplier
	// scales BaseBurst per the abuse.Tier multiplier table.
	BaseRatePerSecond float64
	BaseBurst         float64
}

// Defaults returns the production-safe Config without reading env.
// MaxDepth tracks the documented default in DEPLOY.md §5 (15) so the
// orchestrator's wired-in cap matches what operators read from the
// runbook. ComplexityLimit (1000) likewise mirrors DEPLOY.md §5.
func Defaults() Config {
	return Config{
		MaxDepth:          15,
		ComplexityLimit:   1000,
		ProdMode:          false,
		WSOrigins:         nil,
		CSRFHeader:        "X-Ironflyer-CSRF",
		CSRFCookie:        "ironflyer_csrf",
		BaseRatePerSecond: 30,
		BaseBurst:         60,
	}
}

// Load reads IRONFLYER_GQL_* env vars and overlays them onto Defaults.
//
// The DEPLOY.md §5 knobs (GRAPHQL_DEPTH_LIMIT, GRAPHQL_COMPLEXITY_LIMIT,
// GRAPHQL_APQ_LOCKED, GRAPHQL_APQ_REGISTRY_DIR) take precedence over
// the legacy IRONFLYER_GQL_* names — operators following the runbook
// see the documented surface, the legacy names stay live for older
// deployments.
func Load() Config {
	c := Defaults()
	if v := env.Int("IRONFLYER_GQL_MAX_DEPTH", 0); v > 0 {
		c.MaxDepth = v
	}
	if v := env.Int("IRONFLYER_GQL_COMPLEXITY_LIMIT", 0); v > 0 {
		c.ComplexityLimit = v
	}
	if v := env.Int("GRAPHQL_DEPTH_LIMIT", 0); v > 0 {
		c.MaxDepth = v
	}
	if v := env.Int("GRAPHQL_COMPLEXITY_LIMIT", 0); v > 0 {
		c.ComplexityLimit = v
	}
	if env.Bool("GRAPHQL_APQ_LOCKED", false) {
		c.APQLocked = true
	}
	if v := env.String("GRAPHQL_APQ_REGISTRY_DIR", ""); v != "" {
		c.APQRegistryDir = v
	}
	if env.Bool("IRONFLYER_GQL_PROD", false) {
		c.ProdMode = true
	}
	if origins := env.StringCSV("IRONFLYER_GQL_WS_ORIGINS"); len(origins) > 0 {
		c.WSOrigins = origins
	}
	if v := env.String("IRONFLYER_GQL_CSRF_HEADER", ""); v != "" {
		c.CSRFHeader = v
	}
	if v := env.String("IRONFLYER_GQL_CSRF_COOKIE", ""); v != "" {
		c.CSRFCookie = v
	}
	if v := env.String("IRONFLYER_GQL_CSRF_DOMAIN", ""); v != "" {
		c.CSRFCookieDomain = v
	}
	if v := env.Float64("IRONFLYER_GQL_RL_BASE_RPS", 0); v > 0 {
		c.BaseRatePerSecond = v
	}
	if v := env.Float64("IRONFLYER_GQL_RL_BASE_BURST", 0); v > 0 {
		c.BaseBurst = v
	}
	return c
}
