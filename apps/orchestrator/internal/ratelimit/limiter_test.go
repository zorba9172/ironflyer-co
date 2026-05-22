package ratelimit

import (
	"sync"
	"testing"
	"time"
)

// fakeClock lets us advance time deterministically inside the limiter.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeClock) now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}
func (f *fakeClock) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

func newWithClock(rate, burst float64, c *fakeClock) *Limiter {
	l := New(rate, burst)
	l.now = c.now
	return l
}

func TestAllow_BurstThenBlocks(t *testing.T) {
	c := &fakeClock{t: time.Unix(0, 0)}
	l := newWithClock(10, 3, c) // 3 tokens cap, refill 10/s

	for i := 0; i < 3; i++ {
		ok, wait := l.Allow("u1")
		if !ok || wait != 0 {
			t.Fatalf("expected initial burst i=%d to allow, got (ok=%v, wait=%v)", i, ok, wait)
		}
	}
	ok, wait := l.Allow("u1")
	if ok {
		t.Fatal("4th call should be blocked")
	}
	if wait <= 0 || wait > 200*time.Millisecond {
		t.Errorf("expected wait around 100ms for rate=10/s, got %v", wait)
	}
}

func TestAllow_RefillsOverTime(t *testing.T) {
	c := &fakeClock{t: time.Unix(0, 0)}
	l := newWithClock(10, 2, c) // 2 burst, 10/s

	l.Allow("u1")
	l.Allow("u1")
	if ok, _ := l.Allow("u1"); ok {
		t.Fatal("3rd immediate call should be blocked")
	}
	c.advance(150 * time.Millisecond) // refills 1.5 tokens
	if ok, _ := l.Allow("u1"); !ok {
		t.Fatal("after 150ms one token should be available")
	}
}

func TestAllowN_LargerCost(t *testing.T) {
	c := &fakeClock{t: time.Unix(0, 0)}
	l := newWithClock(10, 5, c)
	ok, _ := l.AllowN("u1", 5)
	if !ok {
		t.Fatal("first full burst should be allowed")
	}
	ok, wait := l.AllowN("u1", 3)
	if ok {
		t.Fatal("second AllowN should block")
	}
	if wait < 250*time.Millisecond {
		t.Errorf("wait too short for cost=3 at 10/s, got %v", wait)
	}
}

func TestAllow_PerKeyIsolation(t *testing.T) {
	c := &fakeClock{t: time.Unix(0, 0)}
	l := newWithClock(10, 1, c)
	if ok, _ := l.Allow("u1"); !ok {
		t.Fatal("u1 should start with a token")
	}
	if ok, _ := l.Allow("u2"); !ok {
		t.Fatal("u2 should start independently with a token")
	}
	if ok, _ := l.Allow("u1"); ok {
		t.Fatal("u1 should now be empty")
	}
}

func TestReset(t *testing.T) {
	c := &fakeClock{t: time.Unix(0, 0)}
	l := newWithClock(10, 1, c)
	l.Allow("u1")
	if ok, _ := l.Allow("u1"); ok {
		t.Fatal("u1 should be empty after one spend")
	}
	l.Reset("u1")
	if ok, _ := l.Allow("u1"); !ok {
		t.Fatal("after Reset, u1 should be full again")
	}
}
