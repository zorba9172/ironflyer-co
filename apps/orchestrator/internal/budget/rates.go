package budget

import (
	"strings"
	"sync"

	"github.com/shopspring/decimal"
)

// Rate is the price for one provider/model pair, per 1M tokens.
type Rate struct {
	Provider   string          `json:"provider"`
	Model      string          `json:"model"`
	InputUSD   decimal.Decimal `json:"inputUSD"`
	OutputUSD  decimal.Decimal `json:"outputUSD"`
	CacheReadUSD     decimal.Decimal `json:"cacheReadUSD"`
	CacheCreateUSD   decimal.Decimal `json:"cacheCreateUSD"`
	// Tier hints used by the Optimizer.
	Capability []string `json:"capability,omitempty"`
}

// RateSheet is the in-memory rate catalogue. Sourced from DB in prod.
type RateSheet struct {
	mu    sync.RWMutex
	rates []Rate
}

func NewRateSheet() *RateSheet { return &RateSheet{} }

func (rs *RateSheet) Register(r Rate) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.rates = append(rs.rates, r)
}

func (rs *RateSheet) All() []Rate {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	out := make([]Rate, len(rs.rates))
	copy(out, rs.rates)
	return out
}

func (rs *RateSheet) Find(provider, model string) (Rate, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	for _, r := range rs.rates {
		if r.Provider == provider && strings.EqualFold(r.Model, model) {
			return r, true
		}
	}
	return Rate{}, false
}

// CostOf returns the dollar cost for an actual call given token usage.
func (rs *RateSheet) CostOf(provider, model string, inTok, outTok, cacheRead, cacheCreate int) decimal.Decimal {
	r, ok := rs.Find(provider, model)
	if !ok {
		return decimal.Zero
	}
	million := decimal.NewFromInt(1_000_000)
	cost := r.InputUSD.Mul(decimal.NewFromInt(int64(inTok)))
	cost = cost.Add(r.OutputUSD.Mul(decimal.NewFromInt(int64(outTok))))
	cost = cost.Add(r.CacheReadUSD.Mul(decimal.NewFromInt(int64(cacheRead))))
	cost = cost.Add(r.CacheCreateUSD.Mul(decimal.NewFromInt(int64(cacheCreate))))
	return cost.Div(million)
}

// DefaultRateSheet seeds the published Anthropic/OpenAI/Gemini list prices.
// Numbers as of early 2026 — update from the official rate cards.
func DefaultRateSheet() *RateSheet {
	rs := NewRateSheet()
	d := decimal.NewFromFloat
	add := func(provider, model string, in, out, cacheRead, cacheCreate float64, caps ...string) {
		rs.Register(Rate{
			Provider: provider, Model: model,
			InputUSD: d(in), OutputUSD: d(out),
			CacheReadUSD: d(cacheRead), CacheCreateUSD: d(cacheCreate),
			Capability: caps,
		})
	}
	// Anthropic Claude family
	add("anthropic", "claude-opus-4-7",    15.00, 75.00, 1.50, 18.75, "reasoning", "code", "thinking", "cache", "tools", "vision")
	add("anthropic", "claude-sonnet-4-6",   3.00, 15.00, 0.30,  3.75, "reasoning", "code", "cache", "tools", "vision")
	add("anthropic", "claude-haiku-4-5-20251001", 1.00, 5.00, 0.10, 1.25, "json", "cheap", "fast", "cache", "tools")
	// OpenAI
	add("openai", "gpt-4o",  5.00, 15.00, 0, 0, "reasoning", "code")
	add("openai", "gpt-4o-mini", 0.15, 0.60, 0, 0, "json", "cheap", "fast")
	// Gemini
	add("gemini", "gemini-2.5-pro",   1.25, 5.00, 0, 0, "reasoning", "vision")
	add("gemini", "gemini-2.5-flash", 0.10, 0.40, 0, 0, "fast", "cheap")
	// Local ONNX — zero marginal cost (compute is fixed).
	add("onnx", "intent-mini",       0, 0, 0, 0, "private", "fast")
	add("onnx", "embed-mini",        0, 0, 0, 0, "private", "fast")
	// Mock provider — costless and capability-broad so dev/test always picks it
	// when no real key is configured.
	add("mock", "mock-1", 0, 0, 0, 0,
		"reasoning", "code", "json", "vision", "cheap", "fast", "private",
		"thinking", "cache", "tools")
	return rs
}
