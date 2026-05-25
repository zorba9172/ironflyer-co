package abuse

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config controls the abuse engine's scoring window + cache behaviour.
// All fields are overridable from IRONFLYER_ABUSE_* env vars; the
// zero-value config picks production-safe defaults.
type Config struct {
	// Window is how far back the engine sums signal weights for the
	// score. Defaults to 24h. Longer windows make scores stickier and
	// slower to recover; shorter windows make the system more reactive
	// but easier to game with short pauses.
	Window time.Duration

	// CacheTTL is how long an Engine.Score result is cached per
	// (tenant,user). Defaults to 30s so the GraphQL hot path stays
	// cheap; signals recorded during the TTL are still persisted
	// immediately, only the derived score reads lag.
	CacheTTL time.Duration

	// HardFloor lets operators pin a minimum score across the tenant
	// in incident mode (e.g. set to 60 to keep everyone restricted
	// while triaging an attack).
	HardFloor int
}

// LoadConfig reads IRONFLYER_ABUSE_* env vars and falls back to
// production-safe defaults. Callers should not retain the returned
// Config across reloads — env changes require a restart by design.
func LoadConfig() Config {
	c := Config{
		Window:    24 * time.Hour,
		CacheTTL:  30 * time.Second,
		HardFloor: 0,
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_ABUSE_WINDOW")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			c.Window = d
		}
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_ABUSE_CACHE_TTL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			c.CacheTTL = d
		}
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_ABUSE_HARD_FLOOR")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 100 {
			c.HardFloor = n
		}
	}
	return c
}
