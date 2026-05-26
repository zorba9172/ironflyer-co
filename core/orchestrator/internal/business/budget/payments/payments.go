// Package payments abstracts the checkout + webhook surface every
// payment provider must satisfy so the orchestrator can host more than
// one provider side-by-side. The Stripe service in the parent budget
// package implements Provider via a thin adapter (stripe_adapter.go);
// Paddle (paddle.go) implements it directly. New providers slot in by
// implementing Provider and registering with the package-level Registry.
package payments

import "context"

// PlanTier mirrors budget.PlanTier without taking a circular import on
// the parent package. The adapter / provider converts strings as needed.
type PlanTier string

const (
	TierFree       PlanTier = "free"
	TierPro        PlanTier = "pro"
	TierTeam       PlanTier = "team"
	TierEnterprise PlanTier = "enterprise"
)

// Event is the provider-neutral webhook event the orchestrator routes
// after a provider has verified its signature. The concrete fields
// surface only what downstream code needs: the user, the tier they
// bought, the raw provider event type so audit logs can keep the
// vendor-specific name. Provider-specific extras live in Raw so a
// future hook can inspect them without changing the interface.
type Event struct {
	Provider string
	Type     string
	UserID   string
	Tier     PlanTier
	Raw      map[string]any
}

// Provider is the contract every payment integration must satisfy.
// Methods take ctx so a slow vendor never blocks the request goroutine
// past its deadline.
type Provider interface {
	// Name is the stable identifier used in webhook routes, logs, and
	// the registry. Lowercase, no spaces ("stripe", "paddle").
	Name() string
	// Enabled is true only when the provider has the credentials it
	// needs to talk to the vendor. Disabled providers are kept in the
	// registry but skipped by Active().
	Enabled() bool
	// CreateCheckoutSession returns the URL the user should be redirected
	// to in order to complete payment for the given tier.
	CreateCheckoutSession(ctx context.Context, userID, email string, tier PlanTier) (url string, err error)
	// VerifyWebhook authenticates the raw body against the provider's
	// signature header and returns the parsed neutral event on success.
	VerifyWebhook(payload []byte, sigHeader string) (Event, error)
}
