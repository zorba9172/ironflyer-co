package costcascade

import (
	"context"
	"math"
	"strings"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/inference"
	"ironflyer/core/orchestrator/internal/ai/providers"
)

// difficulty_scorer.go implements a RouteLLM-class difficulty-aware router:
// given a request it returns a difficulty score in [0,1] (higher = harder =
// deserves a bigger model). The Classifier consumes that score and maps it to
// the cheapest viable tier via the package's locked thresholds — d < 0.33 →
// reflex (Haiku/4o-mini/Flash), 0.33 ≤ d < 0.66 → planning (Sonnet/4o/Pro),
// d ≥ 0.66 → reasoning (Opus/o3). Routing the *easy* majority of traffic down
// to the reflex tier is where the savings live; routing the genuinely hard
// minority up to reasoning is where correctness is protected.
//
// The router is deterministic, fast, allocation-light, and side-effect-free.
// Its primary signal is a hand-tuned lexical/structural heuristic that is
// ALWAYS available (no model load, no network). An optional learned intent
// classifier is folded in as a *booster* when, and only when, it reports
// Available — the heuristic never depends on it and the router never blocks
// on it. Any internal trouble (nil receiver, classifier error, pathological
// input) degrades to "return the heuristic score" or, at worst, to a neutral
// mid score; it never panics and never overrides an explicit premium request
// (that floor is enforced upstream in Classifier.Classify, not here).

// --- calibration constants ------------------------------------------------
//
// The thresholds 0.33 and 0.66 are owned by the Classifier; this file is
// calibrated so that the score *distribution* lands traffic sensibly across
// them. The design intent:
//
//   - A bare, short, capability-free prompt ("rename this var", "format this
//     JSON") sits near 0.0–0.2 and routes to reflex.
//   - A typical feature request with some code and a structured output sits
//     around 0.35–0.6 and routes to planning.
//   - A request that combines hard-problem keywords (architecture,
//     concurrency, distributed, security…), long context, heavy code, tools,
//     and/or extended thinking pushes past 0.66 and routes to reasoning.
//
// Each feature contributes a bounded sub-score in [0,1]; the sub-scores are
// combined as a weighted sum (the weights below) and the linear combination
// is squashed through a logistic so the extremes saturate gracefully instead
// of clipping hard. The logistic centre/steepness are chosen so a raw
// weighted sum of difficultyMidpoint maps to ~0.5.

const (
	// difficultyDefaultMaxPromptChars is the prompt length (in characters)
	// at which the length feature saturates to 1.0. ~6k chars ≈ 1.5k tokens
	// of user intent, which is already firmly "complex brief" territory.
	difficultyDefaultMaxPromptChars = 6000

	// difficultyLogisticSteepness (k) and difficultyLogisticMidpoint (x0)
	// parameterise the final squashing logistic 1/(1+e^{-k(x-x0)}). The
	// midpoint sits at the weighted-sum value we consider "average
	// difficulty"; steepness controls how sharply the score moves around it.
	// These are tuned so that an empty weighted sum yields a low-but-nonzero
	// score and a fully-saturated sum approaches (without reaching) 1.0.
	difficultyLogisticSteepness = 6.0
	difficultyLogisticMidpoint  = 0.45
)

// difficultyWeights documents the relative pull of each heuristic feature on
// the pre-squash weighted sum. They are intentionally NOT normalised to 1.0:
// the squashing logistic absorbs the scale, and keeping them as readable
// "importance" numbers makes the calibration auditable. Rationale per field
// is inline.
type difficultyWeights struct {
	length    float64 // longer brief → more to reason about
	keyword   float64 // architectural / multi-step vocabulary is the strongest single tell
	code      float64 // fenced blocks + dense symbols imply real implementation work
	schema    float64 // a JSONSchema target adds structure-following load
	tools     float64 // each tool the model must orchestrate raises planning depth
	thinking  float64 // the caller already asked to think → treat as harder
	capReason float64 // CapReasoning/CapCode hints from upstream routing
}

// difficultyDefaultWeights is the locked baseline calibration. keyword is the
// heaviest because hard-problem vocabulary is the highest-precision signal we
// have for "this needs a frontier model"; length and code are the next tier
// because they correlate with how much the model must hold and produce;
// schema/tools/thinking/capReason are smaller nudges that compound rather than
// dominate.
var difficultyDefaultWeights = difficultyWeights{
	length:    0.18,
	keyword:   0.38,
	code:      0.24,
	schema:    0.08,
	tools:     0.14,
	thinking:  0.16,
	capReason: 0.20,
}

// difficultyHardKeywords are the multi-step / architectural / high-stakes
// terms whose presence reliably predicts that a small model will under-deliver.
// They are matched case-insensitively as substrings against system+prompt, so
// "refactoring" matches "refactor" and "concurrent" matches "concurren". Keep
// this list lexical and high-precision: a false positive only costs a tier
// upgrade, but a noisy term would erode the whole signal.
var difficultyHardKeywords = []string{
	"architecture", "architect",
	"refactor",
	"design",
	"distributed",
	"concurrency", "concurren", "goroutine", "race condition", "deadlock",
	"algorithm",
	"optimize", "optimise", "performance",
	"security", "vulnerab", "exploit", "auth",
	"debug", "root cause", "stack trace", "panic",
	"migrate", "migration",
	"schema",
	"scalab", "throughput", "latency",
	"transaction", "consistency",
}

// difficultyIntentClassifier is the minimal contract this router needs from an
// injected learned classifier. It is declared locally (not imported from the
// caller) so wiring stays decoupled — anything that can turn a request into an
// inference.IntentPrediction satisfies it. The orchestrator typically adapts
// inference.ClassifyIntent behind a tiny closure that builds the feature
// vector; this router only ever reads the returned Label/Confidence/Available.
//
// The method MUST be fast and non-blocking on the inference path: an
// unavailable or slow classifier should return Available=false (or a nil
// error with the zero prediction) so the router falls straight back to the
// heuristic. It MUST NOT panic.
type difficultyIntentClassifier interface {
	// Classify maps a request to a learned intent prediction. A nil error
	// with Available=false means "model not loaded / not confident enough —
	// use the heuristic", which is the inference package's own contract for
	// ClassifyIntent.
	Classify(ctx context.Context, req providers.Request) (inference.IntentPrediction, error)
}

// DifficultyOptions configures a DifficultyRouter. The zero value is valid and
// yields the locked default calibration with no intent classifier attached
// (pure heuristic) — so `NewDifficultyRouter(DifficultyOptions{})` is always a
// safe, production-ready router.
type DifficultyOptions struct {
	// Logger is used for debug-level traces only; the hot path emits nothing
	// above debug. A zero zerolog.Logger is fine (writes are discarded).
	Logger zerolog.Logger

	// Weights overrides the default feature weights. Any zero-valued field is
	// left at its difficultyDefaultWeights value, so callers can nudge a
	// single dimension without re-specifying the whole struct.
	Weights difficultyWeights

	// MaxPromptChars is the prompt length at which the length feature
	// saturates. <= 0 falls back to difficultyDefaultMaxPromptChars.
	MaxPromptChars int

	// Intent is the optional learned booster. nil → heuristic only. When set,
	// its Available predictions adjust the score toward "harder" for
	// build/refactor/debug intents and toward "easier" for explain/format
	// intents, scaled by the prediction's confidence and IntentBlend.
	Intent difficultyIntentClassifier

	// IntentBlend caps how far an available intent prediction can move the
	// final (post-squash) score, in [0,1]. 0 disables the booster even if a
	// classifier is attached; <=0 falls back to difficultyDefaultIntentBlend.
	// The cap guarantees the heuristic remains the dominant signal — intent
	// can tilt a borderline request across a threshold but cannot, on its
	// own, drag an obviously-easy request into the reasoning tier.
	IntentBlend float64
}

// difficultyDefaultIntentBlend is the default ceiling on the learned booster's
// influence: an available, fully-confident intent prediction can shift the
// final score by at most this much. 0.18 is deliberately modest — roughly half
// the width of one routing band — so the booster nudges borderline cases
// without overpowering the always-available heuristic.
const difficultyDefaultIntentBlend = 0.18

// DifficultyRouter scores request difficulty for the cascade's tier selection.
// It satisfies the package DifficultyScorer interface and is safe for
// concurrent use: it holds only immutable configuration after construction and
// performs no mutation during Score.
type DifficultyRouter struct {
	logger      zerolog.Logger
	weights     difficultyWeights
	maxChars    int
	intent      difficultyIntentClassifier
	intentBlend float64
}

// NewDifficultyRouter builds a router from opts. It never returns nil and never
// errors: unset/invalid options fall back to the locked defaults so the result
// is always a usable, side-effect-free scorer.
func NewDifficultyRouter(opts DifficultyOptions) *DifficultyRouter {
	w := opts.Weights
	if w.length == 0 {
		w.length = difficultyDefaultWeights.length
	}
	if w.keyword == 0 {
		w.keyword = difficultyDefaultWeights.keyword
	}
	if w.code == 0 {
		w.code = difficultyDefaultWeights.code
	}
	if w.schema == 0 {
		w.schema = difficultyDefaultWeights.schema
	}
	if w.tools == 0 {
		w.tools = difficultyDefaultWeights.tools
	}
	if w.thinking == 0 {
		w.thinking = difficultyDefaultWeights.thinking
	}
	if w.capReason == 0 {
		w.capReason = difficultyDefaultWeights.capReason
	}

	maxChars := opts.MaxPromptChars
	if maxChars <= 0 {
		maxChars = difficultyDefaultMaxPromptChars
	}

	blend := opts.IntentBlend
	if blend <= 0 {
		blend = difficultyDefaultIntentBlend
	}
	if blend > 1 {
		blend = 1
	}

	return &DifficultyRouter{
		logger:      opts.Logger.With().Str("component", "costcascade.difficulty").Logger(),
		weights:     w,
		maxChars:    maxChars,
		intent:      opts.Intent,
		intentBlend: blend,
	}
}

// WithIntentClassifier attaches (or replaces) the optional learned booster on
// an already-constructed router and returns the router for chaining. nil is
// ignored so callers can safely pass a possibly-nil adapter. This is the
// decoupled seam: pass any value implementing difficultyIntentClassifier.
func (r *DifficultyRouter) WithIntentClassifier(c difficultyIntentClassifier) *DifficultyRouter {
	if r != nil && c != nil {
		r.intent = c
	}
	return r
}

// Score implements DifficultyScorer. It returns a difficulty estimate in
// [0,1]; higher means the request warrants a bigger model. The heuristic is
// always computed and squashed first; if an intent classifier is attached and
// reports Available, its bias is blended into the squashed score under the
// IntentBlend cap. A nil receiver yields a neutral 0.5 so a mis-wired router
// never silently forces every request to the cheapest tier.
func (r *DifficultyRouter) Score(ctx context.Context, req providers.Request) float64 {
	if r == nil {
		return 0.5
	}

	raw := r.heuristicSum(req)
	score := difficultyLogistic(raw)

	if r.intent != nil && r.intentBlend > 0 {
		if bias, ok := r.intentBias(ctx, req); ok {
			// bias ∈ [-1,1] (negative = easier, positive = harder). Move the
			// score toward the corresponding edge, capped by intentBlend, so
			// the learned signal can tilt but never dominate the heuristic.
			delta := bias * r.intentBlend
			if delta > 0 {
				score += delta * (1 - score) // approach 1 without overshoot
			} else {
				score += delta * score // approach 0 without undershoot
			}
		}
	}

	return difficultyClamp01(score)
}

// heuristicSum extracts the lexical/structural features and returns their
// pre-squash weighted sum. Every sub-feature is bounded to [0,1] before
// weighting so a single dimension can never blow up the total; the weights
// then express relative importance and the caller squashes the result.
func (r *DifficultyRouter) heuristicSum(req providers.Request) float64 {
	// Combine system + prompt for textual analysis; the system prompt often
	// carries the architectural framing that predicts difficulty.
	text := req.System
	if req.Prompt != "" {
		if text != "" {
			text += "\n"
		}
		text += req.Prompt
	}
	lower := strings.ToLower(text)

	// 1) Normalised length. Linear up to maxChars, then saturated at 1.0.
	lengthFeat := difficultyClamp01(float64(len(text)) / float64(r.maxChars))

	// 2) Hard-keyword density. We count *distinct* matched keywords (so a
	// prompt that says "refactor" ten times doesn't out-score one that
	// touches three different hard concepts) and saturate at four distinct
	// hits — by then the request is unambiguously hard.
	keywordFeat := difficultyKeywordFeature(lower)

	// 3) Code-heaviness: fenced code blocks (strong signal of real
	// implementation work) plus raw symbol density (a softer signal that the
	// text is code-like even without fences).
	codeFeat := difficultyCodeFeature(text)

	// 4) Structured-output target. A JSONSchema means the model must produce
	// a valid, constrained artefact — a small, binary "slightly harder" bump.
	var schemaFeat float64
	if strings.TrimSpace(req.JSONSchema) != "" {
		schemaFeat = 1.0
	}

	// 5) Tool orchestration. Each tool the model may call adds a branch of
	// planning; saturate at three tools.
	toolsFeat := difficultyClamp01(float64(len(req.Tools)) / 3.0)

	// 6) Explicit thinking. If the caller enabled extended thinking they have
	// already signalled the task is non-trivial; a budget scales the bump.
	var thinkingFeat float64
	if req.EnableThinking {
		thinkingFeat = 0.6
		if req.ThinkingBudget > 0 {
			// Treat ~8k thinking-token budget as the saturation point.
			thinkingFeat = 0.6 + 0.4*difficultyClamp01(float64(req.ThinkingBudget)/8000.0)
		}
	}

	// 7) Upstream capability hints. CapReasoning is the strongest, CapCode a
	// moderate, CapQuality a mild "the caller expects a serious answer". These
	// are hints, not the explicit-premium floor (that floor is honoured by the
	// Classifier, which skips the scorer entirely for an explicit reasoning
	// tier) — here they only inform the score.
	capFeat := difficultyCapFeature(req.Capabilities)

	w := r.weights
	sum := lengthFeat*w.length +
		keywordFeat*w.keyword +
		codeFeat*w.code +
		schemaFeat*w.schema +
		toolsFeat*w.tools +
		thinkingFeat*w.thinking +
		capFeat*w.capReason

	return sum
}

// intentBias consults the optional learned classifier and returns a bias in
// [-1,1] (negative → easier, positive → harder) scaled by the prediction's
// confidence, plus ok=true when the prediction was usable. Any error or an
// unavailable prediction returns ok=false so Score falls back to the heuristic
// alone. It never panics: a misbehaving classifier is treated as "no signal".
func (r *DifficultyRouter) intentBias(ctx context.Context, req providers.Request) (bias float64, ok bool) {
	if r == nil || r.intent == nil {
		return 0, false
	}

	pred, err := r.intent.Classify(ctx, req)
	if err != nil {
		// Degrade gracefully: a classifier error is not a request error.
		r.logger.Debug().Err(err).Msg("intent classifier errored; using heuristic only")
		return 0, false
	}
	if !pred.Available {
		return 0, false
	}

	conf := float64(pred.Confidence)
	if conf <= 0 {
		return 0, false
	}
	if conf > 1 {
		conf = 1
	}

	// Map the learned label to a difficulty direction. build/refactor/debug
	// are the implementation-heavy intents that benefit from a bigger model;
	// explain/format(unknown→explain-ish) are the cheap-to-answer intents.
	var direction float64
	switch pred.Label {
	case inference.IntentBuild, inference.IntentRefactor, inference.IntentDebug:
		direction = 1.0
	case inference.IntentDeploy:
		// Deploy work is procedural more than reasoning-heavy — mildly harder.
		direction = 0.4
	case inference.IntentExplain:
		direction = -1.0
	case inference.IntentTest:
		// We refuse test work upstream; treat as easy so we never burn a
		// frontier model on a request we won't fulfil anyway.
		direction = -0.6
	default: // IntentUnknown
		return 0, false
	}

	return direction * conf, true
}

// difficultyKeywordFeature returns the distinct-hard-keyword density in [0,1],
// saturating at four distinct matches. lower must already be lower-cased.
func difficultyKeywordFeature(lower string) float64 {
	if lower == "" {
		return 0
	}
	hits := 0
	for _, kw := range difficultyHardKeywords {
		if strings.Contains(lower, kw) {
			hits++
			if hits >= 4 {
				break
			}
		}
	}
	return difficultyClamp01(float64(hits) / 4.0)
}

// difficultyCodeFeature estimates how code-heavy the text is, in [0,1]. It
// blends two signals: fenced code blocks (``` pairs) which are a high-precision
// "here is real code" marker, and raw symbol density which catches inline
// code-like content with no fences. Fences dominate; symbol density tops it up.
func difficultyCodeFeature(text string) float64 {
	if text == "" {
		return 0
	}

	// Fenced blocks: count ``` markers; two markers = one block. Saturate at
	// two blocks (a lot of code to reason over).
	fences := strings.Count(text, "```")
	blocks := fences / 2
	fenceFeat := difficultyClamp01(float64(blocks) / 2.0)

	// Symbol density: fraction of characters that are code-ish punctuation.
	// A natural-language sentence sits well under 0.05; dense code runs 0.15+.
	var symbols int
	for _, c := range text {
		switch c {
		case '{', '}', '(', ')', '[', ']', ';', '<', '>', '=', '/', '\\', '|', '&', '*', '`':
			symbols++
		}
	}
	density := float64(symbols) / float64(len(text))
	// Map density 0..0.15 onto 0..1.
	densityFeat := difficultyClamp01(density / 0.15)

	// Fences are the stronger evidence; weight 0.65/0.35 and clamp.
	return difficultyClamp01(0.65*fenceFeat + 0.35*densityFeat)
}

// difficultyCapFeature folds upstream capability hints into [0,1]. CapReasoning
// is the strongest signal short of the explicit-premium floor; CapCode is
// moderate; CapQuality is mild. CapCheap/CapFast pull the other way because the
// caller is explicitly asking for the budget path. The result is clamped.
func difficultyCapFeature(caps []providers.Capability) float64 {
	var f float64
	for _, c := range caps {
		switch c {
		case providers.CapReasoning, providers.CapThinking:
			f += 0.7
		case providers.CapCode:
			f += 0.4
		case providers.CapQuality:
			f += 0.3
		case providers.CapCheap, providers.CapFast:
			f -= 0.3
		}
	}
	return difficultyClamp01(f)
}

// difficultyLogistic squashes the raw weighted sum into (0,1) via a standard
// logistic centred at difficultyLogisticMidpoint with slope
// difficultyLogisticSteepness. The squash gives graceful saturation: extreme
// inputs approach 0 or 1 asymptotically instead of clipping, which keeps the
// score monotonic and well-behaved near the 0.33/0.66 routing thresholds.
func difficultyLogistic(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-difficultyLogisticSteepness*(x-difficultyLogisticMidpoint)))
}

// difficultyClamp01 constrains v to [0,1] and maps a NaN to a neutral 0.5 so a
// degenerate computation can never propagate as an out-of-range score.
func difficultyClamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0.5
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
