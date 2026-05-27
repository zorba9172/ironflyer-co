// Package httpapi wires the orchestrator's HTTP surface.
//
// V22 collapses this surface to the minimum needed for the new
// architecture:
//
//   - /healthz, /livez, /readyz, /version  — k8s probes (REST forever)
//   - /metrics                             — Prometheus scrape
//   - /graphql, /graphql/sandbox           — the API of record
//   - /budget/webhook                      — Stripe webhook (REST forever)
//
// Everything else — leads, share-links, affiliates, figma import,
// collab, chat, deploy adapter, GitHub OAuth, inline completions, MCP
// proxy, status — was deleted as part of the V22 purge. New features
// land as GraphQL operations against the resolver package.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/atlas"
	"ironflyer/core/orchestrator/internal/ai/completion"
	"ironflyer/core/orchestrator/internal/ai/finisher"
	"ironflyer/core/orchestrator/internal/ai/ideaparser"
	"ironflyer/core/orchestrator/internal/ai/learning"
	"ironflyer/core/orchestrator/internal/ai/memory"
	"ironflyer/core/orchestrator/internal/ai/memorygraph"
	"ironflyer/core/orchestrator/internal/ai/providers"
	"ironflyer/core/orchestrator/internal/ai/repair"
	"ironflyer/core/orchestrator/internal/business/blueprints"
	"ironflyer/core/orchestrator/internal/business/budget"
	"ironflyer/core/orchestrator/internal/business/budget/payments"
	"ironflyer/core/orchestrator/internal/business/compliance"
	"ironflyer/core/orchestrator/internal/business/dashboards"
	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/business/forecast"
	"ironflyer/core/orchestrator/internal/business/guild"
	"ironflyer/core/orchestrator/internal/business/ledger"
	"ironflyer/core/orchestrator/internal/business/profitguard"
	"ironflyer/core/orchestrator/internal/business/provisioning"
	"ironflyer/core/orchestrator/internal/business/sentinel"
	"ironflyer/core/orchestrator/internal/business/shippass"
	"ironflyer/core/orchestrator/internal/business/wallet"
	"ironflyer/core/orchestrator/internal/business/wowloop"
	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/customer/auth/oauth"
	"ironflyer/core/orchestrator/internal/customer/notify"
	"ironflyer/core/orchestrator/internal/operations/arch"
	"ironflyer/core/orchestrator/internal/operations/audit"
	"ironflyer/core/orchestrator/internal/operations/auditexport"
	"ironflyer/core/orchestrator/internal/operations/bus"
	"ironflyer/core/orchestrator/internal/operations/deploy"
	"ironflyer/core/orchestrator/internal/operations/diagnostics"
	"ironflyer/core/orchestrator/internal/operations/gqlhardening"
	graphpkg "ironflyer/core/orchestrator/internal/operations/graph"
	"ironflyer/core/orchestrator/internal/operations/graph/resolver"
	"ironflyer/core/orchestrator/internal/operations/metrics"
	"ironflyer/core/orchestrator/internal/operations/mobile/devicecloud"
	"ironflyer/core/orchestrator/internal/operations/mobile/eas"
	"ironflyer/core/orchestrator/internal/operations/operator"
	"ironflyer/core/orchestrator/internal/operations/patch"
	"ironflyer/core/orchestrator/internal/operations/policy"
	"ironflyer/core/orchestrator/internal/operations/ratelimit"
	"ironflyer/core/orchestrator/internal/operations/securityreport"
	"ironflyer/core/orchestrator/internal/operations/store"
	"ironflyer/core/orchestrator/internal/pkg/httputil"
	"ironflyer/core/orchestrator/internal/suppliers/github_pr"
)

// Deps is the full set of orchestrator dependencies the HTTP layer
// needs to assemble the router. Optional fields stay nil when the
// matching feature is unconfigured — handlers return typed errors at
// request time rather than blocking startup.
type Deps struct {
	// Core finisher surface.
	Projects store.Store
	Engine   *finisher.Engine
	Agents   *agents.Registry
	Patches  *patch.Engine

	// Budget + payments.
	Billing *budget.Billing
	Stripe  *budget.StripeService
	Paddle  *payments.PaddleService
	Guard   *providers.BillingGuard

	// Authentication.
	Auth         *auth.Service
	AuthOptional bool

	// OAuth (Google + GitHub social sign-in). Optional; nil skips
	// /auth/{provider}/start + /auth/{provider}/callback registration.
	OAuth *oauth.Handler

	// AllowedOrigins is the comma-separated list of browser origins the
	// CORS middleware should reflect into Access-Control-Allow-Origin.
	// Empty means "reflect any origin" — only safe in dev. Production
	// MUST set this to the exact list of web origins (e.g.
	// https://app.ironflyer.dev). Tied to the orchestrator's
	// IRONFLYER_CORS_ORIGINS env var.
	AllowedOrigins []string

	// ProdMode mirrors cfg.Env == "prod". Used by the security-headers
	// middleware (HSTS, strict CSP) and the metrics auth check.
	ProdMode bool

	// CSPOverride replaces the default Content-Security-Policy when set.
	// Sourced from IRONFLYER_CSP.
	CSPOverride string

	// MetricsToken protects /metrics. Empty disables auth (dev/staging
	// only — main.go fails fast in prod when unset).
	MetricsToken string

	// Memory + audit + telemetry.
	Memory    memory.Store
	Audit     audit.Store
	Telemetry providers.TelemetrySink
	Bus       *bus.Multiplexer

	// Notifications (email + in-app outbox).
	NotifyPrefs notify.PrefsStore
	Notify      *notify.Engine
	// Notifier is the customer-lifecycle + finisher-event dispatcher.
	// Every Kind flows through the outbox before the worker fans out
	// to email + in-app. Nil-safe — call sites guard before Dispatch.
	Notifier *notify.Dispatcher
	// NotifyStore persists durable in-app notification rows surfaced
	// by the bell. Nil-safe at the resolver layer.
	NotifyStore notify.NotificationStore
	// NotifyHub is the in-process pub/sub the notificationStream
	// GraphQL subscription consumes. Nil-safe.
	NotifyHub *notify.SubscriptionHub

	// Runtime workspace client (for legacy hand-off; the resolver does
	// not yet expose workspace operations in V22).
	RuntimeURL string

	// Build identity surfaced by /version.
	Version   string
	Commit    string
	BuildTime string

	// Dev convenience: when DevEnv == "dev" and DevWalletSeedUSD > 0,
	// the SignUp resolver credits the new user's own wallet so describeIdea
	// works without Stripe. Wired from config.Config in main.go; ignored
	// in staging / prod.
	DevEnv           string
	DevWalletSeedUSD float64

	// Public origin (e.g. https://app.ironflyer.dev). Forwarded to the
	// resolver so generated URLs match the externally visible host.
	PublicBaseURL string

	// Auth commercial table-stakes — verification, password resets,
	// sessions, transactional email. Every field is optional; the
	// resolvers nil-guard each.
	Verifications             auth.VerificationStore
	PasswordResets            auth.PasswordResetStore
	Sessions                  auth.SessionStore
	SessionCache              auth.SessionCache
	EmailVerifier             auth.EmailVerifier
	EmailChanger              resolver.EmailChanger
	PasswordRotator           auth.PasswordRotator
	Email                     notify.EmailSender
	WebBaseURL                string
	AuthAudit                 audit.Store
	PasswordResetIPLimiter    ratelimit.Allower
	PasswordResetEmailLimiter ratelimit.Allower
	ResendVerificationLimiter ratelimit.Allower

	Logger zerolog.Logger

	// ---------- V22 service surface --------------------------------
	// Every field is optional. The resolver returns NOT_CONFIGURED
	// for any V22 query that arrives without the matching dep.
	Wallet        wallet.Service
	WalletToppers *wallet.TopperRegistry
	// Compliance powers the ComplianceGate vertical SKUs (PCI / HIPAA
	// / SOC 2 / GDPR). Nil-safe — the resolver returns NOT_CONFIGURED
	// when the orchestrator was booted without a Postgres pool /
	// in-memory compliance backend.
	Compliance       *compliance.Service
	Ledger           ledger.Service
	Execution        execution.Service
	ExecutionSettler execution.Settler
	ProfitGuard      profitguard.Guard
	ProfitGuardStore profitguard.DecisionStore
	Blueprints       blueprints.Registry
	BlueprintStats   blueprints.StatsService
	IdeaParser       ideaparser.Parser
	Completion       completion.Scorer
	Repair           repair.Genome
	PatchMemory      repair.Memory
	Dashboards       *dashboards.Service

	// V22 Wave-2 surfaces (deploy, hardening, policy). All optional —
	// the API still boots without them; affected resolvers degrade to
	// NOT_CONFIGURED.
	Deploy         deploy.Service
	DeployDomains  deploy.DomainService
	Hardening      *gqlhardening.Config
	PolicyPEP      *policy.PEP
	PersistedStore gqlhardening.Store
	GqlRateLimiter *gqlhardening.Limiter
	IsOperator     func(ctx context.Context) bool

	// MemoryGraph powers the project-delete cascade in the resolver
	// (Agent 30) and is read-only at the HTTP layer — nil-safe.
	MemoryGraph memorygraph.Graph

	// ---------- V22 Wave-3 surfaces (A32-A36) ---------------------
	// All optional; affected resolvers degrade to NOT_CONFIGURED when
	// the matching dep is nil.
	Forecaster            forecast.Forecaster
	WowLoopBuilder        wowloop.Builder
	AuditExporter         auditexport.Exporter
	AuditExportConfig     auditexport.Config
	SecurityReportBuilder securityreport.Builder
	Operator              operator.OperatorService

	// Diagnostics is the in-process error / log ring buffer. Optional —
	// when nil the /admin/logs/tail endpoint returns 503 and the
	// GraphQL recentErrors / recentLogs queries return empty lists.
	Diagnostics *diagnostics.Service

	// EAS is the Expo Application Services REST client used by the
	// mobile resolvers. Nil when EAS_TOKEN is unset; affected resolvers
	// degrade to NOT_CONFIGURED.
	EAS *eas.Client

	// EASPoller drives the background EAS build-status loop. nil-safe.
	EASPoller *eas.Poller

	// DeviceCloud is the Pro-tier real-device session manager. nil-safe —
	// when unwired the resolver returns NOT_CONFIGURED so the cockpit
	// renders the provider chip disabled.
	DeviceCloud *devicecloud.Manager

	// ---------- Code Health Dashboard inputs ----------------------
	// AtlasStore + ArchManifest + HealthReportPaths feed the
	// `healthDashboard` GraphQL field. Every input is optional; the
	// resolver renders sentinel zero / empty values so the cockpit
	// panels can show their "tool not wired" state without surfacing
	// an error to the operator.
	AtlasStore        atlas.Store
	ArchManifest      arch.Manifest
	HealthReportPaths resolver.HealthReportPaths

	// Feedback Brain — learning store + publisher. Both nil-safe at
	// the resolver layer: when LearningStore is nil the
	// learningDashboard query returns an empty snapshot; when
	// LearningPublisher is nil the global publisher fallback (set via
	// learning.SetGlobal at boot) is used.
	LearningStore     learning.Store
	LearningPublisher *learning.Publisher

	// Provisioning is the ProvisionedResource / RevenueEvent vault —
	// Ironflyer-as-issuer revenue rails (Stripe Connect, domain
	// reseller, email partner, hosting). nil-safe at the resolver
	// layer; the /provisioning/webhook/stripe REST route 503s when
	// the vault has no Stripe Connect connector wired.
	Provisioning *provisioning.Vault

	// ShipPass is the outcome-based pricing SKU. Settler binds
	// project IDs to active pass IDs so gate verdicts route to the
	// right pass; both pointers nil-safe at the resolver.
	ShipPass        shippass.Service
	ShipPassSettler *shippass.Settler

	// Sentinel is the predictive budget forecast + Insured Ship SKU.
	// Nil-safe at the resolver.
	Sentinel *sentinel.Service

	// GuildCoord drives every FinisherGuild mutation (task lifecycle,
	// bid lifecycle, template installs). Nil-safe at the resolver.
	GuildCoord *guild.Coordinator
}

// API is the HTTP layer entry point. It assembles the chi router from
// the Deps it was constructed with.
type API struct {
	d Deps
}

// New returns the orchestrator's HTTP handler. The caller mounts the
// returned http.Handler under a single net/http.Server.
func New(d Deps) http.Handler {
	a := &API{d: d}

	r := chi.NewRouter()
	// sentryRecoverer replaces chi.middleware.Recoverer so panics ship to
	// Sentry before the 500 response, alongside the existing zerolog
	// record. No-op when SENTRY_DSN_ORCHESTRATOR / SENTRY_DSN is unset.
	r.Use(sentryRecoverer(a.d.Logger))
	// RequestIDMiddleware mints / honours X-Request-ID and stamps it on
	// ctx + the response header before metrics or auth run, so failure
	// log lines from those layers carry the same correlation id.
	r.Use(RequestIDMiddleware(a.d.Logger))
	r.Use(metrics.HTTP)
	// otelHTTP opens one span per HTTP request, named with the chi route
	// pattern so cardinality stays bounded. No-op when the OTel tracer
	// is the global noop provider.
	r.Use(otelHTTP)
	// CORS — the web SPA runs on a different origin than the
	// orchestrator. The browser blocks credentialed POST requests
	// unless the server explicitly opts in. We reflect Origin (so
	// Access-Control-Allow-Credentials:true is valid; `*` is not
	// allowed with credentials) and allow the headers the SPA sends.
	r.Use(corsMiddleware(a.d.AllowedOrigins))
	r.Use(securityHeadersMiddleware(SecurityHeadersOptions{
		ProdMode:    a.d.ProdMode,
		CSPOverride: a.d.CSPOverride,
	}))

	// Public infra endpoints — NEVER authenticated.
	r.Get("/healthz", a.healthz)
	r.Get("/livez", a.livez)
	r.Get("/readyz", a.readyz)
	r.Get("/version", a.version)
	// /metrics is Prometheus scrape. Protected with a bearer token when
	// IRONFLYER_METRICS_TOKEN is set; main.go fails fast in prod when
	// the token is unset.
	r.Method(http.MethodGet, "/metrics", metricsAuth(a.d.MetricsToken, metrics.Handler()))

	// /debug/leak/snapshot — goroutine snapshot for the goleak smoke
	// harness (scripts/lint/run-goleak-smoke.sh). Enabled only when
	// IRONFLYER_LEAK_PROBE_TOKEN is set; absence makes the route 404
	// so a prod pod never advertises its goroutine stack. REST
	// exception alongside /metrics — same diagnostic class.
	if leakTok := strings.TrimSpace(os.Getenv("IRONFLYER_LEAK_PROBE_TOKEN")); leakTok != "" {
		r.Method(http.MethodGet, "/debug/leak/snapshot", leakProbeHandler(leakTok))
	}

	// Stripe webhook — third-party callback, signature-verified inline.
	r.Post("/budget/webhook", a.stripeWebhook)
	// Paddle webhook — third-party callback, signature-verified inline.
	r.Post("/budget/paddle/webhook", a.paddleWebhook)
	// Stripe Connect webhook — distinct endpoint with its own signing
	// secret (Connect events deliver application_fee.created /
	// account.updated under a separate Stripe webhook). REST exception
	// alongside /budget/webhook because the same constraints apply:
	// third-party callback Stripe cannot drive over GraphQL.
	r.Post("/provisioning/webhook/stripe", a.provisioningStripeWebhook)
	// GitHub PR webhook — third-party callback. The path embeds the
	// projectID + routing secret; the body is HMAC-verified against the
	// per-project GITHUB_WEBHOOK_SECRET. See suppliers/github_pr for
	// the handler implementation and route shape.
	if a.d.Projects != nil {
		ghHandler := github_pr.NewHandler(github_pr.HandlerDeps{
			Projects: a.d.Projects,
			Engine:   a.d.Engine,
			Agents:   a.d.Agents,
			Logger:   a.d.Logger,
		})
		ghHandler.Mount(r)
	}

	// OAuth social sign-in (Google + GitHub). REST exception: the
	// browser cannot drive a GraphQL mutation through the provider's
	// redirect dance, and the JWT must ride back via a URL fragment so
	// it never reaches access logs. See oauth.Handler.
	if a.d.OAuth != nil {
		r.Get("/auth/{provider}/start", a.d.OAuth.Start)
		r.Get("/auth/{provider}/callback", a.d.OAuth.Callback)
	}

	// /admin/logs/tail — operator-gated NDJSON stream of recent log
	// entries from the diagnostics ring buffer. Auth is enforced inside
	// the handler via operator.RequireOperator so the route stays
	// alongside the public infra endpoints rather than being grafted
	// onto an authenticated sub-router. The auth chain still has to
	// inject auth.User onto ctx — auth.Optional is sufficient because
	// the handler returns 403 when no operator role is present.
	if a.d.Diagnostics != nil {
		tailHandler := a.d.Diagnostics.TailHandler()
		if a.d.Auth != nil {
			r.With(auth.Optional(a.d.Auth)).Get("/admin/logs/tail", tailHandler)
		} else {
			r.Get("/admin/logs/tail", tailHandler)
		}
	}

	// GraphQL — the API of record. The schema enforces auth per
	// resolver; the HTTP layer just mounts the gqlgen handler.
	gqlHandler := graphpkg.Handler(graphpkg.Config{
		Resolver:   a.newResolver(),
		Auth:       a.d.Auth,
		Logger:     a.d.Logger,
		Hardening:  a.d.Hardening,
		IsOperator: a.d.IsOperator,
		PolicyPEP:  a.d.PolicyPEP,
	})

	// V22 Wave-2 hardening middleware: CSRF → PersistedQueries →
	// RateLimiter → gqlHandler. Each layer is a no-op when its
	// dependency is unset so dev keeps working with the bare server.
	var gqlChain http.Handler = gqlHandler
	if lim := a.d.GqlRateLimiter; lim != nil {
		gqlChain = wrapGQLRateLimiter(lim, gqlChain)
	}
	if store := a.d.PersistedStore; store != nil && a.d.Hardening != nil {
		gqlChain = gqlhardening.PersistedQueriesMiddleware(store, a.d.Hardening.ProdMode, gqlhardening.OperatorCheck(a.d.IsOperator))(gqlChain)
	}
	// CSRF double-submit only makes sense when the web client and the
	// API share an origin (so the cookie set by the server is readable
	// by the SPA's JS). In dev, the SPA lives on a different origin
	// (localhost:3000 vs :8080) and never sees the cookie — CSRF then
	// rejects every signin / mutation. We therefore only enable the
	// middleware in prod, where same-origin deployment is the
	// expected topology.
	if a.d.Hardening != nil && a.d.Hardening.ProdMode {
		gqlChain = gqlhardening.CSRFMiddleware(gqlhardening.DefaultCSRFOptions(*a.d.Hardening))(gqlChain)
	}
	// Auth: signIn/signUp run anonymously; other resolvers each call
	// requireUser/currentUser. Use Optional so the bearer token is
	// attached when present without 401-ing every request.
	if a.d.Auth != nil {
		gqlChain = auth.Optional(a.d.Auth)(gqlChain)
	}
	r.Handle("/graphql", gqlChain)
	r.Handle("/graphql/", gqlChain)
	r.Handle("/graphql/sandbox", graphpkg.Sandbox("/graphql"))

	// POST /executions/{id}/chat/stream — Server-Sent Events surface
	// for raw LLM assistant token streams. Bypasses the GraphQL
	// transport so per-chunk gqlgen middleware (CSRF, persisted
	// queries, complexity, rate-limit) does not run on every token,
	// and so the wire format stays free of graphql-transport-ws
	// envelopes. Strict auth: anonymous requests are 401 before the
	// handler runs. Owner check + execution lookup happen inside the
	// handler. See chat_stream.go for the event vocabulary.
	if a.d.Auth != nil && a.d.Guard != nil && a.d.Execution != nil {
		r.With(auth.Middleware(a.d.Auth)).
			Post("/executions/{id}/chat/stream", a.chatStream)
	}

	// Light banner so curling the root of a stale deploy points the
	// operator at the new contract surface (GraphQL + V22 plan).
	r.Get("/", a.rootBanner)

	return r
}

// newResolver builds the *resolver.Resolver from the Deps so the
// GraphQL handler can construct its executable schema.
func (a *API) newResolver() *resolver.Resolver {
	return &resolver.Resolver{
		Auth:                      a.d.Auth,
		Billing:                   a.d.Billing,
		Telemetry:                 a.d.Telemetry,
		Projects:                  a.d.Projects,
		Engine:                    a.d.Engine,
		Agents:                    a.d.Agents,
		Patches:                   a.d.Patches,
		Guard:                     a.d.Guard,
		Logger:                    a.d.Logger,
		PublicBaseURL:             a.d.PublicBaseURL,
		Stripe:                    a.d.Stripe,
		AuditStore:                a.d.Audit,
		NotifyPrefs:               a.d.NotifyPrefs,
		Notifier:                  a.d.Notifier,
		NotifyStore:               a.d.NotifyStore,
		NotifyHub:                 a.d.NotifyHub,
		Verifications:             a.d.Verifications,
		PasswordResets:            a.d.PasswordResets,
		Sessions:                  a.d.Sessions,
		SessionCache:              a.d.SessionCache,
		EmailVerifier:             a.d.EmailVerifier,
		EmailChanger:              a.d.EmailChanger,
		PasswordRotator:           a.d.PasswordRotator,
		Email:                     a.d.Email,
		WebBaseURL:                a.d.WebBaseURL,
		AuthAudit:                 a.d.AuthAudit,
		PasswordResetIPLimiter:    a.d.PasswordResetIPLimiter,
		PasswordResetEmailLimiter: a.d.PasswordResetEmailLimiter,
		ResendVerificationLimiter: a.d.ResendVerificationLimiter,

		// Dev convenience wallet seed (no-op outside dev).
		DevEnv:           a.d.DevEnv,
		DevWalletSeedUSD: a.d.DevWalletSeedUSD,

		// V22 service surface.
		WalletSvc:         a.d.Wallet,
		WalletToppers:     a.d.WalletToppers,
		Compliance:        a.d.Compliance,
		LedgerSvc:         a.d.Ledger,
		ExecutionSvc:      a.d.Execution,
		ExecutionSettler:  a.d.ExecutionSettler,
		ProfitGuard:       a.d.ProfitGuard,
		ProfitGuardStore:  a.d.ProfitGuardStore,
		BlueprintsReg:     a.d.Blueprints,
		BlueprintStatsSvc: a.d.BlueprintStats,
		IdeaParser:        a.d.IdeaParser,
		Completion:        a.d.Completion,
		Repair:            a.d.Repair,
		PatchMemory:       a.d.PatchMemory,
		Dashboards:        a.d.Dashboards,
		DeploySvc:         a.d.Deploy,
		DeployDomainSvc:   a.d.DeployDomains,
		MemoryGraph:       a.d.MemoryGraph,

		// V22 Wave-3 services.
		Forecaster:            a.d.Forecaster,
		WowLoopBuilder:        a.d.WowLoopBuilder,
		AuditExporter:         a.d.AuditExporter,
		AuditExportConfig:     a.d.AuditExportConfig,
		SecurityReportBuilder: a.d.SecurityReportBuilder,
		Operator:              a.d.Operator,

		// Diagnostics — operator-gated recentErrors / recentLogs.
		Diagnostics: a.d.Diagnostics,

		// Mobile (EAS) plane.
		EAS:       a.d.EAS,
		EASPoller: a.d.EASPoller,

		// Mobile (device cloud) — Pro-tier real-device sessions.
		DeviceCloud: a.d.DeviceCloud,

		// Code Health Dashboard inputs.
		AtlasStore:        a.d.AtlasStore,
		ArchManifest:      a.d.ArchManifest,
		HealthReportPaths: a.d.HealthReportPaths,

		// Feedback Brain — learning store + publisher.
		LearningStore:     a.d.LearningStore,
		LearningPublisher: a.d.LearningPublisher,

		// Monetization SKUs — Ship Pass, Budget Sentinel, Finisher Guild.
		ShipPass:        a.d.ShipPass,
		ShipPassSettler: a.d.ShipPassSettler,
		Sentinel:        a.d.Sentinel,
		GuildCoord:      a.d.GuildCoord,
	}
}

// wrapGQLRateLimiter is the chi-compatible wrapper around the V22
// gqlhardening.Limiter. The limiter pulls the tenant + user from the
// auth-decorated context and the operation from the request body.
// On deny we return 429 with a Retry-After header so clients can back
// off without polling the resolver.
func wrapGQLRateLimiter(lim *gqlhardening.Limiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Resolve identity from ctx; anonymous traffic still passes
		// through the limiter under the synthetic "anonymous" tenant.
		tenant := "anonymous"
		user := "anonymous"
		if u, ok := auth.FromContext(r.Context()); ok {
			if u.OrgID != "" {
				tenant = u.OrgID
			} else if u.ID != "" {
				tenant = u.ID
			}
			user = u.ID
		}
		// Per-operation bucket: peek the request body for the GraphQL
		// operationName and bucket as "graphql:<opName>" so noisy
		// callers do not drain the budget of unrelated operations.
		// Falls back to "<method> <path>" when the peek yields nothing
		// (non-GraphQL routes, persisted-hash-only requests, multipart
		// uploads). See gqlhardening.OperationFromRequest.
		op := gqlhardening.OperationFromRequest(r, r.Method+" "+r.URL.Path)
		ok, retryAfter, err := lim.Allow(r.Context(), tenant, user, op)
		if err != nil || !ok {
			if retryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			}
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "rate limited",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rootBanner answers GET / with a small JSON pointer to the GraphQL
// endpoint and the V22 documentation. Helps operators who hit a
// browser at the orchestrator origin land somewhere actionable.
func (a *API) rootBanner(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service":  "ironflyer-orchestrator",
		"version":  a.d.Version,
		"graphql":  "/graphql",
		"sandbox":  "/graphql/sandbox",
		"contract": "docs/V22_PLAN.md",
	})
}

// ------------- health / liveness / readiness ---------------------------

var startedAt = time.Now().UTC()

func (a *API) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"version": a.d.Version,
		"uptime":  time.Since(startedAt).Round(time.Second).String(),
	})
}

func (a *API) livez(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"go":   runtime.Version(),
		"pid":  runtime.NumGoroutine(),
		"time": time.Now().UTC().Format(time.RFC3339),
	})
}

// readyz is intentionally cheap: it only confirms that the in-process
// dependencies the V22 surface needs are wired. External probes (Stripe,
// Postgres) are deferred to per-feature health endpoints introduced by
// later agents — a transient outage in one of those shouldn't take a
// healthy orchestrator pod out of rotation.
func (a *API) readyz(w http.ResponseWriter, _ *http.Request) {
	ready := a.d.Projects != nil && a.d.Engine != nil && a.d.Agents != nil
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{
		"ready": ready,
		"deps": map[string]bool{
			"projects": a.d.Projects != nil,
			"engine":   a.d.Engine != nil,
			"agents":   a.d.Agents != nil,
			"billing":  a.d.Billing != nil,
			"auth":     a.d.Auth != nil,
		},
	})
}

func (a *API) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":   a.d.Version,
		"commit":    a.d.Commit,
		"buildTime": a.d.BuildTime,
		"service":   "ironflyer-orchestrator",
	})
}

// ------------- Stripe webhook -----------------------------------------

// stripeWebhook is the only legacy REST surface that survives V22. The
// signature is verified in-band; events tagged metadata.purpose=
// wallet_topup route to wallet.Topper.HandleWebhook, every other event
// passes through to the budget package for plan updates.
func (a *API) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	stripeWalletTopper, _ := a.lookupWalletTopper(wallet.ProviderStripe)
	if a.d.Stripe == nil || !a.d.Stripe.Enabled() {
		// Wallet-only deployments still need to accept the webhook —
		// fall through to the wallet topper if the budget Stripe svc
		// is disabled but a topper is configured.
		if stripeWalletTopper == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "stripe disabled"})
			return
		}
	}
	defer r.Body.Close()
	body, err := readAll(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
		return
	}
	sig := r.Header.Get("Stripe-Signature")

	// Peek at metadata.purpose before signature-verifying so we can
	// route. The peek is on the raw body so the byte slice the
	// downstream verifier sees is identical to what Stripe signed.
	if purpose := peekStripePurpose(body); purpose == "wallet_topup" {
		if stripeWalletTopper == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "wallet topper not configured"})
			return
		}
		if err := stripeWalletTopper.HandleWebhook(r.Context(), body, sig); err != nil {
			a.d.Logger.Warn().Err(err).Msg("wallet webhook: handle failed")
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		// Receipt email — fire after the wallet credit landed. We do
		// not fail the webhook on a notifier outage (legal-relevant
		// but not synchronously required).
		a.sendTopUpReceipt(r.Context(), body)
		writeJSON(w, http.StatusOK, map[string]string{"received": "wallet_topup"})
		return
	}

	// Pass-through for legacy budget events.
	if a.d.Stripe == nil || !a.d.Stripe.Enabled() {
		writeJSON(w, http.StatusOK, map[string]string{"received": "ignored"})
		return
	}
	ev, err := a.d.Stripe.VerifyWebhook(body, sig, 5*time.Minute)
	if err != nil {
		a.d.Logger.Warn().Err(err).Msg("stripe webhook: verify failed")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if a.d.Billing != nil && a.d.Auth != nil {
		if err := a.d.Billing.ApplyStripeEvent(r.Context(), ev, a.d.Auth); err != nil {
			a.d.Logger.Warn().Err(err).Str("type", ev.Type).Msg("stripe webhook: apply event")
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"received": ev.Type})
}

// paddleWebhook is the Paddle Billing webhook receiver. Signature is
// verified inline against PADDLE_WEBHOOK_SECRET; the verified neutral
// event is routed to the billing plan setter when the payload carries
// a user_id + tier in custom_data. Subscription / refund accounting
// stays with Stripe today — Paddle ships behind the same wallet contract
// once the V22 wallet topper grows a Paddle adapter.
func (a *API) paddleWebhook(w http.ResponseWriter, r *http.Request) {
	paddleWalletTopper, _ := a.lookupWalletTopper(wallet.ProviderPaddle)
	if (a.d.Paddle == nil || !a.d.Paddle.Enabled()) && paddleWalletTopper == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "paddle disabled"})
		return
	}
	defer r.Body.Close()
	body, err := readAll(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
		return
	}
	sig := r.Header.Get("Paddle-Signature")

	// Peek at custom_data.purpose so wallet top-ups route to the wallet
	// Paddle topper without going through the subscription billing
	// path. The peek is cheap — only the first ~2KB of the JSON body
	// needs parsing — and on the raw bytes Paddle signed, so the
	// downstream verifier sees an identical slice.
	if purpose := peekPaddlePurpose(body); purpose == "wallet_topup" {
		if paddleWalletTopper == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "wallet topper not configured"})
			return
		}
		if err := paddleWalletTopper.HandleWebhook(r.Context(), body, sig); err != nil {
			a.d.Logger.Warn().Err(err).Msg("paddle wallet webhook: handle failed")
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		// Receipt email mirrors the Stripe path. Failures don't fail the
		// webhook — notifier outages must not block credit acknowledgement.
		a.sendPaddleTopUpReceipt(r.Context(), body)
		writeJSON(w, http.StatusOK, map[string]string{"received": "wallet_topup"})
		return
	}

	// Pass-through for subscription plan events.
	if a.d.Paddle == nil || !a.d.Paddle.Enabled() {
		writeJSON(w, http.StatusOK, map[string]string{"received": "ignored"})
		return
	}
	ev, err := a.d.Paddle.VerifyWebhook(body, sig)
	if err != nil {
		a.d.Logger.Warn().Err(err).Msg("paddle webhook: verify failed")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if a.d.Billing != nil && a.d.Auth != nil && ev.UserID != "" && ev.Tier != "" {
		switch ev.Type {
		case "transaction.completed", "subscription.created", "subscription.activated":
			if err := a.d.Auth.SetPlan(r.Context(), ev.UserID, string(ev.Tier)); err != nil {
				a.d.Logger.Warn().Err(err).Str("type", ev.Type).Msg("paddle webhook: set plan")
			}
			a.d.Billing.AssignPlan(r.Context(), ev.UserID, budget.PlanTier(ev.Tier))
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"received": ev.Type})
}

// lookupWalletTopper resolves the named provider in the topper
// registry. Returns (nil, false) when the registry is unset or the
// provider is disabled, so callers can fall through to a 503 cleanly.
func (a *API) lookupWalletTopper(name string) (wallet.Topper, bool) {
	if a.d.WalletToppers == nil {
		return nil, false
	}
	t, err := a.d.WalletToppers.ByName(name)
	if err != nil || t == nil {
		return nil, false
	}
	return t, true
}

// peekPaddlePurpose decodes just enough of a Paddle webhook body to
// find data.custom_data.purpose. Returns "" on parse failures so the
// caller falls through to the subscription branch.
func peekPaddlePurpose(body []byte) string {
	if len(body) == 0 || !bytes.Contains(body, []byte("custom_data")) {
		return ""
	}
	var peek struct {
		Data struct {
			CustomData map[string]string `json:"custom_data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &peek); err != nil {
		return ""
	}
	return peek.Data.CustomData["purpose"]
}

// peekStripePurpose decodes just enough of the raw event body to find
// data.object.metadata.purpose. Returns "" when the body is not JSON
// or the field is absent. Cheap — never touches the network.
func peekStripePurpose(body []byte) string {
	if len(body) == 0 || !bytes.Contains(body, []byte("metadata")) {
		return ""
	}
	var peek struct {
		Data struct {
			Object struct {
				Metadata map[string]string `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &peek); err != nil {
		return ""
	}
	return peek.Data.Object.Metadata["purpose"]
}

// sendTopUpReceipt fires the receipt email after a wallet top-up
// settles. The Stripe-Signature was already verified inside
// HandleWebhook, so re-parsing the same body for the receipt fields
// is safe. Errors are warning-logged and swallowed — the webhook
// response must not depend on the notifier being live.
func (a *API) sendTopUpReceipt(ctx context.Context, body []byte) {
	if a.d.Notifier == nil {
		a.d.Logger.Warn().Msg("wallet webhook: receipt skipped — notifier not configured")
		return
	}
	if a.d.Auth == nil {
		return
	}
	var peek struct {
		Data struct {
			Object struct {
				ID                string         `json:"id"`
				ClientReferenceID string         `json:"client_reference_id"`
				Currency          string         `json:"currency"`
				AmountTotal       int64          `json:"amount_total"`
				Metadata          map[string]any `json:"metadata"`
				CustomerDetails   struct {
					Email string `json:"email"`
					Name  string `json:"name"`
				} `json:"customer_details"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &peek); err != nil {
		a.d.Logger.Warn().Err(err).Msg("wallet webhook: receipt parse failed")
		return
	}
	obj := peek.Data.Object
	userID := obj.ClientReferenceID
	if userID == "" {
		if v, ok := obj.Metadata["tenant_id"].(string); ok {
			userID = v
		}
	}
	if userID == "" {
		return
	}
	currency := obj.Currency
	if currency == "" {
		currency = "usd"
	}
	amountCents := int(obj.AmountTotal)
	if amountCents == 0 {
		if v, ok := obj.Metadata["amount_usd"].(string); ok {
			if d, err := strconv.ParseFloat(v, 64); err == nil {
				amountCents = int(d * 100)
				if currency == "" {
					currency = "usd"
				}
			}
		}
	}
	email := obj.CustomerDetails.Email
	name := obj.CustomerDetails.Name
	if email == "" {
		u, err := a.d.Auth.GetByID(ctx, userID)
		if err != nil {
			a.d.Logger.Warn().Err(err).Str("user_id", userID).Msg("wallet webhook: receipt user lookup failed")
			return
		}
		email = u.Email
		if name == "" {
			name = u.Name
		}
	}
	if err := a.d.Notifier.Dispatch(ctx, userID, email, notify.KindReceipt, notify.ReceiptPayload{
		Name:            name,
		Currency:        currency,
		AmountCents:     amountCents,
		TransactionID:   obj.ID,
		StripeSessionID: obj.ID,
	}); err != nil {
		a.d.Logger.Warn().Err(err).Str("user_id", userID).Msg("wallet webhook: receipt dispatch failed")
	}
}

// sendPaddleTopUpReceipt mirrors sendTopUpReceipt for the Paddle
// path. Paddle's transaction.completed payload places the amount under
// data.details.totals.grand_total (minor units, string) and the email
// under data.customer.email; otherwise the routing is identical.
func (a *API) sendPaddleTopUpReceipt(ctx context.Context, body []byte) {
	if a.d.Notifier == nil {
		a.d.Logger.Warn().Msg("wallet webhook: receipt skipped — notifier not configured")
		return
	}
	if a.d.Auth == nil {
		return
	}
	var peek struct {
		Data struct {
			ID         string            `json:"id"`
			CustomData map[string]string `json:"custom_data"`
			Customer   struct {
				Email string `json:"email"`
				Name  string `json:"name"`
			} `json:"customer"`
			Details struct {
				Totals struct {
					CurrencyCode string `json:"currency_code"`
					GrandTotal   string `json:"grand_total"`
				} `json:"totals"`
			} `json:"details"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &peek); err != nil {
		a.d.Logger.Warn().Err(err).Msg("wallet webhook: paddle receipt parse failed")
		return
	}
	d := peek.Data
	userID := d.CustomData["tenant_id"]
	if userID == "" {
		return
	}
	currency := strings.ToLower(d.Details.Totals.CurrencyCode)
	if currency == "" {
		currency = "usd"
	}
	amountCents := 0
	if d.Details.Totals.GrandTotal != "" {
		if n, err := strconv.Atoi(d.Details.Totals.GrandTotal); err == nil {
			amountCents = n
		}
	}
	if amountCents == 0 {
		if v, ok := d.CustomData["amount_usd"]; ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				amountCents = int(f * 100)
			}
		}
	}
	email := d.Customer.Email
	name := d.Customer.Name
	if email == "" {
		u, err := a.d.Auth.GetByID(ctx, userID)
		if err != nil {
			a.d.Logger.Warn().Err(err).Str("user_id", userID).Msg("wallet webhook: paddle receipt user lookup failed")
			return
		}
		email = u.Email
		if name == "" {
			name = u.Name
		}
	}
	if err := a.d.Notifier.Dispatch(ctx, userID, email, notify.KindReceipt, notify.ReceiptPayload{
		Name:            name,
		Currency:        currency,
		AmountCents:     amountCents,
		TransactionID:   d.ID,
		StripeSessionID: d.ID,
	}); err != nil {
		a.d.Logger.Warn().Err(err).Str("user_id", userID).Msg("wallet webhook: paddle receipt dispatch failed")
	}
}

// ------------- helpers ------------------------------------------------

// writeJSON is a thin shim for legacy call sites — the implementation
// now lives in internal/pkg/httputil.WriteJSON. New code should import
// httputil directly; this shim exists so the migration is incremental.
var writeJSON = httputil.WriteJSON

// readAll drains an HTTP body with a hard 1 MiB cap so a malicious or
// misconfigured webhook can't OOM the orchestrator.
func readAll(r *http.Request) ([]byte, error) {
	const maxBytes = 1 << 20 // 1 MiB
	limited := http.MaxBytesReader(nil, r.Body, maxBytes)
	buf := make([]byte, 0, 4096)
	chunk := make([]byte, 4096)
	for {
		n, err := limited.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return buf, nil
			}
			return buf, err
		}
	}
}

// keep the bus import referenced even when this minimal V22 surface
// doesn't call it directly — the wallet/ledger agents will reach for
// it as soon as they wire their resolvers.
var (
	_ = bus.NewMultiplexer
	_ = strings.TrimSpace
	_ = context.Background
)
