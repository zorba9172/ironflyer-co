// Package providers — graceful failover when the bandit's top pick
// returns a transient error (5xx, 429, network glitch, or per-attempt
// timeout). The router walks down the bandit-ranked chain up to
// `failoverDepth` times before surrendering. Each attempt gets its
// own 30s per-call context so a wedged provider can't burn the whole
// request budget; the parent context still governs total deadline.
//
// Failover is committed at "first token": once any non-error delta
// has been forwarded to the caller, the chosen provider is locked
// for the rest of the stream and every subsequent error propagates
// as-is (the caller paid for those tokens already; we can't switch
// horses mid-stream without producing two partially-overlapping
// outputs).
//
// Penalties: every failed attempt records a synthetic AgentCall with
// `Error="failover: <reason>"` on the telemetry sink. The bandit re-
// reads the sink on its next Rerank pass, so the failed provider's
// arm naturally falls in the ranking without a separate state machine.

package providers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/operations/metrics"
)

// defaultFailoverDepth is the number of *extra* providers we try after
// the bandit's top pick. 2 means 3 total attempts.
const defaultFailoverDepth = 2

// defaultPerAttemptTimeout caps each individual provider call so a
// hung upstream can't blow the caller's latency budget waiting for
// the next failover slot to kick in.
const defaultPerAttemptTimeout = 30 * time.Second

// failoverReason categorises a single attempt failure for logs +
// bandit penalties. The values are stable strings so dashboards can
// alarm on them.
type failoverReason string

const (
	reason5xx     failoverReason = "5xx"
	reason429     failoverReason = "429"
	reasonNet     failoverReason = "net"
	reasonTimeout failoverReason = "timeout"
	reasonOther   failoverReason = "other"
)

// classifyError maps a provider's start-error or first-delta error
// to a failoverReason. The provider packages all stringify their
// HTTP status (see e.g. openai.go: `openai: status 500: ...`,
// gemini.go: `gemini http status 503: ...`), so a simple substring
// scan is sufficient and avoids dragging a typed error contract
// across every provider implementation.
func classifyError(err error) (failoverReason, bool) {
	if err == nil {
		return "", false
	}
	if errors.Is(err, ErrCircuitOpen) {
		// Treat an open breaker as a transient reason so the chain
		// advances to the next provider. The breaker package owns
		// state-transition metrics; here we just need failover to
		// keep walking.
		return reasonOther, true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return reasonTimeout, true
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "429"),
		strings.Contains(s, "rate limit"),
		strings.Contains(s, "rate-limit"),
		strings.Contains(s, "too many requests"):
		return reason429, true
	case strings.Contains(s, " 500"),
		strings.Contains(s, " 501"),
		strings.Contains(s, " 502"),
		strings.Contains(s, " 503"),
		strings.Contains(s, " 504"),
		strings.Contains(s, "status 5"),
		strings.Contains(s, "internal server error"),
		strings.Contains(s, "bad gateway"),
		strings.Contains(s, "service unavailable"),
		strings.Contains(s, "gateway timeout"):
		return reason5xx, true
	case strings.Contains(s, "timeout"),
		strings.Contains(s, "deadline"):
		return reasonTimeout, true
	case strings.Contains(s, "connection"),
		strings.Contains(s, "eof"),
		strings.Contains(s, "broken pipe"),
		strings.Contains(s, "reset by peer"),
		strings.Contains(s, "no such host"),
		strings.Contains(s, "i/o timeout"),
		strings.Contains(s, "tls"):
		return reasonNet, true
	}
	return reasonOther, false
}

// WithLogger attaches a zerolog logger so the failover code path can
// emit one warn line per fallover attempt. Optional — the router stays
// functional without one (logs are silently dropped).
func (r *Router) WithLogger(l zerolog.Logger) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger = l
	r.hasLogger = true
	return r
}

// WithTelemetry attaches the same TelemetrySink used by BillingGuard so
// the router can record a synthetic "failover" AgentCall for each
// failed attempt — this is the bandit penalty: the next Rerank pass
// will see the error rows and rank the failed provider lower.
func (r *Router) WithTelemetry(s TelemetrySink) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tel = s
	return r
}

// WithFailoverDepth overrides the default failover depth (2 = 3 total
// attempts). Passing 0 disables failover entirely; passing a negative
// value clamps to 0.
func (r *Router) WithFailoverDepth(depth int) *Router {
	if depth < 0 {
		depth = 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failoverDepth = depth
	r.hasFailoverDepth = true
	return r
}

// pickFailoverChain returns the bandit-ranked chain capped at
// `failoverDepth+1` entries — the head is the first attempt, the rest
// are the failovers. Empty cap means "use the configured default".
func (r *Router) pickFailoverChain(caps []Capability) []Provider {
	chain := r.PickChain(caps)
	r.mu.RLock()
	depth := r.failoverDepth
	hasDepth := r.hasFailoverDepth
	r.mu.RUnlock()
	if !hasDepth {
		depth = defaultFailoverDepth
	}
	want := depth + 1
	if want < 1 {
		want = 1
	}
	if len(chain) > want {
		chain = chain[:want]
	}
	return chain
}

// recordFailover sends a synthetic AgentCall to the telemetry sink so
// the bandit penalises the failed provider on its next Rerank pass.
// The reward update is implicit: the sink's Recent() feed becomes the
// bandit's training set, and an `Error != ""` row scores 0 in the
// reward formula.
func (r *Router) recordFailover(prov string, caps []Capability, reason failoverReason, started time.Time, tenant string) {
	r.mu.RLock()
	sink := r.tel
	r.mu.RUnlock()
	if sink == nil {
		return
	}
	sink.Record(AgentCall{
		UserID:       tenant,
		Provider:     prov,
		Capabilities: capsAsStrings(caps),
		StartedAt:    started,
		DurationMS:   time.Since(started).Milliseconds(),
		Error:        "failover: " + string(reason),
	})
}

// CompleteStreamWithFailover is the reliability-hardened entry point.
// Drives the bandit's top-N chain (N = failoverDepth+1), with a 30s
// per-attempt context and a one-shot commit on first content delta.
//
// Behaviour:
//   - Picks the chain via PickChain (bandit applied).
//   - For each candidate: opens its stream under a child context with
//     defaultPerAttemptTimeout; on a transient start-error or a pre-
//     commit DeltaError, classifies the error, records a telemetry
//     penalty, logs at warn level, and advances to the next provider.
//   - Once ANY non-error delta lands on the output channel, the
//     provider is committed: every subsequent delta (including errors)
//     is forwarded unchanged. No mid-stream switch — that would
//     double-bill and produce two partial outputs.
//   - After failoverDepth+1 exhausted candidates, emits the last
//     classified error as a final DeltaError on the output channel.
func (r *Router) CompleteStreamWithFailover(ctx context.Context, req Request) (<-chan Delta, error) {
	caps := req.Capabilities
	if len(req.Attachments) > 0 && !containsCap(caps, CapVision) {
		caps = append([]Capability{CapVision}, caps...)
	}
	chain := r.pickFailoverChain(caps)
	if len(chain) == 0 {
		return nil, errors.New("no providers registered")
	}
	if len(req.Attachments) > 0 {
		filtered := chain[:0]
		for _, p := range chain {
			if containsCap(p.Capabilities(), CapVision) {
				filtered = append(filtered, p)
			}
		}
		if len(filtered) == 0 {
			return nil, errors.New("no vision-capable provider registered for image attachments")
		}
		chain = filtered
	}

	out := make(chan Delta, 32)
	go r.runFailoverChain(ctx, out, chain, req)
	return out, nil
}

func (r *Router) runFailoverChain(ctx context.Context, out chan<- Delta, chain []Provider, req Request) {
	defer close(out)
	var lastErr error
	for idx, p := range chain {
		// Per-attempt timeout: the user's parent context still governs
		// total deadline; this just caps a single provider call.
		attemptCtx, cancel := context.WithTimeout(ctx, defaultPerAttemptTimeout)
		started := time.Now()
		breaker := r.breakerFor(p.Name())

		// committed flips true the first time we forward a non-error
		// delta to the caller. The breaker call signature only sees the
		// final per-attempt error: if commitment happened before the
		// stream failed the breaker treats this as a success (we did
		// produce output — the post-commit error doesn't condemn the
		// provider). If commitment never happened, the start-error or
		// first-delta error is the breaker's failure signal.
		committed := false
		var streamErr error

		breakerErr := breaker.Execute(attemptCtx, func(bctx context.Context) error {
			ch, err := p.CompleteStream(bctx, req)
			if err != nil {
				return err
			}
			for d := range ch {
				if !committed && d.Type == DeltaError {
					streamErr = d.Err
					return d.Err
				}
				if !committed && !isCommitDelta(d) {
					continue
				}
				committed = true
				select {
				case out <- d:
				case <-ctx.Done():
					// Caller went away. Stop forwarding; let the
					// provider goroutine wind down via the deferred
					// cancel below. Returning ctx.Err() would
					// mis-signal a provider failure to the breaker
					// (caller cancel != upstream sick) — so we
					// return nil and rely on the outer ctx check.
					return nil
				}
			}
			// Stream closed without error after at least one delta —
			// genuine success.
			return nil
		})

		if errors.Is(breakerErr, ErrCircuitOpen) {
			metrics.ObserveProviderRequest(p.Name(), "circuit_open", time.Since(started))
			lastErr = breakerErr
			reason, _ := classifyError(breakerErr)
			r.recordFailover(p.Name(), req.Capabilities, reason, started, req.TenantID)
			r.logFailover(p.Name(), nextName(chain, idx), reason)
			cancel()
			continue
		}

		if committed {
			// Even on a post-commit error we count this provider as the
			// outcome owner: it produced tokens. Don't blame the
			// upstream past commitment.
			metrics.ObserveProviderRequest(p.Name(), "success", time.Since(started))
			cancel()
			return
		}

		if breakerErr != nil {
			lastErr = breakerErr
		} else if streamErr != nil {
			lastErr = streamErr
		} else if lastErr == nil {
			lastErr = errors.New("provider closed stream before emitting any delta")
		}
		metrics.ObserveProviderRequest(p.Name(), "fail", time.Since(started))
		cancel()
		reason, _ := classifyError(lastErr)
		r.recordFailover(p.Name(), req.Capabilities, reason, started, req.TenantID)
		r.logFailover(p.Name(), nextName(chain, idx), reason)

		// Caller cancelled — stop iterating immediately so we don't
		// burn through the rest of the chain on a request the client
		// has already abandoned.
		if ctx.Err() != nil {
			out <- Delta{Type: DeltaError, Err: ctx.Err()}
			return
		}
	}
	if lastErr == nil {
		lastErr = errors.New("router: every provider in failover chain refused to start")
	}
	out <- Delta{Type: DeltaError, Err: lastErr}
}

func (r *Router) logFailover(prov, next string, reason failoverReason) {
	r.mu.RLock()
	has := r.hasLogger
	lg := r.logger
	r.mu.RUnlock()
	if !has {
		return
	}
	lg.Warn().
		Str("provider", prov).
		Str("next", next).
		Str("reason", string(reason)).
		Msg("provider failover")
}

func nextName(chain []Provider, idx int) string {
	if idx+1 < len(chain) {
		return chain[idx+1].Name()
	}
	return "(exhausted)"
}
