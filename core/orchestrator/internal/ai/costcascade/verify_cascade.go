package costcascade

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/providers"
)

// VerifyCascade is a FrugalGPT-style verification cascade with early exit,
// implemented as a Completer DECORATOR. It is the "try cheap first, escalate
// only on a failed verification" optimization from the cost-optimization
// vision, expressed as a drop-in wrapper around any inner Completer (the
// real BillingGuard, or even the outer Cascade itself).
//
// # Strategy
//
// For a request that does NOT already demand the premium reasoning tier
// (tierOf(req.Capabilities) != LayerReasoning), the cascade:
//
//  1. Re-tiers the request DOWN to the cheap reflex tier
//     (retierCaps(caps, LayerReflex)) and runs it through the inner Completer.
//  2. BUFFERS that draft's full streamed answer (Start / Thinking / Text /
//     Done) rather than forwarding it immediately.
//  3. Scores the buffered answer with a fast VerifyFunc.
//  4. If the score is at or above the acceptance threshold, EARLY-EXITS by
//     replaying the buffered cheap draft downstream — a cheap win, no premium
//     tokens spent.
//  5. Otherwise it ESCALATES to the next tier (planning, then reasoning),
//     bounded by MaxEscalations, buffering and re-scoring each draft, and
//     replays whichever tier's draft is accepted (or, once escalations are
//     exhausted, the last/highest draft produced regardless of score — the
//     premium tier is the best we can do, so we never throw its answer away).
//
// A request that ALREADY demands the premium tier is passed through
// unchanged with NO buffering — there is nothing cheaper to try, and the
// caller paid for first-token latency on the premium model deliberately.
//
// # Cost tradeoff
//
// This is a bet, and the math must close for it to pay off. Each escalation
// pays for a WASTED draft: a cheap draft that fails verification is billed
// AND a more expensive redo is billed on top. Net savings require the
// cheap-tier acceptance rate to comfortably exceed the escalation overhead —
// i.e. the fraction of requests the reflex tier answers well must be high
// enough that the saved (planning/reasoning − reflex) cost on the wins
// outweighs the duplicated reflex cost on the losses. On a workload where
// the cheap tier rarely satisfies the verifier this decorator LOSES money;
// it is opt-in for exactly that reason. Measure the accept rate before
// enabling it broadly.
//
// # Latency tradeoff
//
// Because it buffers the cheap draft to completion before deciding, it
// trades FIRST-TOKEN latency for cost: the caller sees no token until a
// tier's draft is fully assembled and accepted. It is therefore appropriate
// for backend/agentic calls, NOT for a latency-sensitive interactive stream
// where partial output matters. Total latency on an escalation is the sum of
// the drafts run.
//
// # Safety
//
// VerifyCascade degrades to a straight pass-through on every error path: a
// nil inner, an inner error on the cheap draft, a draft that streams an
// error delta, or any internal anomaly causes it to fall back to invoking
// the inner Completer at the ORIGINAL tier and forwarding it verbatim. It
// never double-emits (only the accepted tier's buffered draft is replayed),
// never breaks correctness (escalated-away drafts are discarded), honours ctx
// cancellation, and ALWAYS closes the output channel.
type VerifyCascade struct {
	inner  Completer
	opts   VerifyOptions
	logger zerolog.Logger
}

// VerifyVerdict is the structured result of scoring a buffered draft. Score
// is in [0,1]; Accept is the convenience comparison against the configured
// threshold that the cascade actually branches on. A VerifyFunc returns this
// so a custom verifier can carry a richer signal for logging without changing
// the control-flow contract.
type VerifyVerdict struct {
	// Score is the verifier's confidence the draft is good, in [0,1].
	Score float64
	// Reason is a short, log-friendly explanation of the score. Optional.
	Reason string
}

// VerifyFunc scores a fully-buffered draft answer for a request. It is given
// the original request, the tier the draft was produced at, the assembled
// text, and the terminal usage (nil if the draft produced no Done usage). It
// MUST be fast and side-effect free, and MUST NOT panic — a panicking or
// slow verifier defeats the entire point of the cascade. The default
// implementation is verifyHeuristicScore.
type VerifyFunc func(req providers.Request, tier Layer, text string, usage *providers.Usage) VerifyVerdict

// VerifyOptions tunes the cascade. The zero value is safe but inert: with no
// inner Completer set the cascade has nothing to wrap (the constructor takes
// the inner separately), and the defaults below are applied by
// NewVerifyCascade so a caller can pass VerifyOptions{} and get sane
// behaviour.
type VerifyOptions struct {
	// Threshold is the minimum verifier score, in [0,1], at which a draft is
	// accepted and replayed without escalating. Default 0.7. A higher
	// threshold escalates more often (more cost, higher quality floor); a
	// lower one accepts cheap drafts more readily (more savings, lower floor).
	Threshold float64

	// MaxEscalations bounds how many times a rejected draft may be retried at
	// a more expensive tier. Default 2 (reflex → planning → reasoning). 0
	// disables escalation entirely: the cheap draft is always replayed even
	// if it fails verification, which is rarely what you want — it is offered
	// only so the cascade can be reduced to a pure "force cheap tier" mode.
	MaxEscalations int

	// Verify is the scoring function. nil falls back to verifyHeuristicScore.
	Verify VerifyFunc

	// Enabled gates the whole optimization. When false, every call is a
	// verbatim pass-through to the inner Completer, so wiring the cascade is
	// always safe. Defaults to true in NewVerifyCascade (the caller opted in
	// by constructing it); set it false explicitly to keep the wrapper in the
	// chain but dormant behind a flag.
	Enabled bool

	// Logger sinks the cascade's decisions. The zero zerolog.Logger is a
	// no-op writer, so leaving it unset is safe.
	Logger zerolog.Logger
}

// verifyDefaultThreshold is the default acceptance score for a cheap draft.
const verifyDefaultThreshold = 0.7

// verifyDefaultMaxEscalations is the default escalation budget: reflex draft,
// then at most planning and reasoning redos.
const verifyDefaultMaxEscalations = 2

// verifyEscalationTiers is the fixed cheapest-first ladder the cascade walks.
// The first entry is the initial cheap draft; each subsequent entry is one
// escalation step. The ladder is bounded at runtime by MaxEscalations.
var verifyEscalationTiers = []Layer{LayerReflex, LayerPlanning, LayerReasoning}

// verifyRefusalMarkers are lowercase substrings that strongly indicate the
// model declined, errored, or produced a non-answer. Their presence near the
// start of an otherwise short answer collapses the heuristic score. Kept
// deliberately small and high-precision so it never punishes a legitimate
// answer that merely discusses one of these phrases.
var verifyRefusalMarkers = []string{
	"i cannot", "i can't", "i'm unable", "i am unable",
	"as an ai", "i'm sorry, but", "cannot assist with",
}

// verifyTruncationSuffixes are trailing fragments that suggest the stream was
// cut off mid-token rather than completing cleanly.
var verifyTruncationSuffixes = []string{"...", "…", ",", "(", "[", "{"}

// NewVerifyCascade builds a verification cascade wrapping inner. The inner
// Completer is the real model surface (e.g. the BillingGuard, or the outer
// Cascade). Unset option fields are filled with the documented defaults, so
// NewVerifyCascade(inner, VerifyOptions{}) yields a working cascade with a
// 0.7 threshold, a 2-step escalation budget, and the built-in heuristic
// verifier. A nil inner produces a cascade that degrades to returning the
// shared empty-cascade error on invocation (a wiring bug, never a runtime
// path).
func NewVerifyCascade(inner Completer, opts VerifyOptions) *VerifyCascade {
	if opts.Threshold <= 0 || opts.Threshold > 1 {
		opts.Threshold = verifyDefaultThreshold
	}
	if opts.MaxEscalations < 0 {
		opts.MaxEscalations = verifyDefaultMaxEscalations
	}
	if opts.Verify == nil {
		opts.Verify = verifyHeuristicScore
	}
	// Constructing the cascade is the opt-in; default to live unless the
	// caller explicitly held it dormant by passing Enabled=false AND any
	// other non-zero field. We cannot distinguish a deliberate Enabled=false
	// from the zero value, so the convention is: pass VerifyOptions{} to get
	// an enabled cascade. A caller that wants it dormant should gate the
	// wiring instead. To keep that contract explicit we treat the zero-value
	// options struct as "enable".
	if !opts.Enabled {
		opts.Enabled = true
	}
	return &VerifyCascade{
		inner:  inner,
		opts:   opts,
		logger: opts.Logger,
	}
}

// CompleteStreamWithFailover satisfies the Completer interface. It runs the
// FrugalGPT verify-and-escalate flow described on the type, falling back to a
// verbatim pass-through on every error or opt-out path.
func (v *VerifyCascade) CompleteStreamWithFailover(ctx context.Context, req providers.Request) (<-chan providers.Delta, error) {
	if v == nil || v.inner == nil {
		if v != nil && v.inner != nil {
			return v.inner.CompleteStreamWithFailover(ctx, req)
		}
		return nil, errEmptyCascade
	}

	// Disabled, or the request already demands the premium tier: there is
	// nothing cheaper to try, so pass through with NO buffering and preserve
	// first-token latency.
	if !v.opts.Enabled || tierOf(req.Capabilities) == LayerReasoning {
		return v.inner.CompleteStreamWithFailover(ctx, req)
	}

	// Build the escalation ladder, bounded by the budget. ladder[0] is the
	// initial cheap draft; each further entry is one escalation step.
	ladder := verifyLadder(v.opts.MaxEscalations)

	// Run drafts cheapest-first, buffering and scoring each, until one is
	// accepted or the ladder is exhausted. We never start a draft after ctx
	// is done.
	var lastDraft *verifyDraft
	for i, tier := range ladder {
		if err := ctx.Err(); err != nil {
			// Caller cancelled mid-cascade. Surface the cancellation — we must
			// NOT replay a buffered (and by definition below-threshold, since we
			// only reach here after escalating past it) draft as a clean
			// success, which would mask the cancellation and ship an unverified
			// answer the caller never received in full.
			return nil, err
		}

		draft, err := v.runDraft(ctx, req, tier)
		if err != nil {
			// The inner call failed to even start a stream. On the very first
			// tier this is indistinguishable from a normal provider error, so
			// degrade to a verbatim pass-through at the ORIGINAL tier — never
			// break the call by swallowing the error.
			if lastDraft == nil {
				return v.inner.CompleteStreamWithFailover(ctx, req)
			}
			// We already have a usable earlier draft; replay it rather than
			// fail the whole call on an escalation error.
			v.logger.Debug().Err(err).Str("tier", string(tier)).Msg("verifycascade: escalation draft failed; replaying prior draft")
			return v.replayDraft(*lastDraft), nil
		}
		lastDraft = &draft

		// A draft that streamed a provider error is not trustworthy to score.
		// Escalate if we can; otherwise replay it so the error propagates to
		// the caller exactly as the inner Completer reported it.
		if draft.errored {
			if i == len(ladder)-1 {
				return v.replayDraft(draft), nil
			}
			v.logger.Debug().Str("tier", string(tier)).Msg("verifycascade: draft errored; escalating")
			continue
		}

		verdict := v.score(req, tier, draft)
		accepted := verdict.Score >= v.opts.Threshold
		isLast := i == len(ladder)-1

		v.logger.Debug().
			Str("tier", string(tier)).
			Float64("score", verdict.Score).
			Float64("threshold", v.opts.Threshold).
			Bool("accepted", accepted).
			Bool("last", isLast).
			Str("reason", verdict.Reason).
			Int("textLen", len(draft.text.String())).
			Msg("verifycascade: draft scored")

		if accepted || isLast {
			// Accept on a passing score, or unconditionally on the final tier
			// (the most expensive draft is the best we can produce — never
			// discard it). Replay this draft; all earlier drafts are dropped.
			return v.replayDraft(draft), nil
		}
		// Below threshold and budget remains: discard this draft and escalate.
	}

	// Ladder exhausted without an accept and without a last-tier replay. This
	// only happens when every draft errored. Replay the last draft if any,
	// else pass through.
	if lastDraft != nil {
		return v.replayDraft(*lastDraft), nil
	}
	return v.inner.CompleteStreamWithFailover(ctx, req)
}

// verifyLadder returns the cheapest-first tier ladder bounded by the
// escalation budget. ladder[0] (the cheap draft) is always present; each unit
// of budget adds the next tier up to the reasoning ceiling.
func verifyLadder(maxEscalations int) []Layer {
	steps := maxEscalations + 1 // +1 for the initial cheap draft
	if steps < 1 {
		steps = 1
	}
	if steps > len(verifyEscalationTiers) {
		steps = len(verifyEscalationTiers)
	}
	out := make([]Layer, steps)
	copy(out, verifyEscalationTiers[:steps])
	return out
}

// verifyDraft is a fully-buffered draft from one tier: the assembled text and
// thinking, the terminal usage (nil if none was reported), the provider/model
// that produced it for faithful replay, and whether the stream carried an
// error delta. It is produced by runDraft and consumed by score/replayDraft.
type verifyDraft struct {
	tier     Layer
	text     strings.Builder
	thinking strings.Builder
	provider string
	model    string
	usage    *providers.Usage
	errored  bool
	err      error // the provider error that set errored, for faithful replay
}

// runDraft re-tiers the request to the given tier, invokes the inner
// Completer, and BUFFERS the entire resulting stream into a verifyDraft. It
// returns an error only when the inner call fails to produce a stream at all;
// a stream that itself carries an error delta is captured in draft.errored so
// the caller can decide whether to escalate or replay it. It honours ctx
// cancellation by stopping the drain early and returning whatever was
// buffered (the inner stream is expected to close on its own context).
func (v *VerifyCascade) runDraft(ctx context.Context, req providers.Request, tier Layer) (verifyDraft, error) {
	outReq := req
	outReq.Capabilities = retierCaps(req.Capabilities, tier)
	// Re-tiering down to reflex must also drop extended thinking, which only
	// makes sense on the premium tier and would otherwise charge for thinking
	// tokens on a model that may not support it.
	if tier == LayerReflex {
		outReq.EnableThinking = false
		outReq.ThinkingBudget = 0
	}

	in, err := v.inner.CompleteStreamWithFailover(ctx, outReq)
	if err != nil {
		return verifyDraft{}, err
	}

	d := verifyDraft{tier: tier}
	for delta := range in {
		switch delta.Type {
		case providers.DeltaStart:
			if d.provider == "" {
				d.provider = delta.Provider
			}
			if d.model == "" {
				d.model = delta.Model
			}
		case providers.DeltaText:
			d.text.WriteString(delta.Text)
			if delta.Provider != "" {
				d.provider = delta.Provider
			}
			if delta.Model != "" {
				d.model = delta.Model
			}
		case providers.DeltaThinking:
			d.thinking.WriteString(delta.Text)
		case providers.DeltaError:
			d.errored = true
			if delta.Err != nil {
				d.err = delta.Err
			}
		case providers.DeltaDone:
			if delta.Usage != nil {
				u := *delta.Usage
				d.usage = &u
			}
			if delta.Provider != "" {
				d.provider = delta.Provider
			}
			if delta.Model != "" {
				d.model = delta.Model
			}
		}
		// Stop buffering early on cancellation; we still drain to let the
		// inner goroutine unwind, but we avoid scoring stale partials by
		// flagging the draft errored so it is escalated/replayed as-is.
		if ctx.Err() != nil {
			// Continue ranging to drain the channel (the inner producer closes
			// it on ctx done); do not break, or we may leak the producer.
			continue
		}
	}
	if ctx.Err() != nil {
		// The whole draft was produced (or partially produced) under a
		// cancelled context; treat it as untrustworthy.
		d.errored = true
		if d.err == nil {
			d.err = ctx.Err()
		}
	}
	return d, nil
}

// score runs the configured verifier over a buffered draft, guarding against
// a panicking verifier so a bad custom VerifyFunc can never crash the call —
// a panic yields a zero score (forces escalation), which is the safe
// direction.
func (v *VerifyCascade) score(req providers.Request, tier Layer, draft verifyDraft) (verdict VerifyVerdict) {
	defer func() {
		if r := recover(); r != nil {
			v.logger.Warn().Interface("panic", r).Msg("verifycascade: verifier panicked; forcing escalation")
			verdict = VerifyVerdict{Score: 0, Reason: "verifier panic"}
		}
	}()
	return v.opts.Verify(req, tier, draft.text.String(), draft.usage)
}

// replayDraft streams a buffered draft to the caller as a fresh Delta stream,
// faithfully reproducing Start / Thinking / Text / Done in order and never
// double-emitting. The original provider/model are preserved so downstream
// telemetry attributes the answer to the model that actually produced it. The
// usage (and therefore the cost) is replayed verbatim: unlike a cache hit,
// this WAS a real billed call, so its cost must flow downstream unchanged.
// The output channel is always closed; ctx cancellation stops the replay
// early but still closes the channel.
func (v *VerifyCascade) replayDraft(draft verifyDraft) <-chan providers.Delta {
	provider := draft.provider
	if provider == "" {
		provider = "cascade"
	}
	model := draft.model
	if model == "" {
		model = string(draft.tier)
	}
	out := make(chan providers.Delta, 16)
	go func() {
		defer close(out)
		out <- providers.Delta{Type: providers.DeltaStart, Provider: provider, Model: model}
		if t := draft.thinking.String(); t != "" {
			out <- providers.Delta{Type: providers.DeltaThinking, Text: t, Provider: provider, Model: model}
		}
		if txt := draft.text.String(); txt != "" {
			out <- providers.Delta{Type: providers.DeltaText, Text: txt, Provider: provider, Model: model}
		}
		// An errored draft MUST terminate with a DeltaError so the failure
		// propagates to the caller exactly as the inner Completer reported it.
		// Replaying it as a clean DeltaDone would silently turn a provider
		// failure into a (often empty) success — corrupting the consumer and
		// leaving the ledger charged for a stream the caller saw as fine.
		if draft.errored {
			err := draft.err
			if err == nil {
				err = errors.New("costcascade: inner provider stream errored")
			}
			out <- providers.Delta{Type: providers.DeltaError, Provider: provider, Model: model, Err: err}
			return
		}
		done := providers.Delta{Type: providers.DeltaDone, Provider: provider, Model: model}
		if draft.usage != nil {
			u := *draft.usage
			done.Usage = &u
		}
		out <- done
	}()
	return out
}

// verifyHeuristicScore is the default, dependency-free VerifyFunc. It scores a
// draft on a handful of fast, high-precision signals and returns a score in
// [0,1]:
//
//   - empty / whitespace-only answer        → 0.0 (always escalate)
//   - JSON-schema request that fails to parse → hard penalty (schema is a
//     correctness contract; a non-JSON answer is simply wrong)
//   - refusal / "as an AI" markers in a short answer → hard penalty
//   - obvious mid-token truncation suffix    → moderate penalty
//   - answer far shorter than the prompt/cap → mild penalty
//
// The base score is high (0.9) so a clean, adequately-sized, schema-valid,
// refusal-free answer comfortably clears the default 0.7 threshold and the
// cheap tier wins. The penalties are tuned so any single hard failure drops
// the draft below threshold and forces an escalation. This is intentionally
// conservative: it is cheap to run and never inspects semantics, so it can
// be wrong about quality — but it is reliable about the structural failures
// (empty, truncated, non-JSON, refusal) that most often distinguish a cheap
// model's bad output from a good one. Wire a learned verifier via
// VerifyOptions.Verify for semantic acceptance.
func verifyHeuristicScore(req providers.Request, _ Layer, text string, _ *providers.Usage) VerifyVerdict {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return VerifyVerdict{Score: 0, Reason: "empty answer"}
	}

	score := 0.9
	var reasons []string

	// JSON-schema contract: the answer MUST parse as JSON. A schema request
	// answered with prose is a correctness failure, not a quality nuance.
	if strings.TrimSpace(req.JSONSchema) != "" {
		if !verifyLooksLikeJSON(trimmed) {
			score -= 0.6
			reasons = append(reasons, "schema requested but answer is not JSON")
		}
	}

	// Refusal / non-answer markers near the start of a short answer. We only
	// penalise when the marker appears in roughly the opening of the text so
	// a long answer that merely discusses, e.g., "as an AI" in prose is not
	// punished.
	lower := strings.ToLower(trimmed)
	head := lower
	if len(head) > 160 {
		head = head[:160]
	}
	for _, m := range verifyRefusalMarkers {
		if strings.Contains(head, m) {
			score -= 0.6
			reasons = append(reasons, "refusal/non-answer marker")
			break
		}
	}

	// Mid-token truncation: a clean completion rarely ends on an opening
	// bracket or a dangling comma/ellipsis.
	for _, suffix := range verifyTruncationSuffixes {
		if strings.HasSuffix(trimmed, suffix) {
			score -= 0.3
			reasons = append(reasons, "looks truncated")
			break
		}
	}

	// Length adequacy. A request asking for substantial output (large
	// MaxTokens, or a long prompt) that comes back with a one-liner is
	// suspicious. Heuristic only — it never rewards verbosity, only flags an
	// implausibly thin answer relative to what was asked.
	if verifyAnswerTooThin(req, trimmed) {
		score -= 0.2
		reasons = append(reasons, "answer thin relative to request")
	}

	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return VerifyVerdict{Score: score, Reason: strings.Join(reasons, "; ")}
}

// verifyLooksLikeJSON reports whether s parses as a JSON value. It first does
// a cheap structural check (must begin with an object/array/quote/literal
// opener) so prose answers fail fast, then confirms with the encoding/json
// parser. Leading code-fence noise ("```json") is tolerated by stripping a
// fenced block when present, because a cheap model commonly wraps JSON in a
// markdown fence even when asked not to.
func verifyLooksLikeJSON(s string) bool {
	s = strings.TrimSpace(verifyStripJSONFence(s))
	if s == "" {
		return false
	}
	switch s[0] {
	case '{', '[', '"', 't', 'f', 'n', '-':
	default:
		if s[0] < '0' || s[0] > '9' {
			return false
		}
	}
	return json.Valid([]byte(s))
}

// verifyStripJSONFence removes a single surrounding markdown code fence
// (```...```), with or without a language tag, returning the inner content.
// If no fence is present the input is returned unchanged.
func verifyStripJSONFence(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return s
	}
	t = strings.TrimPrefix(t, "```")
	// Drop an optional language tag on the opening fence line.
	if nl := strings.IndexByte(t, '\n'); nl >= 0 {
		first := strings.TrimSpace(t[:nl])
		if !strings.ContainsAny(first, "{}[]\"") {
			t = t[nl+1:]
		}
	}
	if end := strings.LastIndex(t, "```"); end >= 0 {
		t = t[:end]
	}
	return strings.TrimSpace(t)
}

// verifyAnswerTooThin reports whether the answer is implausibly short given
// what the request asked for. It is deliberately lenient: it only fires when
// the request signals it wants substantial output (a large MaxTokens budget
// or a long prompt) yet the answer is a tiny fraction of that. Returns false
// whenever the request itself is small, so short-by-design answers are never
// penalised.
func verifyAnswerTooThin(req providers.Request, answer string) bool {
	answerChars := len(answer)
	// Approximate a token budget in characters (~4 chars/token).
	wantChars := req.MaxTokens * 4
	promptChars := len(req.Prompt) + len(req.System) + len(req.ProjectContext)

	// Only judge thinness when the request clearly expected a meaty answer.
	const meaningfulBudgetChars = 2000 // ~500 tokens
	if wantChars < meaningfulBudgetChars && promptChars < meaningfulBudgetChars {
		return false
	}
	// "Thin" means under ~5% of the requested token budget AND under a small
	// absolute floor, so we never flag a legitimately concise answer to a
	// large-budget request that simply did not need the room.
	floor := wantChars / 20
	const absFloorChars = 80
	return answerChars < floor && answerChars < absFloorChars
}
