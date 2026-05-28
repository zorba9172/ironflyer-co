package wireup

// FinisherGuild → ProvisioningVault payout bridge.
//
// The guild package defines a narrow guild.PayoutTransferer port and
// leaves the rail-side transfer unwired. This adapter satisfies that
// port using the provisioning package's Stripe Connect connector:
// given a finisher id, it resolves the finisher's connected payout
// account and issues a Stripe Transfer for their cut.
//
// Connected-account convention: a finisher's Stripe Connect payout
// account is provisioned under (tenant = finisher's user id, project =
// FinisherPayoutProject). The finisher-onboarding flow (a later UI)
// provisions the resource under that reserved key; this adapter reads
// it back. Until that UI exists the lookup returns "no account" and
// the payout is marked failed for manual follow-up — no money moves
// to a non-existent destination.

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/guild"
	"ironflyer/core/orchestrator/internal/business/provisioning"
)

// FinisherPayoutProject is the reserved project key under which a
// finisher's Stripe Connect payout account is provisioned. Kept as a
// constant so the onboarding flow and this adapter agree on one
// string.
const FinisherPayoutProject = "guild-payout"

// stripeConnectPayout implements guild.PayoutTransferer over the
// provisioning Stripe Connect connector + service.
type stripeConnectPayout struct {
	guildSvc guild.Service
	provSvc  provisioning.Service
	connect  *provisioning.StripeConnect
}

// NewStripeConnectPayout builds the bridge. Returns nil when either
// the provisioning service or the Stripe Connect connector is missing
// / disabled — callers treat a nil transferer as "rail not wired" and
// keep payouts queued.
func NewStripeConnectPayout(guildSvc guild.Service, provSvc provisioning.Service, connect *provisioning.StripeConnect) guild.PayoutTransferer {
	if guildSvc == nil || provSvc == nil || connect == nil || !connect.Enabled() {
		return nil
	}
	return &stripeConnectPayout{guildSvc: guildSvc, provSvc: provSvc, connect: connect}
}

// Transfer resolves the finisher's connected account and issues the
// Stripe Transfer. idempotencyKey is passed straight through to Stripe
// so a guild payout retry never double-pays.
func (s *stripeConnectPayout) Transfer(ctx context.Context, finisherID string, finisherCutUSD decimal.Decimal, idempotencyKey string) (string, error) {
	profile, err := s.guildSvc.GetFinisherProfile(ctx, finisherID)
	if err != nil {
		return "", fmt.Errorf("guild payout: resolve finisher %s: %w", finisherID, err)
	}
	// The finisher's own user id is the tenant their payout account
	// is provisioned under. Org-scoped finishers would resolve the
	// org id via the user store; that lookup is deferred until the
	// onboarding UI lands (documented in the package comment).
	tenant := profile.UserID
	if tenant == "" {
		return "", fmt.Errorf("guild payout: finisher %s has no user id", finisherID)
	}
	resources, err := s.provSvc.List(ctx, tenant, FinisherPayoutProject)
	if err != nil {
		return "", fmt.Errorf("guild payout: list payout resources: %w", err)
	}
	acct := ""
	for _, r := range resources {
		if r.Kind == provisioning.KindStripeConnect && r.Status == provisioning.StatusActive && r.ExternalID != "" {
			acct = r.ExternalID
			break
		}
	}
	if acct == "" {
		return "", fmt.Errorf("guild payout: finisher %s has no active connected payout account", finisherID)
	}
	ref, err := s.connect.CreateTransfer(ctx, acct, finisherCutUSD, idempotencyKey)
	if err != nil {
		return "", fmt.Errorf("guild payout: stripe transfer: %w", err)
	}
	return ref, nil
}
