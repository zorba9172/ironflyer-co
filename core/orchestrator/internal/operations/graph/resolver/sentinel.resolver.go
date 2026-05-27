package resolver

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/sentinel"
	"ironflyer/core/orchestrator/internal/business/wallet"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// PurchaseInsurance charges the premium against the wallet and opens
// an active Insured Ship policy.
func (r *mutationResolver) PurchaseInsurance(ctx context.Context, projectID string, capUsd float64, hours int, requestID *string) (*model.InsurancePolicy, error) {
	if r.Sentinel == nil {
		return nil, gqlNotConfigured("sentinel")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rid := ""
	if requestID != nil {
		rid = *requestID
	}
	row, err := r.Sentinel.PurchaseInsurance(ctx, tenantFor(u), projectID, decimal.NewFromFloat(capUsd), hours, rid)
	if err != nil {
		if errors.Is(err, wallet.ErrInsufficient) {
			return nil, gqlInsufficientFunds(r.WebBaseURL)
		}
		return nil, err
	}
	return insurancePolicyToModel(row), nil
}

// SentinelForecast returns the project trajectory snapshot.
func (r *queryResolver) SentinelForecast(ctx context.Context, projectID string) (*model.SentinelForecast, error) {
	if r.Sentinel == nil {
		return nil, gqlNotConfigured("sentinel")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	forecast, _, err := r.Sentinel.Forecast(ctx, tenantFor(u), projectID)
	if err != nil {
		return nil, err
	}
	return forecastToModel(forecast), nil
}

// SentinelReroutes returns the suggested cheaper-completion paths.
func (r *queryResolver) SentinelReroutes(ctx context.Context, projectID string) ([]model.SentinelReroute, error) {
	if r.Sentinel == nil {
		return nil, gqlNotConfigured("sentinel")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	_, routes, err := r.Sentinel.Forecast(ctx, tenantFor(u), projectID)
	if err != nil {
		return nil, err
	}
	out := make([]model.SentinelReroute, 0, len(routes))
	for _, rt := range routes {
		out = append(out, model.SentinelReroute{
			Kind:              string(rt.Kind),
			Label:             rt.Label,
			Description:       rt.Description,
			SavingsUsd:        floatOfDecimal(rt.SavingsUSD),
			SavingsConfidence: rt.SavingsConfidence,
			Reversible:        rt.Reversible,
		})
	}
	return out, nil
}

// InsuranceQuote previews the Insured Ship premium.
func (r *queryResolver) InsuranceQuote(ctx context.Context, projectID string, capUsd float64, hours int) (*model.InsuranceQuote, error) {
	if r.Sentinel == nil {
		return nil, gqlNotConfigured("sentinel")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	q, err := r.Sentinel.InsuranceQuote(ctx, tenantFor(u), projectID, decimal.NewFromFloat(capUsd), hours)
	if err != nil {
		return nil, err
	}
	return &model.InsuranceQuote{
		CapUsd:              floatOfDecimal(q.CapUSD),
		PremiumUsd:          floatOfDecimal(q.PremiumUSD),
		CoverageWindowHours: q.CoverageWindowHours,
		SampleCount:         q.SampleCount,
	}, nil
}

// ActiveInsurance returns the in-flight policy for a project.
func (r *queryResolver) ActiveInsurance(ctx context.Context, projectID string) (*model.InsurancePolicy, error) {
	if r.Sentinel == nil {
		return nil, gqlNotConfigured("sentinel")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.Sentinel.ActiveInsuranceForProject(ctx, tenantFor(u), projectID)
	if err != nil {
		if errors.Is(err, sentinel.ErrPolicyNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return insurancePolicyToModel(row), nil
}

// forecastToModel projects the domain forecast onto the GraphQL shape.
func forecastToModel(f sentinel.Forecast) *model.SentinelForecast {
	return &model.SentinelForecast{
		ProjectID:            f.ProjectID,
		SpentUsd:             floatOfDecimal(f.SpentUSD),
		HardCapUsd:           floatOfDecimal(f.HardCapUSD),
		BurnRatePerHourUsd:   floatOfDecimal(f.BurnRatePerHourUSD),
		ExtrapolatedTotalUsd: floatOfDecimal(f.ExtrapolatedTotalUSD),
		EtaCompletionAt:      f.ETACompletionAt,
		CapBreachAt:          f.CapBreachAt,
		Level:                string(f.Level),
		RemainingHeadroomUsd: floatOfDecimal(f.RemainingHeadroomUSD),
		ProjectionConfidence: f.ProjectionConfidenceFrac,
		ComputedAt:           f.ComputedAt,
	}
}

// insurancePolicyToModel projects the domain policy onto the GraphQL
// shape.
func insurancePolicyToModel(p sentinel.InsurancePolicy) *model.InsurancePolicy {
	return &model.InsurancePolicy{
		ID:                  p.ID,
		ProjectID:           p.ProjectID,
		HardCapUsd:          floatOfDecimal(p.HardCapUSD),
		PremiumUsd:          floatOfDecimal(p.PremiumUSD),
		CoverageWindowHours: p.CoverageWindowHours,
		Status:              p.Status,
		CreatedAt:           p.CreatedAt,
		UpdatedAt:           p.UpdatedAt,
		ExpiresAt:           p.ExpiresAt,
	}
}
