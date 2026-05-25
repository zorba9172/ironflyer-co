// Package abuse implements the orchestrator's abuse scoring engine.
//
// The engine turns a stream of typed "signals" (failed auth, policy
// deny, complexity reject, wallet abuse, etc.) into a clamped 0..100
// score and a four-band tier (normal / elevated / restricted /
// blocked). Tier is the policy input rate limiters, ProfitGuard, and
// the runtime command broker all branch on.
//
// The engine is intentionally simple — sum signal weights over a
// sliding window, clamp, map to tier — because the trust plane has to
// stay explainable. Operators can read the abuse_signals table and
// hand-derive the score; an ML refresher is welcome later but the
// rule-based engine remains the source of truth for policy decisions.
//
// Wiring contract:
//
//   - main.go constructs the Store (Memory or Postgres) once and wraps
//     it in NewEngine(cfg, store). The Engine is then handed to the
//     gqlhardening rate limiter, the runtime command broker, and the
//     policy bundle loader — each of which calls Score() or
//     RecordSignal() on it.
package abuse

import (
	"context"
	"sync"
	"time"
)

// Engine is the public abuse scoring surface the rest of the
// orchestrator consumes. All methods are safe for concurrent use.
type Engine interface {
	// Score returns the current (score, tier) for a (tenant,user)
	// pair. The result is cached for cfg.CacheTTL; signals recorded
	// inside the TTL still persist, only the derived score reads lag
	// — which is the right tradeoff for the GraphQL hot path.
	Score(ctx context.Context, tenantID, userID string) (int, Tier, error)

	// RecordSignal appends a single signal event. The score cache for
	// the (tenant,user) pair is invalidated so the next Score call
	// reflects the new event.
	RecordSignal(ctx context.Context, tenantID, userID string, s SignalType, weight int, context map[string]any) error

	// SetScore forces a score (and derived tier) for a (tenant,user)
	// pair, recording the reason in the audit row. Operator override
	// — bypasses the signal window entirely. Use sparingly.
	SetScore(ctx context.Context, tenantID, userID string, score int, reason string) error

	// Recent returns up to `limit` recent signals across all users in
	// the tenant. Powers the trust dashboard.
	Recent(ctx context.Context, tenantID string, limit int) ([]ScoredSignal, error)
}

// engine is the concrete Engine implementation. It owns the Store +
// Config + a tiny in-process score cache keyed by (tenant,user). The
// cache is not authoritative — invalidated on RecordSignal /
// SetScore — but it keeps Score() off the hot path for repeated reads
// inside a single GraphQL request.
type engine struct {
	cfg   Config
	store Store

	mu    sync.Mutex
	cache map[string]cachedScore
}

type cachedScore struct {
	score   int
	tier    Tier
	expires time.Time
}

// NewEngine wires a Store + Config into a concrete Engine.
func NewEngine(cfg Config, store Store) Engine {
	if cfg.Window <= 0 {
		cfg.Window = 24 * time.Hour
	}
	if cfg.CacheTTL < 0 {
		cfg.CacheTTL = 0
	}
	return &engine{cfg: cfg, store: store, cache: map[string]cachedScore{}}
}

func cacheKey(tenant, user string) string { return tenant + "/" + user }

func (e *engine) Score(ctx context.Context, tenantID, userID string) (int, Tier, error) {
	if e == nil || e.store == nil {
		return e.applyFloor(0), TierFromScore(e.applyFloor(0)), nil
	}
	now := time.Now()
	key := cacheKey(tenantID, userID)
	e.mu.Lock()
	if entry, ok := e.cache[key]; ok && entry.expires.After(now) {
		e.mu.Unlock()
		return entry.score, entry.tier, nil
	}
	e.mu.Unlock()

	// Try the stored score first — operator overrides + the last
	// computed score live there. If absent or stale relative to the
	// signal window, recompute from raw events.
	stored, storedTier, found, err := e.store.GetScore(ctx, tenantID, userID)
	if err != nil {
		return e.applyFloor(0), TierFromScore(e.applyFloor(0)), err
	}
	score := stored
	tier := storedTier
	if !found {
		// First read for this (tenant,user) — compute from the window.
		recomputed, breakdown, sumErr := e.store.SumWeights(ctx, tenantID, userID, now.Add(-e.cfg.Window))
		if sumErr != nil {
			return e.applyFloor(0), TierFromScore(e.applyFloor(0)), sumErr
		}
		score = ClampScore(recomputed)
		tier = TierFromScore(e.applyFloor(score))
		_ = e.store.UpsertScore(ctx, tenantID, userID, score, tier, breakdown, "")
	}
	score = e.applyFloor(score)
	tier = TierFromScore(score)
	e.cacheSet(key, score, tier, now)
	return score, tier, nil
}

func (e *engine) RecordSignal(ctx context.Context, tenantID, userID string, st SignalType, weight int, signalCtx map[string]any) error {
	if e == nil || e.store == nil {
		return ErrStoreUnavailable
	}
	if weight == 0 {
		weight = WeightFor(st)
	}
	if weight < -100 || weight > 100 {
		return ErrInvalidWeight
	}
	if err := e.store.RecordSignal(ctx, tenantID, userID, st, weight, signalCtx); err != nil {
		return err
	}
	observeSignal(st)

	// Recompute the score immediately so policy reads after this call
	// observe the new tier. This is the cheap path — one window SUM
	// per signal record. If profiling shows it as a hot spot we can
	// defer to a background goroutine.
	prev, _, _, _ := e.store.GetScore(ctx, tenantID, userID)
	prevTier := TierFromScore(e.applyFloor(prev))

	now := time.Now()
	total, breakdown, err := e.store.SumWeights(ctx, tenantID, userID, now.Add(-e.cfg.Window))
	if err != nil {
		return err
	}
	newScore := ClampScore(total)
	newTier := TierFromScore(e.applyFloor(newScore))
	if err := e.store.UpsertScore(ctx, tenantID, userID, newScore, newTier, breakdown, ""); err != nil {
		return err
	}
	observeTransition(prevTier, newTier)
	e.cacheInvalidate(cacheKey(tenantID, userID))
	return nil
}

func (e *engine) SetScore(ctx context.Context, tenantID, userID string, score int, reason string) error {
	if e == nil || e.store == nil {
		return ErrStoreUnavailable
	}
	clamped := ClampScore(score)
	tier := TierFromScore(clamped)
	prev, _, _, _ := e.store.GetScore(ctx, tenantID, userID)
	prevTier := TierFromScore(e.applyFloor(prev))
	if err := e.store.UpsertScore(ctx, tenantID, userID, clamped, tier, nil, reason); err != nil {
		return err
	}
	observeTransition(prevTier, tier)
	e.cacheInvalidate(cacheKey(tenantID, userID))
	return nil
}

func (e *engine) Recent(ctx context.Context, tenantID string, limit int) ([]ScoredSignal, error) {
	if e == nil || e.store == nil {
		return nil, ErrStoreUnavailable
	}
	return e.store.Recent(ctx, tenantID, limit)
}

func (e *engine) applyFloor(score int) int {
	if e.cfg.HardFloor > score {
		return ClampScore(e.cfg.HardFloor)
	}
	return ClampScore(score)
}

func (e *engine) cacheSet(key string, score int, tier Tier, now time.Time) {
	if e.cfg.CacheTTL <= 0 {
		return
	}
	e.mu.Lock()
	e.cache[key] = cachedScore{score: score, tier: tier, expires: now.Add(e.cfg.CacheTTL)}
	e.mu.Unlock()
}

func (e *engine) cacheInvalidate(key string) {
	e.mu.Lock()
	delete(e.cache, key)
	e.mu.Unlock()
}

// NoopEngine is a safe zero-value Engine returned in dev mode when no
// store is configured. It reports TierNormal for every read and
// silently drops signals — keeps callers from having to nil-check the
// engine pointer everywhere.
type NoopEngine struct{}

func (NoopEngine) Score(context.Context, string, string) (int, Tier, error) {
	return 0, TierNormal, nil
}
func (NoopEngine) RecordSignal(context.Context, string, string, SignalType, int, map[string]any) error {
	return nil
}
func (NoopEngine) SetScore(context.Context, string, string, int, string) error { return nil }
func (NoopEngine) Recent(context.Context, string, int) ([]ScoredSignal, error) {
	return nil, nil
}
