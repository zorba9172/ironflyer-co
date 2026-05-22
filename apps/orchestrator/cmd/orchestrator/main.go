package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/auth"
	"ironflyer/apps/orchestrator/internal/brainstorm"
	"ironflyer/apps/orchestrator/internal/budget"
	"ironflyer/apps/orchestrator/internal/config"
	"ironflyer/apps/orchestrator/internal/finisher"
	"ironflyer/apps/orchestrator/internal/httpapi"
	"ironflyer/apps/orchestrator/internal/integrations"
	"ironflyer/apps/orchestrator/internal/integrations/github"
	"ironflyer/apps/orchestrator/internal/leads"
	"ironflyer/apps/orchestrator/internal/notify"
	"ironflyer/apps/orchestrator/internal/patch"
	"ironflyer/apps/orchestrator/internal/providers"
	"ironflyer/apps/orchestrator/internal/runtime"
	"ironflyer/apps/orchestrator/internal/store"
	"ironflyer/apps/orchestrator/internal/webhooks"
	"ironflyer/apps/orchestrator/internal/workflow"
)

func main() {
	_ = godotenv.Load(".env", ".env.local")

	cfg, err := config.Load()
	logger := buildLogger(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("config load failed")
	}

	ctx := context.Background()

	// ---------------- Postgres pool (shared by budget + auth) ---------------
	var pgPool *pgxpool.Pool
	if cfg.UsePostgres() {
		p, err := budget.ConnectPostgres(ctx, cfg.PostgresURL)
		if err != nil {
			logger.Fatal().Err(err).Msg("connect postgres")
		}
		pgPool = p
	}

	// ---------------- Project store (in-memory or SurrealDB) ----------------
	var projects store.Store
	if cfg.UseSurreal() {
		db, err := store.ConnectSurreal(ctx, store.SurrealOpts{
			URL: cfg.SurrealURL, Namespace: cfg.SurrealNS, Database: cfg.SurrealDB,
			User: cfg.SurrealUser, Pass: cfg.SurrealPass,
		})
		if err != nil {
			logger.Fatal().Err(err).Msg("connect surrealdb")
		}
		if err := store.BootstrapSurreal(ctx, db); err != nil {
			logger.Fatal().Err(err).Msg("bootstrap surreal schema")
		}
		ss := store.NewSurrealStore(ctx, db)
		if err := ss.Seed(); err != nil {
			logger.Warn().Err(err).Msg("surreal seed failed (continuing)")
		}
		projects = ss
		logger.Info().Str("url", cfg.SurrealURL).Str("ns", cfg.SurrealNS).
			Str("db", cfg.SurrealDB).Msg("SurrealDB store enabled")
	} else {
		ms := store.NewMemoryStore()
		ms.Seed()
		projects = ms
		logger.Info().Msg("memory project store enabled")
	}

	// ---------------- Budget stores (in-memory or Postgres) -----------------
	var ledger budget.LedgerStore
	var vault budget.VaultStore
	if pgPool != nil {
		if err := budget.BootstrapPostgres(ctx, pgPool); err != nil {
			logger.Fatal().Err(err).Msg("bootstrap postgres schema (budget)")
		}
		ledger = budget.NewPostgresLedger(pgPool)
		vault = budget.NewPostgresVault(pgPool)
		logger.Info().Msg("Postgres budget store enabled")
	} else {
		ledger = budget.NewMemoryLedger()
		vault = budget.NewMemoryVault()
		logger.Info().Msg("memory budget store enabled")
	}
	billing := budget.NewBilling(ledger, vault)

	// ---------------- Stripe (optional) -------------------------------------
	stripeSvc := budget.NewStripeService(budget.StripeOpts{
		SecretKey:     cfg.StripeSecretKey,
		WebhookSecret: cfg.StripeWebhookSecret,
		Prices: map[budget.PlanTier]string{
			budget.TierPro:        cfg.StripePricePro,
			budget.TierTeam:       cfg.StripePriceTeam,
			budget.TierEnterprise: cfg.StripePriceEnterprise,
		},
		SuccessURL: cfg.StripeSuccessURL,
		CancelURL:  cfg.StripeCancelURL,
	})
	if stripeSvc.Enabled() {
		logger.Info().Msg("Stripe checkout + webhook enabled")
	} else {
		logger.Warn().Msg("Stripe disabled (set STRIPE_SECRET_KEY + STRIPE_WEBHOOK_SECRET)")
	}

	// ---------------- Auth store + service ----------------------------------
	var userStore auth.UserStore
	if pgPool != nil {
		if err := auth.BootstrapPostgres(ctx, pgPool); err != nil {
			logger.Fatal().Err(err).Msg("bootstrap postgres schema (auth)")
		}
		userStore = auth.NewPostgresUserStore(pgPool)
		logger.Info().Msg("Postgres user store enabled")
	} else {
		userStore = auth.NewMemoryUserStore()
		logger.Info().Msg("memory user store enabled (demo@ironflyer.dev / demo1234)")
	}
	authSvc := auth.NewService(userStore, []byte(cfg.JWTSecret), cfg.JWTIssuer, 7*24*time.Hour)
	// Make sure the seeded demo user has a Pro plan reflected in billing.
	if u, _, err := userStore.GetByEmail(ctx, "demo@ironflyer.dev"); err == nil {
		billing.AssignPlan(ctx, u.ID, budget.TierPro)
	}

	// ---------------- Leads (memory or postgres) ----------------------------
	var leadStore leads.Store
	if pgPool != nil {
		if err := leads.BootstrapPostgres(ctx, pgPool); err != nil {
			logger.Fatal().Err(err).Msg("bootstrap postgres schema (leads)")
		}
		leadStore = leads.NewPostgresStore(pgPool)
		logger.Info().Msg("Postgres lead store enabled")
	} else {
		leadStore = leads.NewMemoryStore()
		logger.Info().Msg("memory lead store enabled")
	}

	// ---------------- Integrations: GitHub ----------------------------------
	var tokenStore integrations.TokenStore
	if pgPool != nil {
		if err := integrations.BootstrapPostgres(ctx, pgPool); err != nil {
			logger.Fatal().Err(err).Msg("bootstrap postgres schema (integrations)")
		}
		tokenStore = integrations.NewPostgresTokenStore(pgPool)
		logger.Info().Msg("Postgres integration token store enabled")
	} else {
		tokenStore = integrations.NewMemoryTokenStore()
		logger.Info().Msg("memory integration token store enabled")
	}
	githubSvc := github.New(github.Config{
		ClientID:     cfg.GitHubClientID,
		ClientSecret: cfg.GitHubClientSecret,
		RedirectURL:  cfg.GitHubRedirectURL,
	}, tokenStore)
	if githubSvc.Enabled() {
		logger.Info().Str("redirect", cfg.GitHubRedirectURL).Msg("GitHub integration enabled")
	} else {
		logger.Warn().Msg("GitHub integration disabled (set GITHUB_CLIENT_ID + GITHUB_CLIENT_SECRET)")
	}

	// ---------------- Providers + Guard + Agents ----------------------------
	router := providers.NewRouter()
	router.Register(providers.NewMockProvider("mock"))
	if cfg.AnthropicAPIKey != "" {
		router.Register(providers.NewAnthropicProvider(providers.AnthropicOpts{
			APIKey: cfg.AnthropicAPIKey, Model: cfg.AnthropicModel,
		}))
		logger.Info().Str("model", cfg.AnthropicModel).Msg("Anthropic provider registered")
	} else {
		logger.Warn().Msg("ANTHROPIC_API_KEY not set — running on mock provider only")
	}
	guard := providers.NewBillingGuard(router, billing)

	registry := agents.NewRegistry(router)
	registry.RegisterDefaults()

	strategist := brainstorm.NewStrategist()
	bsRunner := brainstorm.NewRunner(registry, router)

	patches := patch.NewEngine(projects)
	runtimeClient := runtime.New(cfg.RuntimeURL)
	engine := finisher.NewEngine(projects, registry, patches).
		WithRuntime(runtimeClient).
		WithApplier(runtime.NewApplier(runtimeClient))
	if runtimeClient.Enabled() {
		logger.Info().Str("runtime", cfg.RuntimeURL).
			Msg("finisher wired to runtime: build/test gates exec inside workspaces; approved patches materialise via the File API")
	} else {
		logger.Warn().Msg("runtime not configured — finisher patches will validate but not materialise (set IRONFLYER_RUNTIME_URL)")
	}

	// ---------------- Webhooks + Notifications (optional, nil-safe) ---------
	// Each piece may be nil; the HTTP handlers return 503 on missing deps so a
	// partial config (no email provider, no postgres) still boots cleanly.
	webhookStore := webhooks.NewMemoryStore()
	webhookDispatcher := webhooks.NewDispatcher(webhookStore, logger)
	prefsStore := notify.NewMemoryPrefsStore()
	emailSender := notify.SenderFromEnv(cfg.EmailProvider, cfg.EmailAPIKey, cfg.EmailFromAddress, logger)
	notifyEngine := notify.NewEngine(projects, prefsStore, emailSender, webhookDispatcher, logger).
		WithDashboardURL(cfg.DashboardURL)
	webhookDispatcher.WithNotifier(notifyEngine) // auto-disabled webhooks email the user
	notifyEngine.SubscribeAll(ctx, engine)
	logger.Info().Str("email_provider", cfg.EmailProvider).
		Bool("webhooks", webhookStore != nil).
		Msg("notification pipeline online")

	api := httpapi.New(httpapi.Deps{
		Projects: projects, Engine: engine, Agents: registry, Patches: patches,
		Billing: billing, Strategist: strategist, BSRunner: bsRunner, Guard: guard,
		Auth: authSvc, AuthOptional: cfg.AuthOptional, Stripe: stripeSvc, Leads: leadStore,
		GitHub: githubSvc, GitHubTokens: tokenStore, GitHubPostLoginURL: cfg.GitHubPostLoginURL,
		RuntimeURL: cfg.RuntimeURL,
		Webhooks:   webhookStore, WebhookDispatcher: webhookDispatcher,
		NotifyPrefs: prefsStore, Notify: notifyEngine,
		Logger: logger,
	})

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info().Str("addr", cfg.Addr).Str("env", cfg.Env).
			Str("db", cfg.DBDriver).Bool("auth_optional", cfg.AuthOptional).
			Msg("orchestrator listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server")
		}
	}()

	// ---------------- Temporal worker (optional) ---------------------------
	// Only start a worker when IRONFLYER_TEMPORAL_HOST is explicitly set.
	// Embedded executor remains the default and stays untouched. Worker
	// failures degrade gracefully — a warn log, no fatal — so a misconfigured
	// Temporal host never blocks the rest of the orchestrator from serving.
	if cfg.TemporalHost != "" {
		acts := workflow.NewActivities(projects, registry).WithRuntime(runtimeClient)
		_, tStop, tErr := workflow.StartWorker(workflow.WorkerOptions{
			TemporalAddr: cfg.TemporalHost,
			Namespace:    cfg.TemporalNamespace,
			TaskQueue:    cfg.TemporalTaskQueue,
		}, acts, logger)
		if tErr != nil {
			logger.Warn().Err(tErr).Str("host", cfg.TemporalHost).
				Msg("temporal worker disabled")
		} else {
			logger.Info().Str("host", cfg.TemporalHost).
				Str("ns", cfg.TemporalNamespace).
				Str("queue", cfg.TemporalTaskQueue).
				Msg("temporal worker online")
			defer tStop()
		}
	} else {
		logger.Info().Msg("temporal worker skipped (IRONFLYER_TEMPORAL_HOST not set)")
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info().Msg("shutting down")
	shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shCtx)
	if pgPool != nil {
		pgPool.Close()
	}
}

func buildLogger(cfg config.Config) zerolog.Logger {
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	if cfg.LogFormat == "console" {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
			With().Timestamp().Str("svc", "orchestrator").Logger()
	}
	return zerolog.New(os.Stderr).With().Timestamp().Str("svc", "orchestrator").Logger()
}
