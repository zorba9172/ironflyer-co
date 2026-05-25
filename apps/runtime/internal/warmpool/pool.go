package warmpool

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// LeaseID identifies a single checked-out warm slot.
type LeaseID string

// ErrNoSlot is returned by Lease when no warm slot is available; the
// caller should fall through to cold start.
var ErrNoSlot = errors.New("warmpool: no warm slot available")

// Pool is the warm-inventory API the allocator dials into.
type Pool interface {
	// Lease checks out a warm slot of the given runtimeClass. Returns
	// ErrNoSlot if none is available; the caller decides cold start.
	Lease(ctx context.Context, runtimeClass string) (LeaseID, error)
	// Return marks a leased slot reusable (if still within TTL).
	Return(ctx context.Context, leaseID LeaseID) error
	// Drain destroys idle warm slots when the cooldown window has
	// elapsed with no paid demand. Returns the count drained.
	Drain(ctx context.Context, cooldownWindow time.Duration) (int, error)
	// Floor computes the desired floor for the given paid arrival
	// rate; callers feed the difference between Floor() and current
	// pool size into a warmer task.
	Floor(ctx context.Context, paidArrivalRatePerMin float64) int
}

// slot is one warm inventory item.
type slot struct {
	id           LeaseID
	runtimeClass string
	createdAt    time.Time
	leasedAt     time.Time
	leased       bool
}

// MemoryPool is a single-process Pool used by the runtime allocator.
// Production deployments back this with a Redis-coordinated pool; the
// interface stays the same.
type MemoryPool struct {
	cfg     Config
	logger  zerolog.Logger
	metrics *Metrics

	mu               sync.Mutex
	slots            map[LeaseID]*slot
	lastPaidDemandAt time.Time
}

// New builds an in-memory Pool with no pre-warmed slots; a Warmer
// goroutine (out of this file's scope) is expected to call
// Provision() until the floor is met.
func New(cfg Config, logger zerolog.Logger) *MemoryPool {
	return &MemoryPool{
		cfg:              cfg,
		logger:           logger.With().Str("component", "warmpool").Logger(),
		metrics:          &Metrics{},
		slots:            make(map[LeaseID]*slot),
		lastPaidDemandAt: time.Now(),
	}
}

// MetricsView returns the metrics struct.
func (p *MemoryPool) MetricsView() *Metrics { return p.metrics }

// Provision adds a fresh warm slot to the pool. Called by the warmer
// to bring pool size up to the desired floor.
func (p *MemoryPool) Provision(runtimeClass string) LeaseID {
	id := LeaseID("warm-" + uuid.NewString()[:8])
	p.mu.Lock()
	defer p.mu.Unlock()
	p.slots[id] = &slot{
		id:           id,
		runtimeClass: runtimeClass,
		createdAt:    time.Now(),
	}
	return id
}

// Size returns the current pool size (leased + idle).
func (p *MemoryPool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.slots)
}

// IdleSize returns the number of slots not currently leased.
func (p *MemoryPool) IdleSize() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := 0
	for _, s := range p.slots {
		if !s.leased {
			c++
		}
	}
	return c
}

// Lease implements Pool.
func (p *MemoryPool) Lease(_ context.Context, runtimeClass string) (LeaseID, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastPaidDemandAt = time.Now()
	for _, s := range p.slots {
		if s.leased {
			continue
		}
		if s.runtimeClass != runtimeClass {
			continue
		}
		s.leased = true
		s.leasedAt = time.Now()
		p.metrics.Leases.Add(1)
		p.metrics.LeaseHits.Add(1)
		p.metrics.ActiveLeases.Add(1)
		return s.id, nil
	}
	p.metrics.LeaseMisses.Add(1)
	return "", ErrNoSlot
}

// Return implements Pool. Slots beyond their LeaseTTL are dropped on
// return so a stale lease cannot resurrect a slot the warmer already
// recycled.
func (p *MemoryPool) Return(_ context.Context, id LeaseID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.slots[id]
	if !ok {
		return nil
	}
	p.metrics.Returns.Add(1)
	p.metrics.ActiveLeases.Add(-1)
	if p.cfg.LeaseTTL > 0 && time.Since(s.createdAt) > p.cfg.LeaseTTL {
		delete(p.slots, id)
		return nil
	}
	s.leased = false
	s.leasedAt = time.Time{}
	return nil
}

// Drain implements Pool. Removes idle slots when the queue has been
// empty for cooldownWindow. The caller (the warmer loop) is expected
// to call NotePaidQueueDepth() so the pool knows when "empty" began.
func (p *MemoryPool) Drain(_ context.Context, cooldownWindow time.Duration) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cooldownWindow <= 0 {
		cooldownWindow = p.cfg.CooldownWindow
	}
	if time.Since(p.lastPaidDemandAt) < cooldownWindow {
		return 0, nil
	}
	drained := 0
	for id, s := range p.slots {
		if s.leased {
			continue
		}
		delete(p.slots, id)
		drained++
	}
	p.metrics.Drained.Add(int64(drained))
	return drained, nil
}

// NotePaidQueueDepth records the most recent paid-queue depth so the
// drain loop knows when the cooldown clock should reset. depth > 0
// counts as paid demand and resets the timer.
func (p *MemoryPool) NotePaidQueueDepth(depth int) {
	if depth <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastPaidDemandAt = time.Now()
}

// Floor implements Pool.
func (p *MemoryPool) Floor(_ context.Context, paidArrivalRatePerMin float64) int {
	f := Floor(p.cfg, paidArrivalRatePerMin)
	p.metrics.ActiveFloor.Store(int64(f))
	return f
}

var _ Pool = (*MemoryPool)(nil)
