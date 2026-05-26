package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// NoopAdapter is the dev / no-creds Adapter. It never talks to a
// real provider; it synthesises plausible-looking provider ids and
// preview URLs so the local dev loop can exercise the full deploy
// FSM (plan → preview → approval → promote → rollback) without any
// external dependency.
//
// Cost is reported as zero; a future variant can mock cost numbers
// from a fixture if dashboards want a richer dev signal.
type NoopAdapter struct {
	// PreviewBase, when non-empty, is the URL prefix the synthetic
	// preview URLs use. Defaults to "https://noop.local".
	PreviewBase string
}

// Name returns the adapter registry key.
func (NoopAdapter) Name() string { return string(TargetNoop) }

// Plan returns a synthetic plan with a derived provider project id.
func (NoopAdapter) Plan(_ context.Context, p PlanInput) (PlanResult, error) {
	return PlanResult{
		ProviderProjectID: fmt.Sprintf("noop-%s", p.ProjectID),
		EstimatedCostUSD:  decimal.Zero,
		Notes:             []string{"noop adapter — no provider call"},
	}, nil
}

// BuildPreview synthesises a deployment id + preview URL.
func (a NoopAdapter) BuildPreview(_ context.Context, deployID string, _ PlanResult) (PreviewResult, error) {
	base := a.PreviewBase
	if base == "" {
		base = "https://noop.local"
	}
	id := uuid.NewString()
	return PreviewResult{
		ProviderDeploymentID: id,
		PreviewURL:           fmt.Sprintf("%s/preview/%s", base, deployID),
		CostUSD:              decimal.Zero,
	}, nil
}

// Promote synthesises a production URL.
func (a NoopAdapter) Promote(_ context.Context, deployID, _ string) (PromoteResult, error) {
	base := a.PreviewBase
	if base == "" {
		base = "https://noop.local"
	}
	return PromoteResult{
		ProductionURL: fmt.Sprintf("%s/prod/%s", base, deployID),
		CostUSD:       decimal.Zero,
	}, nil
}

// Rollback returns the requested version (or a synthetic one).
func (NoopAdapter) Rollback(_ context.Context, _ string, _, toVersion string) (RollbackResult, error) {
	if toVersion == "" {
		toVersion = fmt.Sprintf("noop-%d", time.Now().Unix())
	}
	return RollbackResult{ToVersion: toVersion}, nil
}
