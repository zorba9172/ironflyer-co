package sentinel

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/learning"
	"ironflyer/core/orchestrator/internal/business/wallet"
)

// Underwriter is the actuarial model that prices an Insured Ship
// policy. The model reads aggregate completion statistics for
// "similar past projects" (same archetype, similar scope) and
// returns an expected cost; the premium is that expected cost
// times PremiumLoadingFactor.
//
// The interface is intentionally narrow so the wireup can swap in
// (a) a static cohort table for boot, (b) a Feedback-Brain-backed
// cohort lookup once the learning store has enough rows.
type Underwriter interface {
	ExpectedCost(ctx context.Context, tenant, projectID string, capUSD decimal.Decimal) (expected decimal.Decimal, sampleCount int, err error)
}

// InsuranceService manages Insured Ship policies. A policy is a
// flat-fee payment up front in exchange for a hard cap with a
// refund-if-exceeded guarantee on the underlying execution cost.
type InsuranceService interface {
	Quote(ctx context.Context, tenant, projectID string, capUSD decimal.Decimal, hours int) (InsuranceQuote, error)
	Purchase(ctx context.Context, tenant, projectID string, capUSD decimal.Decimal, hours int, requestID string) (InsurancePolicy, error)
	Get(ctx context.Context, tenant, policyID string) (InsurancePolicy, error)
	ActiveForProject(ctx context.Context, tenant, projectID string) (InsurancePolicy, error)
	RecordPayoutIfDue(ctx context.Context, policyID string, actualSpendUSD decimal.Decimal) (InsurancePolicy, error)
	ExpireDue(ctx context.Context, now time.Time) ([]InsurancePolicy, error)
}

// InsuranceQuote is the resolver-facing preview. SampleCount is the
// number of similar past completions backing the price — the
// dashboard renders it as a confidence chip.
type InsuranceQuote struct {
	CapUSD              decimal.Decimal
	PremiumUSD          decimal.Decimal
	CoverageWindowHours int
	SampleCount         int
}

// MemoryInsurance is the in-process backend for the Insured Ship
// SKU. Mirrors shippass.MemoryService in shape so the wireup can
// treat the two SKUs symmetrically.
type MemoryInsurance struct {
	policy Policy
	under  Underwriter
	wallet wallet.IdempotentService

	mu       sync.Mutex
	rows     map[string]*InsurancePolicy
	byTenant map[string][]string
}

// NewMemoryInsurance wires the in-memory implementation.
func NewMemoryInsurance(policy Policy, under Underwriter, walletSvc wallet.IdempotentService) *MemoryInsurance {
	return &MemoryInsurance{
		policy:   policy,
		under:    under,
		wallet:   walletSvc,
		rows:     map[string]*InsurancePolicy{},
		byTenant: map[string][]string{},
	}
}

// Quote returns the price preview.
func (m *MemoryInsurance) Quote(ctx context.Context, tenant, projectID string, capUSD decimal.Decimal, hours int) (InsuranceQuote, error) {
	if !capUSD.IsPositive() || hours <= 0 {
		return InsuranceQuote{}, ErrInvalidPolicy
	}
	expected, samples, err := m.under.ExpectedCost(ctx, tenant, projectID, capUSD)
	if err != nil {
		return InsuranceQuote{}, err
	}
	loading := decimal.NewFromFloat(m.policy.PremiumLoadingFactor)
	premium := expected.Mul(loading)
	return InsuranceQuote{
		CapUSD:              capUSD,
		PremiumUSD:          premium.Round(2),
		CoverageWindowHours: hours,
		SampleCount:         samples,
	}, nil
}

// Purchase charges the premium against the wallet (via DebitWithKey,
// not Hold — the premium is consumed immediately and only refunded
// if the policy pays out) and records the active policy row.
func (m *MemoryInsurance) Purchase(ctx context.Context, tenant, projectID string, capUSD decimal.Decimal, hours int, requestID string) (InsurancePolicy, error) {
	if !capUSD.IsPositive() || hours <= 0 {
		return InsurancePolicy{}, ErrInvalidPolicy
	}
	if requestID == "" {
		requestID = uuid.NewString()
	}
	quote, err := m.Quote(ctx, tenant, projectID, capUSD, hours)
	if err != nil {
		return InsurancePolicy{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pid := range m.byTenant[tenant] {
		row := m.rows[pid]
		if row.ProjectID == projectID && row.Status == "active" {
			return *row, ErrPolicyAlreadyActive
		}
	}

	premiumKey := "insurance-premium-" + requestID
	if err := m.wallet.DebitWithKey(ctx, tenant, quote.PremiumUSD, premiumKey); err != nil {
		return InsurancePolicy{}, err
	}

	now := time.Now().UTC()
	policy := &InsurancePolicy{
		ID:                  uuid.NewString(),
		TenantID:            tenant,
		ProjectID:           projectID,
		HardCapUSD:          capUSD,
		PremiumUSD:          quote.PremiumUSD,
		CoverageWindowHours: hours,
		Status:              "active",
		CreatedAt:           now,
		UpdatedAt:           now,
		ExpiresAt:           now.Add(time.Duration(hours) * time.Hour),
		PremiumOpKey:        premiumKey,
	}
	m.rows[policy.ID] = policy
	m.byTenant[tenant] = append(m.byTenant[tenant], policy.ID)
	publishInsurance(ctx, *policy, "purchased")
	return *policy, nil
}

// Get returns the policy owned by tenant.
func (m *MemoryInsurance) Get(_ context.Context, tenant, policyID string) (InsurancePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.rows[policyID]
	if !ok || row.TenantID != tenant {
		return InsurancePolicy{}, ErrPolicyNotFound
	}
	return *row, nil
}

// ActiveForProject returns the in-flight policy for the project.
func (m *MemoryInsurance) ActiveForProject(_ context.Context, tenant, projectID string) (InsurancePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pid := range m.byTenant[tenant] {
		row := m.rows[pid]
		if row.ProjectID == projectID && row.Status == "active" {
			return *row, nil
		}
	}
	return InsurancePolicy{}, ErrPolicyNotFound
}

// RecordPayoutIfDue checks the actual spend against the policy cap.
// When actual > cap and the policy is active, Sentinel credits the
// wallet for the overage and flips the policy to paid_out. The
// payout is the actual overage — not the full premium — so the
// platform absorbs the loss above the cap while the user keeps
// every dollar they were promised.
func (m *MemoryInsurance) RecordPayoutIfDue(ctx context.Context, policyID string, actualSpendUSD decimal.Decimal) (InsurancePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.rows[policyID]
	if !ok {
		return InsurancePolicy{}, ErrPolicyNotFound
	}
	if row.Status != "active" {
		return *row, nil
	}
	overage := actualSpendUSD.Sub(row.HardCapUSD)
	if !overage.IsPositive() {
		return *row, nil
	}
	payoutKey := "insurance-payout-" + policyID
	// Credit the wallet by topping it up with the overage amount.
	// Using TopUpWithKey here keeps the audit trail clean: the
	// payout reads as a synthetic top-up with a known op key, and
	// the lifetime_topup counter reflects the platform's payout.
	if err := m.wallet.TopUpWithKey(ctx, row.TenantID, overage, "insurance-payout-"+policyID, payoutKey); err != nil {
		return *row, err
	}
	now := time.Now().UTC()
	row.Status = "paid_out"
	row.UpdatedAt = now
	row.PayoutOpKey = payoutKey
	publishInsurance(ctx, *row, "paid_out")
	return *row, nil
}

// ExpireDue flips policies past their coverage window to expired.
func (m *MemoryInsurance) ExpireDue(ctx context.Context, now time.Time) ([]InsurancePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	expired := []InsurancePolicy{}
	for _, row := range m.rows {
		if row.Status != "active" {
			continue
		}
		if !now.After(row.ExpiresAt) {
			continue
		}
		row.Status = "expired"
		row.UpdatedAt = now.UTC()
		expired = append(expired, *row)
		publishInsurance(ctx, *row, "expired")
	}
	return expired, nil
}

// publishInsurance emits an OutcomeEvent for every insurance policy
// transition. Best-effort: a missing global publisher silently
// no-ops.
func publishInsurance(ctx context.Context, p InsurancePolicy, action string) {
	attrs := map[string]any{
		"policy_id":    p.ID,
		"project_id":   p.ProjectID,
		"action":       action,
		"cap_usd":      p.HardCapUSD.String(),
		"premium_usd":  p.PremiumUSD.String(),
		"coverage_hrs": p.CoverageWindowHours,
	}
	evt := learning.OutcomeEvent{
		ID:         uuid.NewString(),
		TenantID:   p.TenantID,
		Kind:       learning.OutcomeKind("insurance_" + action),
		Timestamp:  time.Now().UTC(),
		Attributes: attrs,
		Tags:       map[string]string{"surface": "sentinel.insurance"},
	}
	if action == "paid_out" {
		// The payout reduces platform margin by the overage; track
		// it on MarginUSD so the dashboards reflect the loss.
		overage := p.PremiumUSD.Neg()
		evt.MarginUSD = &overage
	}
	learning.Publish(ctx, evt)
}
