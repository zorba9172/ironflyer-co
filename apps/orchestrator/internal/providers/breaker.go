// Package providers — per-provider circuit breakers.
//
// Round 8 added bandit-driven failover (errors penalise the arm so the
// next Rerank pass demotes it). That keeps callers off a sick provider
// in a steady-state sense, but it still lets every request HIT the
// brittle upstream — if Anthropic is 5xxing for 30 seconds we still
// drive traffic at it 1000 times in that window before the bandit
// catches up. The breaker layer cuts those calls off at the door:
// once a provider has tripped 50%+ of its last 10 calls (or 5 in a
// row), we stop dialling for 30 seconds and emit ErrCircuitOpen
// instantly. Failover treats ErrCircuitOpen exactly like any other
// transient error, so the request transparently rolls to the next
// provider in the chain.
//
// Priority order:
//   1. Breaker:  is this provider open? skip.
//   2. Failover: classify error, advance to next provider on transient.
//   3. Bandit:   record a telemetry penalty, re-rank next request.
//
// Settings are tuned for LLM provider behaviour (long-tailed latency,
// flaky 5xx bursts on platform incidents). The counter resets every
// 60s in closed state so a single bad minute doesn't poison the breaker
// forever; the open-state timeout is 30s so a real outage gets noticed
// fast (and we probe with a single in-flight request via MaxRequests=1
// before reopening the floodgates).

package providers

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/sony/gobreaker/v2"

	"ironflyer/apps/orchestrator/internal/metrics"
)

// ErrCircuitOpen is returned by ProviderBreaker.Execute when the breaker
// is in the open state and refuses to dial the upstream. Sentinel so
// the failover layer can errors.Is(...) against it; classify it as a
// transient failure (counts as a normal failover reason so the chain
// advances).
var ErrCircuitOpen = errors.New("provider: circuit breaker open")

// ProviderBreaker wraps gobreaker for a single LLM provider. One
// breaker per provider name — Router.brakerFor lazily constructs and
// caches them so N enabled providers get N independent breakers (a
// sick Anthropic doesn't trip OpenAI's breaker).
type ProviderBreaker struct {
	name string
	cb   *gobreaker.CircuitBreaker[any]
}

// NewProviderBreaker constructs a breaker tuned for LLM provider
// traffic. Optional logger is used to log every state transition so
// operators see "anthropic: closed → open" in the orchestrator log
// before they notice in Prometheus.
func NewProviderBreaker(name string, logger zerolog.Logger, hasLogger bool) *ProviderBreaker {
	st := gobreaker.Settings{
		Name:        name,
		MaxRequests: 1,                // single probe in half-open
		Interval:    60 * time.Second, // counter reset cadence in closed state
		Timeout:     30 * time.Second, // how long the breaker stays open before half-open trial
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip when 5+ consecutive failures OR when we've seen at
			// least 10 calls and 50%+ failed. The 10-call floor avoids
			// tripping on a single early 500 during low traffic — at
			// low qps the breaker waits for a real signal.
			if counts.ConsecutiveFailures >= 5 {
				return true
			}
			if counts.Requests >= 10 {
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
				return failureRatio >= 0.5
			}
			return false
		},
		OnStateChange: func(provName string, from, to gobreaker.State) {
			if hasLogger {
				logger.Warn().
					Str("provider", provName).
					Str("from", from.String()).
					Str("to", to.String()).
					Msg("provider breaker state change")
			}
			metrics.SetProviderBreakerState(provName, breakerStateLabel(to))
			if to == gobreaker.StateOpen {
				metrics.ObserveProviderBreakerTrip(provName)
			}
		},
	}
	return &ProviderBreaker{
		name: name,
		cb:   gobreaker.NewCircuitBreaker[any](st),
	}
}

// Name returns the provider name this breaker guards. Useful for logs.
func (b *ProviderBreaker) Name() string { return b.name }

// State returns the breaker's current state string ("closed", "open",
// "half_open") for operational endpoints.
func (b *ProviderBreaker) State() string {
	return breakerStateLabel(b.cb.State())
}

// Execute runs fn through the breaker. When the breaker is open it
// returns ErrCircuitOpen immediately without calling fn. When fn
// returns a non-nil error AND that error is classifiable as a real
// provider failure (5xx / 429 / network / timeout), the breaker
// counts it as a failure. 4xx-other-than-429 errors are treated as
// breaker successes — they're caller bugs (bad JSON schema, missing
// auth header, model name typo) and shouldn't open the circuit on
// behalf of an upstream that's perfectly healthy.
func (b *ProviderBreaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	_, err := b.cb.Execute(func() (any, error) {
		callErr := fn(ctx)
		if callErr == nil {
			return nil, nil
		}
		if !isBreakerFailure(callErr) {
			// Tell gobreaker the call "succeeded" so the breaker
			// doesn't open on caller-side 4xx — but still return the
			// error to the caller so it propagates normally.
			return nil, breakerSuccessWithError{err: callErr}
		}
		return nil, callErr
	})
	if err == nil {
		return nil
	}
	// Unwrap our success-with-error sentinel so callers see the real
	// 4xx string. gobreaker treats any non-nil return as failure, so
	// we lie to it with the wrapper above and undo the lie here.
	var swe breakerSuccessWithError
	if errors.As(err, &swe) {
		return swe.err
	}
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return ErrCircuitOpen
	}
	return err
}

// breakerSuccessWithError wraps a caller-side 4xx so we can return
// "success" to gobreaker (don't blame the upstream) while still
// propagating the original error to the caller. Unwrapped in Execute.
type breakerSuccessWithError struct{ err error }

func (e breakerSuccessWithError) Error() string { return e.err.Error() }
func (e breakerSuccessWithError) Unwrap() error { return e.err }

// isBreakerFailure decides whether an error counts toward tripping the
// breaker. 5xx, 429, timeout, and network errors do. 4xx (other than
// 429) does NOT — that's a caller bug and the upstream is fine.
//
// ErrCircuitOpen itself is NOT classified as a failure here (we never
// reach this function when the breaker is open — gobreaker short-
// circuits before fn runs).
func isBreakerFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		// Caller-driven cancellation: don't blame the provider.
		return errors.Is(err, context.DeadlineExceeded)
	}
	s := strings.ToLower(err.Error())
	// 4xx other than 429 → caller error → not a breaker failure.
	if strings.Contains(s, " 400") || strings.Contains(s, " 401") ||
		strings.Contains(s, " 402") || strings.Contains(s, " 403") ||
		strings.Contains(s, " 404") || strings.Contains(s, " 405") ||
		strings.Contains(s, " 409") || strings.Contains(s, " 410") ||
		strings.Contains(s, " 413") || strings.Contains(s, " 422") ||
		strings.Contains(s, "status 400") || strings.Contains(s, "status 401") ||
		strings.Contains(s, "status 403") || strings.Contains(s, "status 404") ||
		strings.Contains(s, "status 422") {
		// 429 catches first — re-check below.
		if !(strings.Contains(s, "429") || strings.Contains(s, "rate limit") ||
			strings.Contains(s, "rate-limit") || strings.Contains(s, "too many requests")) {
			return false
		}
	}
	// Everything classifyError flags as a transient is a breaker failure.
	reason, transient := classifyError(err)
	if transient {
		return true
	}
	// reasonOther: be conservative and count it. An unrecognised error
	// from an upstream still suggests something is wrong with the
	// upstream more often than with the caller.
	_ = reason
	return true
}

// breakerStateLabel maps gobreaker.State to the stable Prometheus
// label string ("closed", "open", "half_open").
func breakerStateLabel(s gobreaker.State) string {
	switch s {
	case gobreaker.StateClosed:
		return "closed"
	case gobreaker.StateOpen:
		return "open"
	case gobreaker.StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// breakerRegistry is the lazy-construct cache hanging off Router. One
// breaker per provider name; safe for concurrent use.
type breakerRegistry struct {
	mu        sync.Mutex
	breakers  map[string]*ProviderBreaker
	logger    zerolog.Logger
	hasLogger bool
}

// newBreakerRegistry constructs an empty registry. The logger is the
// router's zerolog instance; hasLogger is false when WithLogger was
// never called (matches Router.hasLogger semantics).
func newBreakerRegistry(logger zerolog.Logger, hasLogger bool) *breakerRegistry {
	return &breakerRegistry{
		breakers:  map[string]*ProviderBreaker{},
		logger:    logger,
		hasLogger: hasLogger,
	}
}

// breakerFor returns the breaker for `name`, constructing it on first
// observation. Safe for concurrent callers.
func (r *breakerRegistry) breakerFor(name string) *ProviderBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok := r.breakers[name]; ok {
		return b
	}
	b := NewProviderBreaker(name, r.logger, r.hasLogger)
	r.breakers[name] = b
	// Seed the state gauge so dashboards show a value before the first
	// state transition fires.
	metrics.SetProviderBreakerState(name, breakerStateLabel(b.cb.State()))
	return b
}
