package providers

import (
	"context"
	"errors"
	"testing"

	"ironflyer/core/orchestrator/internal/business/budget"
	"ironflyer/core/orchestrator/internal/business/profitguardctx"
)

type tenantTestProvider struct{}

func (tenantTestProvider) Name() string { return "mock" }

func (tenantTestProvider) Capabilities() []Capability {
	return []Capability{CapFast, CapCheap}
}

func (tenantTestProvider) CompleteStream(context.Context, Request) (<-chan Delta, error) {
	ch := make(chan Delta, 1)
	ch <- Delta{
		Type:     DeltaDone,
		Provider: "mock",
		Model:    "mock-1",
		Usage: &Usage{
			InputTokens:  100,
			OutputTokens: 10,
		},
	}
	close(ch)
	return ch, nil
}

func TestBillingGuardRejectsUnscopedBudgetedCalls(t *testing.T) {
	guard := NewBillingGuard(NewRouter(), budget.NewMemoryBilling())

	_, err := guard.CompleteStream(context.Background(), Request{
		Capabilities: []Capability{CapFast},
	})
	if !errors.Is(err, ErrMissingTenant) {
		t.Fatalf("CompleteStream() error = %v, want ErrMissingTenant", err)
	}
}

func TestBillingGuardUsesExecutionTenantWhenRequestTenantMissing(t *testing.T) {
	ctx := profitguardctx.WithExecution(context.Background(), "exec-1", "tenant-private")
	billing := budget.NewMemoryBilling()
	billing.AssignPlan(ctx, "tenant-private", budget.TierPro)
	router := NewRouter()
	router.Register(tenantTestProvider{})
	guard := NewBillingGuard(router, billing)

	ch, err := guard.CompleteStream(ctx, Request{
		Capabilities: []Capability{CapFast},
	})
	if err != nil {
		t.Fatalf("CompleteStream() error = %v", err)
	}
	for range ch {
	}

	rows, err := billing.Ledger.EntriesByUser(ctx, "tenant-private")
	if err != nil {
		t.Fatalf("EntriesByUser(private) error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("private ledger entries = %d, want 1", len(rows))
	}
	if rows[0].UserID != "tenant-private" {
		t.Fatalf("ledger user id = %q, want tenant-private", rows[0].UserID)
	}

	anonymousRows, err := billing.Ledger.EntriesByUser(ctx, "anonymous")
	if err != nil {
		t.Fatalf("EntriesByUser(anonymous) error = %v", err)
	}
	if len(anonymousRows) != 0 {
		t.Fatalf("anonymous ledger entries = %d, want 0", len(anonymousRows))
	}
}
