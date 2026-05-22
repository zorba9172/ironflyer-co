// Package ratelimit is a per-key token-bucket limiter the orchestrator uses
// to keep a single user (or a single IP, on unauthenticated routes) from
// burning provider credits.
//
// The bucket fills at `RatePerSecond` tokens/sec up to `Burst` tokens.
// Allow() returns true when there's a token to spend, otherwise false plus
// the time the caller should wait before retrying. Per-key state is kept
// in a sync.Map and pruned on access — fine for the cardinality we expect
// (per-user, not per-request).
package ratelimit

import (
	"sync"
	"time"
)

// Limiter is a bucket factory keyed by an arbitrary string (user ID, IP).
type Limiter struct {
	RatePerSecond float64
	Burst         float64
	now           func() time.Time

	buckets sync.Map // key string → *bucket
}

// New returns a Limiter that lets each key accrue `burst` tokens at
// `ratePerSecond`. ratePerSecond=10, burst=20 → 20-token cap, refilled at
// 10/s — bursts to 20, sustains at 10.
func New(ratePerSecond, burst float64) *Limiter {
	return &Limiter{
		RatePerSecond: ratePerSecond,
		Burst:         burst,
		now:           time.Now,
	}
}

type bucket struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

// Allow asks the limiter whether `key` can spend 1 token now. Returns
// (true, 0) on success, (false, wait) when the bucket is empty.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	return l.AllowN(key, 1)
}

// AllowN is the variant that spends N tokens at once — useful for endpoints
// where one call costs more (e.g. a brainstorm spends 3 chat units).
func (l *Limiter) AllowN(key string, n float64) (bool, time.Duration) {
	v, _ := l.buckets.LoadOrStore(key, &bucket{tokens: l.Burst, last: l.now()})
	b := v.(*bucket)
	b.mu.Lock()
	defer b.mu.Unlock()

	now := l.now()
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.RatePerSecond
	if b.tokens > l.Burst {
		b.tokens = l.Burst
	}
	b.last = now

	if b.tokens >= n {
		b.tokens -= n
		return true, 0
	}
	missing := n - b.tokens
	wait := time.Duration(missing / l.RatePerSecond * float64(time.Second))
	return false, wait
}

// Reset clears the bucket for `key` — handy for tests and for manual
// unblocks from an admin endpoint.
func (l *Limiter) Reset(key string) { l.buckets.Delete(key) }
