package warmpool

import "sync/atomic"

// Metrics is an additive, lock-free counter surface. Consumed by the
// runtime metrics registry the integration agent wires.
type Metrics struct {
	Leases         atomic.Int64
	LeaseHits      atomic.Int64
	LeaseMisses    atomic.Int64
	Returns        atomic.Int64
	Drained        atomic.Int64
	ActiveFloor    atomic.Int64
	ActiveLeases   atomic.Int64
}
