// This file adds a Redis-backed rate limiter that mirrors the in-memory
// *Limiter surface so HTTP handlers and middlewares can swap one for the
// other without code changes. When the Redis client is nil (Redis is
// disabled at startup), `Wrap` returns the existing in-memory limiter
// untouched — the orchestrator stays single-source-of-truth either way.

package ratelimit

import (
	"context"
	"time"

	"ironflyer/apps/orchestrator/internal/redisbus"
)

// Allower is the minimum surface both Limiter and RedisLimiter
// implement. Handlers that want to be backend-agnostic should take an
// Allower instead of *Limiter.
type Allower interface {
	Allow(key string) (bool, time.Duration)
	AllowN(key string, n float64) (bool, time.Duration)
	Reset(key string)
}

// RedisLimiter is a fixed-window rate limiter backed by Redis. It is
// drop-in compatible with the in-memory *Limiter: same method set,
// same return shape. The Burst field maps to the per-window cap;
// RatePerSecond × Window seconds is approximately the sustained rate
// the underlying fixed window enforces.
type RedisLimiter struct {
	Client *redisbus.Client
	// Prefix keeps keys from a single Redis namespaced per concern
	// (e.g. "rl:user:" vs "rl:ip:"). The full Redis key is Prefix+key.
	Prefix string
	// Limit is the total tokens allowed in one Window.
	Limit int
	// Window is the size of the fixed window.
	Window time.Duration
	// Fallback is the in-process limiter used when the Redis call
	// errors out — keeps the orchestrator usable when Redis goes away
	// mid-flight.
	Fallback *Limiter
}

// NewRedisLimiter constructs a Redis-backed limiter. ratePerSecond +
// burst preserve the call-site shape used by handlers that previously
// constructed a *Limiter; window defaults to 1s when zero so callers
// that only know about burst behave like the in-memory bucket.
func NewRedisLimiter(client *redisbus.Client, prefix string, ratePerSecond, burst float64) *RedisLimiter {
	window := time.Second
	limit := int(burst)
	if limit <= 0 {
		limit = int(ratePerSecond)
	}
	if limit <= 0 {
		limit = 1
	}
	return &RedisLimiter{
		Client:   client,
		Prefix:   prefix,
		Limit:    limit,
		Window:   window,
		Fallback: New(ratePerSecond, burst),
	}
}

// Allow asks the limiter whether `key` can spend 1 token now. Returns
// (true, 0) on success, (false, wait) when the window is exhausted.
func (r *RedisLimiter) Allow(key string) (bool, time.Duration) {
	return r.AllowN(key, 1)
}

// AllowN spends N tokens. Redis INCR is integer-only so fractional N
// is rounded up — handlers that pass non-integer weights should round
// at the call site if they want precise accounting.
func (r *RedisLimiter) AllowN(key string, n float64) (bool, time.Duration) {
	if r == nil || r.Client == nil || r.Client.Client == nil {
		return r.Fallback.AllowN(key, n)
	}
	cost := int(n)
	if float64(cost) < n {
		cost++
	}
	if cost <= 0 {
		cost = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	fullKey := r.Prefix + key
	// Spend `cost` calls in one shot by allowing then nudging the count
	// up to cost-1 more — keeps semantics correct for callers like the
	// brainstorm endpoint that spend more than one unit per request.
	allowed, _, err := r.Client.AllowRate(ctx, fullKey, r.Limit, r.Window)
	if err != nil {
		return r.Fallback.AllowN(key, n)
	}
	if !allowed {
		return false, r.Window
	}
	for i := 1; i < cost; i++ {
		ok, _, err := r.Client.AllowRate(ctx, fullKey, r.Limit, r.Window)
		if err != nil {
			return r.Fallback.AllowN(key, n)
		}
		if !ok {
			return false, r.Window
		}
	}
	return true, 0
}

// Reset clears the Redis bucket for `key` (best-effort) and the local
// fallback. Useful for admin unblocks + tests.
func (r *RedisLimiter) Reset(key string) {
	if r == nil {
		return
	}
	if r.Fallback != nil {
		r.Fallback.Reset(key)
	}
	if r.Client == nil || r.Client.Client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_ = r.Client.Client.Del(ctx, r.Prefix+key).Err()
}

// Wrap returns a backend-aware Allower. When client is nil the existing
// in-memory limiter is returned untouched so callers keep the same
// behaviour they had before Redis was wired in. When client is non-nil
// the in-memory limiter is moved into the Fallback slot.
func Wrap(client *redisbus.Client, prefix string, inMemory *Limiter) Allower {
	if client == nil || client.Client == nil || inMemory == nil {
		return inMemory
	}
	return &RedisLimiter{
		Client:   client,
		Prefix:   prefix,
		Limit:    int(inMemory.Burst),
		Window:   time.Second,
		Fallback: inMemory,
	}
}
