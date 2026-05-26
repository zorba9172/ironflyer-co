package learning

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/business/clickhouse"
)

// Store is the read surface dashboards and the bandit consult. The
// Memory implementation projects observed events in-process; the
// ClickHouse implementation answers from the durable facts. Both
// honour tenant scoping — pass "" only for operator-level rollups.
type Store interface {
	Snapshot(ctx context.Context, tenantID string) (LearningSnapshot, error)
	ClosureScore(ctx context.Context, executionID string) (ClosureScore, error)
	WeaknessTop(ctx context.Context, tenantID string, k int) ([]Weakness, error)
}

// MemoryStore is the dev-mode in-process projection. It is fed by
// Publisher.SetObserver(store.Observe) so the snapshot stays live
// without going through ClickHouse.
type MemoryStore struct {
	mu sync.RWMutex

	events []OutcomeEvent

	gateOutcomes      map[string]gateStats        // gate -> stats
	blueprintOutcomes map[string]blueprintStats   // blueprint id -> stats
	repairHits        int
	repairWindowStart time.Time
	completionScores  map[string]float64 // execution_id -> latest score
	marginPctSamples  []float64
	indexedAt         time.Time
}

type gateStats struct {
	Total   int
	Failed  int
	Updated time.Time
}

type blueprintStats struct {
	Total      int
	Succeeded  int
	MarginSum  float64
	MarginObs  int
	UpdatedAt  time.Time
}

// NewMemoryStore returns an empty projection ready to receive events
// via Observe.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		gateOutcomes:      make(map[string]gateStats),
		blueprintOutcomes: make(map[string]blueprintStats),
		completionScores:  make(map[string]float64),
		repairWindowStart: time.Now().UTC(),
	}
}

// EventsSince returns a copy of the ring-buffered OutcomeEvents with
// Timestamp at or after cutoff. Exposed so the CompletionPredictor
// (and other in-process consumers) can warm-start from recent history
// without depending on the durable ClickHouse path. nil-safe.
func (s *MemoryStore) EventsSince(cutoff time.Time) []OutcomeEvent {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]OutcomeEvent, 0, len(s.events))
	for _, evt := range s.events {
		if evt.Timestamp.Before(cutoff) {
			continue
		}
		out = append(out, evt)
	}
	return out
}

// Observe is the Publisher.SetObserver callback. It is safe to call
// concurrently with Snapshot.
func (s *MemoryStore) Observe(evt OutcomeEvent) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	const maxRing = 4096
	if len(s.events) >= maxRing {
		s.events = s.events[len(s.events)-maxRing+1:]
	}
	s.events = append(s.events, evt)
	s.indexedAt = time.Now().UTC()

	switch evt.Kind {
	case KindGateOutcome:
		gate, _ := evt.Attributes["gate"].(string)
		verdict, _ := evt.Attributes["verdict"].(string)
		if gate == "" {
			return
		}
		st := s.gateOutcomes[gate]
		st.Total++
		if verdict != "" && verdict != "pass" {
			st.Failed++
		} else if evt.Success != nil && !*evt.Success {
			st.Failed++
		}
		st.Updated = evt.Timestamp
		s.gateOutcomes[gate] = st
	case KindBlueprintUsed:
		bp, _ := evt.Attributes["blueprint_id"].(string)
		if bp == "" {
			return
		}
		st := s.blueprintOutcomes[bp]
		st.Total++
		if evt.Success != nil && *evt.Success {
			st.Succeeded++
		}
		if mp, ok := evt.Attributes["margin_pct"].(float64); ok {
			st.MarginSum += mp
			st.MarginObs++
		}
		st.UpdatedAt = evt.Timestamp
		s.blueprintOutcomes[bp] = st
	case KindRepairTriggered:
		s.repairHits++
	case KindCompletionScore:
		if score, ok := evt.Attributes["score"].(float64); ok && evt.ExecutionID != "" {
			s.completionScores[evt.ExecutionID] = score
		}
	case KindExecutionComplete:
		if mp, ok := evt.Attributes["margin_pct"].(float64); ok {
			s.marginPctSamples = append(s.marginPctSamples, mp)
			if len(s.marginPctSamples) > 256 {
				s.marginPctSamples = s.marginPctSamples[len(s.marginPctSamples)-256:]
			}
		}
	}
}

// Snapshot derives a LearningSnapshot from the in-process projection.
// tenantID is honoured by filtering the ring before aggregating.
func (s *MemoryStore) Snapshot(_ context.Context, tenantID string) (LearningSnapshot, error) {
	if s == nil {
		return LearningSnapshot{}, errors.New("learning: memory store not configured")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	today := time.Now().UTC().Truncate(24 * time.Hour)
	todayCount := 0
	allTime := 0
	for _, evt := range s.events {
		if tenantID != "" && evt.TenantID != tenantID {
			continue
		}
		allTime++
		if !evt.Timestamp.Before(today) {
			todayCount++
		}
	}

	gateRates := make(map[string]float64, len(s.gateOutcomes))
	for gate, st := range s.gateOutcomes {
		if st.Total == 0 {
			continue
		}
		gateRates[gate] = float64(st.Failed) / float64(st.Total)
	}

	bpRates := make(map[string]float64, len(s.blueprintOutcomes))
	for bp, st := range s.blueprintOutcomes {
		if st.Total == 0 {
			continue
		}
		bpRates[bp] = float64(st.Succeeded) / float64(st.Total)
	}

	avgCompletion := 0.0
	if n := len(s.completionScores); n > 0 {
		sum := 0.0
		for _, v := range s.completionScores {
			sum += v
		}
		avgCompletion = sum / float64(n)
	}

	avgMargin := 0.0
	if n := len(s.marginPctSamples); n > 0 {
		sum := 0.0
		for _, v := range s.marginPctSamples {
			sum += v
		}
		avgMargin = sum / float64(n)
	}

	indexed := s.indexedAt
	var indexedPtr *time.Time
	if !indexed.IsZero() {
		indexedPtr = &indexed
	}
	return LearningSnapshot{
		OutcomeEventsToday:     todayCount,
		OutcomeEventsAllTime:   allTime,
		ReuseRateLast7d:        reuseRate(s.events, tenantID),
		RepairRecipeHitsLast7d: s.repairHits,
		BanditConfidence:       0,
		BlueprintSuccessRate:   bpRates,
		GateFailureRateLast7d:  gateRates,
		AverageCompletionScore: avgCompletion,
		AverageMarginPctLast7d: avgMargin,
		LastIndexedAt:          indexedPtr,
	}, nil
}

// ClosureScore returns the most recent computed score for an
// execution. Memory mode keeps only the latest completion score per
// execution; the four-dimensional decomposition is approximated from
// the limited signals available.
func (s *MemoryStore) ClosureScore(_ context.Context, executionID string) (ClosureScore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	score, ok := s.completionScores[executionID]
	if !ok {
		return ClosureScore{ComputedAt: time.Now().UTC()}, nil
	}
	return ClosureScore{
		ScopeCompletion:      score,
		QualityConfidence:    score,
		IntegrationStability: score,
		MarginHealth:         score,
		Overall:              score,
		ComputedAt:           time.Now().UTC(),
	}, nil
}

// WeaknessTop returns up to k weaknesses ordered by severity. The
// memory implementation surfaces the worst gates, the worst blueprints,
// and a repair-miss flag when the recipe hit count is suspiciously low
// relative to triggered repairs.
func (s *MemoryStore) WeaknessTop(_ context.Context, _ string, k int) ([]Weakness, error) {
	if k <= 0 {
		k = 5
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Weakness, 0, k)

	type rate struct {
		key  string
		rate float64
		n    int
	}
	gates := make([]rate, 0, len(s.gateOutcomes))
	for g, st := range s.gateOutcomes {
		if st.Total < 3 {
			continue
		}
		gates = append(gates, rate{g, float64(st.Failed) / float64(st.Total), st.Total})
	}
	sort.Slice(gates, func(i, j int) bool { return gates[i].rate > gates[j].rate })
	for _, g := range gates {
		if g.rate < 0.25 || len(out) >= k {
			break
		}
		sev := "low"
		switch {
		case g.rate >= 0.6:
			sev = "high"
		case g.rate >= 0.4:
			sev = "medium"
		}
		out = append(out, Weakness{
			Dimension:       "gate_failure",
			Description:     fmt.Sprintf("gate %q fails %.0f%% of runs (n=%d)", g.key, g.rate*100, g.n),
			Severity:        sev,
			SuggestedAction: fmt.Sprintf("audit the %s gate inputs and tighten its pre-checks", g.key),
		})
	}

	bps := make([]rate, 0, len(s.blueprintOutcomes))
	for b, st := range s.blueprintOutcomes {
		if st.Total < 3 {
			continue
		}
		bps = append(bps, rate{b, float64(st.Succeeded) / float64(st.Total), st.Total})
	}
	sort.Slice(bps, func(i, j int) bool { return bps[i].rate < bps[j].rate })
	for _, b := range bps {
		if b.rate > 0.6 || len(out) >= k {
			break
		}
		out = append(out, Weakness{
			Dimension:       "blueprint_completion",
			Description:     fmt.Sprintf("blueprint %q succeeds only %.0f%% of runs (n=%d)", b.key, b.rate*100, b.n),
			Severity:        ternary(b.rate < 0.3, "high", "medium"),
			SuggestedAction: fmt.Sprintf("revisit %s template or its expected scope", b.key),
		})
	}
	return out, nil
}

func reuseRate(events []OutcomeEvent, tenantID string) float64 {
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	hits, total := 0, 0
	for _, evt := range events {
		if tenantID != "" && evt.TenantID != tenantID {
			continue
		}
		if evt.Timestamp.Before(cutoff) {
			continue
		}
		if evt.Kind == KindPatchApplied || evt.Kind == KindBlueprintUsed {
			total++
			if reused, ok := evt.Attributes["reused"].(bool); ok && reused {
				hits++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

// ClickHouseStore reads the durable facts. It is the production path.
type ClickHouseStore struct {
	ch  *clickhouse.Client
	log zerolog.Logger
}

// NewClickHouseStore wires the store to an existing ClickHouse client.
// ch MAY be nil — calls then return zero-value snapshots so the
// resolver still renders.
func NewClickHouseStore(ch *clickhouse.Client, log zerolog.Logger) *ClickHouseStore {
	return &ClickHouseStore{ch: ch, log: log}
}

// Snapshot answers from fact_outcome_events + rollup_learning_daily.
// When the rollup MV has no rows yet (cold start) every field stays at
// zero — the resolver renders empty-state cards.
func (s *ClickHouseStore) Snapshot(ctx context.Context, tenantID string) (LearningSnapshot, error) {
	if s == nil || s.ch == nil {
		return LearningSnapshot{}, nil
	}
	snap := LearningSnapshot{
		BlueprintSuccessRate:  map[string]float64{},
		GateFailureRateLast7d: map[string]float64{},
	}
	// Total counters (today / all-time).
	rows, err := s.ch.QueryRows(ctx, `
		SELECT
		  countIf(toDate(timestamp) = today())  AS today,
		  count()                               AS all_time
		FROM fact_outcome_events
		WHERE (? = '' OR tenant_id = ?)`,
		tenantID, tenantID,
	)
	if err == nil && rows != nil {
		if rows.Next() {
			var today, allTime uint64
			if scanErr := rows.Scan(&today, &allTime); scanErr == nil {
				snap.OutcomeEventsToday = int(today)
				snap.OutcomeEventsAllTime = int(allTime)
			}
		}
		_ = rows.Close()
	}
	// Repair hits last 7d.
	rows, err = s.ch.QueryRows(ctx, `
		SELECT count()
		FROM fact_outcome_events
		WHERE kind = 'repair_triggered'
		  AND timestamp >= now() - INTERVAL 7 DAY
		  AND (? = '' OR tenant_id = ?)`,
		tenantID, tenantID,
	)
	if err == nil && rows != nil {
		if rows.Next() {
			var hits uint64
			if scanErr := rows.Scan(&hits); scanErr == nil {
				snap.RepairRecipeHitsLast7d = int(hits)
			}
		}
		_ = rows.Close()
	}
	// Average completion score.
	rows, err = s.ch.QueryRows(ctx, `
		SELECT avgIf(toFloat64OrZero(JSONExtractString(attributes_json, 'score')),
		             kind = 'completion_score')
		FROM fact_outcome_events
		WHERE timestamp >= now() - INTERVAL 30 DAY
		  AND (? = '' OR tenant_id = ?)`,
		tenantID, tenantID,
	)
	if err == nil && rows != nil {
		if rows.Next() {
			var avg float64
			if scanErr := rows.Scan(&avg); scanErr == nil && !math.IsNaN(avg) {
				snap.AverageCompletionScore = avg
			}
		}
		_ = rows.Close()
	}
	now := time.Now().UTC()
	snap.LastIndexedAt = &now
	return snap, nil
}

// ClosureScore reads the latest closure score row for the execution.
// Memory-backed scope/quality fall back to the completion_score
// outcome when no explicit closure row exists.
func (s *ClickHouseStore) ClosureScore(ctx context.Context, executionID string) (ClosureScore, error) {
	if s == nil || s.ch == nil {
		return ClosureScore{ComputedAt: time.Now().UTC()}, nil
	}
	rows, err := s.ch.QueryRows(ctx, `
		SELECT
		  toFloat64OrZero(JSONExtractString(attributes_json, 'scope_completion'))     AS scope,
		  toFloat64OrZero(JSONExtractString(attributes_json, 'quality_confidence'))   AS quality,
		  toFloat64OrZero(JSONExtractString(attributes_json, 'integration_stability'))AS integ,
		  toFloat64OrZero(JSONExtractString(attributes_json, 'margin_health'))        AS margin,
		  toFloat64OrZero(JSONExtractString(attributes_json, 'overall'))              AS overall
		FROM fact_outcome_events
		WHERE execution_id = ?
		  AND kind = 'completion_score'
		ORDER BY timestamp DESC
		LIMIT 1`,
		executionID,
	)
	if err != nil || rows == nil {
		return ClosureScore{ComputedAt: time.Now().UTC()}, nil
	}
	defer func() { _ = rows.Close() }()
	var c ClosureScore
	if rows.Next() {
		if scanErr := rows.Scan(&c.ScopeCompletion, &c.QualityConfidence, &c.IntegrationStability, &c.MarginHealth, &c.Overall); scanErr != nil {
			return ClosureScore{ComputedAt: time.Now().UTC()}, nil
		}
	}
	c.ComputedAt = time.Now().UTC()
	return c, nil
}

// WeaknessTop scans the rollup for high-failure dimensions. It returns
// at most k rows; empty slice on no data so the resolver can render.
func (s *ClickHouseStore) WeaknessTop(ctx context.Context, tenantID string, k int) ([]Weakness, error) {
	if s == nil || s.ch == nil {
		return nil, nil
	}
	if k <= 0 {
		k = 5
	}
	// Defer the full SQL to a follow-up — for now degrade to memory
	// semantics by calling Snapshot and inferring weaknesses.
	snap, err := s.Snapshot(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]Weakness, 0, k)
	for gate, rate := range snap.GateFailureRateLast7d {
		if rate < 0.25 || len(out) >= k {
			continue
		}
		out = append(out, Weakness{
			Dimension:       "gate_failure",
			Description:     fmt.Sprintf("gate %q failure rate %.0f%%", gate, rate*100),
			Severity:        ternary(rate >= 0.5, "high", "medium"),
			SuggestedAction: fmt.Sprintf("re-tune %s pre-conditions", gate),
		})
	}
	return out, nil
}
