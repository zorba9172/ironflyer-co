package forecast

// LearnedCostModel is the per-(tenant, provider, capability) running
// cost predictor that tightens ProfitGuard's pre-flight estimate over
// time. Each observed Charge call streams an actual-USD sample into a
// Welford running statistic; Predict converts the running stat into a
// CostPrediction with a mean, a ±1.5σ band, and a confidence value the
// caller (ProfitGuard) uses to decide whether to trust the learned
// estimate or fall back to the static heuristic.
//
// The model is intentionally a small in-process structure: no
// per-prediction database hit on the hot path, full state seeded from
// ClickHouse at boot, online updates via Record. Lock granularity is
// the whole map (RWMutex) because the hot path is Predict (read-only).
//
// The fallback rule (sample count < MinSamplesForTrust) is the
// constitutional safety net: with only a handful of observations the
// running mean is noise, so Predict returns the caller-supplied static
// fallback and stamps FallbackUsed=true so audit / metrics can split
// "trusted learned cost" from "static rate sheet" verdicts.

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

// MinSamplesForTrust is the per-key Welford count below which Predict
// refuses to trust its own mean and returns the supplied static
// fallback. Five samples is the smallest count at which a single
// outlier no longer dominates the running mean — same threshold the
// rest of forecast/ uses (Config.MinTenantSamples).
const MinSamplesForTrust = 5

// MaxConfidenceSamples is the count at which Predict's Confidence
// saturates at 1.0. Below this the confidence rises linearly so the
// caller can blend learned vs. static estimates smoothly.
const MaxConfidenceSamples = 40

// BandSigmas is the half-width of the [LowerBoundUSD, UpperBoundUSD]
// band in standard deviations. 1.5σ ≈ 87% of a normal distribution —
// conservative enough that ProfitGuard's "upper bound > wallet" check
// catches the realistic worst case without choking on routine noise.
const BandSigmas = 1.5

// CostKey is the (tenant, provider, capability) identity of one
// running statistic. The capability label normalises to the
// providers.Capability vocabulary ("reasoning" / "code" / "fast" /
// "cheap" / "quality"), but the model is shape-agnostic — any caller-
// chosen string works as long as Predict and Record agree.
type CostKey struct {
	TenantID   string
	Provider   string
	Capability string
}

// String renders the key for log/metric labels.
func (k CostKey) String() string {
	return fmt.Sprintf("%s/%s/%s", k.TenantID, k.Provider, k.Capability)
}

// runningStat is the Welford online mean/variance accumulator. Mean +
// M2 + Count is all the state required to update mean and variance in
// O(1) per new sample without storing the full history.
type runningStat struct {
	Mean        float64 // running mean, USD
	M2          float64 // running sum of squared deltas (variance numerator)
	Count       int64
	LastUpdated time.Time
}

// Variance is the unbiased sample variance derived from the Welford
// accumulator. Returns 0 when count < 2 because variance is undefined
// at that point.
func (r *runningStat) Variance() float64 {
	if r.Count < 2 {
		return 0
	}
	return r.M2 / float64(r.Count-1)
}

// StdDev is the square root of Variance. Same n<2 caveat.
func (r *runningStat) StdDev() float64 {
	return math.Sqrt(r.Variance())
}

// CostPrediction is the per-call output of LearnedCostModel.Predict.
// FallbackUsed=true means EstimateUSD is the caller's static
// heuristic, not the learned mean — ProfitGuard uses this to decide
// whether to skip the early-refuse rule (no learned upper bound yet).
type CostPrediction struct {
	EstimateUSD   decimal.Decimal
	LowerBoundUSD decimal.Decimal
	UpperBoundUSD decimal.Decimal
	Confidence    float64
	SampleCount   int64
	FallbackUsed  bool
}

// CostSnapshot is the dashboard projection of one (tenant, provider,
// capability) running stat. Floats — the snapshot is read-only and
// driven into Prometheus / GraphQL where decimal precision is wasted.
type CostSnapshot struct {
	Key         CostKey
	MeanUSD     float64
	StdDevUSD   float64
	SampleCount int64
	LastUpdated time.Time
}

// ClickHouseQuerier is the narrow read seam LoadFromHistory needs to
// hydrate the model from ClickHouse fact_provider_chosen rows. We
// declare it locally (rather than depending on the clickhouse package
// directly) so forecast/ stays import-light and the model can be
// exercised with any structurally-equivalent backend.
type ClickHouseQuerier interface {
	QueryRows(ctx context.Context, query string, args ...any) (ClickHouseRows, error)
}

// ClickHouseRows is the minimum row-iterator surface LoadFromHistory
// needs. Mirrors the upstream driver.Rows shape without dragging the
// driver type into this package.
type ClickHouseRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

// LearnedCostModel is the per-tenant, per-(provider, capability) cost
// predictor.
type LearnedCostModel struct {
	stats map[CostKey]*runningStat
	mu    sync.RWMutex
	log   zerolog.Logger
}

// NewLearnedCostModel constructs an empty model. The caller is
// expected to either LoadFromHistory + then keep Record streaming, or
// rely purely on online updates (the model behaves correctly either
// way; LoadFromHistory just warms the cache so the first few
// post-boot calls already enjoy learned predictions).
func NewLearnedCostModel(log zerolog.Logger) *LearnedCostModel {
	return &LearnedCostModel{
		stats: map[CostKey]*runningStat{},
		log:   log.With().Str("component", "learned_cost").Logger(),
	}
}

// Predict returns the learned cost band for key. When count <
// MinSamplesForTrust the prediction degrades to the supplied static
// fallback (FallbackUsed=true, Confidence=0) so callers can keep their
// existing static behaviour during cold start.
func (m *LearnedCostModel) Predict(key CostKey, fallback decimal.Decimal) CostPrediction {
	if m == nil {
		return CostPrediction{
			EstimateUSD:   fallback,
			LowerBoundUSD: fallback,
			UpperBoundUSD: fallback,
			Confidence:    0,
			FallbackUsed:  true,
		}
	}
	m.mu.RLock()
	st, ok := m.stats[key]
	var stat runningStat
	if ok {
		stat = *st
	}
	m.mu.RUnlock()

	if !ok || stat.Count < MinSamplesForTrust {
		return CostPrediction{
			EstimateUSD:   fallback,
			LowerBoundUSD: fallback,
			UpperBoundUSD: fallback,
			Confidence:    confidenceFromCount(stat.Count),
			SampleCount:   stat.Count,
			FallbackUsed:  true,
		}
	}

	mean := stat.Mean
	if mean < 0 {
		mean = 0
	}
	sd := stat.StdDev()
	low := mean - BandSigmas*sd
	if low < 0 {
		low = 0
	}
	high := mean + BandSigmas*sd
	return CostPrediction{
		EstimateUSD:   decimal.NewFromFloat(mean),
		LowerBoundUSD: decimal.NewFromFloat(low),
		UpperBoundUSD: decimal.NewFromFloat(high),
		Confidence:    confidenceFromCount(stat.Count),
		SampleCount:   stat.Count,
		FallbackUsed:  false,
	}
}

// Record streams one actual-USD observation into the running stat for
// key. Online Welford: stable to floating-point drift over millions of
// samples, no need to store the raw history.
//
// Non-positive samples are dropped on the floor — they would either be
// $0 cache hits (already excluded by Charge) or negative refunds (a
// different signal). Either way they would skew the cost predictor.
func (m *LearnedCostModel) Record(key CostKey, actualUSD decimal.Decimal) {
	if m == nil {
		return
	}
	if !actualUSD.IsPositive() {
		return
	}
	v, _ := actualUSD.Float64()
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return
	}
	m.mu.Lock()
	st, ok := m.stats[key]
	if !ok {
		st = &runningStat{}
		m.stats[key] = st
	}
	st.Count++
	delta := v - st.Mean
	st.Mean += delta / float64(st.Count)
	delta2 := v - st.Mean
	st.M2 += delta * delta2
	st.LastUpdated = time.Now().UTC()
	m.mu.Unlock()
}

// LoadFromHistory seeds the model from ClickHouse fact_provider_chosen
// rows recorded within the last `lookback` window. The query pulls
// (tenant_id, provider, capability, cost_usd) for every billed call;
// each row is fed through Record so the resulting state is identical
// to what online updates would have produced over the same period.
//
// Failure is best-effort: a malformed row is skipped, a query error
// is logged + returned but the model is still usable (the per-key map
// is only mutated under the same lock as Record).
func (m *LearnedCostModel) LoadFromHistory(ctx context.Context, ch ClickHouseQuerier, lookback time.Duration) error {
	if m == nil {
		return nil
	}
	if ch == nil {
		m.log.Debug().Msg("LoadFromHistory: nil ClickHouseQuerier; skipping warm-up")
		return nil
	}
	since := time.Now().UTC().Add(-lookback)
	// fact_provider_chosen schema: event_id, tenant_id, execution_id,
	// provider, model, capability, cost_usd, occurred_at. The capability
	// column is empty for legacy rows; we map "" → "default" so the
	// histogram still captures something rather than silently dropping
	// the row.
	const q = `
		SELECT tenant_id, provider, coalesce(capability, '') AS capability, cost_usd
		  FROM fact_provider_chosen
		 WHERE occurred_at >= ?
		   AND cost_usd > 0`
	rows, err := ch.QueryRows(ctx, q, since)
	if err != nil {
		m.log.Warn().Err(err).Msg("LoadFromHistory: query failed; model stays cold")
		return fmt.Errorf("learned_cost: query: %w", err)
	}
	defer rows.Close()

	var loaded int
	tenants := map[string]struct{}{}
	for rows.Next() {
		var tenant, provider, capability string
		var cost decimal.Decimal
		if err := rows.Scan(&tenant, &provider, &capability, &cost); err != nil {
			continue
		}
		if capability == "" {
			capability = "default"
		}
		key := CostKey{TenantID: tenant, Provider: provider, Capability: capability}
		m.Record(key, cost)
		tenants[tenant] = struct{}{}
		loaded++
	}
	if err := rows.Err(); err != nil {
		m.log.Warn().Err(err).Int("loaded", loaded).Msg("LoadFromHistory: iterator error")
		return fmt.Errorf("learned_cost: rows.Err: %w", err)
	}
	m.log.Info().
		Int("samples", loaded).
		Int("tenants", len(tenants)).
		Dur("lookback", lookback).
		Msg("cost_model: warmed from history")
	return nil
}

// Snapshot returns one CostSnapshot per (tenant, provider, capability)
// key the model has observed. Stable ordering (tenant → provider →
// capability) so the dashboard renders deterministically.
func (m *LearnedCostModel) Snapshot() []CostSnapshot {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	out := make([]CostSnapshot, 0, len(m.stats))
	for key, st := range m.stats {
		out = append(out, CostSnapshot{
			Key:         key,
			MeanUSD:     st.Mean,
			StdDevUSD:   st.StdDev(),
			SampleCount: st.Count,
			LastUpdated: st.LastUpdated,
		})
	}
	m.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Key.TenantID != out[j].Key.TenantID {
			return out[i].Key.TenantID < out[j].Key.TenantID
		}
		if out[i].Key.Provider != out[j].Key.Provider {
			return out[i].Key.Provider < out[j].Key.Provider
		}
		return out[i].Key.Capability < out[j].Key.Capability
	})
	return out
}

// TenantCount returns the number of distinct tenants currently
// represented in the model. Used by the boot log line.
func (m *LearnedCostModel) TenantCount() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := map[string]struct{}{}
	for k := range m.stats {
		seen[k.TenantID] = struct{}{}
	}
	return len(seen)
}

// SampleCount returns the total number of observations the model has
// accumulated across all keys.
func (m *LearnedCostModel) SampleCount() int64 {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var n int64
	for _, st := range m.stats {
		n += st.Count
	}
	return n
}

// confidenceFromCount maps a sample count to a [0, 1] confidence using
// linear saturation at MaxConfidenceSamples.
func confidenceFromCount(n int64) float64 {
	if n <= 0 {
		return 0
	}
	if n >= MaxConfidenceSamples {
		return 1
	}
	return float64(n) / float64(MaxConfidenceSamples)
}
