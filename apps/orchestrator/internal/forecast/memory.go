package forecast

import (
	"context"
	"sync"

	"github.com/shopspring/decimal"
)

// MemoryForecaster is the in-process backend. It indexes injected
// cost samples by tenant + blueprint and serves percentile queries
// against the cached slice. It is the canonical estimator for
// development, the demo dataset, and any deployment that does not
// (yet) point at Postgres.
type MemoryForecaster struct {
	cfg Config

	mu       sync.RWMutex
	tenant   map[tenantKey][]decimal.Decimal // tenant + blueprint cost samples
	global   map[string][]decimal.Decimal    // blueprint-only cost samples (all tenants)
}

// tenantKey is the composite (tenant, blueprint) lookup. Samples
// added without a blueprint are also rolled into a synthetic
// "*" blueprint bucket so a tenant-level query (no blueprint) still
// has something to chew on.
type tenantKey struct {
	Tenant    string
	Blueprint string
}

// NewMemoryForecaster builds a MemoryForecaster pre-seeded with
// nothing. Callers feed it via AddSample. Pass cfg=DefaultConfig()
// when in doubt.
func NewMemoryForecaster(cfg Config) *MemoryForecaster {
	return &MemoryForecaster{
		cfg:    cfg,
		tenant: map[tenantKey][]decimal.Decimal{},
		global: map[string][]decimal.Decimal{},
	}
}

// AddSample records one historical cost observation. tenantID may be
// empty (treated as a global-only sample); blueprintID may be empty
// (recorded under the synthetic "*" bucket so wildcard queries can
// still match it).
func (m *MemoryForecaster) AddSample(tenantID, blueprintID string, costUSD decimal.Decimal) {
	if blueprintID == "" {
		blueprintID = "*"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if tenantID != "" {
		k := tenantKey{Tenant: tenantID, Blueprint: blueprintID}
		m.tenant[k] = append(m.tenant[k], costUSD)
	}
	m.global[blueprintID] = append(m.global[blueprintID], costUSD)
}

// Estimate satisfies Forecaster. It tries the tenant-specific
// samples first, then the global samples, then falls back to the
// capability baseline.
func (m *MemoryForecaster) Estimate(ctx context.Context, in EstimateInput) (Estimate, error) {
	if in.TenantID == "" {
		return Estimate{}, ErrInvalidInput
	}
	bp := in.BlueprintID
	if bp == "" {
		bp = "*"
	}

	m.mu.RLock()
	tenantSamples := append([]decimal.Decimal(nil), m.tenant[tenantKey{Tenant: in.TenantID, Blueprint: bp}]...)
	globalSamples := append([]decimal.Decimal(nil), m.global[bp]...)
	m.mu.RUnlock()

	if len(tenantSamples) >= m.cfg.MinTenantSamples {
		return estimateFromSamples(in, tenantSamples, m.cfg), nil
	}
	if len(globalSamples) >= m.cfg.MinGlobalSamples {
		return estimateFromSamples(in, globalSamples, m.cfg), nil
	}
	return estimateBaseline(in, m.cfg), nil
}

// Compile-time check.
var _ Forecaster = (*MemoryForecaster)(nil)
