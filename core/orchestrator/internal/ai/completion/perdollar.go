package completion

// CompletionPerDollar is the unit-economics primitive:
//
//	completion_per_dollar = completion_score_delta / execution_cost_usd
//
// It is the basis for the ProfitGuard "continue / degrade / stop"
// decision per the V22 proof pack
// (01-unit-economics/02-completion-per-dollar.md).
//
// We return 0 when costUSD is non-positive so callers don't have to
// special-case division-by-zero — a "free" step has no per-dollar
// meaning and ProfitGuard already gates the spending side separately.
//
// Negative deltas (regressions) are passed through unchanged; the
// dashboards / ProfitGuard interpret a negative completion-per-dollar
// as "this step actively made the execution worse for money".
func CompletionPerDollar(deltaScore, costUSD float64) float64 {
	if costUSD <= 0 {
		return 0
	}
	return deltaScore / costUSD
}
