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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/auditexport"
	"ironflyer/apps/orchestrator/internal/auth"
	"ironflyer/apps/orchestrator/internal/blueprints"
	"ironflyer/apps/orchestrator/internal/budget"
	"ironflyer/apps/orchestrator/internal/bus"
	"ironflyer/apps/orchestrator/internal/completion"
	"ironflyer/apps/orchestrator/internal/dashboards"
	"ironflyer/apps/orchestrator/internal/deploy"
	"ironflyer/apps/orchestrator/internal/diagnostics"
	"ironflyer/apps/orchestrator/internal/execution"
	"ironflyer/apps/orchestrator/internal/finisher"
	"ironflyer/apps/orchestrator/internal/forecast"
	"ironflyer/apps/orchestrator/internal/gqlhardening"
	graphpkg "ironflyer/apps/orchestrator/internal/graph"
	"ironflyer/apps/orchestrator/internal/graph/resolver"
	"ironflyer/apps/orchestrator/internal/ideaparser"
	"ironflyer/apps/orchestrator/internal/ledger"
	"ironflyer/apps/orchestrator/internal/memory"
	"ironflyer/apps/orchestrator/internal/memorygraph"
	"ironflyer/apps/orchestrator/internal/metrics"
	"ironflyer/apps/orchestrator/internal/notify"
	"ironflyer/apps/orchestrator/internal/operator"
	"ironflyer/apps/orchestrator/internal/patch"
	"ironflyer/apps/orchestrator/internal/policy"
	"ironflyer/apps/orchestrator/internal/profitguard"
	"ironflyer/apps/orchestrator/internal/providers"
	"ironflyer/apps/orchestrator/internal/ratelimit"
	"ironflyer/apps/orchestrator/internal/repair"
	"ironflyer/apps/orchestrator/internal/securityreport"
	"ironflyer/apps/orchestrator/internal/store"
	"ironflyer/apps/orchestrator/internal/wallet"
	"ironflyer/apps/orchestrator/internal/wowloop"
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
	Guard   *providers.BillingGuard

	// Authentication.
	Auth         *auth.Service
	AuthOptional bool

	// AllowedOrigins is the comma-separated list of browser origins the
	// CORS middleware should reflect into Access-Control-Allow-Origin.
	// Empty means "reflect any origin" — only safe in dev. Production
	// MUST set this to the exact list of web origins (e.g.
	// https://app.ironflyer.dev). Tied to the orchestrator's
	// IRONFLYER_CORS_ORIGINS env var.
	AllowedOrigins []string

	// Memory + audit + telemetry.
	Memory    memory.Store
	Audit     audit.Store
	Telemetry providers.TelemetrySink
	Bus       *bus.Multiplexer

	// Notifications (email only in V22).
	NotifyPrefs notify.PrefsStore
	Notify      *notify.Engine

	// Runtime workspace client (for legacy hand-off; the resolver does
	// not yet expose workspace operations in V22).
	RuntimeURL string

	// Build identity surfaced by /version.
	Version   string
	Commit    string
	BuildTime string

	// Dev convenience: when DevEnv == "dev" and DevWalletSeedUSD > 0,
	// the SignUp resolver credits the new wallet so describeIdea works
	// without Stripe. Wired from config.Config in main.go; ignored in
	// staging / prod.
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
	Wallet           wallet.Service
	WalletTopper     *wallet.Topper
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
	r.Use(middleware.Recoverer)
	// RequestIDMiddleware mints / honours X-Request-ID and stamps it on
	// ctx + the response header before metrics or auth run, so failure
	// log lines from those layers carry the same correlation id.
	r.Use(RequestIDMiddleware(a.d.Logger))
	r.Use(metrics.HTTP)
	// CORS — the web SPA runs on a different origin than the
	// orchestrator. The browser blocks credentialed POST requests
	// unless the server explicitly opts in. We reflect Origin (so
	// Access-Control-Allow-Credentials:true is valid; `*` is not
	// allowed with credentials) and allow the headers the SPA sends.
	r.Use(corsMiddleware(a.d.AllowedOrigins))

	// Public infra endpoints — NEVER authenticated.
	r.Get("/healthz", a.healthz)
	r.Get("/livez", a.livez)
	r.Get("/readyz", a.readyz)
	r.Get("/version", a.version)
	r.Method(http.MethodGet, "/metrics", metrics.Handler())

	// Stripe webhook — third-party callback, signature-verified inline.
	r.Post("/budget/webhook", a.stripeWebhook)

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
		WalletTopper:      a.d.WalletTopper,
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
	if a.d.Stripe == nil || !a.d.Stripe.Enabled() {
		// Wallet-only deployments still need to accept the webhook —
		// fall through to the wallet topper if the budget Stripe svc
		// is disabled but a topper is configured.
		if a.d.WalletTopper == nil || !a.d.WalletTopper.Enabled() {
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
		if a.d.WalletTopper == nil || !a.d.WalletTopper.Enabled() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "wallet topper not configured"})
			return
		}
		if err := a.d.WalletTopper.HandleWebhook(r.Context(), body, sig); err != nil {
			a.d.Logger.Warn().Err(err).Msg("wallet webhook: handle failed")
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
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

// ------------- helpers ------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

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
