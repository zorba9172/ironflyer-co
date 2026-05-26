// Command orchestrator boots the Ironflyer V22 orchestrator. The
// surface is intentionally minimal compared to pre-V22: only the
// services the new architecture relies on (Postgres + budget +
// auth + finisher + memory + audit + providers + GraphQL) are wired.
//
// Wallet / ledger / execution / ProfitGuard / blueprints / repair /
// dashboards arrive under their own startup blocks added by Agents 2-7
// per docs/V22_PLAN.md.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	goruntime "runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	surrealdb "github.com/surrealdb/surrealdb.go"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"ironflyer/core/orchestrator/internal/operations/abuse"
	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/operations/audit"
	"ironflyer/core/orchestrator/internal/operations/auditexport"
	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/business/blueprints"
	"ironflyer/core/orchestrator/internal/business/budget"
	"ironflyer/core/orchestrator/internal/business/budget/payments"
	"ironflyer/core/orchestrator/internal/operations/bus"
	"ironflyer/core/orchestrator/internal/business/clickhouse"
	"ironflyer/core/orchestrator/internal/ai/completion"
	"ironflyer/core/orchestrator/internal/operations/config"
	"ironflyer/core/orchestrator/internal/business/dashboards"
	"ironflyer/core/orchestrator/internal/business/dashboards/adapters"
	"ironflyer/core/orchestrator/internal/operations/deploy"
	"ironflyer/core/orchestrator/internal/operations/diagnostics"
	"ironflyer/core/orchestrator/internal/ai/embeddings"
	"ironflyer/core/orchestrator/internal/operations/events"
	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/ai/finisher"
	"ironflyer/core/orchestrator/internal/operations/gqlhardening"
	"ironflyer/core/orchestrator/internal/operations/httpapi"
	"ironflyer/core/orchestrator/internal/business/ledger"
	"ironflyer/core/orchestrator/internal/ai/memory"
	"ironflyer/core/orchestrator/internal/operations/metrics"
	"ironflyer/core/orchestrator/internal/operations/migrate"
	"ironflyer/core/orchestrator/internal/operations/mobile/devicecloud"
	"ironflyer/core/orchestrator/internal/operations/mobile/eas"
	"ironflyer/core/orchestrator/internal/customer/notify"
	"ironflyer/core/orchestrator/internal/business/outboxhooks"
	"ironflyer/core/orchestrator/internal/operations/patch"
	"ironflyer/core/orchestrator/internal/operations/policy"

	"ironflyer/core/orchestrator/internal/business/profitguard"
	"ironflyer/core/orchestrator/internal/business/profitguardbridge"
	"ironflyer/core/orchestrator/internal/ai/providers"
	"ironflyer/core/orchestrator/internal/operations/ratelimit"
	"ironflyer/core/orchestrator/internal/operations/redisbus"
	"ironflyer/core/orchestrator/internal/ai/repair"
	"ironflyer/core/orchestrator/internal/operations/runtime"
	"ironflyer/core/orchestrator/internal/operations/secrets"
	"ironflyer/core/orchestrator/internal/operations/sentryext"
	"ironflyer/core/orchestrator/internal/operations/storage"
	"ironflyer/core/orchestrator/internal/operations/store"
	"ironflyer/core/orchestrator/internal/operations/temporalworker"
	"ironflyer/core/orchestrator/internal/operations/tracing"
	"ironflyer/core/orchestrator/internal/business/wallet"
	"ironflyer/core/orchestrator/internal/operations/wireup"
	"ironflyer/core/orchestrator/migrations"
)

// Build identifiers. Populated at link time via -ldflags. Plain
// `go run` / `go build` leaves the defaults so /version honestly
// reports a dev build.
var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildTime    = "unknown"
)

// metric registry guard: keep the metrics import referenced so the
// HTTP-middleware variable wires in even before the wallet/profit-guard
// agents add their own counters.
var _ = metrics.Handler

func main() {
	// godotenv.Load short-circuits on the first missing file when called
	// variadically, so call once per path. .env.local wins because it is
	// loaded first (godotenv never overwrites already-set env vars).
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load(".env")

	cfg, err := config.Load()
	logger := buildLogger(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("config load failed")
	}

	// ---------------- Go runtime tuning -----------------------------------
	// GOMAXPROCS: containers default to NumCPU which reports the host's
	// cores, not the cgroup limit. When IRONFLYER_GOMAXPROCS is set we
	// honour it explicitly. Otherwise the Go runtime's default applies.
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_GOMAXPROCS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			goruntime.GOMAXPROCS(n)
			logger.Info().Int("gomaxprocs", n).Msg("GOMAXPROCS overridden via env")
		}
	}
	logger.Info().Int("gomaxprocs", goruntime.GOMAXPROCS(0)).Int("numcpu", goruntime.NumCPU()).
		Msg("Go runtime: GOMAXPROCS active")
	// GOMEMLIMIT: Go 1.19+ soft limit prevents the GC from letting heap
	// grow past the container memory cap and triggering an OOM-kill.
	// IRONFLYER_GOMEMLIMIT (in bytes) is honoured first; otherwise we
	// trust the runtime to honour the standard GOMEMLIMIT env.
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_GOMEMLIMIT")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			debug.SetMemoryLimit(n)
			logger.Info().Int64("gomemlimit_bytes", n).Msg("GOMEMLIMIT overridden via env")
		}
	}

	// In-process diagnostics ring + zerolog hook. The hook captures
	// every WARN+ event into a bounded ring (2000 entries) so the
	// operator-gated /admin/logs/tail REST endpoint and the
	// recentErrors / recentLogs GraphQL queries can surface the most
	// recent operational signal without an external aggregator. The
	// hook is non-blocking by construction (TryLock on append).
	diagRing := diagnostics.NewRing(2000)
	logger = logger.Hook(diagnostics.NewZerologHook(diagRing))
	diagSvc := diagnostics.NewService(diagRing, logger)

	ctx, cancelMain := context.WithCancel(context.Background())
	defer cancelMain()

	// ---------------- Sentry (optional) ------------------------------------
	// Sentry handles exceptions + HTTP panics (OTel handles spans). DSN
	// resolves from SENTRY_DSN_ORCHESTRATOR, falling back to SENTRY_DSN
	// so a single shared DSN env works for monorepo deployments.
	sentryDSN := strings.TrimSpace(os.Getenv("SENTRY_DSN_ORCHESTRATOR"))
	if sentryDSN == "" {
		sentryDSN = strings.TrimSpace(os.Getenv("SENTRY_DSN"))
	}
	sentryEnvName := strings.TrimSpace(os.Getenv("IRONFLYER_ENV"))
	if sentryEnvName == "" {
		sentryEnvName = "development"
	}
	sentryFlush, err := sentryext.Init(sentryext.Opts{
		DSN:              sentryDSN,
		Environment:      sentryEnvName,
		Release:          strings.TrimSpace(os.Getenv("IRONFLYER_VERSION")),
		TracesSampleRate: sentryext.FloatFromEnv("SENTRY_TRACES_SAMPLE", 0.05),
		ServerName:       "ironflyer-orchestrator",
	})
	if err != nil {
		logger.Warn().Err(err).Msg("sentry init failed; continuing without exception reporting")
	} else if sentryDSN != "" {
		logger.Info().Str("env", sentryEnvName).Msg("Sentry initialised")
	}
	defer sentryFlush()

	// ---------------- OpenTelemetry tracing (optional) ---------------------
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
		p, err := connectPostgresTuned(ctx, cfg.PostgresURL, logger)
		if err != nil {
			logger.Fatal().Err(err).Msg("connect postgres")
		}
		pgPool = p
	}

	// ---------------- Schema migrations (goose) -----------------------------
	if err := migrate.RunPool(ctx, pgPool, migrations.FS); err != nil {
		logger.Fatal().Err(err).Msg("apply schema migrations")
	}
	if pgPool != nil {
		logger.Info().Msg("schema migrations applied")
	}

	// ---------------- Durable event outbox / Redpanda publisher ------------
	var eventPublisher events.Publisher
	var eventOutbox *events.PostgresOutbox
	var publisherDaemon *events.PublisherDaemon
	if pgPool != nil {
		eventOutbox = events.NewPostgresOutbox(pgPool)
		if strings.TrimSpace(cfg.RedpandaBrokers) != "" {
			pub, err := events.NewRedpandaPublisher(strings.Split(cfg.RedpandaBrokers, ","))
			if err != nil {
				logger.Fatal().Err(err).Msg("configure redpanda publisher")
			}
			eventPublisher = pub
			pump := events.NewPublisherDaemon(eventOutbox, pub, events.PublisherConfig{
				BatchSize:     cfg.EventPumpBatchSize,
				PollInterval:  cfg.EventPumpInterval,
				LeaseDuration: cfg.EventPumpLease,
				MaxAttempts:   cfg.EventPumpMaxAttempts,
				Logger:        logger.With().Str("svc", "event-publisher").Logger(),
			})
			publisherDaemon = pump
			superviseDaemon(ctx, logger, "event-publisher", func(runCtx context.Context) error {
				runCtx, span := tracing.StartSpan(runCtx, "publisher.daemon")
				defer span.End()
				err := pump.Run(runCtx)
				if err != nil && !errors.Is(err, context.Canceled) {
					span.RecordError(err)
				}
				return err
			})
			// Pre-create DLQ topics so the first dead row finds its
			// target ready. Idempotent and best-effort: a failure here
			// is logged but never blocks startup.
			if err := events.EnsureDLQTopics(ctx, strings.Split(cfg.RedpandaBrokers, ","), "outbox-publisher",
				logger.With().Str("svc", "event-publisher").Logger()); err != nil {
				logger.Warn().Err(err).Msg("dlq topic bootstrap")
			}
			logger.Info().Str("brokers", cfg.RedpandaBrokers).Msg("Redpanda event publisher enabled")
		} else {
			logger.Info().Msg("event outbox enabled; Redpanda publisher disabled (set REDPANDA_BROKERS)")
		}
	}

	// ---------------- Schema registry + V22 topic registration ------------
	// Outbox hooks consult the registry to validate payloads before the
	// transactional write commits. Without a registry, schema validation
	// is skipped and the outbox accepts malformed payloads — so we always
	// install at least the in-memory registry as a safety net.
	var schemaReg events.Registry
	if url := os.Getenv("IRONFLYER_SCHEMA_REGISTRY_URL"); url != "" {
		httpReg, err := events.NewHTTPRegistry(url,
			events.WithBasicAuth(os.Getenv("IRONFLYER_SCHEMA_REGISTRY_USER"), os.Getenv("IRONFLYER_SCHEMA_REGISTRY_PASS")),
			events.WithLogger(logger.With().Str("component", "schema-registry").Logger()))
		if err != nil {
			logger.Warn().Err(err).Msg("schema registry unavailable; falling back to memory")
			schemaReg = events.NewMemoryRegistry()
		} else {
			schemaReg = httpReg
		}
	} else {
		schemaReg = events.NewMemoryRegistry()
	}
	if err := events.RegisterV22Topics(ctx, schemaReg, logger); err != nil {
		logger.Warn().Err(err).Msg("topic registration partial failure")
	}
	outboxhooks.SetRegistry(schemaReg)

	// ---------------- Project store (in-memory or SurrealDB) ----------------
	var projects store.Store
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
	var budgetLedger budget.LedgerStore
	var vault budget.VaultStore
	if pgPool != nil {
		budgetLedger = budget.NewPostgresLedger(pgPool)
		vault = budget.NewPostgresVault(pgPool)
		logger.Info().Msg("Postgres budget store enabled")
	} else {
		budgetLedger = budget.NewMemoryLedger()
		vault = budget.NewMemoryVault()
		logger.Info().Msg("memory budget store enabled")
	}
	billing := budget.NewBilling(budgetLedger, vault)

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

	// ---------------- Paddle (optional) -------------------------------------
	paddleSvc := payments.NewPaddleService(payments.PaddleOpts{
		APIKey:        cfg.PaddleAPIKey,
		WebhookSecret: cfg.PaddleWebhookSecret,
		Environment:   cfg.PaddleEnv,
		Prices: map[payments.PlanTier]string{
			payments.TierPro:        cfg.PaddlePricePro,
			payments.TierTeam:       cfg.PaddlePriceTeam,
			payments.TierEnterprise: cfg.PaddlePriceEnterprise,
		},
		SuccessURL: cfg.StripeSuccessURL,
		CancelURL:  cfg.StripeCancelURL,
	})
	if paddleSvc.Enabled() {
		logger.Info().Str("env", cfg.PaddleEnv).Msg("Paddle checkout + webhook enabled")
	} else {
		logger.Warn().Msg("Paddle disabled (set PADDLE_API_KEY + PADDLE_WEBHOOK_SECRET)")
	}

	// ---------------- Payments registry (Stripe + Paddle side-by-side) -----
	paymentsRegistry := payments.NewRegistry()
	paymentsRegistry.Register(budget.NewStripeProvider(stripeSvc))
	paymentsRegistry.Register(paddleSvc)
	{
		names := make([]string, 0, 2)
		for _, p := range paymentsRegistry.Active() {
			names = append(names, p.Name())
		}
		logger.Info().Strs("active", names).Msg("payment providers registered")
	}
	_ = paymentsRegistry // surfaced via GraphQL in a follow-up; webhook routes mount directly today

	// ---------------- Auth store + service ----------------------------------
	var userStore auth.UserStore
	if pgPool != nil {
		userStore = auth.NewPostgresUserStore(pgPool)
		logger.Info().Msg("Postgres user store enabled")
	} else {
		userStore = auth.NewMemoryUserStore()
		logger.Info().Msg("memory user store enabled (demo@ironflyer.dev / demo1234)")
	}
	authSvc := auth.NewService(userStore, []byte(cfg.JWTSecret), cfg.JWTIssuer, 7*24*time.Hour)
	if u, _, err := userStore.GetByEmail(ctx, "demo@ironflyer.dev"); err == nil {
		billing.AssignPlan(ctx, u.ID, budget.TierPro)
	}

	// ---------------- Superuser bootstrap (privileged backdoor) -----------
	if cfg.SuperuserEmail != "" && cfg.SuperuserPassword != "" {
		verifier, _ := userStore.(auth.EmailVerifier)
		roleSetter, _ := userStore.(auth.RoleSetter)
		if _, err := auth.EnsureSuperuser(ctx, authSvc, userStore, verifier, roleSetter,
			cfg.SuperuserEmail, cfg.SuperuserPassword, logger); err != nil {
			logger.Warn().Err(err).Str("email", cfg.SuperuserEmail).
				Msg("superuser bootstrap failed (continuing)")
		}
	}

	// ---------------- Redis (optional, multi-pod coordination) -------------
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

	// ---------------- Cross-pod event bus ---------------------------------
	var backend bus.Bus
	if redisClient != nil && redisClient.Client != nil {
		backend = bus.NewRedisBus(redisClient.Client)
		logger.Info().Str("backend", "redis").Msg("cross-pod event bus ready")
	} else {
		backend = bus.NewMemoryBus()
		logger.Info().Str("backend", "memory").Msg("cross-pod event bus ready (single-pod)")
	}
	eventBus := bus.NewMultiplexer(backend)
	defer func() { _ = eventBus.Close() }()

	// ---------------- Providers + Guard + Agents ----------------------------
	// Mock is only registered when no real provider is configured. This keeps
	// dev usable on a bare clone while making sure that as soon as a real
	// key is set the chat never silently falls back to "[mock] ..." output.
	router := providers.NewRouter()
	realProviders := 0

	if cfg.AnthropicAPIKey != "" {
		router.Register(providers.NewAnthropicProvider(providers.AnthropicOpts{
			APIKey: cfg.AnthropicAPIKey, Model: cfg.AnthropicModel,
		}))
		logger.Info().Str("model", cfg.AnthropicModel).Msg("Anthropic provider registered")
		realProviders++
	}
	if cfg.OpenAIAPIKey != "" {
		router.Register(providers.NewOpenAIProvider(providers.OpenAIOpts{
			APIKey: cfg.OpenAIAPIKey, Model: cfg.OpenAIModel,
		}))
		logger.Info().Str("model", cfg.OpenAIModel).Msg("OpenAI provider registered")
		realProviders++
	}
	if cfg.GeminiAPIKey != "" {
		router.Register(providers.NewGeminiProvider(providers.GeminiOpts{
			APIKey: cfg.GeminiAPIKey, Model: cfg.GeminiModel,
		}))
		logger.Info().Str("model", cfg.GeminiModel).Msg("Gemini provider registered")
		realProviders++
	}
	if cfg.HFAPIKey != "" {
		router.Register(providers.NewHuggingFaceProvider(providers.HuggingFaceOpts{APIKey: cfg.HFAPIKey}))
		logger.Info().Msg("HuggingFace provider registered")
		realProviders++
	}
	if vp := providers.NewVercelAIGatewayProvider(providers.VercelAIGatewayOpts{
		Token:   cfg.VercelAIGatewayToken,
		BaseURL: cfg.VercelAIGatewayURL,
		Model:   cfg.VercelAIGatewayModel,
	}); vp != nil {
		router.Register(vp)
		logger.Info().Str("model", cfg.VercelAIGatewayModel).Str("url", cfg.VercelAIGatewayURL).
			Msg("Vercel AI Gateway provider registered")
		realProviders++
	}
	if cfg.DeepSeekEnabled && cfg.DeepSeekAPIKey != "" {
		ds, err := providers.NewDeepSeek(providers.DeepSeekConfig{
			Token:           cfg.DeepSeekAPIKey,
			BaseURL:         cfg.DeepSeekBaseURL,
			GeneralModel:    cfg.DeepSeekGeneralModel,
			ReasoningModel:  cfg.DeepSeekReasoningModel,
			CoderModel:      cfg.DeepSeekCoderModel,
			PreferV3ForCode: cfg.DeepSeekPreferV3ForCode,
			Enabled:         true,
		}, nil, nil)
		if err != nil {
			logger.Warn().Err(err).Msg("deepseek: construct failed")
		} else {
			router.Register(ds)
			logger.Info().Msg("deepseek: enabled")
			realProviders++
		}
	}

	if realProviders == 0 {
		router.Register(providers.NewMockProvider("mock"))
		logger.Warn().Msg("no real LLM provider configured — registered mock provider (chat will return `[mock] ...`); set ANTHROPIC_API_KEY (or another provider key) to enable real responses")
	} else {
		logger.Info().Int("count", realProviders).Msg("real LLM providers active; mock provider disabled")
	}

	telemetrySink := providers.NewMemorySink(2048).WithBus(eventBus)
	banditStrategy := providers.StrategyFromEnv(os.Getenv("IRONFLYER_BANDIT_STRATEGY"), 0)
	// QualityRegistry feeds per-provider gate-pass EMA into the bandit
	// reward formula. RegisterQuality also installs it as the package
	// global so any bandit that doesn't have its own Quality field set
	// still picks it up via ActiveQuality().
	qualityRegistry := providers.NewQualityRegistry()
	providers.RegisterQuality(qualityRegistry)
	bandit := &providers.Bandit{Sink: telemetrySink, Strategy: banditStrategy, Quality: qualityRegistry}
	router.WithBandit(bandit)
	router.WithLogger(logger).WithTelemetry(telemetrySink)
	providers.RegisterActiveBandit(bandit)
	logger.Info().Str("strategy", banditStrategy.Name()).Msg("Router bandit enabled")
	guard := providers.NewBillingGuard(router, billing).WithTelemetry(telemetrySink).WithLogger(logger)

	// ---------------- Memory + audit stores -------------------------------
	var memoryStore memory.Store
	resolvedMemoryBackend := "memory"
	switch {
	case cfg.MemoryBackend == "surreal" && surrealDB != nil:
		if err := memory.BootstrapSurreal(ctx, surrealDB); err != nil {
			logger.Fatal().Err(err).Msg("memory: surreal bootstrap")
		}
		memoryStore = memory.NewSurrealStore(surrealDB)
		resolvedMemoryBackend = "surreal"
	case cfg.MemoryBackend == "pgvector":
		if pgPool == nil {
			logger.Fatal().Msg("memory: pgvector backend requires POSTGRES_URL")
		}
		emb, _ := selectEmbedder(cfg, logger)
		var cached embeddings.Embedder
		if emb != nil {
			cached = embeddings.NewCachedEmbedder(emb)
		}
		memoryStore = memory.NewPgVectorStore(pgPool, cached, logger)
		resolvedMemoryBackend = "pgvector"
	default:
		memoryStore = memory.NewMemoryStore(4096)
	}
	logger.Info().Str("backend", resolvedMemoryBackend).Msg("memory backend resolved")
	if resolvedMemoryBackend != "pgvector" {
		if emb, label := selectEmbedder(cfg, logger); emb != nil {
			memoryStore = &memory.VectorStore{
				Inner:    memoryStore,
				Embedder: embeddings.NewCachedEmbedder(emb),
			}
			logger.Info().Str("backend", label).Msg("Memory store: semantic re-ranking enabled")
		}
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

	// ---------------- Agents registry + patches + finisher ----------------
	registry := agents.NewRegistry(guard)
	registry.RegisterDefaults()

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

	engine := finisher.NewEngine(projects, registry, patches).
		WithRuntime(runtimeClient).
		WithApplier(runtime.NewApplier(runtimeClient)).
		WithRedis(redisClient).
		WithBus(eventBus).
		WithDBProvisioner(selectDBProvisioner(cfg, logger)).
		WithAuthScaffolder(finisher.DefaultAuthScaffolder{}).
		WithDomainScaffolders(
			finisher.GameScaffolder{},
			finisher.EcommerceScaffolder{},
			finisher.DashboardScaffolder{},
			finisher.LearningScaffolder{},
			finisher.CICDScaffolder{},
		).
		WithMemory(memoryStore).
		WithAudit(auditStore).
		WithBudgetSource(func(ctx context.Context, userID, projectID string) (*finisher.BudgetSnapshot, error) {
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
		logger.Info().Str("runtime", cfg.RuntimeURL).Msg("finisher wired to runtime")
	} else {
		logger.Warn().Msg("runtime not configured (set IRONFLYER_RUNTIME_URL)")
	}

	patches.WithGateRunner(engine)
	if pgPool != nil {
		stagingDB, err := migrate.FromPool(pgPool)
		if err != nil {
			logger.Fatal().Err(err).Msg("patch staging: derive *sql.DB from pgxpool")
		}
		patches.WithStagingStore(patch.NewPostgresStagingStore(stagingDB))
		logger.Info().Msg("patch staging store: Postgres-backed")
	} else {
		logger.Info().Msg("patch staging store: in-memory (no Postgres)")
	}

	// ---------------- Notifications (email only in V22) -------------------
	prefsStore := notify.NewMemoryPrefsStore()
	emailSender := notify.SenderFromEnv(cfg.EmailProvider, cfg.EmailAPIKey, cfg.EmailFromAddress, logger)
	notifyEngine := notify.NewEngine(projects, prefsStore, emailSender, logger).
		WithDashboardURL(cfg.DashboardURL)
	notifyEngine.SubscribeAll(ctx, engine)
	logger.Info().Str("email_provider", cfg.EmailProvider).Msg("notification pipeline online (email)")

	// ---------------- Auth commercial backings ----------------------------
	var (
		verifications  auth.VerificationStore
		passwordResets auth.PasswordResetStore
		sessionStore   auth.SessionStore
	)
	if pgPool != nil {
		verifications = auth.NewPostgresVerificationStore(pgPool)
		passwordResets = auth.NewPostgresPasswordResetStore(pgPool)
		sessionStore = auth.NewPostgresSessionStore(pgPool)
		logger.Info().Msg("Postgres auth backings enabled (verifications, password resets, sessions)")
	} else {
		verifications = auth.NewMemoryVerificationStore()
		passwordResets = auth.NewMemoryPasswordResetStore()
		sessionStore = auth.NewMemorySessionStore()
		logger.Info().Msg("memory auth backings enabled")
	}

	emailVerifier, _ := userStore.(auth.EmailVerifier)
	emailChangerImpl, _ := userStore.(interface {
		SetEmail(ctx context.Context, userID, newEmail string) error
	})
	passwordRotator, _ := userStore.(auth.PasswordRotator)

	var sessionCache auth.SessionCache = auth.NoopSessionCache{}

	webBaseURL := strings.TrimSpace(os.Getenv("WEB_BASE_URL"))
	if webBaseURL == "" {
		webBaseURL = "http://localhost:3000"
	}

	resendVerificationLimiter := ratelimit.Wrap(
		redisClient, "rl:auth:resend:",
		ratelimit.New(1.0/60.0, 1),
	)
	passwordResetIPLimiter := ratelimit.Wrap(
		redisClient, "rl:auth:reset:ip:",
		ratelimit.New(5.0/3600.0, 5),
	)
	passwordResetEmailLimiter := ratelimit.Wrap(
		redisClient, "rl:auth:reset:email:",
		ratelimit.New(3.0/3600.0, 3),
	)

	// ---------------- V22 service surface -----------------------------------
	// Wallet — Postgres when wired, in-memory otherwise.
	var walletSvc wallet.Service
	if pgPool != nil {
		walletSvc = wallet.NewPostgresService(pgPool).WithOutbox()
		logger.Info().Msg("V22 wallet: Postgres backend + durable outbox")
	} else {
		walletSvc = wallet.NewMemoryService()
		logger.Info().Msg("V22 wallet: in-memory backend")
		// Dev convenience: seed the demo tenant with $100 so the
		// describeIdea entrypoint works out of the box. The seed is
		// scoped to dev + in-memory mode so it can't accidentally
		// inflate a production ledger.
		if cfg.Env == "dev" {
			if err := walletSvc.TopUp(ctx, "demo", decimal.NewFromInt(100), "dev-seed"); err != nil {
				logger.Warn().Err(err).Msg("dev seed: wallet top-up failed (demo tenant)")
			} else {
				logger.Info().Msg("dev seed: demo wallet seeded with $100")
			}
		}
	}
	// Wallet Stripe topper (optional — keyed off STRIPE_SECRET_KEY).
	var walletTopper *wallet.Topper
	if cfg.StripeSecretKey != "" {
		walletTopper = wallet.NewTopper(walletSvc, wallet.TopperOpts{
			SecretKey:     cfg.StripeSecretKey,
			WebhookSecret: cfg.StripeWebhookSecret,
			SuccessURL:    cfg.StripeSuccessURL,
			CancelURL:     cfg.StripeCancelURL,
		})
		logger.Info().Msg("V22 wallet topper: Stripe enabled")
	}

	// Ledger.
	var ledgerSvc ledger.Service
	if pgPool != nil {
		ledgerSvc = ledger.NewPostgresService(pgPool).WithOutbox()
		logger.Info().Msg("V22 ledger: Postgres backend + durable outbox")
	} else {
		ledgerSvc = ledger.NewMemoryService()
		logger.Info().Msg("V22 ledger: in-memory backend")
	}

	// ---- Profitability hooks ------------------------------------------------
	// Free→Paid conversion: every plan upgrade now lands a typed
	// EntryFreeToPaidConversion in the ledger so the conversion rate
	// query is O(1). Idempotent via OpKey, so Stripe webhook
	// redelivery doesn't double-count.
	wireup.WireConversionTracking(billing, ledgerSvc)

	// Upsell pressure: when a user's spend crosses the configured
	// usage brackets (67% / 85% / 100% of cost cap), fire a log line
	// and a typed ledger event so the web banner can ratchet without
	// polling. Hook is fire-and-forget; the admission path never
	// blocks on it.
	billing.Enforcer.RegisterUpsellHook(func(ctx context.Context, userID string, tier budget.PlanTier, prev, next budget.UsageBracket, usagePct float64) {
		logger.Info().
			Str("user_id", userID).
			Str("tier", string(tier)).
			Int("prev_bracket", int(prev)).
			Int("next_bracket", int(next)).
			Float64("usage_pct", usagePct).
			Msg("usage bracket transition — fire upsell")
		// Best-effort ledger marker. Tagged Adjustment so the marker
		// doesn't pollute revenue or cost rollups but is still
		// queryable for funnel analysis.
		if tenant, err := uuid.Parse(userID); err == nil {
			_, _ = ledgerSvc.Write(ctx, ledger.Entry{
				TenantID:  tenant,
				EntryType: ledger.EntryRefund, // refund-typed marker so it stays out of cost; OpKey suffix scopes by bracket
				Direction: ledger.CreditDirection,
				AmountUSD: decimal.Zero,
				Metadata: map[string]any{
					"kind":         "upsell_trigger",
					"tier":         string(tier),
					"prev_bracket": int(prev),
					"next_bracket": int(next),
					"usage_pct":    usagePct,
				},
				OpKey: "upsell:" + userID + ":" + string(tier) + ":b" + bracketKey(next),
			})
		}
	})

	// Storage cost ticker. Today the orchestrator has no S3 callers
	// (see internal/storage/s3client.go header), so the biller runs
	// against a NoopUsageSource and emits no entries. The wiring is
	// here so the moment a real source (audit export bucket,
	// artefact retention) lands, swapping NoopUsageSource → real
	// implementation is one line and the rate sheet + ledger entries
	// already work.
	storageBiller := storage.NewBiller(
		storage.NoopUsageSource{},
		ledgerSvc,
		storage.DefaultRate(),
		time.Hour,
		logger,
	)
	storageBillerCtx, storageBillerSpan := tracing.StartSpan(ctx, "storage.biller.daemon")
	storageBiller.Start(storageBillerCtx)
	superviseDaemon(ctx, logger, "storage-biller-span", func(runCtx context.Context) error {
		<-storageBillerCtx.Done()
		storageBillerSpan.End()
		return nil
	})
	logger.Info().
		Str("rate_usd_per_gb_month", storage.DefaultRate().USDPerGBMonth.String()).
		Msg("V22 storage biller armed (NoopUsageSource until a real bucket source lands)")

	// One-line profitability summary so the operator sees which
	// margin-protecting knobs are active right after startup.
	logProfitKnobs(logger, billing)

	// Execution.
	var execSvc execution.Service
	if pgPool != nil {
		execSvc = execution.NewPostgres(pgPool)
		logger.Info().Msg("V22 execution: Postgres backend")
	} else {
		execSvc = execution.NewMemory()
		logger.Info().Msg("V22 execution: in-memory backend")
	}

	// ProfitGuard policy + audit store.
	var guardStore profitguard.DecisionStore
	if pgPool != nil {
		guardStore = profitguard.NewPostgresStore(pgPool).WithOutbox()
		logger.Info().Msg("V22 profitguard store: Postgres backend + durable outbox")
	} else {
		guardStore = profitguard.NewMemoryStore()
		logger.Info().Msg("V22 profitguard store: in-memory backend")
	}
	profitGuard := profitguard.New(profitguard.DefaultPolicy(), guardStore)

	// Hoisted: the snapshotFn closure (and the V22
	// BeforeSandboxAllocation wiring further down) read
	// blueprintsReg + repairGenome through the bridge so the
	// SimilarBlueprintAvailable / SimilarRepairAvailable signals
	// are populated. Declared here so the closure captures live
	// values rather than nil.
	blueprintsReg := blueprints.NewBuiltInRegistry()
	var repairGenome repair.Genome
	var patchMem repair.Memory
	if pgPool != nil {
		repairGenome = repair.NewPostgresGenome(pgPool)
		patchMem = repair.NewPostgresPatchStore(pgPool)
	} else {
		repairGenome = repair.NewMemoryGenome()
		patchMem = repair.NewMemoryPatchStore()
	}

	// V22 integration loop: wire ProfitGuard into the two enforcement
	// seams the foundation agent left dangling.
	//
	//  1. providers.BillingGuard.BeforeModelCall — every paid provider
	//     call. The snapshot func resolves the live execution row via
	//     execSvc.GetState and bridges it into a profitguard.ExecState.
	//     ProviderQuotes are intentionally empty for v22 M1; the policy
	//     still evaluates Stop/KillBranch/PauseForBudget/Degrade. A
	//     follow-up adds a router quote provider so SwitchProvider can
	//     pick the cheapest healthy candidate.
	//  2. finisher.Engine.BeforeRetry — the recovery loop. Each round
	//     consults the policy; a stop verdict short-circuits the retry
	//     budget without making any more agent calls.
	snapshotFn := func(ctx context.Context, executionID string, req providers.Request) (profitguard.ExecState, error) {
		state, err := execSvc.GetState(ctx, executionID)
		if err != nil {
			return profitguard.ExecState{}, err
		}
		// Quote the live provider chain so Decide can evaluate the
		// SwitchProvider branch. Cost is computed at list rates by
		// providers/cost.go; quality + latency come from the bandit's
		// telemetry feed (0.7 / 800ms priors when there's no signal).
		quotes := profitguardbridge.QuotesFromRouter(router.Quote(ctx, req))
		// CurrentProvider — honour the request's explicit preference
		// (set by an earlier ProfitGuard pass), otherwise fall back to
		// the head of the capability-scored chain. PickChain is the
		// same ordering the BillingGuard would land on, so the policy
		// compares candidates against the provider that would actually
		// run if we did nothing.
		current := req.PreferredProvider
		if current == "" {
			chain := router.PickChain(req.Capabilities)
			if len(chain) > 0 {
				current = chain[0].Name()
			}
		}
		return profitguardbridge.StateToGuardInputWithDeps(ctx, state, quotes, profitguardbridge.BridgeFlags{
			CurrentProvider: current,
		}, profitguardbridge.BridgeDeps{
			Registry: blueprintsReg,
			Genome:   repairGenome,
		}), nil
	}
	guard = guard.WithProfitGuard(profitGuard, snapshotFn)
	engine.WithProfitGuard(profitGuardEngineAdapter{guard: profitGuard, exec: execSvc})
	logger.Info().Msg("V22 ProfitGuard wired: BeforeModelCall + BeforeRetryLoop")

	// Blueprints stats — registry itself is hoisted above so the
	// snapshotFn closure and BeforeSandboxAllocation hook can read
	// the live value.
	var blueprintStats blueprints.StatsService
	if pgPool != nil {
		blueprintStats = blueprints.NewPostgresStatsService(pgPool)
		logger.Info().Msg("V22 blueprint stats: Postgres backend")
	} else {
		blueprintStats = blueprints.NewMemoryStatsService()
		logger.Info().Msg("V22 blueprint stats: in-memory backend")
	}

	// V22 lifecycle wiring — Settler closes the money loop on every
	// terminal status (succeeded/failed/stopped/killed). TickReporter
	// is the per-tick sandbox cost plumbing wired into the V22
	// SandboxBiller below: every workspace allocated for an execution
	// gets wrapped in a ticker that lands `sandbox_cost` ledger
	// debits for the lifetime of the allocation.
	executionSettler := execution.NewSettlerWithOutbox(execSvc, walletSvc, ledgerSvc, blueprintStats, pgPool)
	tickReporter := execution.NewTickReporter(execSvc, ledgerSvc)
	sandboxBiller := runtime.NewSandboxBiller(tickReporter,
		logger.With().Str("svc", "sandbox-biller").Logger())
	engine.WithSandboxBiller(sandboxBiller)
	logger.Info().
		Str("rate_usd_per_hour", sandboxBiller.Rate().String()).
		Dur("tick_interval", sandboxBiller.Interval()).
		Msg("V22 execution settler + sandbox biller ready")

	// V22 per-token cost attribution — every charged provider stream
	// writes provider_inference_cost to the execution row and the
	// ledger when the call carried an execution id on ctx (set by
	// profitguardctx.WithExecution). Best-effort: warn but never
	// fail the provider call.
	costAttribLogger := logger
	guard = guard.WithCostAttribution(func(ctx context.Context, executionID, tenantID, providerName, modelName string, costUSD decimal.Decimal, inTokens, outTokens int, capabilities []string) {
		if executionID == "" || !costUSD.IsPositive() {
			return
		}
		if err := execSvc.AddCost(ctx, executionID, execution.CostProvider, costUSD, providerName); err != nil {
			costAttribLogger.Warn().Err(err).Str("execution_id", executionID).
				Str("provider", providerName).Msg("execSvc.AddCost provider attribution")
		}
		if tenantID == "" {
			return
		}
		execUUID, perr := uuid.Parse(executionID)
		if perr != nil {
			return
		}
		tenantUUID := tenantUUIDForMain(tenantID)
		capStr := ""
		if len(capabilities) > 0 {
			capStr = strings.Join(capabilities, ",")
		}
		if _, err := ledgerSvc.Write(ctx, ledger.Entry{
			TenantID:       tenantUUID,
			ExecutionID:    &execUUID,
			EntryType:      ledger.EntryProviderInferenceCost,
			Direction:      ledger.DebitDirection,
			AmountUSD:      costUSD,
			Provider:       providerName,
			Billable:       true,
			MarginRelevant: true,
			Metadata: map[string]any{
				"tokens_in":  inTokens,
				"tokens_out": outTokens,
				"model":      modelName,
				"capability": capStr,
			},
		}); err != nil {
			costAttribLogger.Warn().Err(err).Str("execution_id", executionID).
				Str("provider", providerName).Msg("ledgerSvc.Write provider attribution")
		}
	})
	logger.Info().Msg("V22 BillingGuard: per-token cost attribution wired")

	// Engine settler adapter is wired further down once the learning
	// hooks are available — that lets the blueprint leg of the
	// settler mark Repaired=true on runs that consumed a recovery
	// round. The canonical executionSettler is still threaded into
	// the HTTP/GraphQL surface (Stop/Refund flows go straight to it).

	// Completion scorer.
	var completionSvc completion.Scorer
	if pgPool != nil {
		completionSvc = completion.NewPostgresScorer(pgPool)
	} else {
		completionSvc = completion.NewMemoryScorer()
	}

	// Repair genome + patch memory are hoisted above so the
	// snapshotFn closure has them in scope.

	// V22 LearningHooks — bridge the finisher's runtime hot path to
	// the repair genome + patch memory. Wired on the engine BEFORE the
	// settler adapter is constructed below so the settler can read the
	// per-execution repair counter at terminal settle.
	learningHooks := finisher.NewLearningHooks(repairGenome, patchMem)
	engine.WithLearning(learningHooks)
	logger.Info().Msg("V22 learning hooks wired: repair genome + patch memory")

	// V22 completion scoring + execution service back-reference. The
	// finisher gate loop calls Scorer.Score after every gate verdict
	// and mirrors the new absolute score back onto the execution row
	// via execSvc.SetCompletionScore, so ProfitGuard's next Decide
	// reads fresh signal instead of the always-zero default.
	engine.WithCompletionScorer(completionSvc).WithExecutionService(execSvc)
	engine.WithQualitySink(qualityRegistry)
	logger.Info().Msg("V22 completion scoring wired into finisher gate loop")

	// V22 BeforeSandboxAllocation ProfitGuard hook — the engine
	// snapshots live execution state through profitguardbridge.SnapshotFor
	// and calls Guard.Decide BEFORE letting sandboxBiller.Track land.
	// A Stop/Kill/Pause verdict aborts the run cleanly. BridgeDeps
	// carries the Registry + Genome so SimilarBlueprintAvailable /
	// SimilarRepairAvailable are populated for the policy's reuse
	// branches (ReuseBlueprint / ReuseRepair).
	engine.WithBeforeSandboxAllocation(profitGuard, profitguardbridge.BridgeDeps{
		Registry: blueprintsReg,
		Genome:   repairGenome,
	})
	logger.Info().Msg("V22 BeforeSandboxAllocation hook wired (with blueprint + repair reuse signals)")

	// Re-wire the engine settler so the blueprint leg of Close()
	// receives Repaired=true when the run consumed one or more
	// recovery rounds. We wrap blueprintStats with a thin proxy that
	// promotes Repaired based on the per-execution counter the
	// learning hooks bumped during the run, then hand the wrapped
	// stats service to a fresh Settler the engine adapter calls.
	repairAwareStats := repairAwareBlueprintStats{
		Stats:    blueprintStats,
		Learning: learningHooks,
	}
	engineSettler := execution.NewSettlerWithOutbox(execSvc, walletSvc, ledgerSvc, repairAwareStats, pgPool)
	engine.WithSettler(engineSettlerAdapter{exec: execSvc, settler: engineSettler, logger: logger})

	// Temporal production runner. The embedded path remains the local
	// default; setting IRONFLYER_EXECUTOR=temporal starts a worker that
	// owns Start -> Finisher -> terminal transition -> settlement for
	// already-admitted paid executions.
	temporalHost := strings.TrimSpace(cfg.TemporalHost)
	if temporalHost == "" && strings.EqualFold(cfg.Executor, "temporal") {
		temporalHost = strings.TrimSpace(cfg.TemporalAddr)
	}
	var temporalRuntime *temporalworker.Runtime
	if strings.EqualFold(cfg.Executor, "temporal") && temporalHost != "" {
		rt, err := temporalworker.Start(ctx, temporalworker.Config{
			Host:      temporalHost,
			Namespace: cfg.TemporalNamespace,
			TaskQueue: cfg.TemporalTaskQueue,
			Enabled:   true,
		}, &temporalworker.Deps{
			Execution:   temporalExecutionAdapter{svc: execSvc},
			ProfitGuard: temporalProfitGuardAdapter{guard: profitGuard},
			Engine:      temporalEngineAdapter{engine: engine},
			Settler:     temporalSettlerAdapter{svc: executionSettler, exec: execSvc},
			Events:      temporalEventAdapter{outbox: eventOutbox},
		})
		if err != nil {
			logger.Fatal().Err(err).Msg("Temporal worker start")
		}
		temporalRuntime = rt
		logger.Info().
			Str("host", temporalHost).
			Str("namespace", cfg.TemporalNamespace).
			Str("task_queue", cfg.TemporalTaskQueue).
			Msg("Temporal finisher worker started")
	} else {
		logger.Info().Str("executor", cfg.Executor).Msg("Temporal worker disabled; embedded finisher executor active")
	}

	// ---------------- V22 Wave-2: secrets broker ---------------------------
	secretsCfg := secrets.LoadConfig()
	secretsRes := wireup.BuildSecrets(secretsCfg, pgPool, auditStore, logger)
	if secretsCfg.Enabled {
		logger.Info().Str("default_backend", string(secretsCfg.DefaultBackend)).
			Msg("V22 secrets broker enabled")
	}

	// ---------------- V22 Wave-2: policy plane (OPA/Cedar) -----------------
	policyCfg := policy.LoadConfig()
	policyPEP, err := wireup.BuildPolicyPEP(policyCfg, auditStore, logger)
	if err != nil {
		logger.Warn().Err(err).Msg("V22 policy plane disabled (boot error)")
		policyPEP = nil
	} else if policyPEP != nil {
		logger.Info().Str("bundle_version", policyPEP.BundleVersion()).
			Msg("V22 policy plane wired")
	}

	// ---------------- V22 Wave-2: memory graph (SurrealDB) -----------------
	memGraph := wireup.BuildMemoryGraph(ctx, surrealDB, logger)
	_ = memGraph.Retriever // surfaced via the finisher in a follow-up; the
	// writer is wrapped around the publisher daemon below so projection
	// fan-out lives next to the canonical event flow.
	// AttachMemoryGraphWriter is nil-safe on either side — when the
	// Redpanda publisher is disabled the writer simply receives no
	// events until a daemon is wired.
	wireup.AttachMemoryGraphWriter(publisherDaemon, memGraph.Writer, logger)

	// ---------------- V22 Wave-2: ClickHouse analytics ---------------------
	chCfg := clickhouse.LoadConfig()
	chRes, chErr := wireup.BuildClickHouse(ctx, chCfg, pgPool, cfg.RedpandaBrokers, logger)
	if chErr != nil {
		logger.Warn().Err(chErr).Msg("V22 ClickHouse init failed; falling back to PG dashboards")
		chRes = wireup.ClickHouseResult{}
	}
	if chRes.Consumer != nil {
		superviseDaemon(ctx, logger, "clickhouse-consumer", func(runCtx context.Context) error {
			return chRes.Consumer.Run(runCtx)
		})
	}
	if chRes.Ingester != nil {
		superviseDaemon(ctx, logger, "clickhouse-ingester", func(runCtx context.Context) error {
			return chRes.Ingester.Run(runCtx)
		})
	}
	if chRes.Client != nil {
		correction := clickhouse.NewCorrectionJob(chRes.Client, logger, 14, time.Hour)
		superviseDaemon(ctx, logger, "clickhouse-correction", func(runCtx context.Context) error {
			runCtx, span := tracing.StartSpan(runCtx, "clickhouse.correction.daemon")
			defer span.End()
			err := correction.Run(runCtx)
			if err != nil && !errors.Is(err, context.Canceled) {
				span.RecordError(err)
			}
			return err
		})
	}

	// ---------------- V22 Wave-2: deploy plane -----------------------------
	bridgeDeps := profitguardbridge.BridgeDeps{Registry: blueprintsReg, Genome: repairGenome}
	deploySvc := wireup.BuildDeployService(wireup.DeployDeps{
		Pool:       pgPool,
		Logger:     logger.With().Str("svc", "deploy").Logger(),
		SecretsBrk: secretsRes.Broker,
		Guard:      profitGuard,
		ExecSvc:    execSvc,
		BridgeDeps: bridgeDeps,
	})
	deployDomainSvc := wireup.BuildDeployDomainService(wireup.DeployDeps{
		Pool:       pgPool,
		Logger:     logger.With().Str("svc", "deploy-domain").Logger(),
		SecretsBrk: secretsRes.Broker,
	})
	logger.Info().Msg("V22 deploy plane wired (deploy + domains + registrar adapters)")

	// Approval-expiry sweeper: flips pending approvals past ExpiresAt to
	// the expired terminal state so the deployFeed subscription emits a
	// real approval_decided event even when no operator returns.
	if deploySvc != nil {
		deploySweeper := deploy.NewSweeper(deploySvc, logger, 0)
		superviseDaemon(ctx, logger, "deploy-sweeper", func(runCtx context.Context) error {
			runCtx, span := tracing.StartSpan(runCtx, "deploy.sweeper.daemon")
			defer span.End()
			err := deploySweeper.Run(runCtx)
			if err != nil && !errors.Is(err, context.Canceled) {
				span.RecordError(err)
			}
			return err
		})
	}

	// Policy bundle hot-reloader: only does anything when
	// policy.Config.BundleDir is non-empty AND the active PDP is the
	// in-process LocalPDP. Remote/disabled PDPs are not rebindable from
	// disk; the reloader exits cleanly in those modes.
	if policyPEP != nil {
		var rebinder policy.PDPRebinder
		if rb, ok := policyPEP.PDP().(policy.PDPRebinder); ok {
			rebinder = rb
		}
		policyReloader := policy.NewReloader(policyCfg, rebinder, logger)
		superviseDaemon(ctx, logger, "policy-reloader", func(runCtx context.Context) error {
			return policyReloader.Run(runCtx)
		})
	}

	// ---------------- V22 Wave-2: abuse engine + GraphQL hardening ---------
	var abuseStore abuse.Store
	if pgPool != nil {
		abuseStore = abuse.NewPostgresStore(pgPool)
	} else {
		abuseStore = abuse.NewMemoryStore()
	}
	abuseEngine := abuse.NewEngine(abuse.LoadConfig(), abuseStore)
	hardeningCfg := gqlhardening.Load()
	baseLimiter := ratelimit.New(hardeningCfg.BaseRatePerSecond, hardeningCfg.BaseBurst)
	gqlLimiter := gqlhardening.NewLimiter(baseLimiter, abuseEngine)
	var persistedStore gqlhardening.Store
	if pgPool != nil {
		persistedStore = gqlhardening.NewPostgresStore(pgPool)
	} else {
		persistedStore = gqlhardening.NewMemoryStore()
	}
	// V22 Wave-2 hardening profile banner. Depth + complexity caps are
	// ALWAYS-ON; everything else gates on hardeningCfg.ProdMode so an
	// operator can audit at a glance which production gates actually
	// mounted on this boot without grepping source.
	introspectionOn := !hardeningCfg.ProdMode || strings.EqualFold(strings.TrimSpace(os.Getenv("GRAPHQL_INTROSPECTION")), "on") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("GRAPHQL_INTROSPECTION")), "true") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("GRAPHQL_INTROSPECTION")), "1")
	persistedCount := 0
	if persistedStore != nil {
		if n, err := persistedStore.Count(context.Background()); err == nil {
			persistedCount = n
		}
	}
	logger.Info().
		Bool("prod", hardeningCfg.ProdMode).
		Int("depth", hardeningCfg.MaxDepth).
		Int("complexity", hardeningCfg.ComplexityLimit).
		Bool("apq", true).
		Bool("apq_locked", hardeningCfg.ProdMode).
		Bool("csrf", hardeningCfg.ProdMode).
		Bool("introspection", introspectionOn).
		Bool("error_masking", hardeningCfg.ProdMode).
		Int("persisted_queries", persistedCount).
		Msg("V22 GraphQL hardening profile")

	// ---------------- V22 Wave-2: temporal idempotent ports ----------------
	// The existing temporalworker.Start call (above) wires the
	// execution / profitguard / engine / settler / events ports. The
	// wallet + ledger ports plug in via the package-level dependency
	// bundle so activities that mint a wallet hold (AdmitExecution)
	// land the idempotent variants.
	temporalworker.SetActivityDeps(&temporalworker.Deps{
		Wallet:      wireup.TemporalWalletAdapter{Svc: walletSvc},
		Ledger:      wireup.TemporalLedgerAdapter{Svc: ledgerSvc},
		Execution:   temporalExecutionAdapter{svc: execSvc},
		ProfitGuard: temporalProfitGuardAdapter{guard: profitGuard},
		Engine:      temporalEngineAdapter{engine: engine},
		Settler:     temporalSettlerAdapter{svc: executionSettler, exec: execSvc},
		Events:      temporalEventAdapter{outbox: eventOutbox},
	})

	// ---------------- V22 Wave-2: ProfitGuard hooks on patch + finisher ----
	artifactHook := wireup.ArtifactStoreHookAdapter{
		Guard: profitGuard, Exec: execSvc, BridgeDeps: bridgeDeps,
		Logger: logger.With().Str("hook", "artifact_store").Logger(),
	}
	longVerifyHook := wireup.LongVerificationHookAdapter{
		Guard: profitGuard, Exec: execSvc, BridgeDeps: bridgeDeps,
		Logger: logger.With().Str("hook", "long_verify").Logger(),
	}
	patches.WithArtifactStoreHook(artifactHook, 1<<20)
	engine.WithLongVerificationHook(longVerifyHook)
	logger.Info().Msg("V22 ProfitGuard hooks: artifact store + long verification wired")

	// ---------------- Dashboards (ClickHouse when enabled, PG fallback) ----
	dashboardSvc := &dashboards.Service{
		Ledger:    adapters.LedgerAdapter{Svc: ledgerSvc, Pool: pgPool},
		Exec:      adapters.ExecutionAdapter{Svc: execSvc, Pool: pgPool},
		Blueprint: adapters.BlueprintAdapter{Svc: blueprintStats},
		Scale:     adapters.ScaleAdapter{},
	}
	if chRes.Ledger != nil {
		dashboardSvc.Ledger = chRes.Ledger
		dashboardSvc.Exec = chRes.Execution
		dashboardSvc.Blueprint = chRes.Blueprint
		dashboardSvc.Scale = chRes.Scale
		logger.Info().Msg("V22 dashboards: ClickHouse sources active")
	}
	logger.Info().Msg("V22 dashboards service ready")

	// V22 Wave-3 (A32-A36) — forecast, wow loop, audit export, security
	// report, operator. Each builder is nil-safe at the resolver layer
	// so a partial deploy still boots.
	forecaster := wireup.BuildForecaster(pgPool, logger)
	// Studio-close-out: wow loop reaches into the runtime for the
	// live workspace preview URL while the execution is still in
	// flight. The wireup pipes wowExecutionAdapter.WorkspaceID =
	// execution.ProjectID, and this adapter dereferences projectID →
	// running workspace → preview URL via the runtime client.
	//
	// Service-to-service bearer: IRONFLYER_RUNTIME_BEARER. Optional —
	// when unset the runtime falls back to its "demo" user (only safe
	// in dev installs). Production deploys should set a service token
	// the runtime trusts.
	runtimeServiceBearer := strings.TrimSpace(os.Getenv("IRONFLYER_RUNTIME_BEARER"))
	wowBuilder := wireup.BuildWowLoop(execSvc, ledgerSvc, walletSvc, logger,
		wireup.WithPatchEngine(patches),
		wireup.WithRepairGenome(repairGenome),
		wireup.WithDeployService(deploySvc),
		wireup.WithRuntimeSource(&runtimePreviewAdapter{
			client: runtimeClient,
			bearer: runtimeServiceBearer,
		}),
	)
	auditExporter := wireup.BuildAuditExporter(auditStore, logger)
	auditExportConfig := wireup.BuildAuditExportConfig()
	if secret := []byte(os.Getenv("IRONFLYER_AUDIT_EXPORT_HMAC_SECRET")); len(secret) >= 32 {
		signer, err := auditexport.NewHMACSigner(secret)
		if err != nil {
			logger.Warn().Err(err).Msg("audit export signer disabled")
		} else {
			auditExportConfig.Signer = signer
		}
	} else {
		logger.Warn().Msg("IRONFLYER_AUDIT_EXPORT_HMAC_SECRET unset or too short; audit export downloads disabled")
	}
	secReportBuilder := wireup.BuildSecurityReportBuilder(execSvc, logger)
	operatorSvc := wireup.BuildOperator(deploySvc, abuseEngine, execSvc, walletSvc, auditStore, 0)

	// EAS — Expo Application Services REST client + background poller.
	// EAS_TOKEN is optional: when unset the orchestrator boots, logs a
	// single warning, and every mobile resolver returns NOT_CONFIGURED.
	// Per-project EAS_TOKEN secrets still work (resolved per call via
	// eas.ResolveExpoToken), but the shared client + poller stay nil.
	var (
		easClient *eas.Client
		easPoller *eas.Poller
	)
	if tok := strings.TrimSpace(os.Getenv("EAS_TOKEN")); tok != "" {
		easClient = eas.New(tok, eas.WithLogger(logger.With().Str("component", "eas").Logger()))
		easPoller = eas.NewPoller(easClient, ledgerSvc,
			eas.WithPollerLogger(logger.With().Str("component", "eas-poller").Logger()),
		)
		superviseDaemon(ctx, logger, "eas-poller", func(runCtx context.Context) error {
			runCtx, span := tracing.StartSpan(runCtx, "eas.poller.daemon")
			defer span.End()
			err := easPoller.Start(runCtx)
			if err != nil && !errors.Is(err, context.Canceled) {
				span.RecordError(err)
			}
			return err
		})
	} else {
		logger.Warn().Msg("EAS_TOKEN unset — mobile resolvers will fall back to per-project secrets only")
	}

	api := httpapi.New(httpapi.Deps{
		Projects: projects, Engine: engine, Agents: registry, Patches: patches,
		Billing: billing, Stripe: stripeSvc, Paddle: paddleSvc, Guard: guard,
		Auth: authSvc, AuthOptional: cfg.AuthOptional,
		AllowedOrigins: cfg.CORSOrigins,
		Memory:         memoryStore, Audit: auditStore, Telemetry: telemetrySink,
		Bus:                       eventBus,
		NotifyPrefs:               prefsStore,
		Notify:                    notifyEngine,
		RuntimeURL:                cfg.RuntimeURL,
		PublicBaseURL:             cfg.PublicBaseURL,
		Version:                   buildVersion,
		Commit:                    buildCommit,
		BuildTime:                 buildTime,
		DevEnv:                    cfg.Env,
		DevWalletSeedUSD:          cfg.DevWalletSeedUSD,
		Verifications:             verifications,
		PasswordResets:            passwordResets,
		Sessions:                  sessionStore,
		SessionCache:              sessionCache,
		EmailVerifier:             emailVerifier,
		EmailChanger:              emailChangerImpl,
		PasswordRotator:           passwordRotator,
		Email:                     emailSender,
		WebBaseURL:                webBaseURL,
		AuthAudit:                 auditStore,
		PasswordResetIPLimiter:    passwordResetIPLimiter,
		PasswordResetEmailLimiter: passwordResetEmailLimiter,
		ResendVerificationLimiter: resendVerificationLimiter,
		Logger:                    logger,

		// V22 service surface.
		Wallet:           walletSvc,
		WalletTopper:     walletTopper,
		Ledger:           ledgerSvc,
		Execution:        execSvc,
		ExecutionSettler: executionSettler,
		ProfitGuard:      profitGuard,
		ProfitGuardStore: guardStore,
		Blueprints:       blueprintsReg,
		BlueprintStats:   blueprintStats,
		IdeaParser:       wireup.BuildIdeaParser(router, blueprintsReg, logger.With().Str("component", "ideaparser").Logger()),
		Completion:       completionSvc,
		Repair:           repairGenome,
		PatchMemory:      patchMem,
		Dashboards:       dashboardSvc,

		// V22 Wave-2.
		Deploy:         deploySvc,
		DeployDomains:  deployDomainSvc,
		Hardening:      &hardeningCfg,
		PolicyPEP:      policyPEP,
		PersistedStore: persistedStore,
		GqlRateLimiter: gqlLimiter,
		MemoryGraph:    memGraph.Graph,

		// V22 Wave-3 service surface.
		Forecaster:            forecaster,
		WowLoopBuilder:        wowBuilder,
		AuditExporter:         auditExporter,
		AuditExportConfig:     auditExportConfig,
		SecurityReportBuilder: secReportBuilder,
		Operator:              operatorSvc,

		// In-process diagnostics plane (ring buffer + REST tail + GraphQL).
		Diagnostics: diagSvc,

		// Mobile (EAS) plane — Expo Application Services REST client +
		// background poller. nil when EAS_TOKEN is unset.
		EAS:       easClient,
		EASPoller: easPoller,

		// Mobile (device cloud) — Pro-tier real-device sessions. Manager
		// is constructed unconditionally so the resolver layer is wired
		// even when no provider credentials exist at boot; providers
		// register lazily on first request through the resolver.
		DeviceCloud: buildDeviceCloudManager(logger, ledgerSvc),
	})

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api,
		ReadHeaderTimeout: 5 * time.Second,
		// SSE streams + GraphQL subscriptions can be long-lived, so we
		// favour generous (but finite) body/write/idle deadlines over
		// none-at-all. Slowloris protection still leans on
		// ReadHeaderTimeout. IdleTimeout reclaims wedged keep-alives.
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 0, // streaming endpoints set per-request deadlines
		IdleTimeout:  120 * time.Second,
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error().Interface("panic", r).Bytes("stack", debug.Stack()).
					Msg("http listener panic recovered")
				sentryext.CaptureRecovered(ctx, r)
			}
		}()
		logger.Info().Str("addr", cfg.Addr).Str("env", cfg.Env).
			Str("db", cfg.DBDriver).Bool("auth_optional", cfg.AuthOptional).
			Msg("orchestrator listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	sig := <-stop
	signal.Stop(stop)

	logger.Info().Str("signal", sig.String()).Msg("shutting down")
	// Cancel the root context first so every background daemon receives
	// the cancellation immediately. Then drain HTTP with a wider deadline
	// so in-flight requests can finish (helm terminationGracePeriod gives
	// us 45s; we reserve 30s for the HTTP drain and 15s for the rest).
	cancelMain()
	if temporalRuntime != nil {
		temporalRuntime.Stop()
	}
	shCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shCtx); err != nil {
		logger.Warn().Err(err).Msg("http server shutdown")
	}
	if eventPublisher != nil {
		_ = eventPublisher.Close()
	}
	if pgPool != nil {
		pgPool.Close()
	}
	logger.Info().Msg("shutdown complete")
}

// connectPostgresTuned builds a pgxpool.Pool with production-scale sizing.
func connectPostgresTuned(ctx context.Context, url string, logger zerolog.Logger) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, errors.New("postgres URL empty")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = int32(intEnv("POSTGRES_MAX_CONNS", 50))
	cfg.MinConns = int32(intEnv("POSTGRES_MIN_CONNS", 5))
	cfg.MaxConnLifetime = durationEnv("POSTGRES_MAX_CONN_LIFETIME", 30*time.Minute)
	cfg.MaxConnIdleTime = durationEnv("POSTGRES_MAX_CONN_IDLE", 5*time.Minute)
	cfg.HealthCheckPeriod = durationEnv("POSTGRES_HEALTH_CHECK", 30*time.Second)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	logger.Info().
		Int32("max_conns", cfg.MaxConns).
		Int32("min_conns", cfg.MinConns).
		Dur("max_conn_lifetime", cfg.MaxConnLifetime).
		Dur("max_conn_idle", cfg.MaxConnIdleTime).
		Dur("health_check_period", cfg.HealthCheckPeriod).
		Msg("postgres pool ready")
	return pool, nil
}

func intEnv(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func durationEnv(name string, def time.Duration) time.Duration {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// selectEmbedder resolves IRONFLYER_EMBEDDINGS_BACKEND into a concrete
// Embedder. Returns (nil, "") when nothing can be enabled so the caller
// leaves the memory store unwrapped (substring fallback).
func selectEmbedder(cfg config.Config, logger zerolog.Logger) (embeddings.Embedder, string) {
	backend := strings.ToLower(strings.TrimSpace(cfg.EmbeddingsBackend))
	tryONNX := func() (embeddings.Embedder, bool) {
		e, err := embeddings.NewONNXEmbedder(embeddings.ONNXConfig{
			ModelPath: cfg.ONNXModelPath,
			VocabPath: cfg.ONNXVocabPath,
			Dimension: cfg.ONNXDimension,
		})
		if err != nil {
			logger.Warn().Err(err).Msg("Embeddings: ONNX backend unavailable")
			return nil, false
		}
		logger.Info().Str("model", cfg.ONNXModelPath).Msg("Embeddings: ONNX backend ready")
		return e, true
	}
	tryHF := func() (embeddings.Embedder, bool) {
		if cfg.HFAPIKey == "" {
			return nil, false
		}
		model := cfg.HFEmbeddingsModel
		if model == "" {
			model = cfg.HFEmbedModel
		}
		return embeddings.NewHuggingFaceEmbedder(cfg.HFAPIKey, model), true
	}
	switch backend {
	case "onnx":
		if e, ok := tryONNX(); ok {
			return e, "onnx"
		}
		return nil, ""
	case "auto":
		if e, ok := tryONNX(); ok {
			return e, "onnx"
		}
		if e, ok := tryHF(); ok {
			return e, "hf"
		}
		return nil, ""
	default:
		if e, ok := tryHF(); ok {
			return e, "hf"
		}
		return nil, ""
	}
}

// selectDBProvisioner returns the configured DBProvisioner. Defaults to
// the no-op when nothing is wired so dev still boots cleanly.
func selectDBProvisioner(cfg config.Config, logger zerolog.Logger) finisher.DBProvisioner {
	switch cfg.DBProvisioner {
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

// parseOTelHeaders parses the IRONFLYER_OTEL_HEADERS env value into the
// map shape the OTLP HTTP exporter expects.
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

// profitGuardEngineAdapter satisfies finisher.ProfitGuardHook by
// resolving the live execution state and calling Guard.Decide at the
// BeforeRetryLoop point. Stop-class verdicts short-circuit the
// finisher's retry budget so a doomed patch can't burn the whole
// allowance.
type profitGuardEngineAdapter struct {
	guard profitguard.Guard
	exec  execution.Service
}

type temporalExecutionAdapter struct {
	svc execution.Service
}

func (a temporalExecutionAdapter) GetState(ctx context.Context, executionID string) (temporalworker.ExecStateSnapshot, error) {
	if a.svc == nil {
		return temporalworker.ExecStateSnapshot{}, nil
	}
	state, err := a.svc.GetState(ctx, executionID)
	if err != nil {
		return temporalworker.ExecStateSnapshot{}, err
	}
	stopLoss := decimal.Zero
	if state.StopLossUSD != nil {
		stopLoss = *state.StopLossUSD
	}
	expectedDelta := 0.0
	if state.ExpectedCompletionDelta != nil {
		expectedDelta = *state.ExpectedCompletionDelta
	}
	risk := 0.0
	if state.RiskScore != nil {
		risk = *state.RiskScore
	}
	return temporalworker.ExecStateSnapshot{
		ExecutionID:             state.ID,
		TenantID:                state.TenantID,
		ProjectID:               state.ProjectID,
		Status:                  string(state.Status),
		BudgetUSD:               state.BudgetUSD,
		SpentUSD:                state.SpentUSD,
		ReservedUSD:             state.ReservedUSD,
		StopLossUSD:             stopLoss,
		CompletionScore:         state.CompletionScore,
		ExpectedCompletionDelta: expectedDelta,
		RiskScore:               risk,
	}, nil
}

func (a temporalExecutionAdapter) Admit(ctx context.Context, executionID string) error {
	if a.svc == nil {
		return nil
	}
	if err := a.svc.Admit(ctx, executionID); err != nil && !a.hasStatus(ctx, executionID, execution.StatusAdmitted, execution.StatusRunning) {
		return err
	}
	return nil
}

func (a temporalExecutionAdapter) Start(ctx context.Context, executionID string) error {
	if a.svc == nil {
		return nil
	}
	if err := a.svc.Start(ctx, executionID); err != nil && !a.hasStatus(ctx, executionID, execution.StatusRunning) {
		return err
	}
	return nil
}

func (a temporalExecutionAdapter) Succeed(ctx context.Context, executionID string) error {
	if a.svc == nil {
		return nil
	}
	if err := a.svc.Succeed(ctx, executionID); err != nil && !a.hasStatus(ctx, executionID, execution.StatusSucceeded) {
		return err
	}
	return nil
}

func (a temporalExecutionAdapter) Fail(ctx context.Context, executionID, reason string) error {
	if a.svc == nil {
		return nil
	}
	if err := a.svc.Fail(ctx, executionID, reason); err != nil && !a.hasStatus(ctx, executionID, execution.StatusFailed) {
		return err
	}
	return nil
}

func (a temporalExecutionAdapter) Stop(ctx context.Context, executionID, reason string) error {
	if a.svc == nil {
		return nil
	}
	if err := a.svc.Stop(ctx, executionID, reason); err != nil && !a.hasStatus(ctx, executionID, execution.StatusStopped) {
		return err
	}
	return nil
}

func (a temporalExecutionAdapter) Kill(ctx context.Context, executionID, reason string) error {
	if a.svc == nil {
		return nil
	}
	if err := a.svc.Kill(ctx, executionID, reason); err != nil && !a.hasStatus(ctx, executionID, execution.StatusKilled) {
		return err
	}
	return nil
}

func (a temporalExecutionAdapter) hasStatus(ctx context.Context, executionID string, statuses ...execution.Status) bool {
	if a.svc == nil {
		return false
	}
	row, err := a.svc.Get(ctx, executionID)
	if err != nil {
		return false
	}
	for _, status := range statuses {
		if row.Status == status {
			return true
		}
	}
	return false
}

type temporalProfitGuardAdapter struct {
	guard profitguard.Guard
}

func (a temporalProfitGuardAdapter) Decide(ctx context.Context, point string, snapshot temporalworker.ExecStateSnapshot) (string, string, error) {
	if a.guard == nil {
		return "continue", "profit_guard_unwired", nil
	}
	state := temporalProfitState(snapshot)
	decision, err := a.guard.Decide(ctx, profitguard.EnforcementPoint(point), state)
	if err != nil {
		return "continue", "profit_guard_error", nil
	}
	return string(decision.Action), decision.Reason, nil
}

func (a temporalProfitGuardAdapter) Record(context.Context, string, string, string, string) error {
	// The finisher's hot path already records provider/retry decisions
	// with full state. The Temporal-level preflight is deliberately
	// kept side-effect-light until workflow signals carry richer state.
	return nil
}

func temporalProfitState(s temporalworker.ExecStateSnapshot) profitguard.ExecState {
	estimatedPlatformCost := s.SpentUSD.Add(s.ReservedUSD)
	return profitguard.ExecState{
		ExecutionID:              s.ExecutionID,
		TenantID:                 s.TenantID,
		UserBudgetUSD:            s.BudgetUSD,
		SpentUSD:                 s.SpentUSD,
		ReservedUSD:              s.ReservedUSD,
		EstimatedPlatformCostUSD: estimatedPlatformCost,
		ExpectedCompletionDelta:  s.ExpectedCompletionDelta,
		RiskScore:                s.RiskScore,
		StopLossUSD:              s.StopLossUSD,
	}
}

type temporalEngineAdapter struct {
	engine *finisher.Engine
}

func (a temporalEngineAdapter) RunGate(ctx context.Context, projectID, _ string) (bool, int, decimal.Decimal, error) {
	if a.engine == nil {
		return true, 0, decimal.Zero, nil
	}
	report, err := a.engine.Run(ctx, projectID)
	if err != nil {
		return false, 0, decimal.Zero, err
	}
	issues := 0
	for _, gate := range report.Gates {
		issues += len(gate.Issues)
	}
	return report.Completed, issues, decimal.Zero, nil
}

type temporalSettlerAdapter struct {
	svc  execution.Settler
	exec execution.Service
}

func (a temporalSettlerAdapter) Close(ctx context.Context, executionID, finalStatus string) (temporalworker.SettleOutput, error) {
	status := execution.Status(finalStatus)
	if a.exec != nil {
		if err := a.transitionTerminal(ctx, executionID, status); err != nil {
			return temporalworker.SettleOutput{}, err
		}
	}
	if a.svc == nil {
		return temporalworker.SettleOutput{}, nil
	}
	settlement, err := a.svc.Close(ctx, executionID, status)
	if err != nil {
		return temporalworker.SettleOutput{}, err
	}
	out := temporalworker.SettleOutput{SpentUSD: settlement.SpentUSD}
	if a.exec != nil {
		if row, getErr := a.exec.Get(ctx, executionID); getErr == nil {
			out.CompletionScore = row.CompletionScore
			if row.GrossMarginPct != nil {
				out.GrossMarginPct = *row.GrossMarginPct
			}
		}
	}
	return out, nil
}

func (a temporalSettlerAdapter) transitionTerminal(ctx context.Context, executionID string, status execution.Status) error {
	var err error
	switch status {
	case execution.StatusSucceeded:
		err = a.exec.Succeed(ctx, executionID)
	case execution.StatusStopped:
		err = a.exec.Stop(ctx, executionID, "temporal_workflow_stopped")
	case execution.StatusKilled:
		err = a.exec.Kill(ctx, executionID, "temporal_workflow_killed")
	default:
		err = a.exec.Fail(ctx, executionID, "temporal_workflow_failed")
	}
	if err == nil {
		return nil
	}
	row, getErr := a.exec.Get(ctx, executionID)
	if getErr == nil && row.Status == status {
		return nil
	}
	return err
}

type temporalEventAdapter struct {
	outbox *events.PostgresOutbox
}

func (a temporalEventAdapter) Emit(ctx context.Context, eventType string, payload map[string]any) error {
	if a.outbox == nil {
		return nil
	}
	key := ""
	if v, ok := payload["execution_id"].(string); ok {
		key = v
	}
	if key == "" {
		key = eventType
	}
	_, err := a.outbox.Enqueue(ctx, events.Event{
		Topic:   events.TopicFor("", "execution", "lifecycle", 1),
		Key:     key,
		Type:    eventType,
		Version: 1,
		Payload: payload,
	})
	return err
}

func (a profitGuardEngineAdapter) BeforeRetry(ctx context.Context, executionID, gate string, attempt int, _ float64) (bool, string) {
	if a.guard == nil || a.exec == nil || executionID == "" {
		return true, ""
	}
	state, err := a.exec.GetState(ctx, executionID)
	if err != nil {
		// Execution unreadable — fail open so retries proceed; the
		// BillingGuard remains the harder economic stop.
		return true, ""
	}
	in := profitguardbridge.StateToGuardInput(state, nil, profitguardbridge.BridgeFlags{})
	dec, derr := a.guard.Decide(ctx, profitguard.BeforeRetryLoop, in)
	if derr != nil {
		return true, ""
	}
	_ = a.guard.Record(ctx, executionID, profitguard.BeforeRetryLoop, dec, in)
	switch dec.Action {
	case profitguard.Stop, profitguard.KillBranch, profitguard.PauseForBudget:
		return false, string(dec.Action) + ": " + dec.Reason
	}
	return true, ""
}

// tenantUUIDForMain mirrors resolver.tenantUUIDFor — main.go needs the
// same deterministic mapping so the per-token cost attribution writes
// land under the same tenant key the resolver / settler use.
func tenantUUIDForMain(tenant string) uuid.UUID {
	if id, err := uuid.Parse(tenant); err == nil {
		return id
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(tenant))
}

// engineSettlerAdapter is the finisher.ExecutionSettler implementation
// wired into the engine. It drives the FSM transition through the
// execution service AND calls the canonical execution.Settler.Close in
// the same step, so an end-of-Run() callback fully closes the money
// loop without the engine ever importing the execution package.
type engineSettlerAdapter struct {
	exec    execution.Service
	settler execution.Settler
	logger  zerolog.Logger
}

func (a engineSettlerAdapter) SettleSucceeded(ctx context.Context, executionID string) error {
	if err := a.exec.Succeed(ctx, executionID); err != nil {
		a.logger.Warn().Err(err).Str("execution_id", executionID).Msg("execSvc.Succeed on terminal settle")
	}
	if _, err := a.settler.Close(ctx, executionID, execution.StatusSucceeded); err != nil {
		a.logger.Warn().Err(err).Str("execution_id", executionID).Msg("settler.Close on succeeded")
		return err
	}
	return nil
}

func (a engineSettlerAdapter) SettleFailed(ctx context.Context, executionID, reason string) error {
	if err := a.exec.Fail(ctx, executionID, reason); err != nil {
		a.logger.Warn().Err(err).Str("execution_id", executionID).Msg("execSvc.Fail on terminal settle")
	}
	if _, err := a.settler.Close(ctx, executionID, execution.StatusFailed); err != nil {
		a.logger.Warn().Err(err).Str("execution_id", executionID).Msg("settler.Close on failed")
		return err
	}
	return nil
}

// repairAwareBlueprintStats is the wrapper the engine-side Settler
// uses for its blueprint-stats leg. The canonical execution.Settler
// hard-codes RunOutcome.Repaired=false because, prior to V22 Agent 11,
// nothing on the execution row carried the repair count. This wrapper
// consults the per-execution counter the learning hooks bumped on
// every successful recovery round, then forwards the augmented
// outcome to the underlying StatsService.
//
// The HTTP/GraphQL paths still talk to the original blueprintStats
// directly, so Stop/Refund flows that don't go through the engine
// adapter pay no overhead.
type repairAwareBlueprintStats struct {
	Stats    blueprints.StatsService
	Learning *finisher.LearningHooks
}

func (s repairAwareBlueprintStats) RecordRun(ctx context.Context, o blueprints.RunOutcome) error {
	if s.Stats == nil {
		return nil
	}
	if !o.Repaired && s.Learning != nil {
		execID := ""
		if o.ExecutionID != uuid.Nil {
			execID = o.ExecutionID.String()
		}
		if execID != "" && s.Learning.RepairsFor(execID) > 0 {
			o.Repaired = true
		}
	}
	return s.Stats.RecordRun(ctx, o)
}

func (s repairAwareBlueprintStats) Get(ctx context.Context, blueprintID string) (blueprints.Stats, error) {
	if s.Stats == nil {
		return blueprints.Stats{}, blueprints.ErrNoStats
	}
	return s.Stats.Get(ctx, blueprintID)
}

func (s repairAwareBlueprintStats) All(ctx context.Context) ([]blueprints.Stats, error) {
	if s.Stats == nil {
		return nil, nil
	}
	return s.Stats.All(ctx)
}

func (s repairAwareBlueprintStats) Top(ctx context.Context, byMetric string, limit int) ([]blueprints.Stats, error) {
	if s.Stats == nil {
		return nil, nil
	}
	return s.Stats.Top(ctx, byMetric, limit)
}

// runtimePreviewAdapter implements wowloop.RuntimeSource against the
// runtime HTTP client. The wow loop wireup stamps
// ExecutionSnapshot.WorkspaceID = execution.ProjectID, and this adapter
// resolves projectID → running workspace → preview URL.
//
// The convention avoids a schema migration: the runtime already
// tracks (userBearer, projectID) → workspace, so we defer the lookup
// to it instead of mirroring the column on the executions row. When
// the workspace cannot be resolved we return ("", nil) so the wow
// loop falls back to the deploy preview URL without surfacing an error.
type runtimePreviewAdapter struct {
	client *runtime.Client
	bearer string
}

func (a *runtimePreviewAdapter) PreviewURL(ctx context.Context, idOrWorkspace string) (string, error) {
	if a == nil || a.client == nil || !a.client.Enabled() || idOrWorkspace == "" {
		return "", nil
	}
	// Prefer the caller's JWT (graphql request ctx carries it via the
	// auth middleware) over the static service bearer: the runtime's
	// owner check is per-user, so calling with the wrong bearer 404s.
	// Falls back to the service bearer for non-request paths (the
	// finisher kick goroutine doesn't go through here).
	bearer := auth.BearerFromContext(ctx)
	if bearer == "" {
		bearer = a.bearer
	}
	// A63 + workspace-auto-allocation: the wow loop now stamps
	// ExecutionSnapshot.WorkspaceID = execution.workspaceID when set
	// (real `ws-...` id) and falls back to projectID only for legacy
	// rows. Try the workspace id path first — it's the cheap one
	// (no list scan) and is the right shape for everything written
	// since finisher/engine.go's auto-CreateWorkspace landed. If that
	// 404s, treat the argument as a projectID and walk the list.
	if strings.HasPrefix(idOrWorkspace, "ws-") {
		return a.client.PreviewURL(ctx, bearer, idOrWorkspace)
	}
	ws, err := a.client.FindWorkspaceForProject(ctx, bearer, idOrWorkspace)
	if err != nil || ws.ID == "" {
		return "", nil
	}
	return a.client.PreviewURL(ctx, bearer, ws.ID)
}

// buildDeviceCloudManager constructs the device-cloud manager and
// registers BrowserStack when platform-wide credentials are present in
// the environment. Per-project Secrets override the platform default at
// resolver time (see devicecloud.ResolveCredentials).
func buildDeviceCloudManager(logger zerolog.Logger, ledgerSvc ledger.Service) *devicecloud.Manager {
	mgr := devicecloud.New(logger, ledgerSvc)
	creds := devicecloud.ResolveCredentials(nil)
	if creds.HasBrowserStack() {
		mgr.Register(devicecloud.NewBrowserStackClient(creds.BrowserStackUsername, creds.BrowserStackAccessKey, nil))
	}
	if creds.HasAWSDeviceFarm() {
		mgr.Register(devicecloud.NewAWSDeviceFarmClient(creds.AWSAccessKeyID, creds.AWSSecretAccessKey, ""))
	}
	return mgr
}

// superviseDaemon launches fn in its own goroutine, recovers any panic
// (so a single daemon crash never tears the whole orchestrator down),
// reports the panic to Sentry + zerolog tagged by daemon name, and
// blocks until ctx is cancelled. Used by every long-running background
// worker the orchestrator owns (publisher, biller, sweeper, clickhouse
// consumer/ingester/correction, EAS poller, policy reloader, …).
func superviseDaemon(ctx context.Context, logger zerolog.Logger, name string, fn func(context.Context) error) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				logger.Error().
					Str("daemon", name).
					Interface("panic", r).
					Bytes("stack", stack).
					Msg("daemon goroutine panic recovered")
				sentryext.CaptureRecovered(ctx, r)
			}
		}()
		if err := fn(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn().Str("daemon", name).Err(err).Msg("daemon exited with error")
		}
	}()
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

// bracketKey collapses a UsageBracket to a one-char key used in
// dedupe op-keys. Stable across releases — never repurpose existing
// letters; add new ones if a future bracket lands.
func bracketKey(b budget.UsageBracket) string {
	switch b {
	case budget.BracketLow:
		return "l"
	case budget.BracketApproach:
		return "a"
	case budget.BracketNearCap:
		return "n"
	case budget.BracketExhausted:
		return "e"
	default:
		return "?"
	}
}

// logProfitKnobs surfaces the active margin-protecting env switches
// in one startup line so the operator never has to grep through the
// orchestrator log to confirm what's on. Reads the same env names
// the budget/rates/optimizer/tokencap modules read so this stays
// the single source of truth for what's active.
func logProfitKnobs(logger zerolog.Logger, billing *budget.Billing) {
	cap := budget.DefaultPromptCap()
	discounts := map[string]string{}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "IRONFLYER_PROVIDER_DISCOUNT_PCT_") {
			continue
		}
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		discounts[strings.ToLower(kv[len("IRONFLYER_PROVIDER_DISCOUNT_PCT_"):eq])] = kv[eq+1:]
	}
	aggressive := strings.EqualFold(os.Getenv("IRONFLYER_AGGRESSIVE_ROUTING"), "1") ||
		strings.EqualFold(os.Getenv("IRONFLYER_AGGRESSIVE_ROUTING"), "true")
	privateInline := strings.EqualFold(os.Getenv("IRONFLYER_INLINE_COMPLETIONS_PRIVATE"), "1") ||
		strings.EqualFold(os.Getenv("IRONFLYER_INLINE_COMPLETIONS_PRIVATE"), "true")
	planCount := 0
	if billing != nil {
		planCount = len(billing.Plans)
	}
	logger.Info().
		Bool("aggressive_routing", aggressive).
		Bool("inline_completions_private", privateInline).
		Int("max_prompt_tokens", cap.MaxTotalTokens).
		Int("max_input_tokens", cap.MaxInputTokens).
		Int("max_output_tokens", cap.MaxOutputTokens).
		Int("provider_discounts", len(discounts)).
		Int("plans_loaded", planCount).
		Msg("V22 profitability knobs active")
}
