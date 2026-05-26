package blueprints

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/learning"
)

// MemoryStatsService is the in-process StatsService used in dev
// (IRONFLYER_DB_DRIVER=memory) and as a clean substrate for resolver
// wiring before Postgres is provisioned. All money math is
// decimal.Decimal so the memory and postgres paths produce bit-
// identical averages.
type MemoryStatsService struct {
	mu   sync.Mutex
	rows map[string]*memoryRow
	runs []RunOutcome // append-only audit; kept for parity with Postgres
}

// memoryRow mirrors the blueprint_stats schema so the in-memory
// rollup math stays trivially comparable to the SQL UPSERT.
type memoryRow struct {
	executions            int64
	previewSuccess        int64
	refunds               int64
	repairCount           int64
	totalRevenueUSD       decimal.Decimal
	totalCostUSD          decimal.Decimal
	totalCompletionScore  decimal.Decimal
	timeToPreviewSum      int64
	timeToPreviewCount    int64
	updatedAt             time.Time
}

// NewMemoryStatsService constructs an empty in-memory StatsService.
func NewMemoryStatsService() *MemoryStatsService {
	return &MemoryStatsService{rows: map[string]*memoryRow{}}
}

// RecordRun appends to the audit list and updates the rollup row.
// Both updates happen under the same mutex, so a concurrent Get
// either sees both writes or neither.
func (s *MemoryStatsService) RecordRun(ctx context.Context, o RunOutcome) error {
	if err := validateOutcome(o); err != nil {
		return err
	}
	s.mu.Lock()
	s.runs = append(s.runs, o)

	row, ok := s.rows[o.BlueprintID]
	if !ok {
		row = &memoryRow{}
		s.rows[o.BlueprintID] = row
	}
	row.executions++
	if o.PreviewSuccess {
		row.previewSuccess++
	}
	if o.Refunded {
		row.refunds++
	}
	if o.Repaired {
		row.repairCount++
	}
	row.totalRevenueUSD = row.totalRevenueUSD.Add(o.RevenueUSD)
	row.totalCostUSD = row.totalCostUSD.Add(o.CostUSD)
	row.totalCompletionScore = row.totalCompletionScore.Add(decimal.NewFromFloat(o.CompletionScore))
	if o.TimeToPreviewSeconds > 0 {
		row.timeToPreviewSum += int64(o.TimeToPreviewSeconds)
		row.timeToPreviewCount++
	}
	row.updatedAt = time.Now().UTC()
	s.mu.Unlock()
	publishBlueprintOutcome(ctx, o)
	return nil
}

// publishBlueprintOutcome surfaces one run as a KindBlueprintUsed
// OutcomeEvent. Best-effort; the global publisher is nil-safe.
func publishBlueprintOutcome(ctx context.Context, o RunOutcome) {
	margin := o.RevenueUSD.Sub(o.CostUSD)
	var marginPct float64
	rev, _ := o.RevenueUSD.Float64()
	cost, _ := o.CostUSD.Float64()
	if rev > 0 {
		marginPct = (rev - cost) / rev * 100
	}
	learning.Publish(ctx, learning.OutcomeEvent{
		ExecutionID: o.ExecutionID.String(),
		TenantID:    o.TenantID.String(),
		Kind:        learning.KindBlueprintUsed,
		Attributes: map[string]any{
			"blueprint_id":     o.BlueprintID,
			"preview_success":  o.PreviewSuccess,
			"repaired":         o.Repaired,
			"refunded":         o.Refunded,
			"completion_score": o.CompletionScore,
			"margin_pct":       marginPct,
		},
		Success:   learning.BoolPtr(o.PreviewSuccess && !o.Refunded),
		CostUSD:   learning.DecimalPtr(o.CostUSD),
		MarginUSD: learning.DecimalPtr(margin),
		Tags: map[string]string{
			"blueprint_id": o.BlueprintID,
		},
	})
}

// Get assembles Stats from the rollup row, or returns ErrNoStats.
// Caller MUST NOT mutate the returned Stats — it is a copy already.
func (s *MemoryStatsService) Get(_ context.Context, blueprintID string) (Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[blueprintID]
	if !ok {
		return Stats{}, ErrNoStats
	}
	return s.snapshot(blueprintID, row), nil
}

// All returns one Stats per blueprint that has at least one run.
// The result is sorted by blueprint id so callers get a stable
// order across invocations.
func (s *MemoryStatsService) All(_ context.Context) ([]Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Stats, 0, len(s.rows))
	for id, row := range s.rows {
		out = append(out, s.snapshot(id, row))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BlueprintID < out[j].BlueprintID })
	return out, nil
}

// Top returns the top-N Stats ranked by byMetric.
func (s *MemoryStatsService) Top(ctx context.Context, byMetric string, limit int) ([]Stats, error) {
	all, err := s.All(ctx)
	if err != nil {
		return nil, err
	}
	return applyTop(all, byMetric, limit), nil
}

// snapshot materialises a Stats value from a memoryRow. Caller must
// hold s.mu.
func (s *MemoryStatsService) snapshot(id string, row *memoryRow) Stats {
	stats := Stats{
		BlueprintID:    id,
		Executions:     row.executions,
		PreviewSuccess: row.previewSuccess,
		Refunds:        row.refunds,
		RepairCount:    row.repairCount,
		UpdatedAt:      row.updatedAt,
	}
	return computeDerived(stats, row.totalRevenueUSD, row.totalCostUSD, row.totalCompletionScore, row.timeToPreviewSum, row.timeToPreviewCount)
}
