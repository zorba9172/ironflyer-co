package blueprints

import (
	"context"
	"errors"
	"sort"

	"github.com/shopspring/decimal"
)

// StatsService is the per-blueprint analytics surface. It records
// one outcome per execution and exposes the rolled-up Stats the
// Blueprint Profit Dashboard ranks by. Both the memory and Postgres
// backends implement it.
type StatsService interface {
	// RecordRun persists the outcome as an immutable blueprint_runs
	// row AND atomically updates the matching blueprint_stats
	// rollup. The atomicity matters: the dashboard reads stats
	// expecting it to always equal "sum of all runs", so the two
	// writes happen inside one transaction.
	RecordRun(ctx context.Context, outcome RunOutcome) error
	// Get returns the rolled-up Stats for one blueprint, or
	// ErrNoStats if no runs have landed yet.
	Get(ctx context.Context, blueprintID string) (Stats, error)
	// All returns the rolled-up Stats for every blueprint that has
	// at least one recorded run, ordered by blueprint id.
	All(ctx context.Context) ([]Stats, error)
	// Top returns the top-N Stats ranked by byMetric. Supported
	// values: "executions", "preview_success", "gross_margin_pct",
	// "avg_completion_score", "avg_time_to_preview_sec" (ascending
	// for the last one — faster is better), "avg_cost_usd" (also
	// ascending). Unknown metrics fall back to "executions".
	Top(ctx context.Context, byMetric string, limit int) ([]Stats, error)
}

// ErrNoStats is returned by Get when the blueprint has no runs yet.
// Callers (dashboard, ProfitGuard) treat this as "use the prior".
var ErrNoStats = errors.New("blueprints: no stats for blueprint")

// ErrInvalidOutcome is returned by RecordRun when the outcome would
// produce an unusable row (missing blueprint id, nil uuid, etc).
var ErrInvalidOutcome = errors.New("blueprints: invalid run outcome")

// computeDerived turns the raw accumulators carried on the
// blueprint_stats row into the dashboard-ready averages on Stats.
// Single-call helper so the memory and postgres backends never
// disagree on the formulae.
func computeDerived(s Stats, totalRev, totalCost, totalCompletion decimal.Decimal, ttpSum, ttpCount int64) Stats {
	if s.Executions > 0 {
		execs := decimal.NewFromInt(s.Executions)
		s.AvgRevenueUSD = totalRev.Div(execs)
		s.AvgCostUSD = totalCost.Div(execs)
		s.AvgCompletionScore = totalCompletion.Div(execs)
	}
	if totalRev.Sign() > 0 {
		s.GrossMarginPct = totalRev.Sub(totalCost).Div(totalRev).Mul(decimal.NewFromInt(100))
	}
	if ttpCount > 0 {
		s.AvgTimeToPreviewSec = decimal.NewFromInt(ttpSum).Div(decimal.NewFromInt(ttpCount))
	}
	return s
}

// validateOutcome enforces the minimum shape every backend needs.
func validateOutcome(o RunOutcome) error {
	if o.BlueprintID == "" {
		return errors.Join(ErrInvalidOutcome, errors.New("blueprintID required"))
	}
	// ExecutionID / TenantID are validated by their callers; we do
	// not insist on non-nil here so dev/CLI helpers can record
	// synthetic outcomes for dashboard previews.
	return nil
}

// rankComparator returns a less() function for sort.Slice over a
// []Stats given a byMetric string. Unknown metrics fall back to
// "executions" descending.
func rankComparator(byMetric string) func(a, b Stats) bool {
	switch byMetric {
	case "preview_success":
		return func(a, b Stats) bool { return a.PreviewSuccess > b.PreviewSuccess }
	case "gross_margin_pct":
		return func(a, b Stats) bool { return a.GrossMarginPct.GreaterThan(b.GrossMarginPct) }
	case "avg_completion_score":
		return func(a, b Stats) bool { return a.AvgCompletionScore.GreaterThan(b.AvgCompletionScore) }
	case "avg_time_to_preview_sec":
		// Faster is better — ascending. Zero values (no data) sink to bottom.
		return func(a, b Stats) bool {
			if a.AvgTimeToPreviewSec.IsZero() {
				return false
			}
			if b.AvgTimeToPreviewSec.IsZero() {
				return true
			}
			return a.AvgTimeToPreviewSec.LessThan(b.AvgTimeToPreviewSec)
		}
	case "avg_cost_usd":
		return func(a, b Stats) bool { return a.AvgCostUSD.LessThan(b.AvgCostUSD) }
	default:
		return func(a, b Stats) bool { return a.Executions > b.Executions }
	}
}

// applyTop ranks + truncates a Stats slice in place. Pulled out so
// both backends share the exact same ordering.
func applyTop(in []Stats, byMetric string, limit int) []Stats {
	less := rankComparator(byMetric)
	sort.SliceStable(in, func(i, j int) bool { return less(in[i], in[j]) })
	if limit > 0 && len(in) > limit {
		in = in[:limit]
	}
	return in
}
