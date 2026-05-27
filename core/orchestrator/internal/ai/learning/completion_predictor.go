package learning

// Completion Score Predictor — V22 proprietary model #2.
//
// Given an execution's feature vector BEFORE the expensive reasoning
// loop runs, predict P(success). The signal is the input to a
// pre-execution warning gate (IRONFLYER_COMPLETION_GATE) so the
// operator can abort or top up the wallet before paying for a run
// that the model thinks is unlikely to close.
//
// Math: logistic regression with online SGD. Features are
// standardised inline (the global scaler tracks running mean +
// variance) so weights stay in a numerically sensible range without
// requiring a feature-store dependency. Per-blueprint weights are
// kept alongside a shared global vector so we can specialise the
// model when a blueprint sees enough traffic.
//
// Why pure stdlib: this lives in the orchestrator hot path; we want
// zero new go.mod bills, predictable cold-start cost, and a single
// file the auditor can read end-to-end.
//
// The Predictor is nil-safe at every entry point. An unwired
// instance returns the prior (0.5) and silently no-ops on Update —
// callers can treat it as "off" without checking for nil first.

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// featureCount is the dimension of the global feature vector. Keep
// in sync with featureSlice / featureNames below. Adding a new
// feature is a one-line append in each of those + a new entry in
// ExecutionFeatures.
const featureCount = 6

// featureNames keeps the index-to-name mapping in one place so the
// log emitter and any future explainability output stay aligned.
var featureNames = [featureCount]string{
	"prompt_tokens_norm",
	"num_gates_norm",
	"has_mobile_target",
	"tenant_history_success",
	"similar_past_success",
	"estimated_cost_usd_norm",
}

// learningRate is the SGD step size. 0.05 keeps online updates from
// thrashing the weights when a single outcome arrives; the model
// trends slowly toward the empirical distribution rather than
// oscillating per-event.
const learningRate = 0.05

// l2Regularisation is the per-step weight decay applied alongside
// the gradient update. Small enough to avoid pinning every weight to
// zero while large enough to absorb spurious correlations from low
// per-blueprint sample counts.
const l2Regularisation = 1e-4

// defaultPrior is the predicted probability when the model has no
// signal yet (no historical samples, no per-blueprint weights). 0.5
// is the maximum-entropy default — "no information, equal odds".
const defaultPrior = 0.5

// ExecutionFeatures is the input contract for Predict / Update. The
// caller assembles these from the live Execution row + the tenant /
// blueprint history projections so the predictor never has to reach
// into the execution store itself.
type ExecutionFeatures struct {
	BlueprintID          string
	PromptTokens         int
	NumGates             int
	HasMobileTarget      bool
	TenantHistorySuccess float64 // tenant lifetime success rate, [0, 1]
	SimilarPastSuccess   float64 // success rate of similar past executions, [0, 1]
	EstimatedCostUSD     float64
}

// CompletionPredictor is the online logistic-regression head. The
// global weights vector applies to every execution; per-blueprint
// weights specialise the model when a blueprint accumulates enough
// outcomes for its own coefficients to outperform the prior.
type CompletionPredictor struct {
	store Store
	log   zerolog.Logger

	mu             sync.RWMutex
	globalWeights  [featureCount]float64
	globalBias     float64
	globalSamples  int
	scaler         featureScaler
	blueprintHeads map[string]*blueprintHead
}

// blueprintHead is the per-blueprint specialisation. We keep the
// same shape as the global model so Predict can fall through to the
// global vector when a blueprint has no head yet.
type blueprintHead struct {
	weights [featureCount]float64
	bias    float64
	samples int
}

// NewCompletionPredictor constructs an empty predictor. Call
// LoadFromHistory once at boot to warm-start the weights from the
// last N days of OutcomeEvents; until then Predict returns the
// prior.
func NewCompletionPredictor(store Store, log zerolog.Logger) *CompletionPredictor {
	return &CompletionPredictor{
		store:          store,
		log:            log,
		blueprintHeads: make(map[string]*blueprintHead),
		scaler:         newFeatureScaler(),
	}
}

// Predict returns P(success) in [0, 1] for the supplied features.
// nil-safe at the receiver: returns defaultPrior when the predictor
// is not wired so callers can use the value without a nil check.
func (cp *CompletionPredictor) Predict(_ context.Context, features ExecutionFeatures) float64 {
	if cp == nil {
		return defaultPrior
	}
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	x := cp.scaler.transform(featureSlice(features))
	// Prefer the blueprint head when it has enough samples for the
	// per-blueprint signal to dominate the prior. The threshold (8)
	// is a rule-of-thumb; below it the global vector predicts
	// better than a noisy specialised head.
	const blueprintMinSamples = 8
	if features.BlueprintID != "" {
		if head, ok := cp.blueprintHeads[features.BlueprintID]; ok && head.samples >= blueprintMinSamples {
			return sigmoid(dot(head.weights[:], x[:]) + head.bias)
		}
	}
	if cp.globalSamples == 0 {
		return defaultPrior
	}
	return sigmoid(dot(cp.globalWeights[:], x[:]) + cp.globalBias)
}

// Update applies one SGD step from a realised outcome. succeeded =
// true is the positive label. Safe to call from a Publisher
// observer goroutine.
func (cp *CompletionPredictor) Update(features ExecutionFeatures, succeeded bool) error {
	if cp == nil {
		return nil
	}
	cp.mu.Lock()
	defer cp.mu.Unlock()
	raw := featureSlice(features)
	cp.scaler.observe(raw)
	x := cp.scaler.transform(raw)

	y := 0.0
	if succeeded {
		y = 1.0
	}
	// Global update.
	pred := sigmoid(dot(cp.globalWeights[:], x[:]) + cp.globalBias)
	err := pred - y
	for i := 0; i < featureCount; i++ {
		grad := err*x[i] + l2Regularisation*cp.globalWeights[i]
		cp.globalWeights[i] -= learningRate * grad
	}
	cp.globalBias -= learningRate * err
	cp.globalSamples++

	// Per-blueprint update.
	if features.BlueprintID != "" {
		head, ok := cp.blueprintHeads[features.BlueprintID]
		if !ok {
			head = &blueprintHead{}
			cp.blueprintHeads[features.BlueprintID] = head
		}
		headPred := sigmoid(dot(head.weights[:], x[:]) + head.bias)
		headErr := headPred - y
		for i := 0; i < featureCount; i++ {
			grad := headErr*x[i] + l2Regularisation*head.weights[i]
			head.weights[i] -= learningRate * grad
		}
		head.bias -= learningRate * headErr
		head.samples++
	}
	return nil
}

// LoadFromHistory walks recent OutcomeEvents from the configured
// learning Store and folds each KindExecutionComplete event into the
// online model via Update. The method is best-effort: stores that
// don't expose an event-iterator (e.g. ClickHouse without a
// FactReader) degrade to a no-op so the predictor stays at its
// prior until live traffic warms it up.
//
// lookback is the maximum age of events to fold in. 7 days is the
// operator default; longer windows let the model see more shape
// variety but bias it toward stale blueprint behaviour.
func (cp *CompletionPredictor) LoadFromHistory(ctx context.Context, lookback time.Duration) error {
	if cp == nil {
		return nil
	}
	if lookback <= 0 {
		lookback = 7 * 24 * time.Hour
	}
	cutoff := time.Now().UTC().Add(-lookback)
	events := drainStoreEvents(cp.store, cutoff)
	if len(events) == 0 {
		cp.log.Debug().Dur("lookback", lookback).Msg("completion predictor: no history events to fold in")
		return nil
	}
	// Sort by timestamp so the SGD path is deterministic — last event
	// has the greatest influence on the current weights.
	sort.SliceStable(events, func(i, j int) bool { return events[i].Timestamp.Before(events[j].Timestamp) })
	folded := 0
	for _, evt := range events {
		if evt.Kind != KindExecutionComplete {
			continue
		}
		if evt.Success == nil {
			continue
		}
		features := featuresFromEvent(evt)
		_ = cp.Update(features, *evt.Success)
		folded++
	}
	cp.log.Info().
		Int("events_folded", folded).
		Int("global_samples", cp.GlobalSamples()).
		Int("blueprint_heads", cp.BlueprintHeadCount()).
		Dur("lookback", lookback).
		Msg("completion predictor: warm-start complete")
	return nil
}

// GlobalSamples is the count of Update calls applied to the global
// vector. Surfaced for boot logging + dashboards.
func (cp *CompletionPredictor) GlobalSamples() int {
	if cp == nil {
		return 0
	}
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.globalSamples
}

// BlueprintHeadCount is the number of per-blueprint specialised
// weight sets currently held. Mirrors GlobalSamples for dashboard
// parity.
func (cp *CompletionPredictor) BlueprintHeadCount() int {
	if cp == nil {
		return 0
	}
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return len(cp.blueprintHeads)
}

// Confidence is a 0..1 estimate of how much signal the model has
// learned. It scales with the global sample count (capped at 200)
// and is surfaced in the boot log so operators see the predictor's
// readiness at a glance.
func (cp *CompletionPredictor) Confidence() float64 {
	if cp == nil {
		return 0
	}
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	const calibrationCap = 200.0
	if cp.globalSamples <= 0 {
		return 0
	}
	v := float64(cp.globalSamples) / calibrationCap
	if v > 1 {
		v = 1
	}
	return v
}

// ---------------------------------------------------------------------------
// Feature pipeline
// ---------------------------------------------------------------------------

// featureSlice projects an ExecutionFeatures into the canonical
// [featureCount]float64 vector. Booleans become 0/1; counts and
// costs flow through verbatim — the scaler downstream takes care of
// standardising the dynamic range.
func featureSlice(f ExecutionFeatures) [featureCount]float64 {
	mobile := 0.0
	if f.HasMobileTarget {
		mobile = 1.0
	}
	return [featureCount]float64{
		float64(f.PromptTokens),
		float64(f.NumGates),
		mobile,
		clamp01(f.TenantHistorySuccess),
		clamp01(f.SimilarPastSuccess),
		math.Max(0, f.EstimatedCostUSD),
	}
}

// featuresFromEvent extracts an ExecutionFeatures from an
// OutcomeEvent. The miner serialises these attributes from the
// execution_complete emitter on the engine; missing attributes
// degrade to zero so the SGD step still runs.
func featuresFromEvent(evt OutcomeEvent) ExecutionFeatures {
	f := ExecutionFeatures{}
	if bp, ok := evt.Attributes["blueprint_id"].(string); ok {
		f.BlueprintID = bp
	}
	f.PromptTokens = intAttr(evt.Attributes, "prompt_tokens")
	f.NumGates = intAttr(evt.Attributes, "num_gates")
	if mobile, ok := evt.Attributes["has_mobile_target"].(bool); ok {
		f.HasMobileTarget = mobile
	}
	f.TenantHistorySuccess = floatAttr(evt.Attributes, "tenant_history_success")
	f.SimilarPastSuccess = floatAttr(evt.Attributes, "similar_past_success")
	if evt.CostUSD != nil {
		v, _ := evt.CostUSD.Float64()
		f.EstimatedCostUSD = v
	}
	return f
}

// intAttr is a permissive int extractor — JSON numbers arrive as
// float64 through the publisher path, raw ints when the producer
// kept them typed. Either form should yield the same feature value.
func intAttr(a map[string]any, key string) int {
	switch v := a[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func floatAttr(a map[string]any, key string) float64 {
	switch v := a[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

// drainStoreEvents lifts the in-process OutcomeEvent ring out of the
// supplied Store when the concrete backend exposes it. The default
// MemoryStore in this package satisfies the optional interface;
// ClickHouseStore (durable backend) does not — for that backend a
// future SQL-driven loader will plug in here without touching the
// predictor API.
func drainStoreEvents(store Store, cutoff time.Time) []OutcomeEvent {
	if store == nil {
		return nil
	}
	type eventDrainer interface {
		EventsSince(cutoff time.Time) []OutcomeEvent
	}
	if dr, ok := store.(eventDrainer); ok {
		return dr.EventsSince(cutoff)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Feature scaler: running mean + variance, transforms inputs into
// approximate z-scores so weights stay numerically stable.
// ---------------------------------------------------------------------------

type featureScaler struct {
	mean [featureCount]float64
	m2   [featureCount]float64 // sum of squared deviations from mean
	n    int
}

func newFeatureScaler() featureScaler { return featureScaler{} }

// observe updates the running mean / variance with one new sample.
// Welford's algorithm — numerically stable across long online runs.
func (s *featureScaler) observe(x [featureCount]float64) {
	s.n++
	for i := 0; i < featureCount; i++ {
		delta := x[i] - s.mean[i]
		s.mean[i] += delta / float64(s.n)
		delta2 := x[i] - s.mean[i]
		s.m2[i] += delta * delta2
	}
}

// transform returns the standardised feature vector. When the scaler
// has fewer than two samples the variance is undefined; we fall back
// to the raw input so the SGD step still has signal to chew on.
func (s *featureScaler) transform(x [featureCount]float64) [featureCount]float64 {
	if s.n < 2 {
		return x
	}
	var out [featureCount]float64
	for i := 0; i < featureCount; i++ {
		variance := s.m2[i] / float64(s.n-1)
		if variance <= 0 {
			out[i] = x[i] - s.mean[i]
			continue
		}
		out[i] = (x[i] - s.mean[i]) / math.Sqrt(variance)
	}
	return out
}

// ---------------------------------------------------------------------------
// Math helpers
// ---------------------------------------------------------------------------

func dot(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var s float64
	for i := 0; i < n; i++ {
		s += a[i] * b[i]
	}
	return s
}

func sigmoid(z float64) float64 {
	if z >= 0 {
		ez := math.Exp(-z)
		return 1 / (1 + ez)
	}
	ez := math.Exp(z)
	return ez / (1 + ez)
}

// clamp01 is the package-shared NaN-safe [0,1] clip. adapter.go
// references it under the same name.
func clamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
