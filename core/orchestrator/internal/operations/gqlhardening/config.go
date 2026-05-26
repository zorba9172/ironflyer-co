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

	WSOrigins  []string
	CSRFHeader string
	CSRFCookie string

	// Base rate limit knobs. The composed key is
	// "<tenant>:<operation>:<abuse_tier>" and the abuse-tier multiplier
	// scales BaseBurst per the abuse.Tier multiplier table.
	BaseRatePerSecond float64
	BaseBurst         float64
}

// Defaults returns the production-safe Config without reading env.
func Defaults() Config {
	return Config{
		MaxDepth:          10,
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
func Load() Config {
	c := Defaults()
	if v := env.Int("IRONFLYER_GQL_MAX_DEPTH", 0); v > 0 {
		c.MaxDepth = v
	}
	if v := env.Int("IRONFLYER_GQL_COMPLEXITY_LIMIT", 0); v > 0 {
		c.ComplexityLimit = v
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
	if v := env.Float64("IRONFLYER_GQL_RL_BASE_RPS", 0); v > 0 {
		c.BaseRatePerSecond = v
	}
	if v := env.Float64("IRONFLYER_GQL_RL_BASE_BURST", 0); v > 0 {
		c.BaseBurst = v
	}
	return c
}
