package sentinel

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/learning"
)

// Service is the unified surface resolvers and dashboards consume.
// It composes the predictor + suggestion engine + insurance service
// so the GraphQL layer never has to wire three components together.
type Service struct {
	predictor   *Predictor
	suggester   *SuggestionEngine
	insurance   InsuranceService
	spentLoader SpentLoader
}

// SpentLoader resolves the project's all-time spend so the predictor
// does not have to scan the entire ledger on every Predict call.
type SpentLoader interface {
	Spent(ctx context.Context, tenant, projectID string) (decimal.Decimal, error)
}

// NewService wires the components. Any of the three component
// pointers may be nil — Service nils the dependent path rather than
// crashing, so the wireup can roll out Sentinel before the full
// stack lands. Insurance and suggester are independent of the
// predictor; predictor is independent of both.
func NewService(predictor *Predictor, suggester *SuggestionEngine, insurance InsuranceService, spent SpentLoader) *Service {
	return &Service{
		predictor:   predictor,
		suggester:   suggester,
		insurance:   insurance,
		spentLoader: spent,
	}
}

// Forecast returns the current trajectory + suggested reroutes for a
// project. A nil predictor returns an empty-ish forecast with
// Level=green so the dashboard still renders without crashing.
func (s *Service) Forecast(ctx context.Context, tenant, projectID string) (Forecast, []Reroute, error) {
	if s.predictor == nil {
		return Forecast{ProjectID: projectID, TenantID: tenant, Level: WarnGreen, ComputedAt: nowTruncated()}, nil, nil
	}
	spent := decimal.Zero
	if s.spentLoader != nil {
		v, err := s.spentLoader.Spent(ctx, tenant, projectID)
		if err != nil {
			return Forecast{}, nil, err
		}
		spent = v
	}
	f, err := s.predictor.Predict(ctx, tenant, projectID, spent)
	if err != nil {
		return Forecast{}, nil, err
	}
	publishForecast(ctx, f)
	if s.suggester == nil {
		return f, nil, nil
	}
	routes, err := s.suggester.Suggest(ctx, tenant, projectID, f)
	if err != nil {
		return f, nil, err
	}
	return f, routes, nil
}

// InsuranceQuote forwards to the insurance backend.
func (s *Service) InsuranceQuote(ctx context.Context, tenant, projectID string, capUSD decimal.Decimal, hours int) (InsuranceQuote, error) {
	if s.insurance == nil {
		return InsuranceQuote{}, ErrInvalidPolicy
	}
	return s.insurance.Quote(ctx, tenant, projectID, capUSD, hours)
}

// PurchaseInsurance forwards to the insurance backend.
func (s *Service) PurchaseInsurance(ctx context.Context, tenant, projectID string, capUSD decimal.Decimal, hours int, requestID string) (InsurancePolicy, error) {
	if s.insurance == nil {
		return InsurancePolicy{}, ErrInvalidPolicy
	}
	return s.insurance.Purchase(ctx, tenant, projectID, capUSD, hours, requestID)
}

// ActiveInsuranceForProject returns the in-flight insurance policy.
func (s *Service) ActiveInsuranceForProject(ctx context.Context, tenant, projectID string) (InsurancePolicy, error) {
	if s.insurance == nil {
		return InsurancePolicy{}, ErrPolicyNotFound
	}
	return s.insurance.ActiveForProject(ctx, tenant, projectID)
}

// publishForecast emits a learning OutcomeEvent so the Feedback
// Brain can mine "projects that flipped from green to orange" and
// surface them as risk patterns. Best-effort — silent no-op when no
// publisher is wired.
func publishForecast(ctx context.Context, f Forecast) {
	attrs := map[string]any{
		"project_id":             f.ProjectID,
		"level":                  string(f.Level),
		"spent_usd":              f.SpentUSD.String(),
		"hard_cap_usd":           f.HardCapUSD.String(),
		"extrapolated_total_usd": f.ExtrapolatedTotalUSD.String(),
		"burn_per_hour_usd":      f.BurnRatePerHourUSD.String(),
		"projection_confidence":  f.ProjectionConfidenceFrac,
	}
	if f.CapBreachAt != nil {
		attrs["cap_breach_at"] = f.CapBreachAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	evt := learning.OutcomeEvent{
		ID:         uuid.NewString(),
		TenantID:   f.TenantID,
		Kind:       learning.OutcomeKind("sentinel_forecast"),
		Timestamp:  f.ComputedAt,
		Attributes: attrs,
		Tags:       map[string]string{"surface": "sentinel.forecast", "level": string(f.Level)},
	}
	learning.Publish(ctx, evt)
}
