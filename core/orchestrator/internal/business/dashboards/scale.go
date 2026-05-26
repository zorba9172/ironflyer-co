package dashboards

import (
	"context"
	"time"
)

// ScaleDashboard is the live-scale view per V22 proof pack
// (03-proof-dashboards/02-scale-dashboard.md). Scale is only healthy
// when margin is healthy (hard law 3) — the dashboard surfaces
// utilization + capacity so the operator can decide whether to scale
// up only when ProfitGuard says it pays.
//
// Notes is an internal field that captures which inputs were real
// signals vs. defaulted estimates for the current snapshot. Surfaced
// via logs / Prometheus rather than GraphQL so the schema stays
// backward compatible.
type ScaleDashboard struct {
	ActiveExecutions     int
	QueuedExecutions     int
	QueueWaitSec         float64
	SandboxCapacity      int
	WorkerUtilizationPct float64
	ScaleHealth          float64
	Notes                []string
}

// queueWaitLookback bounds the window used to derive the rolling
// average queue wait. 15 minutes is short enough to react to real
// pressure changes and long enough to smooth noise on a single-pod
// orchestrator.
const queueWaitLookback = 15 * time.Minute

// BuildScale composes one ScaleDashboard from a ScaleSource +
// ExecutionSource pair.
//
// Scale health (per the proof pack) is the product:
//
//	scale_health = capacity_available × recovery_success
//	              × event_lag_health  × margin_health
//
// Inputs:
//   - capacity_available — real. Derived from WorkerUtilizationPct
//     when non-zero, otherwise from active/SandboxCapacity. Clamped
//     to [0, 1].
//   - recovery_success — estimated. Defaults to 1.0; the recovery
//     loop publishes its real signal via Prometheus, not the
//     dashboard read path. Recorded in Notes when used.
//   - event_lag_health — derived. 1.0 when queue wait < 30s, drops
//     linearly to 0 at queue wait > 5 min. Recorded in Notes.
//   - margin_health — estimated. Defaults to 1.0; the live profit
//     dashboard is the source-of-truth for margin and the operator
//     reads it side-by-side. Recorded in Notes.
//
// When execSrc is nil only the ScaleSource is consulted (legacy
// behaviour). All zero-valued returns are honest "no data yet" —
// callers should distinguish via Notes.
func BuildScale(ctx context.Context, src ScaleSource, execSrc ExecutionSource) (ScaleDashboard, error) {
	out := ScaleDashboard{Notes: nil}

	// Live counts: prefer the execution source because it always
	// reflects database truth; fall back to ScaleSource only when the
	// execution source is missing.
	if execSrc != nil {
		active, err := execSrc.ActiveCount(ctx)
		if err == nil {
			out.ActiveExecutions = active
		} else {
			out.Notes = append(out.Notes, "active=execution_source_error")
		}
		queued, err := execSrc.QueuedCount(ctx)
		if err == nil {
			out.QueuedExecutions = queued
		} else {
			out.Notes = append(out.Notes, "queued=execution_source_error")
		}
		wait, err := execSrc.AverageQueueWaitSec(ctx, time.Now().UTC().Add(-queueWaitLookback))
		if err == nil {
			out.QueueWaitSec = wait
		} else {
			out.Notes = append(out.Notes, "queue_wait=execution_source_error")
		}
	} else if src != nil {
		active, _ := src.ActiveExecutions(ctx)
		out.ActiveExecutions = active
		queued, _ := src.QueueDepth(ctx)
		out.QueuedExecutions = queued
		out.Notes = append(out.Notes, "queue_wait=estimated_zero_no_exec_source")
	}

	// Sandbox capacity comes from the runtime-aware ScaleSource (env
	// var or static cap). Defaults to 1 to avoid divide-by-zero in
	// the utilization fallback.
	if src != nil {
		cap, err := src.SandboxCapacity(ctx)
		if err == nil && cap > 0 {
			out.SandboxCapacity = cap
		} else {
			out.SandboxCapacity = 1
			out.Notes = append(out.Notes, "sandbox_capacity=defaulted_to_1")
		}
	} else {
		out.SandboxCapacity = 1
		out.Notes = append(out.Notes, "sandbox_capacity=no_source")
	}

	// Worker utilization: prefer the explicit ScaleSource reading
	// when non-zero; otherwise derive from active/capacity so the
	// dashboard still moves when the runtime hasn't published its
	// own utilization gauge.
	if src != nil {
		if util, err := src.WorkerUtilizationPct(ctx); err == nil && util > 0 {
			out.WorkerUtilizationPct = util
		}
	}
	if out.WorkerUtilizationPct == 0 && out.SandboxCapacity > 0 {
		derived := float64(out.ActiveExecutions) / float64(out.SandboxCapacity) * 100.0
		if derived > 100 {
			derived = 100
		}
		if derived < 0 {
			derived = 0
		}
		out.WorkerUtilizationPct = derived
		if derived > 0 {
			out.Notes = append(out.Notes, "utilization=derived_active_over_capacity")
		}
	}

	// ------------- Scale Health composition -------------
	capacityAvailable := 1.0 - (out.WorkerUtilizationPct / 100.0)
	if capacityAvailable < 0 {
		capacityAvailable = 0
	}
	if capacityAvailable > 1 {
		capacityAvailable = 1
	}

	// event_lag_health: 1.0 at queue wait <= 30s, linear decay to 0
	// at queue wait >= 5 min. Real signal — driven by queue wait.
	eventLagHealth := 1.0
	switch {
	case out.QueueWaitSec <= 30:
		eventLagHealth = 1.0
	case out.QueueWaitSec >= 300:
		eventLagHealth = 0.0
	default:
		eventLagHealth = 1.0 - ((out.QueueWaitSec - 30.0) / (300.0 - 30.0))
	}

	// recovery_success + margin_health stay estimated at 1.0 in v1.
	// The dashboard service can override these by composing with the
	// profit dashboard externally; documenting the estimate keeps
	// the operator honest.
	recoverySuccess := 1.0
	marginHealth := 1.0
	out.Notes = append(out.Notes,
		"recovery_success=estimated_1.0",
		"margin_health=estimated_1.0",
	)

	out.ScaleHealth = capacityAvailable * recoverySuccess * eventLagHealth * marginHealth

	return out, nil
}
