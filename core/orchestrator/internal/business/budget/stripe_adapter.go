package budget

import (
	"context"
	"time"

	"ironflyer/core/orchestrator/internal/business/budget/payments"
)

// StripeProviderAdapter wraps *StripeService so it satisfies the
// payments.Provider interface without rewriting the existing Stripe
// surface. The orchestrator builds one of these at boot and hands it
// to payments.Registry alongside Paddle.
type StripeProviderAdapter struct {
	S *StripeService
	// Tolerance is the timestamp skew StripeService.VerifyWebhook
	// enforces. The HTTP handler historically used 5 minutes; we mirror
	// that here so the adapter behaves identically.
	Tolerance time.Duration
}

// NewStripeProvider builds the adapter with the default 5-minute
// tolerance.
func NewStripeProvider(s *StripeService) *StripeProviderAdapter {
	return &StripeProviderAdapter{S: s, Tolerance: 5 * time.Minute}
}

// Name implements payments.Provider.
func (a *StripeProviderAdapter) Name() string { return "stripe" }

// Enabled implements payments.Provider.
func (a *StripeProviderAdapter) Enabled() bool {
	return a != nil && a.S != nil && a.S.Enabled()
}

// CreateCheckoutSession implements payments.Provider.
func (a *StripeProviderAdapter) CreateCheckoutSession(ctx context.Context, userID, email string, tier payments.PlanTier) (string, error) {
	return a.S.CreateCheckoutSession(ctx, userID, email, PlanTier(tier))
}

// VerifyWebhook implements payments.Provider. The neutral Event keeps
// the verified Stripe event reachable through Raw so the legacy
// Billing.ApplyStripeEvent path can still consume it without changes.
func (a *StripeProviderAdapter) VerifyWebhook(payload []byte, sigHeader string) (payments.Event, error) {
	ev, err := a.S.VerifyWebhook(payload, sigHeader, a.Tolerance)
	if err != nil {
		return payments.Event{}, err
	}
	out := payments.Event{
		Provider: "stripe",
		Type:     ev.Type,
		UserID:   stripeUserID(ev.Data.Object),
		Tier:     payments.PlanTier(stripeTier(ev.Data.Object)),
		Raw:      ev.Data.Object,
	}
	return out, nil
}

var _ payments.Provider = (*StripeProviderAdapter)(nil)
