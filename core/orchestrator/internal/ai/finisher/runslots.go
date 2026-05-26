package finisher

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
)

// ErrRunThrottled is returned by Engine.Run when the global or per-user
// concurrency cap is saturated and a slot can't be acquired before the
// admission deadline. HTTP handlers map this to 429 Too Many Requests.
var ErrRunThrottled = errors.New("finisher: run throttled")

// runSlots gates how many Run() calls execute concurrently inside a
// single orchestrator process. The provider router already has per-arm
// circuit breakers and connection-pool ceilings, but those guard
// outbound traffic; runSlots guards the pod itself — CPU, memory, and
// the worst-case finisher loop fan-out (planner+architect+coder per
// run, plus N gates that each may shell into the workspace runtime).
//
// Two layers:
//
//  1. global: a Weighted semaphore caps total concurrent Run() calls.
//     Default 8 — tuned for a 2-vCPU pod doing real generative work; a
//     larger pod can opt up via IRONFLYER_MAX_CONCURRENT_RUNS.
//
//  2. per-owner: a map of in-flight counters per OwnerID prevents one
//     user from saturating the global pool. Default 2 in-flight per
//     user — enough for a workspace + a one-off rerun without locking
//     the queue.
//
// Both layers wait up to admissionDeadline before failing fast. The
// short wait window matters: an HTTP/GraphQL caller is holding a
// connection; we'd rather 429 immediately than have the request hang
// for minutes behind a slow neighbour.
type runSlots struct {
	global     *semaphore.Weighted
	perOwner   map[string]int
	maxPerUser int
	mu         sync.Mutex
}

// admissionDeadline is the max time Run() blocks waiting for a slot
// before returning ErrRunThrottled. Short on purpose — callers should
// see a fast 429 rather than a long hang.
const admissionDeadline = 2 * time.Second

// newRunSlots constructs a runSlots with the supplied caps. Zero or
// negative values fall back to safe defaults (8 global, 2 per user).
func newRunSlots(maxConcurrent, maxPerUser int) *runSlots {
	if maxConcurrent <= 0 {
		maxConcurrent = 8
	}
	if maxPerUser <= 0 {
		maxPerUser = 2
	}
	return &runSlots{
		global:     semaphore.NewWeighted(int64(maxConcurrent)),
		perOwner:   make(map[string]int, 64),
		maxPerUser: maxPerUser,
	}
}

// acquire takes a slot for ownerID. Blocks up to admissionDeadline for
// the global semaphore; rejects immediately when the per-owner cap is
// already saturated. ownerID == "" is treated as a single shared
// bucket (anonymous traffic shouldn't be able to starve named users).
func (s *runSlots) acquire(ctx context.Context, ownerID string) (release func(), err error) {
	if s == nil {
		// Caller didn't wire slots — degrade to no-op (preserves the
		// single-pod / dev path where global caps aren't needed).
		return func() {}, nil
	}

	// Per-owner check first — cheap and avoids burning a global slot
	// just to free it again.
	s.mu.Lock()
	if s.perOwner[ownerID] >= s.maxPerUser {
		s.mu.Unlock()
		return nil, ErrRunThrottled
	}
	s.perOwner[ownerID]++
	s.mu.Unlock()

	// Bounded wait for the global slot. Don't honour the caller's full
	// deadline here — long admission waits make the symptom (slow
	// streaming) worse than the cure.
	acqCtx, cancel := context.WithTimeout(ctx, admissionDeadline)
	defer cancel()
	if err := s.global.Acquire(acqCtx, 1); err != nil {
		// Roll back the per-owner increment we just took.
		s.mu.Lock()
		if s.perOwner[ownerID] > 0 {
			s.perOwner[ownerID]--
		}
		if s.perOwner[ownerID] == 0 {
			delete(s.perOwner, ownerID)
		}
		s.mu.Unlock()
		return nil, ErrRunThrottled
	}

	return func() {
		s.global.Release(1)
		s.mu.Lock()
		if s.perOwner[ownerID] > 0 {
			s.perOwner[ownerID]--
		}
		if s.perOwner[ownerID] == 0 {
			delete(s.perOwner, ownerID)
		}
		s.mu.Unlock()
	}, nil
}
