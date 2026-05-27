package resolver

import (
	"context"
	"errors"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/business/shippass"
	"ironflyer/core/orchestrator/internal/business/wallet"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// PurchaseShipPass reserves the tier price against the wallet and
// opens an active pass. Idempotent on requestID.
func (r *mutationResolver) PurchaseShipPass(ctx context.Context, projectID string, tierKey string, requestID *string) (*model.ShipPass, error) {
	if r.ShipPass == nil {
		return nil, gqlNotConfigured("ship_pass")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rid := ""
	if requestID != nil {
		rid = *requestID
	}
	row, err := r.ShipPass.Purchase(ctx, tenantFor(u), projectID, tierKey, rid)
	if err != nil {
		if errors.Is(err, wallet.ErrInsufficient) {
			return nil, gqlInsufficientFunds(r.WebBaseURL)
		}
		return nil, err
	}
	if r.ShipPassSettler != nil {
		r.ShipPassSettler.Bind(projectID, row.ID)
	}
	return shipPassToModel(row), nil
}

// CancelShipPass releases the hold and flips the pass to cancelled.
func (r *mutationResolver) CancelShipPass(ctx context.Context, passID string) (*model.ShipPass, error) {
	if r.ShipPass == nil {
		return nil, gqlNotConfigured("ship_pass")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.ShipPass.Cancel(ctx, tenantFor(u), passID)
	if err != nil {
		return nil, err
	}
	if r.ShipPassSettler != nil {
		r.ShipPassSettler.Unbind(row.ProjectID)
	}
	return shipPassToModel(row), nil
}

// ShipPassTiers returns the static price catalogue.
func (r *queryResolver) ShipPassTiers(_ context.Context) ([]model.ShipPassTier, error) {
	tiers := shippass.Tiers()
	out := make([]model.ShipPassTier, 0, len(tiers))
	for _, t := range tiers {
		out = append(out, model.ShipPassTier{
			Key:           t.Key,
			Label:         t.Label,
			PriceUsd:      floatOfDecimal(t.PriceUSD),
			RequiredGates: gateNamesToStrings(t.RequiredGates),
			DeadlineDays:  t.DeadlineDays,
			Description:   t.Description,
		})
	}
	return out, nil
}

// ShipPassQuote previews the buy price + wallet shortfall.
func (r *queryResolver) ShipPassQuote(ctx context.Context, projectID string, tierKey string) (*model.ShipPassQuote, error) {
	if r.ShipPass == nil {
		return nil, gqlNotConfigured("ship_pass")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	q, err := r.ShipPass.Quote(ctx, tenantFor(u), projectID, tierKey)
	if err != nil {
		return nil, err
	}
	return &model.ShipPassQuote{
		TierKey:            q.TierKey,
		PriceUsd:           floatOfDecimal(q.PriceUSD),
		RequiredGates:      gateNamesToStrings(q.RequiredGates),
		DeadlineDays:       q.DeadlineDays,
		WalletShortfallUsd: floatOfDecimal(q.WalletShortfall),
	}, nil
}

// ActiveShipPass returns the in-flight pass for a project; nil when
// no pass is active.
func (r *queryResolver) ActiveShipPass(ctx context.Context, projectID string) (*model.ShipPass, error) {
	if r.ShipPass == nil {
		return nil, gqlNotConfigured("ship_pass")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.ShipPass.ActiveForProject(ctx, tenantFor(u), projectID)
	if err != nil {
		if errors.Is(err, shippass.ErrPassNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return shipPassToModel(row), nil
}

// ShipPasses returns the tenant's pass history newest first.
func (r *queryResolver) ShipPasses(ctx context.Context, limit *int) ([]model.ShipPass, error) {
	if r.ShipPass == nil {
		return nil, gqlNotConfigured("ship_pass")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	lim := 25
	if limit != nil && *limit > 0 {
		lim = *limit
	}
	rows, err := r.ShipPass.List(ctx, tenantFor(u), lim)
	if err != nil {
		return nil, err
	}
	out := make([]model.ShipPass, 0, len(rows))
	for _, row := range rows {
		out = append(out, *shipPassToModel(row))
	}
	return out, nil
}

// ShipPassProgress returns the gate observation log for a pass.
func (r *queryResolver) ShipPassProgress(ctx context.Context, passID string) ([]model.ShipPassGateProgress, error) {
	if r.ShipPass == nil {
		return nil, gqlNotConfigured("ship_pass")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.ShipPass.ProgressFor(ctx, tenantFor(u), passID)
	if err != nil {
		return nil, err
	}
	out := make([]model.ShipPassGateProgress, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.ShipPassGateProgress{
			ID:         row.ID,
			Gate:       string(row.Gate),
			Passed:     row.Passed,
			Reason:     row.Reason,
			ObservedAt: row.ObservedAt,
		})
	}
	return out, nil
}

// ShipPassStats returns the headline billing counters.
func (r *queryResolver) ShipPassStats(ctx context.Context) (*model.ShipPassLifetimeStats, error) {
	if r.ShipPass == nil {
		return nil, gqlNotConfigured("ship_pass")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	stats, err := r.ShipPass.LifetimeStats(ctx, tenantFor(u))
	if err != nil {
		return nil, err
	}
	return &model.ShipPassLifetimeStats{
		TotalPurchased: stats.TotalPurchased,
		TotalShipped:   stats.TotalShipped,
		TotalRefunded:  stats.TotalRefunded,
		TotalCancelled: stats.TotalCancelled,
		RevenueUsd:     floatOfDecimal(stats.RevenueUSD),
	}, nil
}

// shipPassToModel projects the domain row onto the GraphQL shape.
func shipPassToModel(row shippass.ShipPass) *model.ShipPass {
	return &model.ShipPass{
		ID:         row.ID,
		ProjectID:  row.ProjectID,
		TierKey:    row.TierKey,
		PriceUsd:   floatOfDecimal(row.PriceUSD),
		Status:     string(row.Status),
		DeadlineAt: row.DeadlineAt,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
		SettledAt:  row.SettledAt,
	}
}

// gateNamesToStrings projects a slice of domain.GateName onto plain
// strings for the GraphQL boundary.
func gateNamesToStrings(in []domain.GateName) []string {
	out := make([]string, len(in))
	for i, g := range in {
		out[i] = string(g)
	}
	return out
}
