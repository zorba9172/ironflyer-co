// Package deploy is the V22 Deploy plane. It owns the lifecycle of a
// provider deploy (Plan → BuildPreview → RequestApproval → Decide →
// Promote → optional Rollback), the durable rows backing it
// (deploys + deploy_events + deploy_approvals), the per-provider
// Adapter contract (Vercel v1 ships first, Fly / Cloudflare / k8s
// land later under the same surface), and the ProfitGuard
// BeforeVercelDeploy enforcement helper that every production deploy
// runs through.
//
// The package is intentionally self-contained: it does NOT import
// internal/secrets (we declare a local SecretResolver interface that
// the integration agent satisfies with a secrets.Broker adapter),
// does NOT import internal/profitguard at the call sites that guard
// production deploys (we declare a local ProfitGuardChecker
// interface), and does NOT touch cmd/orchestrator/main.go — the
// integration agent wires everything at the end.
//
// Money values flow through as shopspring/decimal.Decimal so no
// precision is lost between the ledger, ProfitGuard, and the
// provider-cost-attribution path.
package deploy

import (
	"time"

	"github.com/shopspring/decimal"
)

// Status is the durable deploy_status enum. The string values match
// the migration verbatim — do NOT rename without coordinating an
// ALTER TYPE migration.
type Status string

const (
	StatusPlanned          Status = "planned"
	StatusPreviewBuilding  Status = "preview_building"
	StatusPreviewReady     Status = "preview_ready"
	StatusAwaitingApproval Status = "awaiting_approval"
	StatusPromoting        Status = "promoting"
	StatusPromoted         Status = "promoted"
	StatusRolledBack       Status = "rolled_back"
	StatusFailed           Status = "failed"
	StatusCancelled        Status = "cancelled"
)

// String makes Status satisfy fmt.Stringer for log/metric formatters.
func (s Status) String() string { return string(s) }

// Environment is the deploy target environment. Production deploys
// pull in the full approval + ProfitGuard path; preview deploys are
// gated but do not require an approval row.
type Environment string

const (
	EnvironmentPreview    Environment = "preview"
	EnvironmentProduction Environment = "production"
)

// String makes Environment satisfy fmt.Stringer.
func (e Environment) String() string { return string(e) }

// Target identifies the provider adapter. Values are the registry
// key the Service uses to look up an Adapter; "vercel" is the v1
// implementation that ships with this package.
type Target string

const (
	TargetVercel     Target = "vercel"
	TargetFly        Target = "fly"
	TargetCloudflare Target = "cloudflare"
	TargetK8s        Target = "k8s"
	TargetNoop       Target = "noop"
)

// String makes Target satisfy fmt.Stringer.
func (t Target) String() string { return string(t) }

// ApprovalStatus is the deploy_approvals.status text enum.
type ApprovalStatus string

const (
	ApprovalPending   ApprovalStatus = "pending"
	ApprovalApproved  ApprovalStatus = "approved"
	ApprovalRejected  ApprovalStatus = "rejected"
	ApprovalExpired   ApprovalStatus = "expired"
	ApprovalWithdrawn ApprovalStatus = "withdrawn"
)

// String makes ApprovalStatus satisfy fmt.Stringer.
func (a ApprovalStatus) String() string { return string(a) }

// Deploy is the GraphQL-aligned projection of one `deploys` row.
// Internal services (Service, Adapter, the resolver layer) all pass
// this shape so the wire and durable models stay aligned.
type Deploy struct {
	ID                   string
	TenantID             string
	ProjectID            string
	ExecutionID          string // optional; empty for operator-driven deploys
	BlueprintID          string
	Target               Target
	Environment          Environment
	Status               Status
	ProviderDeploymentID string
	PreviewURL           string
	ProductionURL        string
	DiffHash             string
	ArtifactHash         string
	GateSummary          map[string]string
	CostUSD              decimal.Decimal
	Metadata             map[string]any
	CreatedAt            time.Time
	PreviewReadyAt       *time.Time
	PromotedAt           *time.Time
	RolledBackAt         *time.Time
}

// Approval is the GraphQL projection of one `deploy_approvals` row.
type Approval struct {
	ID                string
	DeployID          string
	TenantID          string
	RequestedByUserID string // empty when an AI execution opened the approval
	DecidedByUserID   string // empty until decided
	Status            ApprovalStatus
	DiffHash          string
	ArtifactHash      string
	GateSummary       map[string]string
	CostImpactUSD     decimal.Decimal
	ExpiresAt         time.Time
	DecisionNote      string
	PolicyDecisionID  string
	AuditChainEventID string
	RequestedAt       time.Time
	DecidedAt         *time.Time
}

// UserRef is the minimum identity payload the Service needs to
// attribute an approval request / decision. Resolvers fill it from
// the authenticated user; AI-driven approval requests pass an empty
// UserRef and the row records NULL for requested_by_user_id.
type UserRef struct {
	UserID   string
	TenantID string
}

// PlanInput is the Service.Plan parameter envelope. ArtifactRef is
// the workspace / S3 snapshot reference the adapter consumes when it
// builds the preview; DiffHash binds the deploy to a specific patch
// set so the audit chain can prove what shipped.
type PlanInput struct {
	TenantID    string
	ProjectID   string
	ExecutionID string // optional
	BlueprintID string // optional
	Target      Target // "vercel" | ...
	Environment Environment
	ArtifactRef string
	DiffHash    string
	GateSummary map[string]string
	Metadata    map[string]any
}

// Event is one row from deploy_events flattened for the GraphQL
// deployFeed subscription. payload is event-type-specific JSON.
type Event struct {
	DeployID  string
	EventType string
	Payload   map[string]any
	CreatedAt time.Time
}

// Decision is the literal string the resolver passes into Service.Decide.
// We accept "approve"/"approved" + "reject"/"rejected" so the GraphQL
// shape (a Boolean) and any future CLI shape (a verb) both land on
// the same enum.
const (
	DecisionApprove = "approve"
	DecisionReject  = "reject"
)
