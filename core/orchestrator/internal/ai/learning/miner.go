package learning

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Miner runs periodically (default every hour) and converts the
// recent OutcomeEvent stream into PatternObservations. It is a
// deliberately small Bayesian aggregator — counts + win rates +
// simple updates. The strategy adapter consumes the published
// observations and rewrites bandit / blueprint / forecast knobs.
//
// Miner is a fail-soft daemon: a ClickHouse outage or a malformed
// fact row logs a warning and lets the next tick recover. It never
// blocks producers.
type Miner struct {
	store     Store
	publisher *Publisher
	interval  time.Duration
	log       zerolog.Logger

	tenantsMu sync.Mutex
	tenants   map[string]struct{}
}

// NewMiner constructs a miner that polls store every interval. A zero
// interval defaults to 1h.
func NewMiner(store Store, pub *Publisher, interval time.Duration, log zerolog.Logger) *Miner {
	if interval <= 0 {
		interval = time.Hour
	}
	return &Miner{
		store:     store,
		publisher: pub,
		interval:  interval,
		log:       log,
		tenants:   make(map[string]struct{}),
	}
}

// TrackTenant adds a tenant id to the rotation. The platform-wide
// scan (tenantID == "") always runs regardless.
func (m *Miner) TrackTenant(tenantID string) {
	if m == nil || tenantID == "" {
		return
	}
	m.tenantsMu.Lock()
	defer m.tenantsMu.Unlock()
	m.tenants[tenantID] = struct{}{}
}

// Run blocks until ctx is cancelled. Each tick performs one mining
// pass per tracked tenant + the platform-wide rollup. Errors are
// logged and the loop continues.
func (m *Miner) Run(ctx context.Context) error {
	if m == nil {
		return errors.New("learning: miner not configured")
	}
	m.log.Info().Dur("interval", m.interval).Msg("learning miner started")
	// Do an immediate pass at boot so the dashboard isn't empty for
	// the full interval window.
	m.tickOnce(ctx)
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			m.tickOnce(ctx)
		}
	}
}

func (m *Miner) tickOnce(ctx context.Context) {
	if m.store == nil {
		return
	}
	// Platform-wide scan first; per-tenant follow.
	m.mineTenant(ctx, "")
	m.tenantsMu.Lock()
	tenants := make([]string, 0, len(m.tenants))
	for t := range m.tenants {
		tenants = append(tenants, t)
	}
	m.tenantsMu.Unlock()
	for _, t := range tenants {
		m.mineTenant(ctx, t)
	}
}

func (m *Miner) mineTenant(ctx context.Context, tenantID string) {
	snap, err := m.store.Snapshot(ctx, tenantID)
	if err != nil {
		m.log.Warn().Err(err).Str("tenant", tenantID).Msg("learning miner: snapshot failed")
		return
	}
	// Gate failure rates → PatternObservation per gate, severity by
	// failure rate. Confidence = monotonically increasing function of
	// observed samples (clamped to 1).
	for gate, rate := range snap.GateFailureRateLast7d {
		conf := 0.5
		if rate >= 0.5 {
			conf = 0.85
		}
		direction := "down"
		if rate < 0.1 {
			direction = "neutral"
		}
		m.publishObs(ctx, PatternObservation{
			TenantID:   tenantID,
			Pattern:    "gate_failure_rate",
			Target:     gate,
			Direction:  direction,
			Confidence: conf,
			Evidence: map[string]any{
				"failure_rate": rate,
				"window":       "7d",
			},
		})
	}
	// Blueprint success rates → up/down direction for selection weight.
	for bp, success := range snap.BlueprintSuccessRate {
		direction := "up"
		if success < 0.5 {
			direction = "down"
		}
		m.publishObs(ctx, PatternObservation{
			TenantID:   tenantID,
			Pattern:    "blueprint_success_rate",
			Target:     bp,
			Direction:  direction,
			Confidence: 0.6,
			Evidence: map[string]any{
				"success_rate": success,
			},
		})
	}
	if snap.RepairRecipeHitsLast7d > 0 {
		m.publishObs(ctx, PatternObservation{
			TenantID:   tenantID,
			Pattern:    "repair_recipe_hits",
			Target:     "global",
			Direction:  "up",
			Confidence: 0.75,
			Evidence: map[string]any{
				"hits": snap.RepairRecipeHitsLast7d,
			},
		})
	}
}

func (m *Miner) publishObs(ctx context.Context, obs PatternObservation) {
	if m.publisher == nil {
		return
	}
	m.publisher.PublishPattern(ctx, obs)
}
