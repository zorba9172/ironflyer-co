package deploy

import (
	"context"

	"github.com/shopspring/decimal"
)

// Adapter is the per-provider deploy contract. Implementations are
// stateless from the Service's point of view: the Service holds the
// durable Deploy row and asks the Adapter to perform the
// provider-side side effects.
//
// Every Adapter MUST be safe to call more than once with the same
// deployID; the Vercel implementation uses Idempotency-Key headers to
// uphold that on the network side.
type Adapter interface {
	// Name returns the registry key (matches Target). The Service
	// uses Name() to pick an Adapter for a given PlanInput.Target.
	Name() string

	// Plan performs the no-side-effect provider lookup that converts
	// the abstract PlanInput into a concrete provider-side plan
	// (project id, projected cost, advisory notes). Plan MUST NOT
	// create or mutate provider resources.
	Plan(ctx context.Context, p PlanInput) (PlanResult, error)

	// BuildPreview kicks off the provider-side preview build. The
	// returned ProviderDeploymentID is the handle the Service stores
	// in deploys.provider_deployment_id and reuses for Promote /
	// Rollback.
	BuildPreview(ctx context.Context, deployID string, plan PlanResult) (PreviewResult, error)

	// Promote flips a previously-built preview to production. The
	// caller (Service.Promote) already verified an approved approval
	// row exists or a policy obligation explicitly allowed
	// auto-deploy.
	Promote(ctx context.Context, deployID, providerDeploymentID string) (PromoteResult, error)

	// Rollback reverts a production deploy to the supplied version.
	// toVersion may be empty — the adapter is free to interpret that
	// as "the previous successful deploy".
	Rollback(ctx context.Context, deployID, providerDeploymentID, toVersion string) (RollbackResult, error)
}

// PlanResult is the no-side-effect projection returned by Adapter.Plan.
type PlanResult struct {
	ProviderProjectID string
	EstimatedCostUSD  decimal.Decimal
	Notes             []string
}

// PreviewResult is the outcome of Adapter.BuildPreview.
type PreviewResult struct {
	ProviderDeploymentID string
	PreviewURL           string
	CostUSD              decimal.Decimal
}

// PromoteResult is the outcome of Adapter.Promote.
type PromoteResult struct {
	ProductionURL string
	CostUSD       decimal.Decimal
}

// RollbackResult is the outcome of Adapter.Rollback. ToVersion is the
// version the provider rolled back to (echoed for the audit row).
type RollbackResult struct {
	ToVersion string
}

// SecretResolver is the integration seam between the deploy package
// and internal/secrets. Declared here (rather than importing the
// secrets package) so the package stays cycle-free and so the
// integration agent can wire either the production Broker or a
// memory-backed test double from main.go.
//
// Resolve MUST return the raw secret value or an error; deploy never
// caches the value beyond the in-flight provider call.
type SecretResolver interface {
	Resolve(ctx context.Context, tenantID, projectID, name string) ([]byte, error)
}
