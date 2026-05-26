// health.go is the GraphQL-facing adapter for the Code Health
// Dashboard (playbook §8.11, docs/ANTI_BLOAT_ENGINE.md). It composes
// the Anti-Bloat lane's signals into a single struct the resolver can
// project into the cockpit pane: reuse rate, dedup rate, dead code,
// complexity histogram, dependency cycles, and "LOC per resolved
// capability".
//
// Most signals depend on persisted tool reports that the orchestrator
// does not yet collect. The shape is what matters now; population
// follows. Each field has an explicit "missing report" sentinel
// (zero or nil) so the UI can render a "tool not wired" chip without
// crashing.
//
// Wiring contract:
//
//   - The Atlas tracks Capability counts + last index time.
//   - The audit chain stores PreflightDecision events; ReuseRate is
//     decisions_with_reuse_or_extend / total_decisions.
//   - The wowloop GateSource projects the latest gate verdicts; the
//     evidence-stub gates carry the reports we project here once an
//     operator wires the per-tool env var (see
//     IRONFLYER_*_REPORT_PATH in finisher/gates_antibloat.go).
//   - LedgerSource provides the `loc_added` / `loc_removed` /
//     `loc_net` audit attrs that ProfitGuard already records on
//     every applied patch.

package dashboards

import (
	"context"
	"time"
)

// HealthDashboard is the operator-facing Code Health snapshot for a
// project. Every field has documented zero-value semantics so the
// resolver can render the "tool not wired" state without a separate
// "is loaded" flag per field.
type HealthDashboard struct {
	ProjectID string    `json:"projectId"`
	AsOf      time.Time `json:"asOf"`

	// ReuseRate is the fraction of recent PreflightDecisions whose
	// Action was `reuse` or `extend`. -1 means "no decisions
	// recorded yet" so the UI can distinguish "0% reuse" from
	// "feature dark".
	ReuseRate float64 `json:"reuseRate"`

	// DedupRate is the duplication ratio reported by jscpd / dupl.
	// -1 means no report wired.
	DedupRate float64 `json:"dedupRate"`

	// DeadcodeCount is the count of unused exports / files reported
	// by knip / ts-prune / unparam. -1 means no report wired.
	DeadcodeCount int `json:"deadcodeCount"`

	// ComplexityHistogram is a fixed-bin distribution of cognitive
	// complexity per function. Bins: <=5, 6-10, 11-15, 16-20, 21+.
	// Nil means no report wired; an empty slice means "report wired
	// but project had no findings".
	ComplexityHistogram []int `json:"complexityHistogram,omitempty"`

	// DependencyCycles is the number of import cycles detected by
	// dependency-cruiser / madge / the Manifest cycle pass. -1
	// means no report wired.
	DependencyCycles int `json:"dependencyCycles"`

	// LOCPerCapability is the net-LOC-added divided by the number of
	// Resolved Capabilities (story acceptance criteria marked
	// Validated). Lower is better. Zero means no patches landed
	// yet.
	LOCPerCapability float64 `json:"locPerCapability"`

	// AtlasCapabilityCount is the total number of capabilities the
	// Atlas has indexed for this project's stack. Zero means the
	// Atlas has not run an index pass yet.
	AtlasCapabilityCount int `json:"atlasCapabilityCount"`

	// LastIndexedAt is the timestamp of the most recent Atlas index
	// pass. Zero means the Atlas has never run.
	LastIndexedAt time.Time `json:"lastIndexedAt"`
}

// HealthSource is the operator-replaceable adapter the Service uses
// to populate HealthDashboard. The MVP implementation lives in
// dashboards/adapters; production wires it to the audit + Atlas +
// ledger sources.
type HealthSource interface {
	// ReuseStats returns (decisionsWithReuseOrExtend, totalDecisions)
	// over the last 30 days for projectID. Both zero → no decisions
	// recorded; the dashboard reports ReuseRate = -1.
	ReuseStats(ctx context.Context, projectID string) (matched, total int, err error)
	// LatestDedupReport returns the persisted jscpd / dupl rate for
	// projectID, or ok=false when no report has landed.
	LatestDedupReport(ctx context.Context, projectID string) (rate float64, ok bool, err error)
	// LatestDeadcodeReport returns the unused-exports count for
	// projectID, or ok=false when no report has landed.
	LatestDeadcodeReport(ctx context.Context, projectID string) (count int, ok bool, err error)
	// LatestComplexityHistogram returns the per-function complexity
	// histogram for projectID, or ok=false when no report has
	// landed.
	LatestComplexityHistogram(ctx context.Context, projectID string) (bins []int, ok bool, err error)
	// LatestDependencyCycles returns the cycle count for projectID,
	// or ok=false when no report has landed.
	LatestDependencyCycles(ctx context.Context, projectID string) (count int, ok bool, err error)
	// LedgerLOC returns the net LOC added and the count of resolved
	// capabilities for projectID over the last 30 days. Zero/zero
	// is a legal first-run state — the dashboard reports
	// LOCPerCapability = 0.
	LedgerLOC(ctx context.Context, projectID string) (netLOC int, resolvedCapabilities int, err error)
	// AtlasCount returns the total capabilities + last-indexed time
	// for projectID. Zero count + zero time means the Atlas has
	// never run.
	AtlasCount(ctx context.Context, projectID string) (count int, lastIndexed time.Time, err error)
}

// HealthDashboardBuilder composes one HealthDashboard from a
// HealthSource. Embedded in dashboards.Service via Health() once an
// operator wires a concrete source.
func BuildHealth(ctx context.Context, src HealthSource, projectID string, now time.Time) (HealthDashboard, error) {
	out := HealthDashboard{ProjectID: projectID, AsOf: now}
	if src == nil {
		out.ReuseRate = -1
		out.DedupRate = -1
		out.DeadcodeCount = -1
		out.DependencyCycles = -1
		return out, nil
	}
	matched, total, err := src.ReuseStats(ctx, projectID)
	if err != nil {
		return HealthDashboard{}, err
	}
	if total <= 0 {
		out.ReuseRate = -1
	} else {
		out.ReuseRate = float64(matched) / float64(total)
	}
	rate, ok, err := src.LatestDedupReport(ctx, projectID)
	if err != nil {
		return HealthDashboard{}, err
	}
	if ok {
		out.DedupRate = rate
	} else {
		out.DedupRate = -1
	}
	count, ok, err := src.LatestDeadcodeReport(ctx, projectID)
	if err != nil {
		return HealthDashboard{}, err
	}
	if ok {
		out.DeadcodeCount = count
	} else {
		out.DeadcodeCount = -1
	}
	bins, ok, err := src.LatestComplexityHistogram(ctx, projectID)
	if err != nil {
		return HealthDashboard{}, err
	}
	if ok {
		out.ComplexityHistogram = bins
	}
	cycles, ok, err := src.LatestDependencyCycles(ctx, projectID)
	if err != nil {
		return HealthDashboard{}, err
	}
	if ok {
		out.DependencyCycles = cycles
	} else {
		out.DependencyCycles = -1
	}
	netLOC, resolved, err := src.LedgerLOC(ctx, projectID)
	if err != nil {
		return HealthDashboard{}, err
	}
	if resolved > 0 {
		out.LOCPerCapability = float64(netLOC) / float64(resolved)
	}
	atlasCount, lastIndexed, err := src.AtlasCount(ctx, projectID)
	if err != nil {
		return HealthDashboard{}, err
	}
	out.AtlasCapabilityCount = atlasCount
	out.LastIndexedAt = lastIndexed
	return out, nil
}

// Health is the Service facade. It mirrors the per-tenant methods on
// dashboards.Service so the GraphQL resolver can call it the same
// way. Returns ErrTenantRequired when projectID is empty.
func (s *Service) Health(ctx context.Context, projectID string) (HealthDashboard, error) {
	if projectID == "" {
		return HealthDashboard{}, ErrTenantRequired
	}
	return BuildHealth(ctx, s.Health_Source(), projectID, time.Now().UTC())
}

// Health_Source returns the HealthSource the Service is configured
// with, or nil. Kept as a method on Service so callers don't need to
// add a struct field today (the wireup PR that adds the concrete
// adapter will add the field; until then BuildHealth returns the
// "tool not wired" sentinel shape).
func (s *Service) Health_Source() HealthSource {
	// MVP placeholder. The wireup follow-up extends Service with a
	// Health field; until then the Service returns the stub shape so
	// the resolver can render the dashboard chrome without crashing.
	return nil
}
