// Package providers — telemetry sink. Every BillingGuard call passes
// through here, so it is the natural chokepoint to capture a structured
// record per agent call: who asked, which provider answered, how many
// tokens at what cost, and how long it took.
//
// Two consumers benefit:
//   1. The operator dashboard — "which agent is the most expensive on
//      Friday afternoons" is a real question and the ledger alone does
//      not carry latency or role context.
//   2. Auto-optimisation — when the planner knows "Sonnet beat Opus on
//      this story type 9 / 10 times last week", the router can pick
//      accordingly. The telemetry feed is the input to that learning.
//
// We default to a memory ring buffer so the orchestrator boots without
// extra infra. Operators can swap in a Postgres sink, a ClickHouse
// shipper, or an OTel exporter via WithTelemetrySink.

package providers

import (
	"sync"
	"time"
)

// AgentCall is one structured row of telemetry. Times are UTC; durations
// are milliseconds for human readability in JSON dashboards.
type AgentCall struct {
	UserID          string    `json:"userId"`
	ProjectID       string    `json:"projectId,omitempty"`
	Role            string    `json:"role,omitempty"`
	Provider        string    `json:"provider"`
	Model           string    `json:"model"`
	Capabilities    []string  `json:"capabilities,omitempty"`
	InputTokens     int       `json:"inputTokens"`
	OutputTokens    int       `json:"outputTokens"`
	CacheReadTokens int       `json:"cacheReadTokens,omitempty"`
	CacheNewTokens  int       `json:"cacheNewTokens,omitempty"`
	CostUSD         float64   `json:"costUSD"`
	DurationMS      int64     `json:"durationMs"`
	StartedAt       time.Time `json:"startedAt"`
	Error           string    `json:"error,omitempty"`
}

// TelemetrySink is the operator-replaceable contract. Record() must not
// block its caller — implementations buffer + flush as needed.
type TelemetrySink interface {
	Record(c AgentCall)
	Recent(limit int) []AgentCall
}

// MemorySink is a bounded ring buffer. Default sink so the orchestrator
// always has a usable feed without external infra.
type MemorySink struct {
	mu    sync.Mutex
	calls []AgentCall
	max   int
}

func NewMemorySink(max int) *MemorySink {
	if max <= 0 {
		max = 1024
	}
	return &MemorySink{max: max, calls: make([]AgentCall, 0, max)}
}

func (s *MemorySink) Record(c AgentCall) {
	if c.StartedAt.IsZero() {
		c.StartedAt = time.Now().UTC()
	}
	s.mu.Lock()
	if len(s.calls) >= s.max {
		// Drop the oldest. Cheaper than a true ring buffer for our N.
		copy(s.calls, s.calls[1:])
		s.calls = s.calls[:len(s.calls)-1]
	}
	s.calls = append(s.calls, c)
	s.mu.Unlock()
}

// Recent returns up to `limit` most-recent records, newest first. Safe
// to call from HTTP handlers; we copy under the lock.
func (s *MemorySink) Recent(limit int) []AgentCall {
	if limit <= 0 {
		limit = 100
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.calls)
	if limit > n {
		limit = n
	}
	out := make([]AgentCall, limit)
	for i := 0; i < limit; i++ {
		out[i] = s.calls[n-1-i]
	}
	return out
}

// WithTelemetry registers a sink on the BillingGuard. The guard emits
// one AgentCall per CompleteStream, fired on DeltaDone (success) or on
// the channel close after a DeltaError (failure with whatever usage was
// reported before the error).
func (g *BillingGuard) WithTelemetry(s TelemetrySink) *BillingGuard {
	g.tel = s
	return g
}
