package wireup

// conversion.go — wires the Free→Paid plan-upgrade signal from the
// budget package into the internal/ledger so dashboards can answer
// "what's our conversion rate this week?" with a single typed query
// instead of scraping subscription history.
//
// Why a wireup file: the budget package owns the plan store and the
// internal/ledger package owns typed entries; neither imports the
// other (and shouldn't). This file is the small seam that knows
// both.

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/budget"
	"ironflyer/core/orchestrator/internal/business/ledger"
	"ironflyer/core/orchestrator/internal/operations/logctx"
)

// WireConversionTracking registers a PlanChangeHook on Billing that
// writes an EntryFreeToPaidConversion to the supplied ledger whenever
// a user moves from TierFree to a paying tier. Other transitions
// (Pro→Team, Pro→Free downgrade) are no-ops here — they have their
// own ledger entries elsewhere (subscription updates / refunds).
//
// Safe to call with nil dependencies — the function returns without
// registering, which is the right behaviour for tests / dev runs
// that don't wire a ledger.
func WireConversionTracking(billing *budget.Billing, lg ledger.Service) {
	if billing == nil || lg == nil {
		return
	}
	billing.RegisterPlanChangeHook(func(ctx context.Context, userID string, previous, next budget.PlanTier, plan budget.Plan) {
		if previous != budget.TierFree || next == budget.TierFree {
			return
		}
		// Tenant is the user — V22 personal accounts share user id
		// with tenant id (`tenantFor` helper in the resolver layer
		// does the same). A bad userID just means we skip the
		// entry; the conversion itself already happened.
		tenant, err := uuid.Parse(userID)
		if err != nil {
			return
		}
		entry := ledger.Entry{
			TenantID:       tenant,
			EntryType:      ledger.EntryFreeToPaidConversion,
			Direction:      ledger.CreditDirection,
			AmountUSD:      plan.MonthlyPrice,
			Billable:       false,
			MarginRelevant: false,
			Metadata: map[string]any{
				"previous_tier": string(previous),
				"new_tier":      string(next),
				"plan_name":     plan.Name,
				"monthly_price": plan.MonthlyPrice.String(),
				"converted_at":  time.Now().UTC().Format(time.RFC3339),
			},
			// OpKey idempotency: one conversion event per user, ever.
			// A retry of AssignPlan (Stripe webhook redelivery) lands
			// at the same OpKey and dedupes via the ledger's unique
			// index. The next upgrade (Pro→Team) doesn't trip this
			// hook so the key collision is desirable.
			OpKey: "free_to_paid:" + userID,
		}
		if _, err := lg.Write(ctx, entry); err != nil {
			l := logctx.From(ctx)
			l.Warn().
				Err(err).
				Str("user_id", userID).
				Str("new_tier", string(next)).
				Float64("monthly_price_usd", floatOrZero(plan.MonthlyPrice)).
				Msg("conversion ledger entry write failed")
		}
	})
}

func floatOrZero(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}
