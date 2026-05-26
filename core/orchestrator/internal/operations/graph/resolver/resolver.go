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

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/atlas"
	"ironflyer/core/orchestrator/internal/operations/arch"
	"ironflyer/core/orchestrator/internal/operations/audit"
	"ironflyer/core/orchestrator/internal/operations/auditexport"
	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/business/blueprints"
	"ironflyer/core/orchestrator/internal/business/budget"
	"ironflyer/core/orchestrator/internal/ai/completion"
	"ironflyer/core/orchestrator/internal/business/dashboards"
	"ironflyer/core/orchestrator/internal/operations/deploy"
	"ironflyer/core/orchestrator/internal/operations/diagnostics"
	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/ai/finisher"
	"ironflyer/core/orchestrator/internal/business/forecast"
	"ironflyer/core/orchestrator/internal/ai/ideaparser"
	"ironflyer/core/orchestrator/internal/business/ledger"
	"ironflyer/core/orchestrator/internal/ai/memorygraph"
	"ironflyer/core/orchestrator/internal/operations/mobile/appetize"
	"ironflyer/core/orchestrator/internal/operations/mobile/devicecloud"
	"ironflyer/core/orchestrator/internal/operations/mobile/eas"
	"ironflyer/core/orchestrator/internal/customer/notify"
	"ironflyer/core/orchestrator/internal/operations/operator"
	"ironflyer/core/orchestrator/internal/operations/patch"
	"ironflyer/core/orchestrator/internal/business/profitguard"
	"ironflyer/core/orchestrator/internal/ai/providers"
	"ironflyer/core/orchestrator/internal/operations/ratelimit"
	"ironflyer/core/orchestrator/internal/ai/repair"
	"ironflyer/core/orchestrator/internal/operations/securityreport"
	"ironflyer/core/orchestrator/internal/operations/store"
	"ironflyer/core/orchestrator/internal/business/wallet"
	"ironflyer/core/orchestrator/internal/business/wowloop"
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
	WalletSvc         wallet.Service
	WalletTopper      *wallet.Topper
	LedgerSvc         ledger.Service
	ExecutionSvc      execution.Service
	ExecutionSettler  execution.Settler
	ProfitGuard       profitguard.Guard
	ProfitGuardStore  profitguard.DecisionStore
	BlueprintsReg     blueprints.Registry
	BlueprintStatsSvc blueprints.StatsService
	// IdeaParser (A54) — turns a free-text idea into a structured
	// Idea (blueprint pick + suggested budget + tags) for the
	// studio describeIdea entrypoint. Nil-safe at the resolver layer:
	// the resolver falls back to the existing keyword heuristic when
	// the parser was not wired.
	IdeaParser  ideaparser.Parser
	Completion  completion.Scorer
	Repair      repair.Genome
	PatchMemory repair.Memory
	Dashboards  *dashboards.Service

	// Appetize is the Free-tier iOS preview façade. Nil-safe — the
	// resolver returns gqlNotConfigured when the orchestrator was
	// booted without an APPETIZE_TOKEN.
	Appetize *appetize.Service

	// Deploy plane — V22 Wave 2 (Trust). Plan → Preview → Approval →
	// Promote/Rollback/Cancel. Nil-safe at the resolver layer.
	DeploySvc       deploy.Service
	DeployDomainSvc deploy.DomainService

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

	// ---------- Mobile (EAS) -------------------------------------
	// EAS is the typed REST client for Expo Application Services.
	// Mobile resolvers (mobileTriggerBuild / mobileSubmitToStore /
	// mobilePublishUpdate / mobileBuilds / mobileSubmissions) reach
	// for it. Nil-safe — when nil the mobile resolvers return
	// gqlNotConfigured("mobile"). The orchestrator constructs a
	// global-token Client at boot; a per-project token (carried in
	// Project.Secrets["EAS_TOKEN"]) overrides it for paid customers
	// who run on their own Expo account.
	EAS *eas.Client

	// EASPoller drives the background EAS build-status loop. The
	// mobile resolvers Track() new builds on it so the subscription
	// fan-out (mobileBuildStatus) gets a snapshot per status change.
	// Nil-safe — when nil the resolvers still respond to one-shot
	// GetBuild calls but cannot stream the subscription.
	EASPoller *eas.Poller

	// ---------- Mobile (device cloud) ------------------------------
	// DeviceCloud is the Pro-tier real-device session manager.
	// BrowserStack App Live (interactive) + AWS Device Farm (batched)
	// sit behind a single facade so the cockpit talks to one resolver
	// regardless of vendor. Nil-safe — when nil the resolver returns
	// gqlNotConfigured("device cloud").
	DeviceCloud *devicecloud.Manager

	// ---------- Code Health Dashboard inputs ----------------------
	// Atlas is the Capability Atlas the indexer populates at boot
	// and on every reindex tick. HealthDashboard surfaces its Stats
	// (total capabilities, last indexed time). Nil-safe — the
	// resolver reports zero capabilities and a nil timestamp when
	// the store was not wired.
	AtlasStore atlas.Store
	// ArchManifest is the parsed .ironflyer/architecture.json. The
	// HealthDashboard projects its layers + rules + cycles policy
	// onto the Architecture sub-shape. Zero value means the
	// manifest was missing at boot; the resolver returns empty
	// layers + rules so the panel can render its "manifest not
	// wired" empty state.
	ArchManifest arch.Manifest
	// HealthReportPaths is the operator-configured map of Anti-Bloat
	// report file paths (jscpd / knip / gocognit /
	// dependency-cruiser / bundle-analyzer). Plumbed from main.go
	// via the IRONFLYER_*_REPORT_PATH env vars; each field is
	// optional. Missing files yield empty slices + sentinel zero
	// values so the cockpit panels render their "report not wired"
	// empty states without surfacing an error.
	HealthReportPaths HealthReportPaths
}

// HealthReportPaths captures the file paths the resolver consults to
// project the Anti-Bloat reports (jscpd / knip / gocognit /
// dependency-cruiser / bundle-analyzer) into the HealthDashboard.
// Every field is optional; the resolver tolerates missing files and
// returns the empty / sentinel shape per health.go.
type HealthReportPaths struct {
	Dedup      string
	Deadcode   string
	Complexity string
	DepCycle   string
	Bundle     string
}
