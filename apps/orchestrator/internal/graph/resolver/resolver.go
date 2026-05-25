// Package resolver hosts every GraphQL resolver method as a value
// receiver on the Resolver struct.
//
// V22 trims this surface to the foundations the V22 plan keeps —
// authentication, projects + finisher, gates, patches, budget,
// completions, audit, agents. Wallet, ledger, ProfitGuard, blueprints,
// repair, completion-score, and dashboards arrive under their own
// resolver files added by later agents.
package resolver

import (
	"context"

	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/auditexport"
	"ironflyer/apps/orchestrator/internal/auth"
	"ironflyer/apps/orchestrator/internal/blueprints"
	"ironflyer/apps/orchestrator/internal/budget"
	"ironflyer/apps/orchestrator/internal/completion"
	"ironflyer/apps/orchestrator/internal/dashboards"
	"ironflyer/apps/orchestrator/internal/deploy"
	"ironflyer/apps/orchestrator/internal/diagnostics"
	"ironflyer/apps/orchestrator/internal/execution"
	"ironflyer/apps/orchestrator/internal/finisher"
	"ironflyer/apps/orchestrator/internal/forecast"
	"ironflyer/apps/orchestrator/internal/ideaparser"
	"ironflyer/apps/orchestrator/internal/ledger"
	"ironflyer/apps/orchestrator/internal/memorygraph"
	"ironflyer/apps/orchestrator/internal/notify"
	"ironflyer/apps/orchestrator/internal/operator"
	"ironflyer/apps/orchestrator/internal/patch"
	"ironflyer/apps/orchestrator/internal/profitguard"
	"ironflyer/apps/orchestrator/internal/providers"
	"ironflyer/apps/orchestrator/internal/ratelimit"
	"ironflyer/apps/orchestrator/internal/repair"
	"ironflyer/apps/orchestrator/internal/securityreport"
	"ironflyer/apps/orchestrator/internal/store"
	"ironflyer/apps/orchestrator/internal/wallet"
	"ironflyer/apps/orchestrator/internal/wowloop"
)

// EmailChanger is the user-store surface the resolver uses to flip a
// user's primary email after a confirmed email-change flow. Both the
// memory and Postgres user stores satisfy it.
type EmailChanger interface {
	SetEmail(ctx context.Context, userID, newEmail string) error
}

// Resolver carries every dependency the V22 resolver graph can reach.
// Later agents add fields as they wire wallet / ledger / execution /
// profitguard / blueprints / completion / repair / dashboards. Keep
// the field set additive — removing a field is a breaking change for
// the other in-flight agents.
type Resolver struct {
	Auth      *auth.Service
	Billing   *budget.Billing
	Telemetry providers.TelemetrySink
	Projects  store.Store
	Engine    *finisher.Engine
	Agents    *agents.Registry
	Patches   *patch.Engine
	Guard     *providers.BillingGuard
	Logger    zerolog.Logger

	// PublicBaseURL is forwarded into resolvers that emit external
	// URLs (e.g. audit export links) so the URL matches the
	// orchestrator's externally visible host.
	PublicBaseURL string

	// Stripe powers the startCheckout mutation. Optional — when nil
	// the resolver returns a typed error.
	Stripe *budget.StripeService

	// AuditStore backs the audit / verifyAudit / export-URL resolvers.
	AuditStore audit.Store

	// NotifyPrefs holds the per-user channel/event matrix used by
	// notification preference resolvers (added under a later agent).
	NotifyPrefs notify.PrefsStore

	// Auth commercial table-stakes used by verifyEmail / resetPassword
	// / sessions resolvers. Each store is nil-safe at the resolver
	// layer so a partial orchestrator config still boots.
	Verifications   auth.VerificationStore
	PasswordResets  auth.PasswordResetStore
	Sessions        auth.SessionStore
	SessionCache    auth.SessionCache
	EmailVerifier   auth.EmailVerifier
	EmailChanger    EmailChanger
	PasswordRotator auth.PasswordRotator
	Email           notify.EmailSender
	WebBaseURL      string
	AuthAudit       audit.Store

	// Rate limiters for the password-reset and resend-verification
	// auth flows. Backed by Redis when wired, in-memory otherwise.
	PasswordResetIPLimiter    ratelimit.Allower
	PasswordResetEmailLimiter ratelimit.Allower
	ResendVerificationLimiter ratelimit.Allower

	// AdminUserIDs is the optional allowlist of users authorised for
	// admin-only resolvers. Empty falls back to "any authenticated
	// user" so the dev box stays usable.
	AdminUserIDs map[string]bool

	// DevWalletSeedUSD — convenience credit applied by SignUp in dev
	// only, so a fresh account can immediately run describeIdea
	// without Stripe being configured. Wired from
	// config.Config.DevWalletSeedUSD (gated by Env=="dev").
	DevWalletSeedUSD float64
	// DevEnv reflects config.Env so SignUp can gate the seed.
	DevEnv string

	// ---------- V22 service surface --------------------------------
	// Each pointer is nil-safe — resolvers return gqlNotConfigured if
	// the matching dependency was not wired by main.go.
	WalletSvc            wallet.Service
	WalletTopper         *wallet.Topper
	LedgerSvc            ledger.Service
	ExecutionSvc         execution.Service
	ExecutionSettler     execution.Settler
	ProfitGuard          profitguard.Guard
	ProfitGuardStore     profitguard.DecisionStore
	BlueprintsReg        blueprints.Registry
	BlueprintStatsSvc    blueprints.StatsService
	// IdeaParser (A54) — turns a free-text idea into a structured
	// Idea (blueprint pick + suggested budget + tags) for the
	// studio describeIdea entrypoint. Nil-safe at the resolver layer:
	// the resolver falls back to the existing keyword heuristic when
	// the parser was not wired.
	IdeaParser           ideaparser.Parser
	Completion           completion.Scorer
	Repair               repair.Genome
	PatchMemory          repair.Memory
	Dashboards           *dashboards.Service

	// Deploy plane — V22 Wave 2 (Trust). Plan → Preview → Approval →
	// Promote/Rollback/Cancel. Nil-safe at the resolver layer.
	DeploySvc deploy.Service

	// MemoryGraph is the AI Memory Graph used by the finisher for
	// IntentGateRepair traversal. The resolver only touches it for
	// project-deletion cascade (Agent 30) — every other graph access
	// happens through the finisher / repair packages. optional;
	// nil-safe — when nil, deleteProject skips graph cleanup.
	MemoryGraph memorygraph.Graph

	// ---------- V22 Wave-3 services (A32-A36) ----------------------
	// Each pointer is nil-safe — resolvers return gqlNotConfigured if
	// the matching dependency was not wired by main.go.
	Forecaster            forecast.Forecaster
	WowLoopBuilder        wowloop.Builder
	AuditExporter         auditexport.Exporter
	AuditExportConfig     auditexport.Config
	SecurityReportBuilder securityreport.Builder
	Operator              operator.OperatorService

	// Diagnostics powers the recentErrors / recentLogs operator
	// queries. Nil-safe — resolvers return an empty list when the
	// service was not wired (dev boots without the ring buffer).
	Diagnostics *diagnostics.Service
}
