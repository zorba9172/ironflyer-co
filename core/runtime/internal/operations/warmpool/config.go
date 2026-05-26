// Package warmpool maintains warm inventory (images / prebooted
// sandboxes / paused microVMs) so that paid arrivals do not pay the
// full cold-start tax. Inventory is bounded by the recent paid arrival
// rate per ARCHITECTURE_RUNTIME_SCALE.md "Warm Pools" and drained when
// the paid queue empties for a cooldown window.
package warmpool

import (
	"os"
	"strconv"
	"time"
)

// Kind tags the inventory type so a single Pool can hold image,
// sandbox, and microvm leases without separate plumbing.
type Kind string

const (
	KindImage    Kind = "image"
	KindSandbox  Kind = "sandbox"
	KindMicroVM  Kind = "microvm"
	KindWorkspace Kind = "workspace_hot"
)

// Config wires the Pool. The defaults match the spec's "Warm-pool
// policy" section: a small floor derived from paid arrivals, capped
// so a misconfiguration cannot bleed runtime overhead.
type Config struct {
	// TargetSLAColdStartSeconds is the cold-start latency target
	// used in the floor formula.
	TargetSLAColdStartSeconds int
	// MaxFloor caps the floor regardless of arrival rate. Read
	// from IRONFLYER_WARMPOOL_MAX_FLOOR; default 20.
	MaxFloor int
	// MinFloor is the static minimum (default 1, can be 0 for full
	// scale-to-zero when no paid traffic).
	MinFloor int
	// CooldownWindow is how long the queue must remain empty before
	// drain begins.
	CooldownWindow time.Duration
	// LeaseTTL is the upper bound on how long a leased slot may sit
	// before being recycled.
	LeaseTTL time.Duration
}

// DefaultConfig pulls IRONFLYER_WARMPOOL_MAX_FLOOR from env (default
// 20) and provides sensible defaults for the rest.
func DefaultConfig() Config {
	max := 20
	if raw := os.Getenv("IRONFLYER_WARMPOOL_MAX_FLOOR"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			max = v
		}
	}
	return Config{
		TargetSLAColdStartSeconds: 5,
		MaxFloor:                  max,
		MinFloor:                  1,
		CooldownWindow:            5 * time.Minute,
		LeaseTTL:                  15 * time.Minute,
	}
}
