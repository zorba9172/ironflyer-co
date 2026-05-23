package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	surrealdb "github.com/surrealdb/surrealdb.go"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/auth"
	"ironflyer/apps/orchestrator/internal/brainstorm"
	"ironflyer/apps/orchestrator/internal/budget"
	"ironflyer/apps/orchestrator/internal/config"
	"ironflyer/apps/orchestrator/internal/context7"
	"ironflyer/apps/orchestrator/internal/embeddings"
	"ironflyer/apps/orchestrator/internal/figma"
	"ironflyer/apps/orchestrator/internal/finisher"
	"ironflyer/apps/orchestrator/internal/httpapi"
	"ironflyer/apps/orchestrator/internal/imagegen"
	"ironflyer/apps/orchestrator/internal/integrations"
	"ironflyer/apps/orchestrator/internal/integrations/github"
	"ironflyer/apps/orchestrator/internal/leads"
	"ironflyer/apps/orchestrator/internal/memory"
	"ironflyer/apps/orchestrator/internal/notify"
	"ironflyer/apps/orchestrator/internal/patch"
	"ironflyer/apps/orchestrator/internal/providers"
	"ironflyer/apps/orchestrator/internal/redisbus"
	"ironflyer/apps/orchestrator/internal/runtime"
	"ironflyer/apps/orchestrator/internal/store"
	"ironflyer/apps/orchestrator/internal/tracing"
	"ironflyer/apps/orchestrator/internal/webhooks"
	"ironflyer/apps/orchestrator/internal/workflow"
)

// Build identifiers. Populated at link time via -ldflags (see
// apps/orchestrator/Makefile). In a plain `go run` / `go build`
// development invocation these stay at their defaults so /version
// still answers — it just reports "dev" / "unknown", which is the
// correct signal that the binary was not produced by the release
// pipeline.
var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildTime    = "unknown"
)

func main() {
	_ = godotenv.Load(".env", ".env.local")

	cfg, err := config.Load()
	logger := buildLogger(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("config load failed")
	}

	ctx := context.Background()

	// ---------------- OpenTelemetry tracing (optional) ---------------------
	// Tracing must come up before anything that issues spans. Init is fail-
	// soft: any exporter problem logs a warning and leaves the no-op provider
	// installed so the orchestrator boots regardless of collector state.
	tracingShutdown, err := tracing.Init(ctx, tracing.Opts{
		Exporter:       cfg.OTelExporter,
		Endpoint:       cfg.OTelEndpoint,
		Insecure:       cfg.OTelInsecure,
		ServiceName:    "ironflyer-orchestrator",
		ServiceVersion: "1.0",
		SampleRatio:    cfg.OTelSampleRatio,
		Headers:        parseOTelHeaders(cfg.OTelHeaders),
	})
	if err != nil {
		logger.Warn().Err(err).Msg("tracing init failed; continuing without OTel")
	} else if cfg.OTelExporter != "none" && cfg.OTelExporter != "" {
		logger.Info().Str("exporter", cfg.OTelExporter).Msg("OTel tracing initialised")
	}
	defer func() {
		if tracingShutdown != nil {
			shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tracingShutdown(shCtx)
		}
	}()

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
	// Hoisted to the outer scope so the memory + audit construction
	// sites below can attach to the same SurrealDB connection.
	var surrealDB *surrealdb.DB
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
		surrealDB = db
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

	// ---------------- Redis (optional, multi-pod coordination) -------------
	// When IRONFLYER_REDIS_ENABLED=true the orchestrator can run as 2+ pods
	// safely: the finisher Engine acquires a distributed lock per project
	// run, and the rate limiter swap-in points use Redis as the single
	// source of truth across pods. Left disabled, every helper degrades to
	// the in-process implementation that ships single-pod by default.
	var redisClient *redisbus.Client
	if cfg.RedisEnabled {
		rc, err := redisbus.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
		if err != nil {
			logger.Fatal().Err(err).Msg("connect redis")
		}
		if err := rc.Ping(ctx); err != nil {
			logger.Fatal().Err(err).Msg("ping redis")
		}
		redisClient = rc
		logger.Info().Str("addr", cfg.RedisAddr).Msg("Redis enabled (distributed locks + rate limit)")
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
	if cfg.OpenAIAPIKey != "" {
		router.Register(providers.NewOpenAIProvider(providers.OpenAIOpts{
			APIKey: cfg.OpenAIAPIKey, Model: cfg.OpenAIModel,
		}))
		logger.Info().Str("model", cfg.OpenAIModel).Msg("OpenAI provider registered")
	}
	if cfg.GeminiAPIKey != "" {
		router.Register(providers.NewGeminiProvider(providers.GeminiOpts{
			APIKey: cfg.GeminiAPIKey, Model: cfg.GeminiModel,
		}))
		logger.Info().Str("model", cfg.GeminiModel).Msg("Gemini provider registered")
	}
	if cfg.HFAPIKey != "" {
		router.Register(providers.NewHuggingFaceProvider(providers.HuggingFaceOpts{
			APIKey: cfg.HFAPIKey,
		}))
		logger.Info().Msg("HuggingFace provider registered (Llama 3.3 / Qwen / DeepSeek / Mixtral)")
	}
	telemetrySink := providers.NewMemorySink(2048)
	router.WithBandit(&providers.Bandit{Sink: telemetrySink})
	logger.Info().Msg("Router bandit enabled (UCB1 over telemetry)")
	guard := providers.NewBillingGuard(router, billing).WithTelemetry(telemetrySink)
	// ---------------- Memory + audit stores (in-memory or SurrealDB) -------
	// The "surreal" backend only kicks in when the project-store branch
	// above also connected to SurrealDB. Otherwise we keep the bounded
	// ring buffer so the orchestrator stays bootable with zero infra.
	var memoryStore memory.Store
	if cfg.MemoryBackend == "surreal" && surrealDB != nil {
		if err := memory.BootstrapSurreal(ctx, surrealDB); err != nil {
			logger.Fatal().Err(err).Msg("memory: surreal bootstrap")
		}
		memoryStore = memory.NewSurrealStore(surrealDB)
		logger.Info().Msg("Memory store: SurrealDB (persistent)")
	} else {
		memoryStore = memory.NewMemoryStore(4096)
		logger.Info().Msg("Memory store: in-process ring buffer")
	}
	// Semantic-search wrapper: when HF_API_KEY is configured, every
	// memory.Query with a non-empty Substring is re-ranked by cosine
	// similarity against HuggingFace-encoded embeddings. Nil-safe: an
	// unset key leaves the underlying store untouched (substring fallback).
	if cfg.HFAPIKey != "" {
		memoryStore = &memory.VectorStore{
			Inner:    memoryStore,
			Embedder: embeddings.NewHuggingFaceEmbedder(cfg.HFAPIKey, cfg.HFEmbedModel),
		}
		logger.Info().Str("model", cfg.HFEmbedModel).Msg("Memory store: HF semantic re-ranking enabled")
	}

	var auditStore audit.Store
	if cfg.AuditBackend == "surreal" && surrealDB != nil {
		s, err := audit.NewSurrealStore(ctx, surrealDB)
		if err != nil {
			logger.Fatal().Err(err).Msg("audit: surreal bootstrap")
		}
		auditStore = s
		logger.Info().Msg("Audit store: SurrealDB (persistent hash chain)")
	} else {
		auditStore = audit.NewMemoryStore(16 * 1024)
		logger.Info().Msg("Audit store: in-process hash chain")
	}

	registry := agents.NewRegistry(router)
	registry.RegisterDefaults()

	// ---------------- MCP clients (optional, Coder-only) ------------------
	// IRONFLYER_MCP_SERVERS is a comma-separated list of "name=URL" pairs;
	// per-server bearer tokens come from IRONFLYER_MCP_TOKEN_<UPPER>.
	// Failures here are degraded to warnings — a misconfigured outbound
	// server should never block the orchestrator from booting.
	mcpRegistry := providers.NewMCPClientRegistry()
	mcpNames := parseMCPServers(cfg.MCPServers, mcpRegistry, logger)
	// Context7 is registered before WithMCPClients so its tools are
	// part of the catalogue the Coder sees from the first turn. It is
	// safe to add even when the operator already listed "context7=..."
	// in IRONFLYER_MCP_SERVERS — the registry doesn't dedupe by name,
	// so we skip the auto-registration in that case to avoid two
	// `context7.*` prefixes shadowing each other.
	if cfg.Context7Enabled {
		alreadyConfigured := false
		for _, n := range mcpNames {
			if n == context7.Name {
				alreadyConfigured = true
				break
			}
		}
		if alreadyConfigured {
			logger.Info().Msg("Context7: skipped auto-registration (already listed in IRONFLYER_MCP_SERVERS)")
		} else {
			c7Client := context7.NewClient(cfg.Context7AuthToken)
			mcpRegistry.Register(c7Client)
			mcpNames = append(mcpNames, context7.Name)
			c7Tool := &context7.Tool{Client: c7Client}
			registry.WithBuiltinTool(c7Tool.Spec(), c7Tool.Call)
			logger.Info().Msg("Context7: registered as MCP server + builtin lookup_docs tool")
		}
	}
	registry.WithMCPClients(mcpRegistry)
	if len(mcpNames) > 0 {
		logger.Info().Int("count", len(mcpNames)).Strs("servers", mcpNames).
			Msg("MCP clients configured")
	} else {
		logger.Info().Msg("MCP clients: 0 configured (set IRONFLYER_MCP_SERVERS=name=URL,...)")
	}

	strategist := brainstorm.NewStrategist()
	bsRunner := brainstorm.NewRunner(registry, router)

	patches := patch.NewEngine(projects)
	patches.
		WithOnProposed(func(p patch.Patch) {
			_, _ = auditStore.Record(ctx, audit.Entry{
				Action:    audit.ActionPatchProposed,
				Outcome:   audit.OutcomeSuccess,
				ProjectID: p.ProjectID,
				Summary:   "patch_proposed id=" + p.ID + " title=" + p.Title,
				Attrs:     map[string]any{"changeCount": len(p.Changes)},
			})
		}).
		WithOnApplied(func(p patch.Patch) {
			_, _ = auditStore.Record(ctx, audit.Entry{
				Action:    audit.ActionPatchApplied,
				Outcome:   audit.OutcomeSuccess,
				ProjectID: p.ProjectID,
				Summary:   "patch_applied id=" + p.ID + " title=" + p.Title,
				Attrs:     map[string]any{"changeCount": len(p.Changes)},
			})
		}).
		WithOnRolledBack(func(p patch.Patch, snapID string) {
			_, _ = auditStore.Record(ctx, audit.Entry{
				Action:    audit.ActionPatchRolledBack,
				Outcome:   audit.OutcomeSuccess,
				ProjectID: p.ProjectID,
				Summary:   "patch_rolled_back id=" + p.ID + " snapshot=" + snapID,
			})
		})
	runtimeClient := runtime.New(cfg.RuntimeURL)

	// ---------------- Built-in Coder tools (generate_image) ---------------
	// The image-generation tool gives the Coder parity with Lovable's
	// inline image generation: mid-Run it can produce a PNG asset and
	// reference it in the patch it then emits. Provider selection is
	// best-effort — when no API key is configured we still register
	// the tool but back it with a Noop provider so calls fail with a
	// readable "image generation disabled" rather than a missing-tool
	// error from the model.
	var imgProv imagegen.Provider = imagegen.NoopProvider{}
	imgKey := cfg.OpenAIImageAPIKey
	if imgKey == "" {
		imgKey = cfg.OpenAIAPIKey
	}
	if imgKey != "" {
		imgProv = &imagegen.OpenAIImagesProvider{APIKey: imgKey}
		logger.Info().Msg("Image generation: OpenAI Images")
	} else {
		logger.Warn().Msg("Image generation disabled (OPENAI_IMAGE_API_KEY or OPENAI_API_KEY missing)")
	}
	imgTool := &imagegen.Tool{
		Gen:       imgProv,
		Writer:    runtimeClient,
		AssetsDir: "public/assets",
		MaxBytes:  4 << 20,
	}
	registry.WithBuiltinTool(imgTool.Spec(), imgTool.Call)

	// ---------------- Built-in Coder tools (figma_import) -----------------
	// The Figma importer turns a Figma file URL into the design-tokens
	// manifest + component inventory the Coder consumes. The tool is
	// always registered so the model sees it — when FIGMA_TOKEN is
	// empty, calls fail with "figma token not configured" rather than
	// a missing-tool error.
	figmaClient := figma.New(cfg.FigmaToken)
	figmaTool := &figma.Tool{Client: figmaClient, Writer: runtimeClient}
	registry.WithBuiltinTool(figmaTool.Spec(), figmaTool.Call)
	if cfg.FigmaToken != "" {
		logger.Info().Msg("Figma import enabled")
	}

	engine := finisher.NewEngine(projects, registry, patches).
		WithRuntime(runtimeClient).
		WithApplier(runtime.NewApplier(runtimeClient)).
		WithRedis(redisClient).
		WithDBProvisioner(selectDBProvisioner(cfg, logger)).
		WithAuthScaffolder(finisher.DefaultAuthScaffolder{}).
		WithStripeScaffolder(finisher.DefaultStripeScaffolder{}).
		WithDomainScaffolders(
			finisher.GameScaffolder{},
			finisher.EcommerceScaffolder{},
			finisher.MobileScaffolder{},
			finisher.SocialScaffolder{},
			finisher.LearningScaffolder{},
			finisher.DashboardScaffolder{},
			// Native-language backend scaffolders (Rust, Go, Python).
			finisher.RustScaffolder{},
			finisher.GoHTTPScaffolder{},
			finisher.PythonFastAPIScaffolder{},
			// JVM + native mobile scaffolders.
			finisher.JavaSpringScaffolder{},
			finisher.KotlinAndroidScaffolder{},
			finisher.SwiftIOSScaffolder{},
			// Long-tail backend scaffolders (Ruby, PHP, .NET).
			finisher.RailsScaffolder{},
			finisher.LaravelScaffolder{},
			finisher.DotNetScaffolder{},
			// Production CI/CD bundle (GitHub Actions + Argo + K8s manifests).
			finisher.CICDScaffolder{},
		).
		WithMemory(memoryStore).
		WithAudit(auditStore).
		WithBudgetSource(func(ctx context.Context, userID, projectID string) (*finisher.BudgetSnapshot, error) {
			// Resolve the user's plan, sum what they've spent this period,
			// and project the gate-readable posture. Unauthenticated runs
			// (userID == "") still get a snapshot keyed against the Free
			// plan so the cap is meaningful on the seed/demo project.
			tier := billing.PlanFor(userID)
			var plan budget.Plan
			for _, p := range billing.Plans {
				if p.Tier == tier {
					plan = p
					break
				}
			}
			spent, err := billing.Ledger.SpentByUser(ctx, userID)
			if err != nil {
				return nil, err
			}
			capF, _ := plan.CostCapUSD.Float64()
			spentF, _ := spent.Float64()
			return &finisher.BudgetSnapshot{
				CapUSD:       capF,
				SpentUSD:     spentF,
				RemainingUSD: capF - spentF,
				HardStop:     plan.HardStop,
				Reason:       "plan=" + string(plan.Tier),
			}, nil
		})
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
		Telemetry: telemetrySink,
		Memory: memoryStore,
		Audit: auditStore,
		Version:   buildVersion,
		Commit:    buildCommit,
		BuildTime: buildTime,
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

// selectDBProvisioner returns the configured DBProvisioner. Defaults to
// the no-op implementation when nothing is wired so the finisher loop
// still boots cleanly in dev. Misconfigured Supabase (missing PAT or
// org id) logs a warning and falls back to no-op rather than crashing —
// projects that need a database will simply fail the Test gate, which
// is the right surface for that failure.
func selectDBProvisioner(cfg config.Config, logger zerolog.Logger) finisher.DBProvisioner {
	switch cfg.DBProvisioner {
	case "supabase":
		if cfg.SupabasePAT == "" || cfg.SupabaseOrgID == "" {
			logger.Warn().Msg("IRONFLYER_DB_PROVISIONER=supabase but IRONFLYER_SUPABASE_PAT / IRONFLYER_SUPABASE_ORG_ID are missing — falling back to no-op")
			return finisher.NoopDBProvisioner{}
		}
		logger.Info().Str("org", cfg.SupabaseOrgID).Str("region", cfg.SupabaseRegion).
			Msg("DB provisioner: Supabase Management API")
		return &finisher.SupabaseProvisioner{
			AccessToken:    cfg.SupabasePAT,
			OrganizationID: cfg.SupabaseOrgID,
			Region:         cfg.SupabaseRegion,
		}
	case "shared-postgres":
		if cfg.SharedPostgresAdminDSN == "" {
			logger.Warn().Msg("IRONFLYER_DB_PROVISIONER=shared-postgres but IRONFLYER_SHARED_POSTGRES_ADMIN_DSN is missing — falling back to no-op")
			return finisher.NoopDBProvisioner{}
		}
		logger.Info().Str("public_host", cfg.SharedPostgresPublicHost).
			Msg("DB provisioner: shared Postgres (per-project role + database)")
		return &finisher.SharedPostgresProvisioner{
			AdminDSN:   cfg.SharedPostgresAdminDSN,
			PublicHost: cfg.SharedPostgresPublicHost,
			PublicPort: cfg.SharedPostgresPublicPort,
		}
	default:
		return finisher.NoopDBProvisioner{}
	}
}

// parseMCPServers takes the IRONFLYER_MCP_SERVERS env value (a
// comma-separated list of name=URL pairs) and registers one
// providers.MCPClient per entry. Tokens are pulled at startup from
// IRONFLYER_MCP_TOKEN_<UPPERCASE_NAME>. Malformed entries are
// skipped with a warning so a typo in one server doesn't kill the
// rest of the catalogue.
func parseMCPServers(spec string, reg *providers.MCPClientRegistry, logger zerolog.Logger) []string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}
	var names []string
	for _, raw := range strings.Split(spec, ",") {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		eq := strings.IndexByte(entry, '=')
		if eq <= 0 || eq == len(entry)-1 {
			logger.Warn().Str("entry", entry).Msg("MCP server entry malformed (want name=URL); skipping")
			continue
		}
		name := strings.TrimSpace(entry[:eq])
		url := strings.TrimSpace(entry[eq+1:])
		if name == "" || url == "" {
			logger.Warn().Str("entry", entry).Msg("MCP server entry has empty name or URL; skipping")
			continue
		}
		token := os.Getenv("IRONFLYER_MCP_TOKEN_" + strings.ToUpper(name))
		auth := ""
		if token != "" {
			auth = "Bearer " + token
		}
		reg.Register(&providers.MCPClient{
			Name:          name,
			Endpoint:      url,
			Authorization: auth,
		})
		names = append(names, name)
	}
	return names
}

// parseOTelHeaders parses the IRONFLYER_OTEL_HEADERS env value — a
// comma-separated list of "key=value" pairs — into the map shape the
// OTLP HTTP exporter expects. Empty / malformed entries are skipped
// silently so a stray comma doesn't break startup.
func parseOTelHeaders(s string) map[string]string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	out := map[string]string{}
	for _, raw := range strings.Split(s, ",") {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		eq := strings.IndexByte(entry, '=')
		if eq <= 0 || eq == len(entry)-1 {
			continue
		}
		key := strings.TrimSpace(entry[:eq])
		val := strings.TrimSpace(entry[eq+1:])
		if key == "" || val == "" {
			continue
		}
		out[key] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
