// Package providers — multi-armed bandit that re-ranks the router's
// candidate chain using historical performance recorded by the
// TelemetrySink.
//
// Why a bandit here:
// The router's base scoring is a static count of capability-tag overlap.
// It can tell that Claude advertises CapCode but not that Claude was
// 3× faster than Gemini on the last 50 code-capability calls in this
// deployment. The bandit closes that gap — it nudges the chain toward
// providers that historically succeeded, were fast, and were cheap on
// the same kind of task — while exploration keeps cold or rarely-used
// providers in the running.
//
// Two strategies are pluggable:
//
//   - UCB1 (default): score = mean + c · sqrt(2 · ln(total) / n).
//     Frequentist upper-confidence-bound; deterministic.
//   - ThompsonSampling: each arm carries a Beta(α, β) posterior over
//     reward; Choose samples once from each arm's posterior and picks
//     the max. Bayesian, randomized — performs better on noisy or
//     heavy-tailed rewards (provider latency / cost both qualify).
//
// Per arm = (provider, capability-set). An arm's reward on one call is
//
//	costNorm     = min(cost/MaxCostUSD, 1)
//	latNorm      = min(duration/MaxLatencyMS, 1)
//	qualityBoost = 0.2 * (qualityEMA - 0.5)   // ±0.1 swing
//	r            = clamp(1 - costNorm - latNorm + qualityBoost, 0, 1)
//
// where qualityEMA is the per-provider gate-pass rate (an EMA of gate
// verdicts, see providers/quality.go) and qualityBoost is gated by
// MinQualitySamples — providers with too few outcomes get
// qualityBoost = 0 (neutral). Errors keep their zero reward.
// Reward stays in [0, 1] so the Thompson Beta update
// (α += r, β += 1-r) is well-formed without renormalisation.

package providers

import (
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
)

// Arm captures one provider's aggregated stats inside a single Rerank
// call. Strategies consume `[]Arm` and produce a ranking.
type Arm struct {
	Provider string
	N        int     // number of matched records
	Sum      float64 // sum of rewards in [0,1]
	// Alpha / Beta — Beta-distribution posterior used by ThompsonSampling.
	// Seeded as (1, 1) so an arm with no data is sampled uniformly.
	Alpha float64
	Beta  float64
}

// Mean returns the empirical mean reward, 0 when the arm has no data.
func (a Arm) Mean() float64 {
	if a.N <= 0 {
		return 0
	}
	return a.Sum / float64(a.N)
}

// Strategy is the swappable bandit policy. Choose returns the picked
// arm index plus a confidence score in [0,1] reflecting how decisive
// the choice was. Update folds one reward into the arm in-place.
type Strategy interface {
	Name() string
	Choose(arms []Arm, total int, explore float64) (int, float64)
	Update(arm *Arm, reward float64)
}

// UCB1 — frequentist upper-confidence-bound bandit. Deterministic.
type UCB1 struct{}

func (UCB1) Name() string { return "ucb1" }

func (UCB1) Update(arm *Arm, reward float64) {
	if arm == nil {
		return
	}
	if reward < 0 {
		reward = 0
	}
	if reward > 1 {
		reward = 1
	}
	arm.N++
	arm.Sum += reward
	arm.Alpha += reward
	arm.Beta += 1 - reward
}

// Choose returns argmax of UCB1 score and a confidence equal to the
// normalised margin between winner and runner-up.
func (UCB1) Choose(arms []Arm, total int, explore float64) (int, float64) {
	if len(arms) == 0 {
		return -1, 0
	}
	lnTotal := math.Log(float64(total))
	if lnTotal < 0 || math.IsNaN(lnTotal) || math.IsInf(lnTotal, 0) {
		lnTotal = 0
	}
	scores := make([]float64, len(arms))
	for i, a := range arms {
		if a.N == 0 {
			scores[i] = explore * math.Sqrt(2*lnTotal)
			continue
		}
		scores[i] = a.Mean() + explore*math.Sqrt(2*lnTotal/float64(a.N))
	}
	winner, runner := 0, -1
	for i := 1; i < len(scores); i++ {
		if scores[i] > scores[winner] {
			runner = winner
			winner = i
		} else if runner < 0 || scores[i] > scores[runner] {
			runner = i
		}
	}
	conf := 1.0
	if runner >= 0 && scores[winner] > 0 {
		conf = (scores[winner] - scores[runner]) / scores[winner]
		if conf < 0 {
			conf = 0
		}
		if conf > 1 {
			conf = 1
		}
	} else if scores[winner] == 0 {
		conf = 0
	}
	return winner, conf
}

// ThompsonSampling — Bayesian bandit with Beta(α, β) posteriors.
// rng is owned by the strategy so a seeded operator-debug run produces
// reproducible picks; concurrent Rerank calls are serialised by mu.
type ThompsonSampling struct {
	mu  sync.Mutex
	rng *rand.Rand
	// ConfidenceSamples controls the rerun count used to compute
	// confidence (default 50).
	ConfidenceSamples int
}

// NewThompsonSampling returns a strategy seeded from the given seed.
// Passing seed == 0 reads from time-based randomness so production
// runs are non-deterministic by default.
func NewThompsonSampling(seed int64) *ThompsonSampling {
	src := rand.NewSource(seed)
	if seed == 0 {
		// Keep deterministic-by-default off in production: callers that
		// want a fixed seed pass one explicitly.
		src = rand.NewSource(rand.Int63())
	}
	return &ThompsonSampling{rng: rand.New(src), ConfidenceSamples: 50}
}

func (*ThompsonSampling) Name() string { return "thompson" }

func (t *ThompsonSampling) Update(arm *Arm, reward float64) {
	if arm == nil {
		return
	}
	if reward < 0 {
		reward = 0
	}
	if reward > 1 {
		reward = 1
	}
	arm.N++
	arm.Sum += reward
	arm.Alpha += reward
	arm.Beta += 1 - reward
}

// Choose samples once from each arm's Beta posterior, picks the
// argmax, then reruns the sampling ConfidenceSamples times to measure
// how often the same arm wins — that fraction is the confidence.
func (t *ThompsonSampling) Choose(arms []Arm, _ int, _ float64) (int, float64) {
	if len(arms) == 0 {
		return -1, 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	// Primary draw.
	winner := t.argmaxSample(arms)

	// Confidence: empirical fraction of the time `winner` wins in
	// ConfidenceSamples independent re-draws.
	samples := t.ConfidenceSamples
	if samples <= 0 {
		samples = 50
	}
	wins := 0
	for i := 0; i < samples; i++ {
		if t.argmaxSample(arms) == winner {
			wins++
		}
	}
	return winner, float64(wins) / float64(samples)
}

// argmaxSample samples one Beta draw per arm and returns argmax.
// Beta(α, β) is realised via two Gamma(α,1) and Gamma(β,1) draws —
// see Marsaglia & Tsang (2000), which handles α ≥ 1 directly and
// recurses on (α+1) with a U^(1/α) correction for α < 1.
func (t *ThompsonSampling) argmaxSample(arms []Arm) int {
	best := 0
	bestVal := -1.0
	for i, a := range arms {
		alpha := a.Alpha
		beta := a.Beta
		if alpha <= 0 {
			alpha = 1
		}
		if beta <= 0 {
			beta = 1
		}
		x := gammaMT(t.rng, alpha)
		y := gammaMT(t.rng, beta)
		var v float64
		if x+y > 0 {
			v = x / (x + y)
		}
		if v > bestVal {
			bestVal = v
			best = i
		}
	}
	return best
}

// gammaMT — Marsaglia & Tsang gamma(shape, 1) sampler. Shape must be
// positive. Returns a draw from Gamma(shape, 1).
func gammaMT(rng *rand.Rand, shape float64) float64 {
	if shape <= 0 {
		shape = 1
	}
	if shape < 1 {
		// Boost: sample at shape+1, scale by U^(1/shape).
		u := rng.Float64()
		if u <= 0 {
			u = 1e-12
		}
		return gammaMT(rng, shape+1) * math.Pow(u, 1.0/shape)
	}
	d := shape - 1.0/3.0
	c := 1.0 / math.Sqrt(9*d)
	for {
		var x, v float64
		for {
			x = rng.NormFloat64()
			v = 1 + c*x
			if v > 0 {
				break
			}
		}
		v = v * v * v
		u := rng.Float64()
		if u < 1-0.0331*x*x*x*x {
			return d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}

// StrategyFromEnv returns the strategy selected by `value`. Accepts
// "ucb1" (default, also "" / unknown) and "thompson". seed is forwarded
// to ThompsonSampling so operators can reproduce a run.
func StrategyFromEnv(value string, seed int64) Strategy {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "thompson", "ts":
		return NewThompsonSampling(seed)
	default:
		return UCB1{}
	}
}

// activeBandit holds the most recently constructed Bandit so the HTTP
// telemetry endpoint can surface the strategy name + last confidence
// without dragging the bandit instance through the API dependency
// graph. Set in Bandit.Rerank (idempotent) and on StrategyFromEnv-side
// construction via registerBandit. RWMutex-guarded.
var (
	activeBanditMu sync.RWMutex
	activeBandit   *Bandit
)

// RegisterActiveBandit records the bandit instance the router is using
// so /telemetry/bandit can report its strategy and last confidence.
// Safe to call from any goroutine; idempotent.
func RegisterActiveBandit(b *Bandit) {
	activeBanditMu.Lock()
	activeBandit = b
	activeBanditMu.Unlock()
}

// ActiveBanditInfo returns the strategy name and last-decision
// confidence reported by the live bandit. Returns ("ucb1", 0) when no
// bandit has been registered yet so the telemetry endpoint can always
// respond.
func ActiveBanditInfo() (strategy string, confidence float64) {
	activeBanditMu.RLock()
	b := activeBandit
	activeBanditMu.RUnlock()
	if b == nil {
		return "ucb1", 0
	}
	return b.StrategyName(), b.LastConfidence()
}

// SeedPrior is an operator-supplied warm-start for a new provider arm.
// Reward ∈ [0,1] is the mean we want to inject; Samples is how many
// synthetic observations of that reward we want to count. A higher
// Samples makes the prior harder to wash out — use small values (1–3)
// so the arm gets explored without dominating.
type SeedPrior struct {
	Reward  float64
	Samples int
}

// Bandit re-ranks the router's candidate chain using historical
// performance from the telemetry sink.
type Bandit struct {
	Sink         TelemetrySink
	LookbackN    int     // pull the last N records from Sink.Recent. Default 256.
	MaxCostUSD   float64 // for cost normalisation. Default 0.10 — typical agent call.
	MaxLatencyMS int64   // for latency normalisation. Default 30_000 (30s).
	// ExploreBonus is the UCB1 c-constant. Default sqrt(2) ≈ 1.414.
	ExploreBonus float64
	// Strategy selects between UCB1 and Thompson sampling. Defaults to
	// UCB1{} when nil so existing call-sites stay frozen.
	Strategy Strategy
	// Quality is the per-provider gate-pass rate source consulted on
	// every Rerank. Nil falls back to the package-global registered via
	// providers.RegisterQuality; missing both yields neutral quality
	// (qualityBoost = 0) so existing call sites keep their old reward
	// formula until the finisher is wired.
	Quality QualityStatsProvider

	// lastConfidence is the confidence reported by the most recent
	// Rerank — surfaced by LastConfidence for the telemetry endpoint.
	mu             sync.RWMutex
	lastConfidence float64
	// seedPriors gives new providers an optimistic warm-start so the
	// bandit will explore them before defaulting to incumbents. Keyed by
	// provider name. Guarded by mu.
	seedPriors map[string]SeedPrior
}

// RegisterPrior installs a warm-start prior for a provider arm. Called
// once at startup as a provider is registered (typically: a new entrant
// like the Vercel AI Gateway gets Reward=0.6, Samples=2 so it's tried a
// handful of times before incumbents lap it). Subsequent calls overwrite
// the previous prior for the same provider name.
func (b *Bandit) RegisterPrior(provider string, prior SeedPrior) {
	if b == nil || provider == "" {
		return
	}
	if prior.Reward < 0 {
		prior.Reward = 0
	}
	if prior.Reward > 1 {
		prior.Reward = 1
	}
	if prior.Samples < 0 {
		prior.Samples = 0
	}
	b.mu.Lock()
	if b.seedPriors == nil {
		b.seedPriors = map[string]SeedPrior{}
	}
	b.seedPriors[provider] = prior
	b.mu.Unlock()
}

// StrategyName returns the human-readable name of the active strategy
// ("ucb1" or "thompson"). Safe to call before Rerank has fired.
func (b *Bandit) StrategyName() string {
	if b == nil || b.Strategy == nil {
		return "ucb1"
	}
	return b.Strategy.Name()
}

// LastConfidence is the [0,1] confidence the strategy reported on its
// most recent decision. Returns 0 before any Rerank has run.
func (b *Bandit) LastConfidence() float64 {
	if b == nil {
		return 0
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastConfidence
}

// Rerank takes a chain of providers (already filtered + capability-
// scored by Pick/PickChain) plus the request's capability tags and
// returns the same providers re-ordered by descending bandit score.
// Stable when the bandit has no signal — falls back to the input order.
func (b *Bandit) Rerank(chain []Provider, caps []Capability) []Provider {
	if b == nil || b.Sink == nil || len(chain) <= 1 {
		return chain
	}

	lookback := b.LookbackN
	if lookback <= 0 {
		lookback = 256
	}
	maxCost := b.MaxCostUSD
	if maxCost <= 0 {
		maxCost = 0.10
	}
	maxLatency := b.MaxLatencyMS
	if maxLatency <= 0 {
		maxLatency = 30_000
	}
	explore := b.ExploreBonus
	if explore <= 0 {
		explore = math.Sqrt2
	}
	strategy := b.Strategy
	if strategy == nil {
		strategy = UCB1{}
	}
	// Resolve the quality source. Per-bandit field wins; otherwise we
	// pull the package-global so the integration agent can wire one
	// QualityRegistry to all bandits at boot. Nil-safe: a missing
	// provider means qualityBoost = 0.
	quality := b.Quality
	if quality == nil {
		quality = ActiveQuality()
	}

	// Snapshot priors under the lock so we don't hold mu during the
	// rest of Rerank. We pull priors BEFORE the records-empty fast
	// path so a freshly-registered provider with a warm-start prior
	// still influences the very first reranking pass.
	b.mu.RLock()
	priors := make(map[string]SeedPrior, len(b.seedPriors))
	for k, v := range b.seedPriors {
		priors[k] = v
	}
	b.mu.RUnlock()

	records := b.Sink.Recent(lookback)
	// Only short-circuit when there's neither telemetry NOR any prior
	// that applies to a provider in this chain — otherwise the prior
	// would never get a chance to nudge the order.
	if len(records) == 0 {
		anyPrior := false
		for _, p := range chain {
			if pr, ok := priors[p.Name()]; ok && pr.Samples > 0 {
				anyPrior = true
				break
			}
		}
		if !anyPrior {
			return chain
		}
	}

	// Build a quick lookup of the requested capabilities.
	wantSet := make(map[Capability]struct{}, len(caps))
	for _, c := range caps {
		wantSet[c] = struct{}{}
	}

	// Aggregate per-provider arms. Arms are seeded with Beta(1,1) so
	// Thompson sampling on an unseen provider draws from a uniform.
	// When a registered prior exists for this provider we fold it into
	// the seed (N, Sum, Alpha, Beta) so the arm starts at the prior
	// mean. The prior wears off naturally as real observations
	// accumulate.
	armIdx := make(map[string]int, len(chain))
	arms := make([]Arm, len(chain))
	for i, p := range chain {
		a := Arm{Provider: p.Name(), Alpha: 1, Beta: 1}
		if prior, ok := priors[p.Name()]; ok && prior.Samples > 0 {
			a.N = prior.Samples
			a.Sum = prior.Reward * float64(prior.Samples)
			a.Alpha += prior.Reward * float64(prior.Samples)
			a.Beta += (1 - prior.Reward) * float64(prior.Samples)
		}
		arms[i] = a
		armIdx[p.Name()] = i
	}

	total := 0
	for _, rec := range records {
		if len(wantSet) > 0 {
			overlap := false
			for _, rc := range rec.Capabilities {
				if _, ok := wantSet[Capability(rc)]; ok {
					overlap = true
					break
				}
			}
			if !overlap {
				continue
			}
		}
		i, ok := armIdx[rec.Provider]
		if !ok {
			// Provider not in the chain we are reranking — count it
			// toward `total` so UCB1's exploration term still uses the
			// real action count, but skip the arm update.
			total++
			continue
		}
		total++
		if rec.Error != "" {
			strategy.Update(&arms[i], 0)
			continue
		}
		costNorm := rec.CostUSD / maxCost
		if costNorm > 1 {
			costNorm = 1
		}
		if costNorm < 0 {
			costNorm = 0
		}
		latNorm := float64(rec.DurationMS) / float64(maxLatency)
		if latNorm > 1 {
			latNorm = 1
		}
		if latNorm < 0 {
			latNorm = 0
		}
		// qualityBoost shifts the reward by ±0.1 based on the provider's
		// EMA gate-pass rate. Below MinQualitySamples observations we
		// trust the EMA too little to act on it — boost stays at 0.
		// Above that threshold:
		//   PassRate = 1.0 ->  +0.1 (great provider)
		//   PassRate = 0.5 ->   0   (neutral)
		//   PassRate = 0.0 ->  -0.1 (bad provider)
		qualityBoost := 0.0
		if quality != nil {
			qs := quality.ProviderQuality(rec.Provider)
			if qs.N >= MinQualitySamples {
				qualityBoost = 0.2 * (qs.PassRate - 0.5)
			}
		}
		r := 1 - costNorm - latNorm + qualityBoost
		if r < 0 {
			r = 0
		}
		if r > 1 {
			r = 1
		}
		strategy.Update(&arms[i], r)
	}

	// Fold prior samples into `total` so UCB1's exploration term doesn't
	// degenerate to 0 when the only signal in this chain is a warm-start
	// prior. Each prior bumps `total` by its Samples count.
	for _, p := range chain {
		if pr, ok := priors[p.Name()]; ok && pr.Samples > 0 {
			total += pr.Samples
		}
	}
	if total == 0 {
		return chain
	}

	// Ask the strategy to pick the head of the chain, then sort the
	// rest by the same per-arm scoring so the full chain reflects the
	// bandit's preference order. For UCB1 this is identical to the
	// previous Rerank ordering; for Thompson we sort by posterior mean
	// (α / (α+β)) as a stable tiebreaker around the sampled winner.
	winner, conf := strategy.Choose(arms, total, explore)
	b.mu.Lock()
	b.lastConfidence = conf
	b.mu.Unlock()

	type ranked struct {
		p     Provider
		idx   int
		score float64
	}
	out := make([]ranked, len(chain))
	switch strategy.(type) {
	case UCB1:
		lnTotal := math.Log(float64(total))
		if lnTotal < 0 {
			lnTotal = 0
		}
		for i, p := range chain {
			a := arms[i]
			if a.N == 0 {
				out[i] = ranked{p: p, idx: i, score: explore * math.Sqrt(2*lnTotal)}
				continue
			}
			out[i] = ranked{p: p, idx: i, score: a.Mean() + explore*math.Sqrt(2*lnTotal/float64(a.N))}
		}
	default:
		// Thompson / future strategies: rank by posterior mean. The
		// sampled winner is forced to slot 0 to honour the stochastic
		// pick the strategy actually made.
		for i, p := range chain {
			a := arms[i]
			out[i] = ranked{p: p, idx: i, score: a.Alpha / (a.Alpha + a.Beta)}
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score == out[j].score {
			return out[i].idx < out[j].idx
		}
		return out[i].score > out[j].score
	})

	// If Thompson picked a non-deterministic winner that doesn't lead
	// by posterior mean, hoist it to the head; preserve the rest of
	// the sort.
	if _, isUCB := strategy.(UCB1); !isUCB && winner >= 0 && len(out) > 0 && out[0].idx != winner {
		for i := range out {
			if out[i].idx == winner {
				w := out[i]
				copy(out[1:i+1], out[0:i])
				out[0] = w
				break
			}
		}
	}

	result := make([]Provider, len(out))
	for i, r := range out {
		result[i] = r.p
	}
	return result
}
