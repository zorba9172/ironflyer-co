// Package finisher — gate-level telemetry wrapper.
//
// Every Gate.Check call funnels through `runGateInstrumented`, which
// stops a wall-clock, records the duration histogram, classifies each
// Issue's severity, and ticks the per-gate findings counter. The
// surrounding logic in engine.go remains unchanged: the wrapper returns
// the same []domain.Issue slice the inner Check produced.
//
// The duration histogram is partitioned by gate + outcome so dashboards
// can answer "which gate is slowest when it's failing?" and "which
// gate passes fastest in steady state?" without re-deriving the data.
//
// Severity normalisation: the domain.Severity vocabulary
// (info|warning|error|critical) maps to the Prometheus label vocabulary
// (info|low|medium|high|critical) so the metric stays compatible with
// industry-standard severity tooling.

package finisher

import (
	"context"
	"time"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/metrics"
)

// runGateInstrumented wraps Gate.Check with a duration histogram and a
// per-severity findings counter. The verdict label on the duration
// histogram is the worst severity seen across the returned issues:
//
//   - any critical|error  → "fail"
//   - any warning         → "warn"
//   - empty issues        → "pass"
//
// Panics in Check are NOT recovered here — the engine's outer recover()
// catches them; this layer only adds observability.
func runGateInstrumented(ctx context.Context, gate Gate, env *GateEnv) []domain.Issue {
	name := string(gate.Name())
	start := time.Now()
	issues := gate.Check(ctx, env)
	outcome := classifyGateOutcome(issues)
	metrics.ObserveGateDuration(name, outcome, time.Since(start))
	for _, iss := range issues {
		metrics.ObserveGateFinding(name, mapSeverityToPromLabel(iss.Severity))
	}
	return issues
}

// classifyGateOutcome returns the dashboard label for the duration
// histogram. Mirrors the engine's pass/fail bookkeeping so the duration
// metric and the gate-state field always agree.
func classifyGateOutcome(issues []domain.Issue) string {
	if len(issues) == 0 {
		return "pass"
	}
	hasWarnOnly := true
	for _, iss := range issues {
		switch iss.Severity {
		case domain.SeverityCritical, domain.SeverityError:
			return "fail"
		case domain.SeverityWarning:
			// stays warn unless we see a higher class
		default:
			hasWarnOnly = false
		}
	}
	if hasWarnOnly {
		return "warn"
	}
	return "pass"
}

// mapSeverityToPromLabel converts the domain.Severity vocabulary to the
// Prometheus label vocabulary used by gate_findings_total.
func mapSeverityToPromLabel(s domain.Severity) string {
	switch s {
	case domain.SeverityCritical:
		return "critical"
	case domain.SeverityError:
		return "high"
	case domain.SeverityWarning:
		return "medium"
	case domain.SeverityInfo:
		return "info"
	}
	return "info"
}
