package costcascade

import (
	"context"
	"strings"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/providers"
)

// Speculative is a latency-and-cost Completer decorator that opts ELIGIBLE
// requests into the provider router's racing path by tagging the delegated
// request with providers.CapSpeculative. The router's CompleteStream sees
// that tag and calls RaceFirst(ctx, req, 2): it launches the top two
// capability-matching providers in parallel, returns the channel of the
// FIRST one to emit a committing delta, and cancels the loser (see
// providers/router.go). The net effect is "fastest correct answer wins,"
// trading a second, usually-cancelled provider call for a meaningful cut in
// p50/p95 latency on calls where any non-error answer is equally valid.
//
// THIS IS DISTINCT FROM THE VERIFICATION CASCADE. The verification cascade
// escalates a call to a stronger model when the cheap answer fails a QUALITY
// bar — it cares which model answers. Speculative racing is the opposite: it
// is only ever applied to calls where the model's IDENTITY does not change
// downstream behaviour, because the winner of a race is "whoever started
// talking first," not "whoever was best." Racing a Coder against a Coder
// would be a correctness bug — the two patches could differ and we would
// silently keep the faster, not the better, one. So eligibility here is a
// conservative, allow-listing heuristic, and the safe default is to leave a
// request untouched.
//
// CORRECTNESS ARGUMENT — why racing is safe for the eligible classes:
//
//   - Idempotent-answer calls. For intent classification, routing verdicts,
//     short brainstorm rounds, summarisation, yes/no critic-style judgements,
//     and other "any well-formed answer is acceptable" calls, two competent
//     providers return answers that are interchangeable for the caller's
//     purpose. Returning whichever arrives first cannot violate a downstream
//     invariant because no downstream step is keyed on which model produced
//     the text.
//   - Non-identity-sensitive. We refuse to race anything that is premium
//     (CapQuality/CapReasoning/CapThinking), tool-using (CapTools / non-empty
//     Tools), vision-bearing (CapVision / Attachments), or schema-bound in a
//     way we cannot vouch for. Those are exactly the calls where the model's
//     identity, a tool-call trajectory, or a strict structured contract makes
//     "first to speak" a poor proxy for "correct."
//   - Bounded blast radius. Tagging is purely additive on the delegated
//     request; if the router has fewer than two matching providers,
//     RaceFirst degrades to a plain single-provider CompleteStream, so the
//     tag is always safe to set. Any internal error in this decorator falls
//     straight through to the inner Completer with the request UNCHANGED.
//
// PREDICTED-OUTPUTS / EDIT-ACCELERATION MODE. When the prompt is an edit of
// content the model already has verbatim in req.ProjectContext (a large
// shared block appears in both), most output tokens are copied from the
// input. Such calls are excellent racing candidates — the providers are
// largely transcribing known text, so "first token wins" is an even better
// proxy for "correct," and we may tighten the MaxTokens expectation. This is
// gated behind SpecOptions.PredictedOutputs and detected by a cheap
// substring/winnowing heuristic (specSharedBlockRatio).
//
// Speculative satisfies the Completer interface and is a drop-in decorator:
// wrap it around the BillingGuard (or around the Cascade's downstream) and
// non-eligible traffic passes through byte-for-byte.
type Speculative struct {
	inner  Completer
	opts   SpecOptions
	logger zerolog.Logger
}

// SpecOptions tunes the speculative decorator. The zero value is a safe,
// production-ready default: racing enabled with conservative size bounds and
// predicted-outputs detection off.
type SpecOptions struct {
	// Disabled makes the decorator a transparent pass-through. Lets an
	// operator kill the racing behaviour without unwiring it.
	Disabled bool

	// MaxPromptChars is the upper bound on System+Prompt length for a request
	// to be eligible. Racing pays off on short-to-medium calls where the
	// second provider's wasted work is cheap; long prompts make the loser's
	// cancelled call expensive enough to erase the latency win. Default
	// specDefaultMaxPromptChars when <= 0.
	MaxPromptChars int

	// MinPromptChars guards against racing trivially short prompts where the
	// network/setup cost dominates and racing buys nothing. Default
	// specDefaultMinPromptChars when <= 0.
	MinPromptChars int

	// AllowJSONSchema permits racing schema-bound (req.JSONSchema != "")
	// requests. Off by default: a strict structured contract is the kind of
	// "identity-sensitive" call where two providers may emit subtly different
	// shapes, and "first to speak" is a poor selector. Enable only when the
	// schema is simple and every candidate provider is known to honour it.
	AllowJSONSchema bool

	// PredictedOutputs enables edit-acceleration mode: when the prompt shares
	// a large verbatim block with req.ProjectContext, keep racing and tighten
	// the MaxTokens expectation, since most output tokens are copied input.
	PredictedOutputs bool

	// PredictedMaxTokensFloor is the smallest MaxTokens we will tighten to in
	// predicted-outputs mode. Prevents an over-aggressive clamp from
	// truncating a legitimately longer edit. Default
	// specDefaultPredictedMaxTokensFloor when <= 0; ignored when
	// PredictedOutputs is false.
	PredictedMaxTokensFloor int
}

const (
	// specDefaultMaxPromptChars caps eligible prompt size. ~8 KiB of
	// System+Prompt (~2k tokens) is the band where a second, usually-
	// cancelled provider call is cheap relative to the latency saved.
	specDefaultMaxPromptChars = 8 * 1024

	// specDefaultMinPromptChars floors eligible prompt size so we don't race
	// near-empty prompts whose latency is setup-dominated.
	specDefaultMinPromptChars = 16

	// specDefaultPredictedMaxTokensFloor is the smallest MaxTokens
	// predicted-outputs mode will tighten a request to.
	specDefaultPredictedMaxTokensFloor = 256

	// specSharedBlockMinChars is the minimum length of a verbatim run that
	// the prompt and ProjectContext must share before predicted-outputs mode
	// treats the call as an edit of known content. Short coincidental
	// overlaps (a shared identifier, a boilerplate line) must not trip it.
	specSharedBlockMinChars = 256

	// specWinnowWindow is the substring window the winnowing heuristic slides
	// over the prompt when probing ProjectContext for a shared verbatim
	// block. Large enough to ignore incidental overlap, small enough to keep
	// the probe O(prompt) cheap.
	specWinnowWindow = 128
)

// NewSpeculative builds a speculative-racing decorator around inner. When
// inner is nil the decorator still satisfies Completer; its
// CompleteStreamWithFailover returns errEmptyCascade rather than panicking,
// matching the Cascade's empty-downstream contract. opts is normalised so a
// zero value is a valid production configuration.
func NewSpeculative(inner Completer, opts SpecOptions) *Speculative {
	if opts.MaxPromptChars <= 0 {
		opts.MaxPromptChars = specDefaultMaxPromptChars
	}
	if opts.MinPromptChars <= 0 {
		opts.MinPromptChars = specDefaultMinPromptChars
	}
	if opts.PredictedMaxTokensFloor <= 0 {
		opts.PredictedMaxTokensFloor = specDefaultPredictedMaxTokensFloor
	}
	return &Speculative{inner: inner, opts: opts}
}

// WithLogger attaches a zerolog.Logger for eligibility/decision telemetry.
// Returns the decorator for chaining. Logging is purely observational — a
// nil/zero logger never changes behaviour.
func (s *Speculative) WithLogger(l zerolog.Logger) *Speculative {
	if s != nil {
		s.logger = l
	}
	return s
}

// CompleteStreamWithFailover is the decorator entry point. For ELIGIBLE
// requests it tags a COPY of the request with providers.CapSpeculative (and,
// in predicted-outputs mode, may tighten MaxTokens) before delegating, so the
// router races the top providers and returns the first to commit. For every
// other request it delegates the original request byte-for-byte. It never
// mutates the caller's request, never blocks the provider stream, honours
// ctx through the inner Completer, and degrades to a plain pass-through on
// any internal error.
func (s *Speculative) CompleteStreamWithFailover(ctx context.Context, req providers.Request) (<-chan providers.Delta, error) {
	if s == nil || s.inner == nil {
		if s != nil && s.inner != nil {
			return s.inner.CompleteStreamWithFailover(ctx, req)
		}
		return nil, errEmptyCascade
	}
	if s.opts.Disabled {
		return s.inner.CompleteStreamWithFailover(ctx, req)
	}

	if !s.specEligible(req) {
		return s.inner.CompleteStreamWithFailover(ctx, req)
	}

	out := s.specTag(req)
	if s.logger.GetLevel() <= zerolog.DebugLevel {
		s.logger.Debug().
			Int("prompt_chars", len(out.System)+len(out.Prompt)).
			Int("max_tokens", out.MaxTokens).
			Bool("predicted_outputs", s.opts.PredictedOutputs).
			Msg("costcascade: speculative racing tagged")
	}
	return s.inner.CompleteStreamWithFailover(ctx, out)
}

// specEligible decides whether a request may be raced. It is deliberately
// conservative: it allow-lists "any correct answer suffices, model identity
// is irrelevant" calls and refuses everything it is unsure about. Returning
// false is always safe — the call simply runs unchanged on a single provider.
func (s *Speculative) specEligible(req providers.Request) bool {
	// Tool use: a tool-call trajectory is identity-sensitive — two providers
	// could choose different tools/arguments. Never race.
	if len(req.Tools) > 0 {
		return false
	}
	// Vision: an attachment-bearing call narrows the provider set and is
	// rarely an "any answer is fine" call. Never race. We check BOTH inline
	// attachments AND the CapVision tag — a vision call may reference its
	// image via URL/ProjectContext (no inline Attachments) yet still be
	// identity-sensitive (two vision providers can analyse the same image
	// differently, and "first to speak" picks the faster, not the better).
	if len(req.Attachments) > 0 {
		return false
	}
	for _, c := range req.Capabilities {
		if c == providers.CapVision {
			return false
		}
	}
	// A caller pinning a specific provider has already foreclosed the choice;
	// racing the chain would defeat that intent.
	if req.PreferredProvider != "" {
		return false
	}
	// Extended thinking is a premium, identity-sensitive reasoning path.
	if req.EnableThinking || req.ThinkingBudget > 0 {
		return false
	}
	// Premium-tier capabilities (quality / reasoning / thinking) are exactly
	// the identity-sensitive calls where "first to speak" is a poor selector.
	for _, c := range req.Capabilities {
		if premiumCaps[c] {
			return false
		}
	}
	// Schema-bound calls are off unless explicitly allowed: two providers may
	// emit subtly different structures, and the race picks for speed, not
	// shape-fidelity.
	if strings.TrimSpace(req.JSONSchema) != "" && !s.opts.AllowJSONSchema {
		return false
	}
	// Size band. Short-to-medium prompts keep the cancelled loser cheap; very
	// long prompts make the wasted call expensive enough to erase the win.
	n := len(req.System) + len(req.Prompt)
	if n < s.opts.MinPromptChars || n > s.opts.MaxPromptChars {
		return false
	}
	return true
}

// specTag returns a COPY of req opted into racing. It clones the capability
// slice before appending CapSpeculative so the caller's slice is never
// aliased or mutated, then — in predicted-outputs mode and only when the
// prompt is detectably an edit of req.ProjectContext — tightens MaxTokens
// toward the floor, since most output tokens are copied from the input. The
// caller's req is never modified in place.
func (s *Speculative) specTag(req providers.Request) providers.Request {
	out := req
	// Fresh capability slice with CapSpeculative appended. Never alias the
	// caller's slice — appending in place could clobber adjacent caller data.
	caps := make([]providers.Capability, len(req.Capabilities), len(req.Capabilities)+1)
	copy(caps, req.Capabilities)
	out.Capabilities = append(caps, providers.CapSpeculative)

	if s.opts.PredictedOutputs && specIsEdit(req) {
		out.MaxTokens = specTightenMaxTokens(req.MaxTokens, s.opts.PredictedMaxTokensFloor)
	}
	return out
}

// specTightenMaxTokens lowers an output-token budget for an edit-of-known-
// content call, where most tokens are copied from the input, without ever
// dropping below floor or raising an existing budget. A zero/unset budget is
// left at zero so the provider's own default applies — we only ever tighten a
// budget the caller already chose, never invent one.
func specTightenMaxTokens(current, floor int) int {
	if current <= 0 {
		return current
	}
	tightened := current / 2
	if tightened < floor {
		tightened = floor
	}
	if tightened > current {
		return current
	}
	return tightened
}

// specIsEdit reports whether req.Prompt is plausibly an edit of content the
// model already holds in req.ProjectContext — i.e. they share a large
// verbatim block. It uses a cheap winnowing probe: slide a fixed-width window
// across the prompt at a coarse stride and test whether any window is a
// substring of ProjectContext; on a hit, confirm the contiguous shared run is
// at least specSharedBlockMinChars. This is O(len(prompt)) substring probes
// and intentionally errs toward NOT firing — a false negative just means we
// skip the (optional) MaxTokens tightening; correctness is unaffected either
// way because the call is still raced.
func specIsEdit(req providers.Request) bool {
	prompt := req.Prompt
	ctxBlock := req.ProjectContext
	if len(prompt) < specSharedBlockMinChars || len(ctxBlock) < specSharedBlockMinChars {
		return false
	}
	w := specWinnowWindow
	if w > len(prompt) {
		w = len(prompt)
	}
	// Coarse stride so the probe stays cheap on large prompts; we only need
	// to find ONE anchored window, then we expand around it.
	stride := w / 2
	if stride < 1 {
		stride = 1
	}
	for i := 0; i+w <= len(prompt); i += stride {
		window := prompt[i : i+w]
		at := strings.Index(ctxBlock, window)
		if at < 0 {
			continue
		}
		// Anchor found. Measure the contiguous shared run by expanding the
		// match left and right in both buffers.
		shared := specSharedRunLen(prompt, i, ctxBlock, at, w)
		if shared >= specSharedBlockMinChars {
			return true
		}
	}
	return false
}

// specSharedRunLen returns the length of the maximal contiguous byte run that
// is identical in a (starting at ai) and b (starting at bi), given the
// initial window of `width` already known to match. It expands the match
// leftward and rightward from the anchor. Pure and bounds-checked.
func specSharedRunLen(a string, ai int, b string, bi int, width int) int {
	left := 0
	for ai-left-1 >= 0 && bi-left-1 >= 0 && a[ai-left-1] == b[bi-left-1] {
		left++
	}
	right := 0
	for ai+width+right < len(a) && bi+width+right < len(b) && a[ai+width+right] == b[bi+width+right] {
		right++
	}
	return left + width + right
}
