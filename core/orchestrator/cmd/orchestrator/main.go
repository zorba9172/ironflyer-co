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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	"golang.org/x/sync/errgroup"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/atlas"
	"ironflyer/core/orchestrator/internal/ai/completion"
	"ironflyer/core/orchestrator/internal/ai/embeddings"
	"ironflyer/core/orchestrator/internal/ai/finisher"
	"ironflyer/core/orchestrator/internal/ai/inference"
	"ironflyer/core/orchestrator/internal/ai/learning"
	"ironflyer/core/orchestrator/internal/ai/memory"
	"ironflyer/core/orchestrator/internal/ai/refactor"
	"ironflyer/core/orchestrator/internal/business/blueprints"
	"ironflyer/core/orchestrator/internal/business/budget"
	"ironflyer/core/orchestrator/internal/business/budget/payments"
	"ironflyer/core/orchestrator/internal/business/clickhouse"
	"ironflyer/core/orchestrator/internal/business/compliance"
	"ironflyer/core/orchestrator/internal/business/dashboards"
	"ironflyer/core/orchestrator/internal/business/dashboards/adapters"
	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/business/ledger"
	"ironflyer/core/orchestrator/internal/business/outboxhooks"
	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/customer/auth/oauth"
	"ironflyer/core/orchestrator/internal/customer/notify"
	"ironflyer/core/orchestrator/internal/operations/abuse"
	"ironflyer/core/orchestrator/internal/operations/appconsole"
	"ironflyer/core/orchestrator/internal/operations/arch"
	"ironflyer/core/orchestrator/internal/operations/audit"
	"ironflyer/core/orchestrator/internal/operations/auditexport"
	"ironflyer/core/orchestrator/internal/operations/bus"
	"ironflyer/core/orchestrator/internal/operations/config"
	"ironflyer/core/orchestrator/internal/operations/deploy"
	"ironflyer/core/orchestrator/internal/operations/diagnostics"
	"ironflyer/core/orchestrator/internal/operations/events"
	"ironflyer/core/orchestrator/internal/operations/gqlhardening"
	"ironflyer/core/orchestrator/internal/operations/httpapi"
	"ironflyer/core/orchestrator/internal/operations/metrics"
	"ironflyer/core/orchestrator/internal/operations/migrate"
	"ironflyer/core/orchestrator/internal/operations/mobile/devicecloud"
	"ironflyer/core/orchestrator/internal/operations/mobile/eas"
	"ironflyer/core/orchestrator/internal/operations/patch"
	"ironflyer/core/orchestrator/internal/operations/policy"

	"ironflyer/core/orchestrator/internal/ai/providers"
	"ironflyer/core/orchestrator/internal/ai/repair"
	"ironflyer/core/orchestrator/internal/business/guild"
	"ironflyer/core/orchestrator/internal/business/profitguard"
	"ironflyer/core/orchestrator/internal/business/profitguardbridge"
	"ironflyer/core/orchestrator/internal/business/profitguardctx"
	"ironflyer/core/orchestrator/internal/business/provisioning"
	"ironflyer/core/orchestrator/internal/business/sentinel"
	"ironflyer/core/orchestrator/internal/business/shippass"
	"ironflyer/core/orchestrator/internal/business/wallet"
	"ironflyer/core/orchestrator/internal/operations/ratelimit"
	"ironflyer/core/orchestrator/internal/operations/redisbus"
	"ironflyer/core/orchestrator/internal/operations/runtime"
	"ironflyer/core/orchestrator/internal/operations/secrets"
	"ironflyer/core/orchestrator/internal/operations/sentryext"
	"ironflyer/core/orchestrator/internal/operations/storage"
	"ironflyer/core/orchestrator/internal/operations/store"
	"ironflyer/core/orchestrator/internal/operations/temporalworker"
	"ironflyer/core/orchestrator/internal/operations/tracing"
	"ironflyer/core/orchestrator/internal/operations/wireup"
	"ironflyer/core/orchestrator/internal/suppliers/context7"
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
	if !cfg.IsProd() && len(cfg.CORSOrigins) == 0 {
		logger.Warn().Str("env", cfg.Env).
			Msg("CORS open-mode: any browser Origin is reflected — set IRONFLYER_CORS_ORIGINS before promoting to prod")
	}
	if !cfg.IsProd() && cfg.MetricsToken == "" {
		logger.Warn().Str("env", cfg.Env).
			Msg("/metrics is unauthenticated — set IRONFLYER_METRICS_TOKEN before promoting to prod")
	}
	if cfg.AuditRedact == "off" {
		logger.Warn().Msg("audit PII redaction disabled via IRONFLYER_AUDIT_REDACT=off — raw emails, IPs, and provider keys may land in the audit chain")
		audit.SetRedactionEnabled(false)
	} else {
		audit.SetRedactionEnabled(true)
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

	// ---------------- Parallel init of independent subsystems --------------
	// Cold-start optimization: Sentry, OTel, Postgres, Redis, and Surreal
	// have no startup-time dependencies on each other. Booting them in
	// parallel via errgroup turns the cold-start wait from the SUM of each
	// init duration into MAX(each). The migrations step still waits on
	// pgPool because migrations are a hard precondition for stores that
	// build on Postgres. Each leg logs its own duration so the operator
	// can see where boot time actually goes.
	bootStart := time.Now()
	var (
		sentryFlush     func()
		tracingShutdown func(context.Context) error
		pgPool          *pgxpool.Pool
		surrealDB       *surrealdb.DB
		redisClient     *redisbus.Client
	)
	bootGroup, bootCtx := errgroup.WithContext(ctx)

	// Sentry.
	bootGroup.Go(func() error {
		start := time.Now()
		sentryDSN := strings.TrimSpace(os.Getenv("SENTRY_DSN_ORCHESTRATOR"))
		if sentryDSN == "" {
			sentryDSN = strings.TrimSpace(os.Getenv("SENTRY_DSN"))
		}
		sentryEnvName := strings.TrimSpace(os.Getenv("IRONFLYER_ENV"))
		if sentryEnvName == "" {
			sentryEnvName = "development"
		}
		flush, ierr := sentryext.Init(sentryext.Opts{
			DSN:              sentryDSN,
			Environment:      sentryEnvName,
			Release:          strings.TrimSpace(os.Getenv("IRONFLYER_VERSION")),
			TracesSampleRate: sentryext.FloatFromEnv("SENTRY_TRACES_SAMPLE", 0.05),
			ServerName:       "ironflyer-orchestrator",
		})
		if ierr != nil {
			logger.Warn().Err(ierr).Msg("sentry init failed; continuing without exception reporting")
		} else if sentryDSN != "" {
			logger.Info().Str("env", sentryEnvName).Dur("took", time.Since(start)).Msg("Sentry initialised")
		}
		sentryFlush = flush
		return nil
	})

	// OpenTelemetry.
	bootGroup.Go(func() error {
		start := time.Now()
		sh, ierr := tracing.Init(bootCtx, tracing.Opts{
			Exporter:       cfg.OTelExporter,
			Endpoint:       cfg.OTelEndpoint,
			Insecure:       cfg.OTelInsecure,
			ServiceName:    "ironflyer-orchestrator",
			ServiceVersion: "1.0",
			SampleRatio:    cfg.OTelSampleRatio,
			Headers:        parseOTelHeaders(cfg.OTelHeaders),
		})
		if ierr != nil {
			logger.Warn().Err(ierr).Msg("tracing init failed; continuing without OTel")
		} else if cfg.OTelExporter != "none" && cfg.OTelExporter != "" {
			logger.Info().Str("exporter", cfg.OTelExporter).Dur("took", time.Since(start)).
				Msg("OTel tracing initialised")
		}
		tracingShutdown = sh
		return nil
	})

	// Postgres pool. ParseConfig + NewWithConfig are non-blocking under
	// pgx v5 (idle conns are created in a background goroutine); the Ping
	// roundtrip is the only blocking call. IRONFLYER_PG_LAZY=true skips
	// the Ping so first-request lazy-loads when boot speed is paramount.
	if cfg.UsePostgres() {
		bootGroup.Go(func() error {
			start := time.Now()
			p, ierr := connectPostgresTuned(bootCtx, cfg.PostgresURL, logger)
			if ierr != nil {
				return ierr
			}
			pgPool = p
			logger.Info().Dur("took", time.Since(start)).Msg("postgres pool ready (parallel)")
			return nil
		})
	}

	// SurrealDB project + memory store (only when configured).
	if cfg.UseSurreal() {
		bootGroup.Go(func() error {
			start := time.Now()
			db, ierr := store.ConnectSurreal(bootCtx, store.SurrealOpts{
				URL: cfg.SurrealURL, Namespace: cfg.SurrealNS, Database: cfg.SurrealDB,
				User: cfg.SurrealUser, Pass: cfg.SurrealPass,
			})
			if ierr != nil {
				return ierr
			}
			if ierr := store.BootstrapSurreal(bootCtx, db); ierr != nil {
				return ierr
			}
			surrealDB = db
			logger.Info().Str("url", cfg.SurrealURL).Dur("took", time.Since(start)).
				Msg("SurrealDB connected (parallel)")
			return nil
		})
	}

	// Redis (optional, multi-pod coordination).
	if cfg.RedisEnabled {
		bootGroup.Go(func() error {
			start := time.Now()
			rc, ierr := redisbus.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
			if ierr != nil {
				return ierr
			}
			if !envBoolTrue("IRONFLYER_REDIS_LAZY") {
				if ierr := rc.Ping(bootCtx); ierr != nil {
					return ierr
				}
			}
			redisClient = rc
			logger.Info().Str("addr", cfg.RedisAddr).Dur("took", time.Since(start)).
				Msg("Redis ready (parallel)")
			return nil
		})
	}

	if err := bootGroup.Wait(); err != nil {
		logger.Fatal().Err(err).Msg("parallel init failed")
	}
	logger.Info().Dur("took", time.Since(bootStart)).Msg("parallel init complete")
	defer func() {
		if sentryFlush != nil {
			sentryFlush()
		}
	}()
	defer func() {
		if tracingShutdown != nil {
			shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tracingShutdown(shCtx)
		}
	}()

	// ---------------- Schema migrations (goose) -----------------------------
	// IRONFLYER_SKIP_MIGRATE=true: operators that run migrations through a
	// dedicated helm pre-install hook (infra/helm/ironflyer/templates/
	// migrate-job.yaml) skip the in-process check to shave several
	// hundred ms off cold start.
	if envBoolTrue("IRONFLYER_SKIP_MIGRATE") {
		logger.Info().Msg("schema migrations skipped (IRONFLYER_SKIP_MIGRATE=true)")
	} else {
		migStart := time.Now()
		if err := migrate.RunPool(ctx, pgPool, migrations.FS); err != nil {
			logger.Fatal().Err(err).Msg("apply schema migrations")
		}
		if pgPool != nil {
			logger.Info().Dur("took", time.Since(migStart)).Msg("schema migrations applied")
		}
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
	// Only write to the outbox when a publisher exists to drain it.
	// Without Redpanda, rows would pile up unbounded inside every
	// business transaction; the dashboards read Postgres directly, not
	// the outbox, so disabling the write loses nothing in lean mode.
	outboxhooks.SetWritesEnabled(eventPublisher != nil)

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
	// surrealDB is dialed during the parallel boot block above; we only
	// derive the store + seed here so the rest of the wiring stays linear.
	var projects store.Store
	if cfg.UseSurreal() && surrealDB != nil {
		ss := store.NewSurrealStore(ctx, surrealDB)
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

	var bootstrappedSuperuser auth.User
	hasBootstrappedSuperuser := false

	// ---------------- Superuser bootstrap (privileged backdoor) -----------
	if cfg.SuperuserEmail != "" && cfg.SuperuserPassword != "" {
		// In prod the env bootstrap is gated by an explicit force flag
		// so the credentials can't be accidentally promoted by a stray
		// env var leak. Set IRONFLYER_SUPERUSER_FORCE=true in prod when
		// you intentionally want the env bootstrap (e.g. first install,
		// or rotating the superuser).
		if cfg.IsProd() && !envBoolTrue("IRONFLYER_SUPERUSER_FORCE") {
			logger.Warn().
				Str("ignored_env", "IRONFLYER_SUPERUSER_EMAIL,IRONFLYER_SUPERUSER_PASSWORD").
				Msg("superuser env bootstrap disabled in prod; set IRONFLYER_SUPERUSER_FORCE=true to override")
		} else {
			verifier, _ := userStore.(auth.EmailVerifier)
			roleSetter, _ := userStore.(auth.RoleSetter)
			superuser, err := auth.EnsureSuperuser(ctx, authSvc, userStore, verifier, roleSetter,
				cfg.SuperuserEmail, cfg.SuperuserPassword, logger)
			if err != nil {
				logger.Warn().Err(err).Str("email", cfg.SuperuserEmail).
					Msg("superuser bootstrap failed (continuing)")
			} else {
				bootstrappedSuperuser = superuser
				hasBootstrappedSuperuser = true
			}
		}
	}

	// ---------------- Redis (optional, multi-pod coordination) -------------
	// Redis is dialed during the parallel boot block above; the variable
	// is hoisted there so the rest of the wiring (event bus, rate limit,
	// finisher) reads it directly without a second connect.
	if cfg.RedisEnabled && redisClient != nil {
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
	// Register HuggingFace when either a hosted token (HF_API_KEY) or a
	// self-hosted endpoint (HF_BASE_URL) is configured. A self-hosted base
	// URL is the "data never leaves your infra" private path: the provider
	// advertises CapPrivate, so privacy-sensitive traffic drains to YOUR
	// TGI/vLLM/SGLang box instead of any cloud LLM.
	if cfg.HFAPIKey != "" || cfg.HFBaseURL != "" {
		router.Register(providers.NewHuggingFaceProvider(providers.HuggingFaceOpts{
			APIKey:       cfg.HFAPIKey,
			BaseURL:      cfg.HFBaseURL,
			Model:        cfg.HFModel,
			CheapModel:   cfg.HFCheapModel,
			PowerModel:   cfg.HFPowerModel,
			PrivateModel: cfg.HFPrivateModel,
		}))
		selfHosted := cfg.HFBaseURL != "" && !strings.Contains(cfg.HFBaseURL, "huggingface.co")
		logger.Info().
			Bool("self_hosted", selfHosted).
			Bool("private", selfHosted || cfg.HFPrivateModel != "").
			Str("private_model", cfg.HFPrivateModel).
			Msg("HuggingFace provider registered")
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
	if cfg.PromptGuardEnabled {
		guard.WithPromptGuard(providers.NewPromptGuard(providers.PromptGuardConfig{
			MaxUserCharsPerMessage: cfg.PromptGuardMaxUserChars,
			MaxTotalRequestChars:   cfg.PromptGuardMaxTotalChars,
			BlockMode:              cfg.PromptGuardBlock,
		}, logger, nil))
		logger.Info().Bool("block_mode", cfg.PromptGuardBlock).
			Int("max_user_chars", cfg.PromptGuardMaxUserChars).
			Int("max_total_chars", cfg.PromptGuardMaxTotalChars).
			Msg("promptguard: enabled")
	} else if cfg.IsProd() {
		logger.Warn().Msg("promptguard: disabled in prod via IRONFLYER_PROMPTGUARD_ENABLED=false — defense-in-depth layer is off")
	}

	// ---------------- Memory + audit stores -------------------------------
	var memoryStore memory.Store
	resolvedMemoryBackend := "memory"
	surrealNativeVector := false
	switch {
	case cfg.MemoryBackend == "surreal" && surrealDB != nil:
		if err := memory.BootstrapSurreal(ctx, surrealDB); err != nil {
			logger.Fatal().Err(err).Msg("memory: surreal bootstrap")
		}
		ss := memory.NewSurrealStore(surrealDB)
		// Native SurrealDB HNSW semantic search: durable + cross-pod, unlike
		// the in-process VectorStore cache. When the embedder is wired we
		// define the HNSW index and let the store answer Substring queries
		// with native KNN; a failed index definition degrades to substring.
		if emb, label := selectEmbedder(cfg, logger); emb != nil {
			dim := memory.DefaultVectorDim
			if err := memory.BootstrapSurrealVectorIndex(ctx, surrealDB, dim); err != nil {
				logger.Warn().Err(err).Msg("memory: surreal HNSW index — semantic search degraded to substring")
			} else {
				ss = ss.WithVectorSearch(embeddings.NewCachedEmbedder(emb), dim)
				surrealNativeVector = true
				logger.Info().Str("backend", label).Int("dim", dim).Msg("Memory store: native SurrealDB HNSW semantic search enabled")
			}
		}
		memoryStore = ss
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
	// The in-process VectorStore wrapper is the fallback semantic layer for
	// backends without native vector search. Skip it when SurrealDB is doing
	// native HNSW (surrealNativeVector) or pgvector owns vectors — otherwise
	// we'd embed twice and shadow the durable index with an ephemeral cache.
	if resolvedMemoryBackend != "pgvector" && !surrealNativeVector {
		if emb, label := selectEmbedder(cfg, logger); emb != nil {
			memoryStore = &memory.VectorStore{
				Inner:    memoryStore,
				Embedder: embeddings.NewCachedEmbedder(emb),
			}
			logger.Info().Str("backend", label).Msg("Memory store: semantic re-ranking enabled (in-process)")
		}
	}

	// ---------------- Local ONNX inference (deep-learning v2) ------------
	// Small scoring models served on-cluster — no third-party API call.
	// Default build wires NoopService (every Score returns
	// ErrModelUnavailable so callers fall back to their heuristic
	// priors); -tags onnx + IRONFLYER_INFERENCE_ENABLED=true upgrades to
	// the real ORT-backed Service. See docs/DEEP_LEARNING.md.
	var infSvc inference.Service = inference.NewNoopService(logger)
	if cfg.InferenceEnabled {
		infSvc = inference.NewOnnxService(cfg.ModelsDir, logger)
		if path := strings.TrimSpace(cfg.CompletionScorerModelFile); path != "" {
			if err := infSvc.LoadModel(ctx, inference.CompletionScorerModel(path)); err != nil {
				logger.Warn().Err(err).Msg("inference: completion-scorer load failed; falling back to heuristic")
			}
		}
		if path := strings.TrimSpace(cfg.HallucinationModelFile); path != "" {
			if err := infSvc.LoadModel(ctx, inference.HallucinationModel(path)); err != nil {
				logger.Warn().Err(err).Msg("inference: hallucination-detector load failed; verifier will not block on it")
			}
		}
		if path := strings.TrimSpace(cfg.IntentClassifierModelFile); path != "" {
			if err := infSvc.LoadModel(ctx, inference.IntentClassifierModel(path)); err != nil {
				logger.Warn().Err(err).Msg("inference: intent-classifier load failed; router will use lexical fallback")
			}
		}
		logger.Info().Int("models", len(infSvc.Models())).Msg("inference: local ONNX scoring service ready")
	} else {
		logger.Info().Msg("inference: disabled (IRONFLYER_INFERENCE_ENABLED=false) — using NoopService")
	}
	_ = infSvc // wired into CompletionPredictor/RepairMatcher by the parallel learning-v2 work; expose now so the contract is locked.

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

	// ---------------- Anti-Bloat Engine wiring ----------------------------
	// architecture.json + Capability Atlas. See
	// docs/ANTI_BLOAT_ENGINE.md. Both degrade to "dark" if the
	// manifest is missing or the embedder is offline — the gate
	// stubs surface the degraded state in the dashboard.
	archManifest, archErr := arch.Load(strings.TrimSpace(os.Getenv("IRONFLYER_ARCHITECTURE_MANIFEST")))
	if archErr != nil {
		logger.Warn().Err(archErr).Msg("arch manifest: not loaded (dep_graph / arch_boundary gates will degrade)")
	} else {
		logger.Info().Int("layers", len(archManifest.Layers)).Int("rules", len(archManifest.Rules)).Msg("arch manifest loaded")
	}
	var atlasStore atlas.Store
	if memoryStore != nil {
		atlasStore = atlas.NewMemoryBackedStore(memoryStore, "", "")
		logger.Info().Msg("capability atlas: memory-backed (wraps configured memory.Store)")
	} else {
		atlasStore = atlas.NewMemoryStore(16 * 1024)
		logger.Info().Msg("capability atlas: in-process (no memory.Store wired)")
	}
	// Atlas-aware embedder for the patch engine's Reuse-First
	// Preflight. Re-use the selectEmbedder path so the same model
	// powers both memory rerank and Atlas search. Nil-safe; the
	// patch engine's preflight degrades to a lexical search when the
	// embedder is offline.
	var atlasEmbed embeddings.Embedder
	if e, _ := selectEmbedder(cfg, logger); e != nil {
		atlasEmbed = embeddings.NewCachedEmbedder(e)
	}

	// Refactor Proposer (playbook §8.6). Anchored to the resolved
	// repository root so target util paths land relative to the
	// monorepo. Nil-safe: the DedupGate keeps working without it,
	// just without the "and here's the fix" upgrade.
	refactorRoot := strings.TrimSpace(os.Getenv("IRONFLYER_REPO_ROOT"))
	if refactorRoot == "" {
		if cwd, werr := os.Getwd(); werr == nil {
			refactorRoot = findRepoRoot(cwd, 6)
		}
	}
	refactorSvc := refactor.NewService(refactorRoot)
	logger.Info().Str("root", refactorRoot).Msg("refactor proposer: ready")

	// ---------------- Capability Atlas first-index daemon ----------------
	// V22 closure: the Atlas package existed but was never populated.
	// Boot wires one IndexRepo pass at startup, then re-indexes every
	// IRONFLYER_ATLAS_REINDEX_INTERVAL (default 6h). Opt-out via
	// IRONFLYER_ATLAS_DISABLED=true so dev iteration isn't slowed.
	// The Indexer is the canonical producer; failures log at WARN
	// only — Atlas is a quality-of-life feature, never load-bearing.
	if !envBool(os.Getenv("IRONFLYER_ATLAS_DISABLED")) {
		reindexInterval := 6 * time.Hour
		if v := strings.TrimSpace(os.Getenv("IRONFLYER_ATLAS_REINDEX_INTERVAL")); v != "" {
			if parsed, perr := time.ParseDuration(v); perr == nil && parsed > 0 {
				reindexInterval = parsed
			}
		}
		// Roots: index our actual source — core/ + clients/. Default
		// override via IRONFLYER_ATLAS_ROOT (single dir, legacy) /
		// IRONFLYER_ATLAS_ROOTS (comma-separated).
		repoRoot := strings.TrimSpace(os.Getenv("IRONFLYER_REPO_ROOT"))
		if repoRoot == "" {
			if cwd, werr := os.Getwd(); werr == nil {
				// Climb until we see a directory containing core/ AND
				// clients/, capped at 6 levels so a misconfigured CWD
				// can't walk to /.
				repoRoot = findRepoRoot(cwd, 6)
			}
		}
		var atlasRoots []string
		if v := strings.TrimSpace(os.Getenv("IRONFLYER_ATLAS_ROOTS")); v != "" {
			for _, p := range strings.Split(v, ",") {
				if p = strings.TrimSpace(p); p != "" {
					atlasRoots = append(atlasRoots, p)
				}
			}
		} else if repoRoot != "" {
			atlasRoots = []string{
				filepath.Join(repoRoot, "core"),
				filepath.Join(repoRoot, "clients"),
			}
		}
		atlasIndexer := &atlas.Indexer{
			Store: atlasStore,
			Root:  repoRoot,
			Roots: atlasRoots,
		}
		superviseDaemon(ctx, logger, "atlas-indexer", func(runCtx context.Context) error {
			runIndex := func() {
				started := time.Now()
				stats, err := atlasIndexer.IndexRepo(runCtx)
				if err != nil {
					logger.Warn().Err(err).Dur("elapsed", time.Since(started)).Msg("atlas: index failed (non-fatal)")
					return
				}
				logger.Info().
					Int("total", stats.Total).
					Interface("by_kind", stats.ByKind).
					Int("with_embedding", stats.WithEmbedding).
					Dur("elapsed", time.Since(started)).
					Strs("roots", atlasRoots).
					Msg("atlas indexed")
			}
			runIndex()
			ticker := time.NewTicker(reindexInterval)
			defer ticker.Stop()
			for {
				select {
				case <-runCtx.Done():
					return runCtx.Err()
				case <-ticker.C:
					runIndex()
				}
			}
		})
	} else {
		logger.Info().Msg("atlas: IRONFLYER_ATLAS_DISABLED=true — skipping boot index")
	}

	// ---------------- Agents registry + patches + finisher ----------------
	registry := agents.NewRegistry(guard)
	registry.RegisterDefaults()

	// Context7 live-docs grounding: when enabled (IRONFLYER_CONTEXT7_ENABLED,
	// default true) and a token is present (IRONFLYER_CONTEXT7_TOKEN), give
	// the code-writing agents a built-in `lookup_docs` tool that fetches
	// up-to-date, version-accurate library documentation before they write
	// code against a third-party API. This grounds generation against
	// current APIs instead of training-cutoff memory — the capability fast
	// studios lack. A lookup failure is non-fatal: the agent is told to
	// proceed with its own knowledge and flag uncertain APIs for review.
	if c7 := context7.New(cfg.Context7AuthToken); cfg.Context7Enabled && c7 != nil {
		registry.WithBuiltinTool(providers.ToolSpec{
			Name:        "lookup_docs",
			Description: "Fetch up-to-date, version-accurate documentation and code examples for a library, framework, or API. Call this BEFORE writing code against any third-party dependency to avoid hallucinated or outdated APIs.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"library": map[string]any{
						"type":        "string",
						"description": "Library/framework/API name (e.g. \"next.js\", \"stripe\", \"fastapi\") or a context7 id like \"/vercel/next.js\".",
					},
					"query": map[string]any{
						"type":        "string",
						"description": "What you need to know — the specific function, API, or task.",
					},
				},
				"required": []any{"library", "query"},
			},
		}, func(toolCtx context.Context, _, _ string, args map[string]any) (string, error) {
			library, _ := args["library"].(string)
			query, _ := args["query"].(string)
			docs, err := c7.Lookup(toolCtx, library, query)
			if err != nil {
				return "lookup_docs failed: " + err.Error() + " — proceed using your own knowledge and mark any uncertain API for review.", nil
			}
			return docs, nil
		})
		logger.Info().Msg("Context7 live-docs grounding enabled (lookup_docs tool)")
	}

	patches := patch.NewEngine(projects)
	patches.
		WithAtlas(atlasStore, atlasEmbed, 0).
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
		}).
		WithOnPreflightDecision(func(p patch.Patch, d agents.PreflightDecision) {
			attrs := map[string]any{
				"action":     string(d.Action),
				"query":      d.Query,
				"hitCount":   len(d.AtlasHits),
				"patchId":    p.ID,
				"patchTitle": p.Title,
			}
			outcome := audit.OutcomeSuccess
			if err := d.Validate(); err != nil {
				outcome = audit.OutcomeBlocked
				attrs["error"] = err.Error()
			}
			if len(d.AtlasHits) > 0 {
				top := d.AtlasHits[0]
				attrs["topPath"] = top.Capability.Path
				attrs["topSymbol"] = top.Capability.Symbol
				attrs["topScore"] = top.Score
			}
			if d.TargetPath != "" {
				attrs["targetPath"] = d.TargetPath
			}
			_, _ = auditStore.Record(ctx, audit.Entry{
				Action:    audit.ActionPreflightDecision,
				Outcome:   outcome,
				ProjectID: p.ProjectID,
				Summary: "preflight_decision id=" + p.ID +
					" action=" + string(d.Action),
				Attrs: attrs,
			})
		})
	runtimeClient := runtime.New(cfg.RuntimeURL)

	engine := finisher.NewEngine(projects, registry, patches).
		WithRuntime(runtimeClient).
		WithApplier(runtime.NewApplier(runtimeClient)).
		WithRedis(redisClient).
		WithBus(eventBus).
		WithArchManifest(archManifestPtr(archManifest, archErr)).
		WithRefactor(refactorSvc).
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

	// ---------------- Notifications (email + in-app outbox) ---------------
	prefsStore := notify.NewMemoryPrefsStore()
	emailSender := notify.BuildSenderWithFailover(
		cfg.EmailProvider, cfg.EmailAPIKey, cfg.EmailFromAddress,
		strings.TrimSpace(os.Getenv("IRONFLYER_EMAIL_SECONDARY_PROVIDER")),
		strings.TrimSpace(os.Getenv("IRONFLYER_EMAIL_SECONDARY_API_KEY")),
		envBoolTrue("IRONFLYER_EMAIL_FAILOVER"),
		logger,
	)
	var notifyStore notify.NotificationStore
	var notifyOutbox notify.OutboxStore
	if pgPool != nil {
		notifyStore = notify.NewPostgresNotificationStore(pgPool)
		notifyOutbox = notify.NewPostgresOutboxStore(pgPool)
		logger.Info().Msg("notify: Postgres in-app + outbox stores enabled")
	} else {
		notifyStore = notify.NewMemoryNotificationStore()
		notifyOutbox = notify.NewMemoryOutboxStore()
		logger.Info().Msg("notify: memory in-app + outbox stores enabled")
	}
	notifyHub := notify.NewSubscriptionHub()
	notifyDispatcher := notify.NewDispatcher(emailSender, prefsStore, cfg.DashboardURL, cfg.EmailFromAddress, logger).
		WithOutbox(notifyOutbox)
	notifyEngine := notify.NewEngine(projects, prefsStore, notifyDispatcher, logger)
	notifyEngine.SubscribeAll(ctx, engine)
	notifyWorker := notify.NewWorker(notifyStore, notifyOutbox, emailSender, prefsStore, notifyHub,
		notify.WorkerOpts{From: cfg.EmailFromAddress, DashboardURL: cfg.DashboardURL}, logger)
	go notifyWorker.Run(ctx)
	logger.Info().Str("email_provider", cfg.EmailProvider).Msg("notification pipeline online (email + in-app outbox)")

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
	}
	if cfg.Env == "dev" && cfg.DevWalletSeedUSD > 0 && hasBootstrappedSuperuser {
		seedTenant, err := seedDevSuperuserWallet(ctx, walletSvc, bootstrappedSuperuser, cfg.DevWalletSeedUSD)
		if err != nil {
			logger.Warn().Err(err).
				Str("tenant_id", seedTenant).
				Str("user_id", bootstrappedSuperuser.ID).
				Float64("seed_usd", cfg.DevWalletSeedUSD).
				Msg("dev seed: superuser wallet top-up failed")
		} else {
			logger.Info().
				Str("tenant_id", seedTenant).
				Str("user_id", bootstrappedSuperuser.ID).
				Float64("seed_usd", cfg.DevWalletSeedUSD).
				Msg("dev seed: superuser wallet seeded")
		}
	} else if cfg.Env == "dev" && cfg.DevWalletSeedUSD > 0 {
		logger.Info().
			Float64("seed_usd", cfg.DevWalletSeedUSD).
			Msg("dev seed: no superuser bootstrap configured; skipping shared wallet seed")
	}
	// Wallet topper registry — Stripe + Paddle, primary picked by
	// IRONFLYER_WALLET_PRIMARY_PROVIDER. The user picks ("Card declined?
	// Pay with the alternative checkout") in the UI; we do NOT do
	// server-side auto-failover, which would create double-credit
	// surface area on late webhook redelivery.
	var (
		stripeWalletTopper *wallet.StripeTopper
		paddleWalletTopper *wallet.PaddleTopper
	)
	if cfg.StripeSecretKey != "" {
		stripeWalletTopper = wallet.NewStripeTopper(walletSvc, wallet.StripeTopperOpts{
			SecretKey:     cfg.StripeSecretKey,
			WebhookSecret: cfg.StripeWebhookSecret,
			SuccessURL:    cfg.StripeSuccessURL,
			CancelURL:     cfg.StripeCancelURL,
		})
		logger.Info().Msg("V22 wallet topper: Stripe enabled")
	}
	if cfg.PaddleAPIKey != "" {
		paddleWalletTopper = wallet.NewPaddleTopper(walletSvc, wallet.PaddleTopperOpts{
			APIKey:        cfg.PaddleAPIKey,
			WebhookSecret: cfg.PaddleWebhookSecret,
			Environment:   cfg.PaddleEnv,
			SuccessURL:    cfg.PaddleWalletSuccessURL,
			CancelURL:     cfg.PaddleWalletCancelURL,
		})
		logger.Info().Str("paddle_env", cfg.PaddleEnv).Msg("V22 wallet topper: Paddle enabled")
	}
	// Build registry in primary order. Both Topper.Enabled methods are
	// nil-safe so the typed-nil-into-interface conversion below filters
	// itself out of Active() / Primary() without extra plumbing.
	stripeWalletTopperIfc := wallet.Topper(stripeWalletTopper)
	paddleWalletTopperIfc := wallet.Topper(paddleWalletTopper)
	var walletToppers *wallet.TopperRegistry
	switch cfg.WalletPrimaryProvider {
	case wallet.ProviderPaddle:
		walletToppers = wallet.NewTopperRegistry(paddleWalletTopperIfc, stripeWalletTopperIfc)
	default:
		walletToppers = wallet.NewTopperRegistry(stripeWalletTopperIfc, paddleWalletTopperIfc)
	}
	if !walletToppers.Enabled() {
		logger.Warn().Msg("V22 wallet topper: no provider enabled (set STRIPE_SECRET_KEY or PADDLE_API_KEY)")
	}

	// Wallet reconciler — sweeps pending wallet_topups older than the
	// vendor settlement window, queries the originating provider, and
	// credits / fails the row accordingly. Closes the "missed webhook"
	// gap so a vendor outage during deploy never silently drops a
	// paid top-up.
	if walletToppers.Enabled() {
		reconciler := wallet.NewReconciler(walletSvc, walletToppers, wallet.ReconcilerOpts{
			Threshold: 10 * time.Minute,
			Interval:  5 * time.Minute,
			Logger:    logger,
		})
		go reconciler.Start(ctx)
	}

	// ComplianceGate verticals — premium per-project SKUs (PCI / HIPAA
	// / SOC 2 / GDPR) sold at $199-$499/month and billed against the
	// wallet via the compliance package. Backend is Postgres when
	// wired, in-memory otherwise. Attestation secret is loaded once
	// from env; absence disables the audit-bundle export with a typed
	// NOT_CONFIGURED error rather than minting unsigned attestations.
	var complianceBackend compliance.Backend
	if pgPool != nil {
		complianceBackend = compliance.NewPostgresBackend(pgPool)
		logger.Info().Msg("V22 compliance: Postgres backend")
	} else {
		complianceBackend = compliance.NewMemoryBackend()
		logger.Info().Msg("V22 compliance: in-memory backend")
	}
	complianceSvc := compliance.NewService(
		complianceBackend,
		projects,
		walletSvc,
		os.Getenv("IRONFLYER_ATTESTATION_SECRET"),
		logger.With().Str("component", "compliance").Logger(),
	)
	complianceReconciler := compliance.NewReconciler(complianceSvc, compliance.ReconcilerOpts{})
	go complianceReconciler.Start(ctx)
	if os.Getenv("IRONFLYER_ATTESTATION_SECRET") == "" {
		logger.Warn().Msg("V22 compliance: IRONFLYER_ATTESTATION_SECRET unset — audit bundle export will return NOT_CONFIGURED")
	}

	// ProvisioningVault — Ironflyer-as-issuer revenue rails (Stripe
	// Connect, domain reseller, email partner, hosting). Mirrors the
	// wallet pattern: Service + ConnectorRegistry + Reconciler. Env
	// flag IRONFLYER_PROVISIONING_ENABLED gates registration so a
	// half-credentialed staging env can opt out cleanly.
	var provisioningVault *provisioning.Vault
	// Hoisted out of the gate block so the Finisher Guild payout
	// bridge (below) can reuse the same Stripe Connect connector +
	// service to pay finishers. Both stay nil when provisioning is
	// disabled, which keeps the guild on queued-only payouts.
	var (
		provPayoutSvc     provisioning.Service
		provPayoutConnect *provisioning.StripeConnect
	)
	if strings.EqualFold(strings.TrimSpace(os.Getenv("IRONFLYER_PROVISIONING_ENABLED")), "true") {
		var provSvc provisioning.Service
		if pgPool != nil {
			provSvc = provisioning.NewPostgresService(pgPool)
			logger.Info().Msg("V22 provisioning: Postgres backend")
		} else {
			provSvc = provisioning.NewMemoryService()
			logger.Info().Msg("V22 provisioning: in-memory backend")
		}
		provPayoutSvc = provSvc
		policies := provisioning.NewMemoryPolicyStore()
		registry := provisioning.NewConnectorRegistry()
		// Stripe Connect — Ironflyer-issued payment rail. Application
		// fees are the cut; per-charge cadence; webhook lands events
		// on /provisioning/webhook/stripe with STRIPE_CONNECT_WEBHOOK_SECRET.
		provPayoutConnect = provisioning.NewStripeConnect(provisioning.StripeConnectOpts{
			SecretKey:     os.Getenv("STRIPE_CONNECT_SECRET_KEY"),
			WebhookSecret: os.Getenv("STRIPE_CONNECT_WEBHOOK_SECRET"),
			ReturnURL:     os.Getenv("STRIPE_CONNECT_RETURN_URL"),
			RefreshURL:    os.Getenv("STRIPE_CONNECT_REFRESH_URL"),
			Policies:      policies,
		})
		registry.Register(provPayoutConnect)
		// Domain reseller + email partner ship as connector skeletons
		// until partner-program creds land. Enabled() short-circuits
		// false so they register without exposing themselves to users.
		registry.Register(provisioning.NewDomainReseller(provisioning.DomainResellerOpts{
			APIKey:           os.Getenv("PROVISIONING_DOMAIN_API_KEY"),
			ResellerProvider: os.Getenv("PROVISIONING_DOMAIN_RESELLER"),
			WebhookSecret:    os.Getenv("PROVISIONING_DOMAIN_WEBHOOK_SECRET"),
			Policies:         policies,
		}))
		registry.Register(provisioning.NewEmailPartner(provisioning.EmailPartnerOpts{
			APIKey:        os.Getenv("PROVISIONING_EMAIL_API_KEY"),
			Provider:      os.Getenv("PROVISIONING_EMAIL_PROVIDER"),
			WebhookSecret: os.Getenv("PROVISIONING_EMAIL_WEBHOOK_SECRET"),
			Policies:      policies,
		}))
		provisioningVault = provisioning.NewVault(provSvc, registry, policies)
		if provisioningVault.Enabled() {
			provReconciler := provisioning.NewReconciler(provisioningVault, provisioning.ReconcilerOpts{
				Interval: time.Hour,
				Logger:   logger,
			})
			go provReconciler.Start(ctx)
			logger.Info().Int("connectors", len(registry.Active())).Msg("V22 provisioning: vault enabled")
		} else {
			logger.Warn().Msg("V22 provisioning: enabled but no connector credentials wired")
		}
	}

	// Ship Pass — outcome-based SKU on top of the wallet. The wallet
	// service satisfies wallet.IdempotentService on both backends
	// (memory + postgres), so the cast below is safe in dev and prod;
	// a future backend that only satisfies wallet.Service would land
	// as a nil cast and the resolver would surface NOT_CONFIGURED.
	var (
		shipPassSvc     shippass.Service
		shipPassSettler *shippass.Settler
	)
	if walletIdem, ok := walletSvc.(wallet.IdempotentService); ok {
		if pgPool != nil {
			shipPassSvc = shippass.NewPostgresService(pgPool, walletIdem,
				logger.With().Str("component", "shippass").Logger())
			logger.Info().Msg("V22 shippass: Postgres backend")
		} else {
			shipPassSvc = shippass.NewMemoryService(walletIdem)
			logger.Info().Msg("V22 shippass: in-memory backend")
		}
		shipPassSettler = shippass.NewSettler(shipPassSvc,
			logger.With().Str("component", "shippass.settler").Logger())
		// Periodic deadline sweep: expired active passes flip to
		// refunded and the wallet hold is released back to the user.
		go func() {
			ticker := time.NewTicker(15 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case t := <-ticker.C:
					if _, err := shipPassSvc.ExpireDue(ctx, t); err != nil {
						logger.Warn().Err(err).Msg("V22 shippass: expire sweep failed")
					}
				}
			}
		}()
	} else {
		logger.Warn().Msg("V22 shippass: wallet does not satisfy IdempotentService; disabled")
	}

	// Budget Sentinel — predictive forecast layer + Insured Ship SKU.
	// Adapters (history, completion estimate, project context, spent
	// loader) default to nil-loader stubs so the dashboard renders
	// from boot before deeper integration lands; the Insured Ship SKU
	// is fully live regardless via StaticUnderwriter.
	var sentinelSvc *sentinel.Service
	if walletIdem, ok := walletSvc.(wallet.IdempotentService); ok {
		sentinelPolicy := sentinel.DefaultPolicy()
		sentinelPredictor := sentinel.NewPredictor(sentinelPolicy, nil, nil)
		sentinelSuggester := sentinel.NewSuggestionEngine(nil)
		sentinelInsurance := sentinel.NewMemoryInsurance(sentinelPolicy,
			sentinel.NewStaticUnderwriter(), walletIdem)
		sentinelSvc = sentinel.NewService(sentinelPredictor, sentinelSuggester,
			sentinelInsurance, nil)
		// Hourly expiry sweep: policies past their coverage window
		// flip to expired (no payout) so the active set stays clean.
		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case t := <-ticker.C:
					if _, err := sentinelInsurance.ExpireDue(ctx, t); err != nil {
						logger.Warn().Err(err).Msg("V22 sentinel: insurance expire sweep failed")
					}
				}
			}
		}()
		logger.Info().Msg("V22 sentinel: forecast + Insured Ship SKU wired")
	}

	// Finisher Guild — two-sided marketplace for human finishers +
	// templates. Gated on IRONFLYER_GUILD_ENABLED so a half-credentialed
	// staging env can opt out cleanly. The router subscribes to the
	// learning publisher's gate_outcome stream via AttachRouterObserver
	// once both this bundle and the publisher exist (publisher is
	// constructed downstream; we attach below where it lands).
	var guildBundle *wireup.GuildBundle
	if strings.EqualFold(strings.TrimSpace(os.Getenv("IRONFLYER_GUILD_ENABLED")), "true") {
		guildBundle = wireup.WireGuild(wireup.GuildOpts{
			Pool:         pgPool,
			WalletSvc:    walletSvc,
			ProjectStore: projects,
			Logger:       logger.With().Str("component", "guild").Logger(),
		})
		if guildBundle != nil {
			// Wire the Stripe Connect payout bridge using the bundle's
			// own Service so finisher-profile lookups hit the same
			// store the coordinator writes to. nil bridge (provisioning
			// rail disabled) keeps payouts queued-only.
			if bridge := wireup.NewStripeConnectPayout(guildBundle.Service, provPayoutSvc, provPayoutConnect); bridge != nil {
				guildBundle.Payouts.SetTransferer(bridge)
				logger.Info().Msg("V22 guild: Stripe Connect payout bridge wired")
			}
			go guildBundle.Reconciler.Start(ctx)
			logger.Info().Msg("V22 guild: coordinator + reconciler wired")
		}
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

	// Low-balance watcher: wrap the wallet so every Debit that crosses
	// the configured threshold fires KindLowBalance. Per-user same-day
	// idempotency on the dispatched payload prevents spam.
	walletSvc = notify.NewLowBalanceWalletService(
		walletSvc,
		notifyDispatcher,
		func(ctx context.Context, tenant string) (string, string) {
			if userStore == nil {
				return tenant, ""
			}
			u, err := userStore.GetByID(ctx, tenant)
			if err != nil {
				return tenant, ""
			}
			return u.ID, u.Email
		},
		cfg.WalletLowBalanceThresholdCents,
		"USD",
		logger,
	)
	logger.Info().Int("threshold_cents", cfg.WalletLowBalanceThresholdCents).Msg("V22 wallet: low-balance watcher armed")

	// Weekly digest runner: ships KindWeeklyDigest at Sunday 09:00 UTC
	// for every user with WeeklyDigest=true.
	digestRunner := notify.NewDigestRunner(prefsStore, execSvc, ledgerSvc, notifyDispatcher, logger)
	go digestRunner.Run(ctx)
	logger.Info().Msg("notify: weekly digest runner scheduled (Sunday 09:00 UTC, opt-in)")

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

	// V22 Semantic Repair Matcher (proprietary model #1) — opt-in via
	// IRONFLYER_REPAIR_SEMANTIC=true. Attaches a SemanticIndex to the
	// active Genome so LearningHooks.LookupRecipe falls through to
	// embedding-based similarity when the exact signature misses.
	// Threshold tunable via IRONFLYER_REPAIR_SEMANTIC_THRESHOLD (default
	// 0.82). Warm-start runs async so a cold embedder never blocks boot.
	if semanticRepairEnabled() {
		if attacher, ok := repairGenome.(repair.SemanticAttacher); ok {
			emb, embLabel := selectEmbedder(cfg, logger)
			if emb == nil {
				logger.Warn().Msg("V22 semantic repair: no embedder available; staying on exact-match only")
			} else {
				cached := embeddings.NewCachedEmbedder(emb)
				threshold := semanticRepairThreshold()
				idx := repair.NewSemanticIndex(cached, repairGenome, repair.WithSemanticThreshold(threshold))
				attacher.AttachSemanticIndex(idx)
				go func() {
					reindexCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
					defer cancel()
					if err := idx.ReindexAll(reindexCtx, ""); err != nil {
						logger.Warn().Err(err).Msg("V22 semantic repair: warm-start reindex failed (will warm online)")
						return
					}
					logger.Info().Int("indexed", idx.Size()).Str("embedder", embLabel).Float64("threshold", threshold).Msg("V22 semantic repair: index warmed")
				}()
				logger.Info().Str("embedder", embLabel).Float64("threshold", threshold).Msg("V22 semantic repair: SemanticIndex attached (opt-in)")
			}
		}
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
		// Pre-execution synthetic context (V22 deferred site closed):
		// callers like ideaparser.describeIdea run BEFORE any paid
		// execution exists, so there is no execution row to read. The
		// resolver stamps the ctx with a synthetic id of the form
		// "pre_execution:<tenant>" so attribution stays distinct from
		// "no_execution" and the BillingGuard still gets a Decide.
		// The synthetic state uses a small standalone cost band — the
		// idea-parsing call is bounded by budget.DefaultPromptCap and
		// must not consume an execution's wallet hold.
		if strings.HasPrefix(executionID, "pre_execution:") {
			tenant := strings.TrimPrefix(executionID, "pre_execution:")
			return profitguard.ExecState{
				ExecutionID:              executionID,
				TenantID:                 tenant,
				UserBudgetUSD:            decimal.NewFromFloat(0.10),
				EstimatedNextStepCostUSD: decimal.NewFromFloat(0.01),
				EstimatedPlatformCostUSD: decimal.NewFromFloat(0.01),
				StopLossUSD:              decimal.NewFromFloat(0.10),
				CurrentProvider:          req.PreferredProvider,
			}, nil
		}
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

	// ---------------- V22 Wave-3: Feedback Brain --------------------------
	// Learning publisher + store + miner. The publisher writes
	// OutcomeEvents to the same Postgres outbox the rest of V22 uses;
	// the store reads them back from ClickHouse (when wired) and from
	// an in-process projection (always). The miner runs every hour and
	// converts recent outcomes into PatternObservations that the
	// strategy adapter feeds back into the bandit + blueprint weights.
	learningLog := logger.With().Str("component", "learning").Logger()
	var learnPubOutbox events.Outbox
	if eventOutbox != nil {
		learnPubOutbox = eventOutbox
	}
	learningPublisher := learning.NewPublisher(learnPubOutbox, learningLog)
	learning.SetGlobal(learningPublisher)
	var learningStore learning.Store
	if chRes.Client != nil {
		learningStore = learning.NewClickHouseStore(chRes.Client, learningLog)
		learningLog.Info().Msg("learning store: ClickHouse backend wired")
	} else {
		memStore := learning.NewMemoryStore()
		learningPublisher.SetObserver(memStore.Observe)
		learningStore = memStore
		learningLog.Info().Msg("learning store: memory backend wired (ClickHouse not configured)")
	}
	learningMiner := learning.NewMiner(learningStore, learningPublisher, time.Hour, learningLog)
	superviseDaemon(ctx, logger, "learning-miner", func(runCtx context.Context) error {
		runCtx, span := tracing.StartSpan(runCtx, "learning.miner.daemon")
		defer span.End()
		err := learningMiner.Run(runCtx)
		if err != nil && !errors.Is(err, context.Canceled) {
			span.RecordError(err)
		}
		return err
	})
	learningLog.Info().Msg("Feedback Brain online (publisher + store + miner)")

	// V22 Feedback-Brain → ProfitGuard closure. The PolicyAdapter
	// subscribes to the SAME PatternObservation feed the miner produces
	// (Publisher.SetPatternObserver) and nudges ProfitGuard's
	// completion-per-dollar floor from observed gate-failure rates —
	// tightening when failures are high, loosening back toward the
	// static default when they subside. Adjustments are clamped to a
	// band around the boot default so a bad signal can never zero-out
	// or explode the floor. Entirely opt-in: when the Guard is not a
	// PolicyTuner (or the publisher is nil) this is a no-op.
	if tuner, ok := profitguard.AsPolicyTuner(profitGuard); ok {
		policyAdapter := learning.NewPolicyAdapter(tuner, learning.PolicyAdapterConfig{}, learningLog)
		learningPublisher.SetPatternObserver(policyAdapter.Subscribe())
		learningLog.Info().
			Float64("default_completion_per_dollar_floor", tuner.DefaultCompletionPerDollarFloor()).
			Msg("V22 ProfitGuard policy adapter wired (learns completion-per-dollar floor)")
	} else {
		learningLog.Debug().Msg("V22 ProfitGuard policy adapter skipped (guard not tunable)")
	}

	// V22 Completion Score Predictor (proprietary model #2). Logistic
	// regression over execution features; online SGD on every
	// KindExecutionComplete event. Predicts P(success) BEFORE expensive
	// Opus reasoning runs so the pre-execution gate
	// (IRONFLYER_COMPLETION_GATE) can warn on low-confidence runs.
	//
	// Warm-start from the last 7 days runs async — production boots
	// must not block on a long history fold. The predictor is always
	// constructed (zero-cost when unused) so live executions train it
	// for whoever flips the gate on next.
	completionPredictor := learning.NewCompletionPredictor(learningStore, learningLog)
	go func() {
		if err := completionPredictor.LoadFromHistory(ctx, 7*24*time.Hour); err != nil {
			learningLog.Warn().Err(err).Msg("V22 completion predictor: warm-start failed")
		}
		learningLog.Info().Float64("confidence", completionPredictor.Confidence()).Int("samples", completionPredictor.GlobalSamples()).Msg("V22 completion predictor: online (predictor: confidence=warm)")
	}()
	// Chain the predictor onto the existing publisher observer so each
	// terminal outcome trains the weights without an extra goroutine.
	prevObserver := func(evt learning.OutcomeEvent) {}
	if chRes.Client == nil {
		// Memory store already owns the observer slot; preserve it via a
		// rebind. NewMemoryStore is held inside learningStore as
		// *MemoryStore — re-cast so we can keep observing.
		if mem, ok := learningStore.(*learning.MemoryStore); ok {
			prevObserver = mem.Observe
		}
	}
	learningPublisher.SetObserver(func(evt learning.OutcomeEvent) {
		prevObserver(evt)
		if evt.Kind != learning.KindExecutionComplete || evt.Success == nil {
			return
		}
		_ = completionPredictor.Update(featuresFromEventAttrs(evt), *evt.Success)
	})
	learning.SetGlobal(learningPublisher)

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
	if cfg.IsProd() {
		hardeningCfg.ProdMode = true
	}
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
		// EAS retry-loop ProfitGuard hook (closes the V22 deferred site
		// at eas/client.go:299). The outer BeforeMobileBuild envelope
		// already gates each build; this hook adds a per-attempt cost
		// cap so a single rate-limited or 5xx-burst request cannot
		// burn 3 extra EAS minutes after margin has already collapsed.
		easRetryGuard := func(rctx context.Context, _, _ string, _ int) error {
			execID, ok := profitguardctx.ExecutionID(rctx)
			if !ok || execSvc == nil {
				// Pre-execution or unattributed call — let the outer
				// envelope decide; the retry budget is bounded.
				return nil
			}
			state, err := execSvc.GetState(rctx, execID)
			if err != nil {
				return nil
			}
			in := profitguardbridge.StateToGuardInput(state, nil, profitguardbridge.BridgeFlags{})
			dec, derr := profitGuard.Decide(rctx, profitguard.BeforeRetryLoop, in)
			if derr != nil {
				return nil
			}
			_ = profitGuard.Record(rctx, execID, profitguard.BeforeRetryLoop, dec, in)
			switch dec.Action {
			case profitguard.Stop, profitguard.KillBranch, profitguard.PauseForBudget:
				return fmt.Errorf("%s: %s", dec.Action, dec.Reason)
			}
			return nil
		}
		easClient = eas.New(tok,
			eas.WithLogger(logger.With().Str("component", "eas").Logger()),
			eas.WithRetryGuard(easRetryGuard),
		)
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

	oauthHandler := buildOAuthHandler(cfg, authSvc, sessionStore, notifyDispatcher, logger)
	if oauthHandler != nil {
		logger.Info().Bool("github", cfg.GitHubClientID != "").
			Bool("google", cfg.GoogleClientID != "").
			Msg("oauth: social sign-in enabled")
	}

	api := httpapi.New(httpapi.Deps{
		Projects: projects, Engine: engine, Agents: registry, Patches: patches,
		Billing: billing, Stripe: stripeSvc, Paddle: paddleSvc, Guard: guard,
		Auth: authSvc, AuthOptional: cfg.AuthOptional,
		OAuth:          oauthHandler,
		AllowedOrigins: cfg.CORSOrigins,
		ProdMode:       cfg.IsProd(),
		CSPOverride:    cfg.CSP,
		MetricsToken:   cfg.MetricsToken,
		Memory:         memoryStore, Audit: auditStore, Telemetry: telemetrySink,
		Bus:                       eventBus,
		NotifyPrefs:               prefsStore,
		Notify:                    notifyEngine,
		Notifier:                  notifyDispatcher,
		NotifyStore:               notifyStore,
		NotifyHub:                 notifyHub,
		RuntimeURL:                cfg.RuntimeURL,
		PublicBaseURL:             cfg.PublicBaseURL,
		Version:                   buildVersion,
		Commit:                    buildCommit,
		BuildTime:                 buildTime,
		PrivateInference:          (cfg.HFBaseURL != "" && !strings.Contains(cfg.HFBaseURL, "huggingface.co")) || cfg.HFPrivateModel != "",
		SelfHostedInference:       cfg.HFBaseURL != "" && !strings.Contains(cfg.HFBaseURL, "huggingface.co"),
		PrivateInferenceModel:     cfg.HFPrivateModel,
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
		Wallet:          walletSvc,
		WalletToppers:   walletToppers,
		Compliance:      complianceSvc,
		Provisioning:    provisioningVault,
		ShipPass:        shipPassSvc,
		ShipPassSettler: shipPassSettler,
		Sentinel:        sentinelSvc,
		GuildCoord: func() *guild.Coordinator {
			if guildBundle == nil {
				return nil
			}
			return guildBundle.Coordinator
		}(),

		// Operate console — post-deploy "run the app" surfaces. In-memory
		// store: config surfaces persist for the process lifetime; reflective
		// surfaces derive from the project id.
		AppConsole:       appconsole.NewStore(),
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

		// Feedback Brain — learning surface for dashboards + miner.
		LearningStore:     learningStore,
		LearningPublisher: learningPublisher,
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
	// pgxpool.NewWithConfig is already lazy: idle resources are created
	// in a background goroutine, so cold start does not wait on TCP +
	// TLS + auth. The Ping below is the one blocking roundtrip. When
	// IRONFLYER_PG_LAZY=true we skip the Ping so the first request
	// pays the connect cost instead of blocking boot — useful in
	// preview / autoscale scenarios where the boot SLO is tighter than
	// the first-request SLO.
	if !envBoolTrue("IRONFLYER_PG_LAZY") {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			pool.Close()
			return nil, err
		}
	}
	logger.Info().
		Int32("max_conns", cfg.MaxConns).
		Int32("min_conns", cfg.MinConns).
		Dur("max_conn_lifetime", cfg.MaxConnLifetime).
		Dur("max_conn_idle", cfg.MaxConnIdleTime).
		Dur("health_check_period", cfg.HealthCheckPeriod).
		Bool("lazy", envBoolTrue("IRONFLYER_PG_LAZY")).
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

// envBoolTrue returns true when the env var is set to a truthy value
// (1 / true / on / yes). Used by the cold-start short-circuits.
func envBoolTrue(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
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

func seedDevSuperuserWallet(ctx context.Context, svc wallet.Service, u auth.User, seedUSD float64) (string, error) {
	tenant := canonicalTenantForUser(u)
	if tenant == "" {
		return "", nil
	}
	amount := decimal.NewFromFloat(seedUSD)
	return tenant, svc.TopUp(ctx, tenant, amount, "dev-seed-"+tenant)
}

func canonicalTenantForUser(u auth.User) string {
	if u.OrgID != "" {
		return u.OrgID
	}
	return u.ID
}

// featuresFromEventAttrs lifts an ExecutionFeatures from an
// OutcomeEvent's Attributes bag. Mirrors the private helper inside the
// learning package; kept here so the publisher observer can train the
// predictor without the predictor having to consume Publisher types.
func featuresFromEventAttrs(evt learning.OutcomeEvent) learning.ExecutionFeatures {
	f := learning.ExecutionFeatures{}
	if bp, ok := evt.Attributes["blueprint_id"].(string); ok {
		f.BlueprintID = bp
	}
	f.PromptTokens = intAttr(evt.Attributes, "prompt_tokens")
	f.NumGates = intAttr(evt.Attributes, "num_gates")
	if mobile, ok := evt.Attributes["has_mobile_target"].(bool); ok {
		f.HasMobileTarget = mobile
	}
	f.TenantHistorySuccess = floatAttr(evt.Attributes, "tenant_history_success")
	f.SimilarPastSuccess = floatAttr(evt.Attributes, "similar_past_success")
	if evt.CostUSD != nil {
		v, _ := evt.CostUSD.Float64()
		f.EstimatedCostUSD = v
	}
	return f
}

func intAttr(a map[string]any, key string) int {
	switch v := a[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func floatAttr(a map[string]any, key string) float64 {
	switch v := a[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

// semanticRepairEnabled reads IRONFLYER_REPAIR_SEMANTIC. Truthy strings
// ("1", "true", "yes", "on") enable the V22 Semantic Repair Matcher;
// everything else (default) keeps the exact-match-only behaviour.
func semanticRepairEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("IRONFLYER_REPAIR_SEMANTIC"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// semanticRepairThreshold reads IRONFLYER_REPAIR_SEMANTIC_THRESHOLD.
// Falls back to repair.DefaultSemanticThreshold when unset or invalid.
func semanticRepairThreshold() float64 {
	raw := strings.TrimSpace(os.Getenv("IRONFLYER_REPAIR_SEMANTIC_THRESHOLD"))
	if raw == "" {
		return repair.DefaultSemanticThreshold
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 || v > 1 {
		return repair.DefaultSemanticThreshold
	}
	return v
}

// completionGateEnabled reads IRONFLYER_COMPLETION_GATE. Toggles the
// pre-execution warning surface; predictor itself always runs and folds
// outcomes into its weights so the model keeps improving regardless.
func completionGateEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("IRONFLYER_COMPLETION_GATE"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
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

// buildOAuthHandler assembles the social-login Handler from cfg. The
// state-signing secret + at least one provider client_id must be set
// or it returns nil and main.go skips route registration entirely.
func buildOAuthHandler(cfg config.Config, authSvc *auth.Service, sessions auth.SessionStore, notifier *notify.Dispatcher, logger zerolog.Logger) *oauth.Handler {
	if authSvc == nil || !cfg.HasOAuthProvider() {
		return nil
	}
	if cfg.OAuthStateSecret == "" {
		logger.Warn().Msg("oauth: providers configured but IRONFLYER_OAUTH_STATE_SECRET is unset — refusing to enable")
		return nil
	}
	var provs []oauth.Provider
	postLogin := cfg.GitHubPostLoginURL
	if p := oauth.NewGitHubProvider(cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.GitHubRedirectURL); p != nil {
		if cfg.GitHubClientSecret == "" {
			logger.Warn().Msg("oauth: GITHUB_CLIENT_ID set without GITHUB_CLIENT_SECRET — provider will fail at exchange")
		}
		provs = append(provs, p)
	}
	if p := oauth.NewGoogleProvider(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL); p != nil {
		if cfg.GoogleClientSecret == "" {
			logger.Warn().Msg("oauth: GOOGLE_CLIENT_ID set without GOOGLE_CLIENT_SECRET — provider will fail at exchange")
		}
		provs = append(provs, p)
		if cfg.GoogleClientID != "" && postLogin == "" {
			postLogin = cfg.GooglePostLoginURL
		}
	}
	if postLogin == "" {
		postLogin = cfg.GooglePostLoginURL
	}
	return oauth.New(oauth.Config{
		Providers:           provs,
		Auth:                authSvc,
		Sessions:            sessions,
		StateSecret:         []byte(cfg.OAuthStateSecret),
		Logger:              logger.With().Str("component", "oauth").Logger(),
		DefaultPostLoginURL: postLogin,
		Notifier:            notifier,
	})
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

// envBool parses a permissive truthy string ("1", "true", "yes", "on")
// case-insensitive; anything else (including the empty string) is
// false. Used for opt-in / opt-out env switches that don't need the
// stricter strconv.ParseBool error semantics.
func envBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// archManifestPtr returns a pointer to the parsed manifest when Load
// succeeded, or nil when the caller's load returned an error. We pass
// nil to finisher.Engine.WithArchManifest so the DepGraph /
// ArchBoundary gates degrade to SeverityInfo "manifest not loaded"
// rather than panic on a zero-Layers struct.
func archManifestPtr(m arch.Manifest, err error) *arch.Manifest {
	if err != nil {
		return nil
	}
	out := m
	return &out
}

// findRepoRoot climbs from start up to maxLevels parents looking for
// a directory that contains BOTH core/ and clients/ subdirs — the
// canonical Ironflyer monorepo shape. Returns "" if not found inside
// the cap (caller falls back to indexing nothing rather than walking /).
func findRepoRoot(start string, maxLevels int) string {
	dir := start
	for i := 0; i <= maxLevels; i++ {
		core := filepath.Join(dir, "core")
		clients := filepath.Join(dir, "clients")
		if ci, err := os.Stat(core); err == nil && ci.IsDir() {
			if li, err := os.Stat(clients); err == nil && li.IsDir() {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
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
