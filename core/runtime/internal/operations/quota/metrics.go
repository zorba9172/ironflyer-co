package quota

import "sync/atomic"

// Metrics is an additive, lock-free counter surface for admission
// outcomes. Wired into the integration layer's metrics registry by
// the wireup agent.
type Metrics struct {
	Admitted          atomic.Int64
	DeniedQuota       atomic.Int64
	DeniedBudget      atomic.Int64
	DeniedCapacity    atomic.Int64
	DeniedDegrade     atomic.Int64
	DeniedStopLoss    atomic.Int64
	Releases          atomic.Int64
}

// ObserveAdmit increments the per-reason counter; success counts on
// Admitted.
func (m *Metrics) ObserveAdmit(allow bool, reason Reason) {
	if m == nil {
		return
	}
	if allow {
		m.Admitted.Add(1)
		return
	}
	switch reason {
	case ReasonPauseForBudget:
		m.DeniedBudget.Add(1)
	case ReasonCapacityWait:
		m.DeniedCapacity.Add(1)
	case ReasonDegradeRuntime:
		m.DeniedDegrade.Add(1)
	case ReasonStopLoss:
		m.DeniedStopLoss.Add(1)
	default:
		m.DeniedQuota.Add(1)
	}
}

// ObserveRelease increments the release counter.
func (m *Metrics) ObserveRelease() {
	if m == nil {
		return
	}
	m.Releases.Add(1)
}
