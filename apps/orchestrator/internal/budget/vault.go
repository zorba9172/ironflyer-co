package budget

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// VaultEntry is one accounting movement (revenue credit or provider debit).
type VaultEntry struct {
	ID        string          `json:"id"`
	Kind      VaultEntryKind  `json:"kind"`
	UserID    string          `json:"userId,omitempty"`
	Amount    decimal.Decimal `json:"amount"` // positive = into vault, negative = out
	Note      string          `json:"note,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

type VaultEntryKind string

const (
	VaultRevenue      VaultEntryKind = "revenue"
	VaultProviderCost VaultEntryKind = "provider_cost"
	VaultRefund       VaultEntryKind = "refund"
	VaultAdjustment   VaultEntryKind = "adjustment"
)

// VaultStore is the contract every backend implements.
type VaultStore interface {
	Record(ctx context.Context, e VaultEntry) (VaultEntry, error)
	Balance(ctx context.Context) (decimal.Decimal, error)
	Snapshot(ctx context.Context) (VaultSnapshot, error)
}

type VaultSnapshot struct {
	Revenue      decimal.Decimal `json:"revenue"`
	ProviderCost decimal.Decimal `json:"providerCost"`
	Refunds      decimal.Decimal `json:"refunds"`
	Adjustments  decimal.Decimal `json:"adjustments"`
	Margin       decimal.Decimal `json:"margin"`
}

// MemoryVault keeps everything in process. Useful for dev and tests.
type MemoryVault struct {
	mu      sync.RWMutex
	entries []VaultEntry
}

func NewMemoryVault() *MemoryVault { return &MemoryVault{} }

func (v *MemoryVault) Record(_ context.Context, e VaultEntry) (VaultEntry, error) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	v.mu.Lock()
	v.entries = append(v.entries, e)
	v.mu.Unlock()
	return e, nil
}

func (v *MemoryVault) Balance(_ context.Context) (decimal.Decimal, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	sum := decimal.Zero
	for _, e := range v.entries {
		sum = sum.Add(e.Amount)
	}
	return sum, nil
}

func (v *MemoryVault) Snapshot(_ context.Context) (VaultSnapshot, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	s := VaultSnapshot{}
	for _, e := range v.entries {
		switch e.Kind {
		case VaultRevenue:
			s.Revenue = s.Revenue.Add(e.Amount)
		case VaultProviderCost:
			s.ProviderCost = s.ProviderCost.Add(e.Amount.Abs())
		case VaultRefund:
			s.Refunds = s.Refunds.Add(e.Amount.Abs())
		case VaultAdjustment:
			s.Adjustments = s.Adjustments.Add(e.Amount)
		}
	}
	s.Margin = s.Revenue.Sub(s.ProviderCost).Sub(s.Refunds).Add(s.Adjustments)
	return s, nil
}

var _ VaultStore = (*MemoryVault)(nil)
