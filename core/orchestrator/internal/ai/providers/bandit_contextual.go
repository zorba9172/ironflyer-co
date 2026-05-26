// Package providers — contextual (LinUCB) bandit for per-tenant
// provider routing. Extends the frequentist UCB1 / Bayesian Thompson
// strategies that live in bandit.go with a feature-aware policy.
//
// LinUCB models each arm (provider) as a linear function of a context
// feature vector x ∈ R^d:
//
//	estimated reward(arm, x) = θ_arm · x
//
// where θ_arm is learnt via per-arm ridge regression:
//
//	A_arm  ← A_arm + x xᵀ     // d×d Gram matrix (init: I_d)
//	b_arm  ← b_arm + r · x    // d-vector accumulator     (init: 0)
//	θ_arm  = A_arm⁻¹ · b_arm
//
// Select scores each arm with the upper-confidence-bound:
//
//	score(arm) = θ_arm · x + α · sqrt(xᵀ · A_arm⁻¹ · x)
//
// The square-root term shrinks as A_arm accumulates information in the
// direction of x, so exploration concentrates on arm-feature
// combinations the model is unsure about. α controls how aggressively
// the policy explores.
//
// Implementation notes:
//   - Pure []float64 math. No new deps (gonum/mat is NOT in go.mod).
//   - A is kept inverted lazily via Sherman–Morrison so a per-call
//     d×d inversion is avoided. The d×d matrices are tiny (d = 14
//     here) so the cost is negligible even without the rank-1
//     update — but the rank-1 update is the textbook LinUCB
//     formulation and we keep it for numerical stability.
//   - The reward signal the RouterModel feeds in is
//     margin_per_dollar · (1.0 if execution_complete else 0.5),
//     clipped to [0, 1]. See learning/router_model.go.
//   - Safe for concurrent Select / Update via mu (RWMutex).

package providers

import (
	"context"
	"math"
	"sort"
	"sync"
)

// RouteContext is the feature payload the contextual bandit sees on
// every Select. Fields are intentionally simple types so the bandit
// can be fed from anywhere in the orchestrator without dragging the
// providers package's heavier types around.
type RouteContext struct {
	Capability   string // "reasoning" | "code" | "json" | "cheap" | "fast" | "vision" | …
	PromptTokens int    // estimated prompt size (rough upper-bound is fine)
	TenantTier   string // "free" | "pro" | "team" | "enterprise"
	TimeOfDay    int    // 0..23 — can affect provider load
	HasThinking  bool
	HasTools     bool
}

// ContextualSnapshot is the dashboard projection used by /telemetry
// endpoints. It mirrors the basic shape of the existing UCB1 telemetry
// so the operator UI can render either bandit interchangeably.
type ContextualSnapshot struct {
	Strategy        string                       `json:"strategy"`
	Dim             int                          `json:"dim"`
	Alpha           float64                      `json:"alpha"`
	Samples         int                          `json:"samples"`
	Arms            []string                     `json:"arms"`
	ExpectedReward  map[string]float64           `json:"expected_reward"`
	LastConfidence  float64                      `json:"last_confidence"`
}

// ContextualOpt configures the bandit at construction time.
type ContextualOpt func(*ContextualBandit)

// WithAlpha overrides the exploration coefficient (default 1.0).
// Higher α explores more aggressively; α=0 collapses to pure greedy.
func WithAlpha(alpha float64) ContextualOpt {
	return func(b *ContextualBandit) {
		if alpha < 0 {
			alpha = 0
		}
		b.alpha = alpha
	}
}

// linearArmModel carries the per-arm LinUCB state. Aside from the
// Gram matrix A (and its inverse) and the bias vector b, we keep a
// running sample count so Snapshot can report it without scanning the
// outbox.
type linearArmModel struct {
	A    [][]float64 // d×d gram matrix
	Ainv [][]float64 // d×d inverse, kept in sync with A
	b    []float64   // d-vector
	n    int         // number of observed (x, r) updates
}

// newArmModel returns A = ridge I_d, A^-1 = I_d / ridge, b = 0.
func newArmModel(d int) *linearArmModel {
	const ridge = 1.0 // ridge regularisation; 1.0 matches the LinUCB paper
	A := make([][]float64, d)
	Ainv := make([][]float64, d)
	for i := 0; i < d; i++ {
		A[i] = make([]float64, d)
		Ainv[i] = make([]float64, d)
		A[i][i] = ridge
		Ainv[i][i] = 1.0 / ridge
	}
	return &linearArmModel{A: A, Ainv: Ainv, b: make([]float64, d)}
}

// ContextualBandit chooses an arm (provider) given context features.
// Uses LinUCB with per-arm linear models. Safe for concurrent use.
type ContextualBandit struct {
	arms   []string
	idx    map[string]int
	models map[string]*linearArmModel
	dim    int
	alpha  float64

	mu             sync.RWMutex
	totalSamples   int
	lastConfidence float64
}

// NewContextualBandit constructs a contextual bandit over the named
// arms with the canonical feature dimensionality used by routeFeatures
// (see below). Operators wire one instance per orchestrator process
// — per-tenant separation is achieved by replaying tenant-scoped
// OutcomeEvents through Update.
func NewContextualBandit(arms []string, opts ...ContextualOpt) *ContextualBandit {
	dim := contextualFeatureDim
	b := &ContextualBandit{
		arms:   append([]string(nil), arms...),
		idx:    make(map[string]int, len(arms)),
		models: make(map[string]*linearArmModel, len(arms)),
		dim:    dim,
		alpha:  1.0,
	}
	for i, a := range arms {
		b.idx[a] = i
		b.models[a] = newArmModel(dim)
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Arms returns the bandit's arm list in registration order.
func (b *ContextualBandit) Arms() []string {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]string, len(b.arms))
	copy(out, b.arms)
	return out
}

// AddArm registers a new arm at runtime (e.g. a provider that came
// online after boot). No-op if the arm already exists.
func (b *ContextualBandit) AddArm(name string) {
	if b == nil || name == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.idx[name]; ok {
		return
	}
	b.idx[name] = len(b.arms)
	b.arms = append(b.arms, name)
	b.models[name] = newArmModel(b.dim)
}

// Alpha returns the current exploration coefficient.
func (b *ContextualBandit) Alpha() float64 {
	if b == nil {
		return 0
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.alpha
}

// Select picks the best arm for the context using LinUCB. Returns the
// empty string when the bandit has no arms registered.
//
// The ctx parameter is reserved for future per-tenant scoping
// (cancellation / deadline propagation) — the math itself is local.
func (b *ContextualBandit) Select(_ context.Context, rc RouteContext) string {
	if b == nil {
		return ""
	}
	// Full write lock: Select stamps lastConfidence and the per-call
	// cost (a handful of d×d mat-vecs with d=15) is negligible.
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.arms) == 0 {
		return ""
	}
	x := routeFeatures(rc)

	bestArm := ""
	bestScore := math.Inf(-1)
	runnerScore := math.Inf(-1)
	for _, arm := range b.arms {
		m := b.models[arm]
		if m == nil {
			continue
		}
		// θ = A^-1 · b
		theta := matVec(m.Ainv, m.b)
		mean := dot(theta, x)
		// Variance term: sqrt(xᵀ · A^-1 · x)
		Ax := matVec(m.Ainv, x)
		variance := dot(x, Ax)
		if variance < 0 {
			// Numerical guard — should never go negative on a PSD inverse.
			variance = 0
		}
		ucb := mean + b.alpha*math.Sqrt(variance)
		if ucb > bestScore {
			runnerScore = bestScore
			bestScore = ucb
			bestArm = arm
		} else if ucb > runnerScore {
			runnerScore = ucb
		}
	}

	// Confidence as the (winner − runner) margin, normalised.
	conf := 0.0
	if !math.IsInf(runnerScore, -1) && bestScore > 0 {
		margin := bestScore - runnerScore
		conf = margin / bestScore
		if conf < 0 {
			conf = 0
		}
		if conf > 1 {
			conf = 1
		}
	} else if !math.IsInf(bestScore, -1) {
		conf = 1
	}
	b.lastConfidence = conf
	return bestArm
}

// Update folds one (rc, reward) observation into the named arm's
// LinUCB model. Reward is clipped to [0, 1]. Unknown arms are
// silently registered so a provider that comes online mid-run still
// benefits from the next outcome.
func (b *ContextualBandit) Update(arm string, rc RouteContext, reward float64) {
	if b == nil || arm == "" {
		return
	}
	if reward < 0 {
		reward = 0
	}
	if reward > 1 {
		reward = 1
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	m, ok := b.models[arm]
	if !ok {
		b.idx[arm] = len(b.arms)
		b.arms = append(b.arms, arm)
		m = newArmModel(b.dim)
		b.models[arm] = m
	}
	x := routeFeatures(rc)
	// Rank-1 Sherman–Morrison update of A^-1:
	//   A' = A + x xᵀ
	//   A'^-1 = A^-1 - (A^-1 x xᵀ A^-1) / (1 + xᵀ A^-1 x)
	Ax := matVec(m.Ainv, x)
	denom := 1.0 + dot(x, Ax)
	if denom <= 0 {
		denom = 1e-9 // guard against pathological inputs
	}
	for i := 0; i < b.dim; i++ {
		for j := 0; j < b.dim; j++ {
			m.Ainv[i][j] -= (Ax[i] * Ax[j]) / denom
			m.A[i][j] += x[i] * x[j]
		}
	}
	for i := 0; i < b.dim; i++ {
		m.b[i] += reward * x[i]
	}
	m.n++
	b.totalSamples++
}

// Samples returns the lifetime count of Update calls across all arms.
// Used by the boot logger and the operator dashboard.
func (b *ContextualBandit) Samples() int {
	if b == nil {
		return 0
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.totalSamples
}

// LastConfidence is the [0,1] confidence the bandit reported on its
// most recent Select. Returns 0 before any Select has run.
func (b *ContextualBandit) LastConfidence() float64 {
	if b == nil {
		return 0
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastConfidence
}

// Snapshot returns a dashboard projection. The per-arm expected reward
// is evaluated under a neutral context vector (zeros + the always-on
// bias term) so the operator gets an apples-to-apples view rather than
// one biased by whatever capability they last queried with.
func (b *ContextualBandit) Snapshot() ContextualSnapshot {
	if b == nil {
		return ContextualSnapshot{Strategy: "contextual"}
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	snap := ContextualSnapshot{
		Strategy:       "contextual",
		Dim:            b.dim,
		Alpha:          b.alpha,
		Samples:        b.totalSamples,
		Arms:           append([]string(nil), b.arms...),
		ExpectedReward: make(map[string]float64, len(b.arms)),
		LastConfidence: b.lastConfidence,
	}
	// Neutral feature vector — only the bias term (index 0) is set.
	x := make([]float64, b.dim)
	x[0] = 1.0
	for _, arm := range b.arms {
		m := b.models[arm]
		if m == nil {
			snap.ExpectedReward[arm] = 0
			continue
		}
		theta := matVec(m.Ainv, m.b)
		snap.ExpectedReward[arm] = dot(theta, x)
	}
	// Sort arms for deterministic dashboard order.
	sort.Strings(snap.Arms)
	return snap
}

// ---- feature encoding ------------------------------------------------------

// contextualFeatureDim is the fixed dimensionality of the LinUCB
// feature vector. Bumping this means every arm's A / A^-1 / b must
// resize — we treat it as a process constant so warm-starts from
// historical OutcomeEvents stay coherent.
//
// Layout:
//
//	[0]       bias term (always 1.0)
//	[1..6]    one-hot of Capability bucket
//	          {reasoning, code, json, cheap, fast, vision}
//	[7]       log10(1 + PromptTokens) / 6                  ∈ [0, ~1]
//	[8..11]   one-hot of TenantTier {free, pro, team, enterprise}
//	[12]      TimeOfDay / 23                                ∈ [0, 1]
//	[13]      HasThinking (1.0 / 0.0)
//	[14]      HasTools    (1.0 / 0.0)
const contextualFeatureDim = 15

// routeFeatures encodes a RouteContext into the canonical 15-dim
// feature vector consumed by every per-arm linear model.
func routeFeatures(rc RouteContext) []float64 {
	x := make([]float64, contextualFeatureDim)
	x[0] = 1.0 // bias

	switch rc.Capability {
	case "reasoning":
		x[1] = 1
	case "code":
		x[2] = 1
	case "json":
		x[3] = 1
	case "cheap":
		x[4] = 1
	case "fast":
		x[5] = 1
	case "vision":
		x[6] = 1
	}

	if rc.PromptTokens > 0 {
		// log10 scaling so 100 tokens vs 10_000 tokens isn't a 100×
		// difference in feature space. The /6 normalises a 1M-token
		// prompt to roughly 1.0.
		x[7] = math.Log10(1+float64(rc.PromptTokens)) / 6.0
		if x[7] > 1 {
			x[7] = 1
		}
	}

	switch rc.TenantTier {
	case "free":
		x[8] = 1
	case "pro":
		x[9] = 1
	case "team":
		x[10] = 1
	case "enterprise":
		x[11] = 1
	}

	if rc.TimeOfDay >= 0 && rc.TimeOfDay <= 23 {
		x[12] = float64(rc.TimeOfDay) / 23.0
	}
	if rc.HasThinking {
		x[13] = 1
	}
	if rc.HasTools {
		x[14] = 1
	}
	return x
}

// ---- tiny linalg helpers ---------------------------------------------------

// matVec computes M · v for a d×d matrix M and a d-vector v.
func matVec(M [][]float64, v []float64) []float64 {
	d := len(v)
	out := make([]float64, d)
	for i := 0; i < d; i++ {
		row := M[i]
		s := 0.0
		for j := 0; j < d; j++ {
			s += row[j] * v[j]
		}
		out[i] = s
	}
	return out
}

// dot is the standard inner product of two equal-length vectors.
func dot(a, b []float64) float64 {
	s := 0.0
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}
