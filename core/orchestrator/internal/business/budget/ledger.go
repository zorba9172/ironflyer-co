package budget

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// LedgerEntry is one charge attributable to a user.
type LedgerEntry struct {
	ID           string          `json:"id"`
	UserID       string          `json:"userId"`
	ProjectID    string          `json:"projectId,omitempty"`
	Provider     string          `json:"provider"`
	Model        string          `json:"model"`
	InputTokens  int             `json:"inputTokens"`
	OutputTokens int             `json:"outputTokens"`
	CacheRead    int             `json:"cacheRead"`
	CacheCreate  int             `json:"cacheCreate"`
	CostUSD      decimal.Decimal `json:"costUSD"`
	CreatedAt    time.Time       `json:"createdAt"`
}

// LedgerStore is the contract every backend (memory / postgres) implements.
// All methods take a Context so they can be cancelled and traced.
type LedgerStore interface {
	Charge(ctx context.Context, e LedgerEntry) (LedgerEntry, error)
	SpentByUser(ctx context.Context, userID string) (decimal.Decimal, error)
	SpentTotal(ctx context.Context) (decimal.Decimal, error)
	EntriesByUser(ctx context.Context, userID string) ([]LedgerEntry, error)
}

// MemoryLedger is the dev/test implementation. Period = current calendar
// month (UTC).
type MemoryLedger struct {
	mu       sync.RWMutex
	entries  []LedgerEntry
	byUser   map[string][]int
	periodAt time.Time
}

func NewMemoryLedger() *MemoryLedger {
	return &MemoryLedger{
		byUser:   make(map[string][]int),
		periodAt: monthStart(time.Now().UTC()),
	}
}

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func (l *MemoryLedger) Charge(_ context.Context, e LedgerEntry) (LedgerEntry, error) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	idx := len(l.entries)
	l.entries = append(l.entries, e)
	l.byUser[e.UserID] = append(l.byUser[e.UserID], idx)
	return e, nil
}

func (l *MemoryLedger) SpentByUser(_ context.Context, userID string) (decimal.Decimal, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	sum := decimal.Zero
	for _, idx := range l.byUser[userID] {
		if l.entries[idx].CreatedAt.Before(l.periodAt) {
			continue
		}
		sum = sum.Add(l.entries[idx].CostUSD)
	}
	return sum, nil
}

func (l *MemoryLedger) SpentTotal(_ context.Context) (decimal.Decimal, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	sum := decimal.Zero
	for _, e := range l.entries {
		if e.CreatedAt.Before(l.periodAt) {
			continue
		}
		sum = sum.Add(e.CostUSD)
	}
	return sum, nil
}

func (l *MemoryLedger) EntriesByUser(_ context.Context, userID string) ([]LedgerEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]LedgerEntry, 0, len(l.byUser[userID]))
	for _, idx := range l.byUser[userID] {
		out = append(out, l.entries[idx])
	}
	return out, nil
}

var _ LedgerStore = (*MemoryLedger)(nil)
