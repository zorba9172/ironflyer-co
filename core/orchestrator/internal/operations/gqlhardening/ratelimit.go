package gqlhardening

import (
	"context"
	"net/http"
	"strings"
	"time"

	"ironflyer/core/orchestrator/internal/operations/abuse"
	"ironflyer/core/orchestrator/internal/operations/ratelimit"
)

// Limiter is the trust-plane wrapper over the existing per-key token
// bucket from internal/ratelimit. The composed key is
//
//	"<tenant>:<operation>:<abuse_tier>"
//
// and the abuse tier multiplier scales how many tokens the caller
// gets to spend before being rejected. The base limiter is the
// orchestrator's existing limiter — Redis-backed in production via
// ratelimit.Wrap — so we inherit its single-source-of-truth behavior
// across replicas.
//
// Multiplier table (from abuse.Tier.Multiplier):
//
//	normal     → 1.0  (full budget)
//	elevated   → 0.5  (half budget; expensive endpoints surface 429 faster)
//	restricted → 0.1  (tenth of budget; effectively read-only feel)
//	blocked    → 0.0  (no tokens; every request rejected)
//
// The limiter is safe for concurrent use.
type Limiter struct {
	base  ratelimit.Allower
	abuse abuse.Engine

	defaultCost float64
}

// NewLimiter wires the base allower + abuse engine. defaultCost is
// the per-call token cost when the caller doesn't pass an explicit
// weight; 1.0 is the right default for the GraphQL path.
func NewLimiter(base ratelimit.Allower, eng abuse.Engine) *Limiter {
	if eng == nil {
		eng = abuse.NoopEngine{}
	}
	return &Limiter{base: base, abuse: eng, defaultCost: 1.0}
}

// Allow returns (true, 0) when the request fits the bucket, or
// (false, retryAfter) when the (tenant,operation,tier) bucket is
// empty. When the abuse engine reports TierBlocked the call is
// rejected outright without spending a token — the integration agent
// surfaces this back as a typed GraphQL error code so the client can
// distinguish "rate limited" from "abuse-blocked".
//
// A nil base limiter is treated as "no limit configured" so dev mode
// doesn't fail closed; the call still hits the abuse engine for the
// tier check.
func (l *Limiter) Allow(ctx context.Context, tenantID, userID, operation string) (bool, time.Duration, error) {
	tier := abuse.TierNormal
	if l.abuse != nil {
		_, t, err := l.abuse.Score(ctx, tenantID, userID)
		if err == nil {
			tier = t
		}
	}
	mult := tier.Multiplier()
	if mult <= 0 {
		rateLimitRejects.WithLabelValues(operation, string(tier)).Inc()
		return false, time.Second, nil
	}
	if l.base == nil {
		return true, 0, nil
	}
	key := composeKey(tenantID, operation, tier)
	cost := l.defaultCost / mult
	if cost < l.defaultCost {
		cost = l.defaultCost
	}
	ok, wait := l.base.AllowN(key, cost)
	if !ok {
		rateLimitRejects.WithLabelValues(operation, string(tier)).Inc()
	}
	return ok, wait, nil
}

// AllowOperator bypasses the abuse engine and base limiter. Used by
// the integration agent for operator-tagged requests so on-call
// activity can't be rate-limited out of triage.
func (l *Limiter) AllowOperator() (bool, time.Duration, error) {
	return true, 0, nil
}

// Reset clears the underlying bucket for the composed key. Useful for
// operator unblocks after triage.
func (l *Limiter) Reset(tenantID, operation string, tier abuse.Tier) {
	if l == nil || l.base == nil {
		return
	}
	l.base.Reset(composeKey(tenantID, operation, tier))
}

// OperationFromRequest is the canonical helper the HTTP rate-limit
// middleware uses to derive the `operation` argument passed to
// Limiter.Allow. It peeks the request body for the GraphQL
// operationName; when one is present the returned bucket name is
// "graphql:<opName>" so per-operation buckets isolate noisy callers
// from quiet ones (a runaway dashboard polling subscription cannot
// drain the budget of an unrelated mutation). When the peek yields
// nothing — non-GraphQL routes, anonymous queries, persisted-query
// hashes the peek didn't resolve, multipart uploads — we fall back to
// the structural identifier "<method> <path>" so the bucket still has
// a stable key.
//
// The function never returns an empty string; an empty fallback is
// replaced with "<method> <path>" derived from r.
func OperationFromRequest(r *http.Request, fallback string) string {
	if r == nil {
		if fallback == "" {
			return "unknown"
		}
		return fallback
	}
	if name := PeekOperationName(r); name != "" {
		return "graphql:" + name
	}
	if fallback != "" {
		return fallback
	}
	return r.Method + " " + r.URL.Path
}

func composeKey(tenantID, operation string, tier abuse.Tier) string {
	if tenantID == "" {
		tenantID = "anon"
	}
	if operation == "" {
		operation = "unknown"
	}
	var b strings.Builder
	b.Grow(len(tenantID) + len(operation) + len(tier) + 2)
	b.WriteString(tenantID)
	b.WriteByte(':')
	b.WriteString(operation)
	b.WriteByte(':')
	b.WriteString(string(tier))
	return b.String()
}
