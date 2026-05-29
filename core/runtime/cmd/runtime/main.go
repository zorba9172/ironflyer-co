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
	"golang.org/x/sync/errgroup"

	"ironflyer/core/runtime/internal/customer/auth"
	"ironflyer/core/runtime/internal/operations/config"
	"ironflyer/core/runtime/internal/operations/httpapi"
	rtmigrate "ironflyer/core/runtime/internal/operations/migrate"
	"ironflyer/core/runtime/internal/operations/sandbox"
	"ironflyer/core/runtime/internal/operations/sentryext"
	"ironflyer/core/runtime/internal/operations/snapshot"
	"ironflyer/core/runtime/internal/operations/snapshots"
	"ironflyer/core/runtime/internal/operations/state"
	"ironflyer/core/runtime/internal/operations/wireup"
	"ironflyer/core/runtime/internal/operations/workspaces"
	migrations "ironflyer/core/runtime/migrations"
)

func main() {
	_ = godotenv.Load(".env", ".env.local")
	cfg, err := config.Load()
	logger := buildLogger(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("config")
	}

	// ---------------- Go runtime tuning -----------------------------------
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_GOMAXPROCS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			goruntime.GOMAXPROCS(n)
			logger.Info().Int("gomaxprocs", n).Msg("GOMAXPROCS overridden via env")
		}
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_GOMEMLIMIT")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			debug.SetMemoryLimit(n)
			logger.Info().Int64("gomemlimit_bytes", n).Msg("GOMEMLIMIT overridden via env")
		}
	}
	logger.Info().Int("gomaxprocs", goruntime.GOMAXPROCS(0)).Int("numcpu", goruntime.NumCPU()).
		Msg("Go runtime: GOMAXPROCS active")

	// ---------------- Parallel init of independent subsystems --------------
	// Cold-start optimization: Sentry, the scale plane (S3 snapshots), and
	// Postgres dial are entirely independent. Booting them in parallel via
	// errgroup turns the cold-start wait from SUM(t_each) into MAX(t_each).
	// The downstream wiring (driver, archiver, allocator) still waits for
	// all legs to finish because the docker driver and HTTP API depend on
	// the scale plane result.
	bootStart := time.Now()
	bgCtxScale, bgCancelScale := context.WithCancel(context.Background())
	defer bgCancelScale()
	var (
		sentryFlush func()
		scaleRes    wireup.ScaleResult
		scaleErr    error
	)
	bootGroup, bootCtx := errgroup.WithContext(context.Background())

	bootGroup.Go(func() error {
		start := time.Now()
		dsn := strings.TrimSpace(os.Getenv("RUNTIME_SENTRY_DSN"))
		if dsn == "" {
			dsn = os.Getenv("SENTRY_DSN")
		}
		sentryEnv := strings.TrimSpace(os.Getenv("IRONFLYER_ENV"))
		if sentryEnv == "" {
			sentryEnv = "development"
		}
		flush, ierr := sentryext.Init(sentryext.Opts{
			DSN:              dsn,
			Environment:      sentryEnv,
			Release:          strings.TrimSpace(os.Getenv("IRONFLYER_VERSION")),
			TracesSampleRate: sentryext.FloatFromEnv("SENTRY_TRACES_SAMPLE", 0.05),
			ServerName:       "ironflyer-runtime",
		})
		if ierr != nil {
			logger.Warn().Err(ierr).Msg("sentry init failed; continuing without exception reporting")
		} else if dsn != "" {
			logger.Info().Str("env", sentryEnv).Dur("took", time.Since(start)).Msg("Sentry initialised")
		}
		sentryFlush = flush
		return nil
	})

	bootGroup.Go(func() error {
		start := time.Now()
		// V22 Wave-2 runtime scale plane: snapshots.S3Manager / quota /
		// warmpool / allocator / runtimeclass. The docker driver below
		// dials into the SnapshotShim adapter for cross-pod restore.
		scaleRes, scaleErr = wireup.BuildScale(bgCtxScale, snapshots.Config{
			Bucket:    strings.TrimSpace(os.Getenv("WORKSPACE_BUCKET")),
			Region:    strings.TrimSpace(os.Getenv("AWS_REGION")),
			Endpoint:  strings.TrimSpace(os.Getenv("WORKSPACE_S3_ENDPOINT")),
			Prefix:    "snapshots",
			Retention: 5,
			KMSKeyID:  strings.TrimSpace(os.Getenv("WORKSPACE_KMS_KEY_ID")),
			Excludes:  snapshots.LoadExcludesFromEnv(),
		}, logger)
		logger.Info().Dur("took", time.Since(start)).Msg("scale plane init complete")
		return nil
	})
	_ = bootCtx
	if err := bootGroup.Wait(); err != nil {
		logger.Fatal().Err(err).Msg("runtime parallel init failed")
	}
	logger.Info().Dur("took", time.Since(bootStart)).Msg("runtime parallel init complete")
	defer func() {
		if sentryFlush != nil {
			sentryFlush()
		}
	}()
	if scaleErr != nil {
		logger.Warn().Err(scaleErr).Msg("runtime scale plane disabled (boot error)")
	}
	if scaleRes.Drainer != nil {
		superviseDaemon(bgCtxScale, logger, "warmpool-drainer", func(runCtx context.Context) error {
			scaleRes.Drainer.Run(runCtx)
			return nil
		})
		logger.Info().Msg("warm pool drainer started")
	}

	// ---------------- Driver --------------------------------------------
	var driver sandbox.Driver
	switch cfg.Driver {
	case "docker":
		dd := sandbox.NewDockerDriver(cfg.DockerImage).
			WithEFS(cfg.EFSMount, cfg.WorkspaceDir).
			// Web IDE image + container port. The canonical IDE is the
			// branded Eclipse Theia app — select it with IRONFLYER_IDE_IMAGE=
			// ironflyer/theia-ide:latest + IRONFLYER_IDE_CONTAINER_PORT=3030.
			// Empty IDEImage falls through to the registry-pullable
			// code-server fallback (8080) so an unconfigured runtime still
			// boots a working IDE; behavior is unchanged unless these env
			// vars are set.
			WithIDEImage(cfg.IDEImage).
			WithContainerPort(cfg.IDEContainerPort)
		if scaleRes.Snapshots != nil {
			dd = dd.WithSnapshotShim(wireup.SnapshotShimAdapter{Mgr: scaleRes.Snapshots})
			logger.Info().Msg("docker driver: snapshot shim wired")
		}
		driver = dd
		logger.Info().Str("image", dd.Image).Int("idePort", dd.ContainerPort).
			Str("efs", cfg.EFSMount).Msg("docker driver enabled")
	default:
		driver = sandbox.NewMockDriver(cfg.WorkspaceDir)
		logger.Info().Str("dir", cfg.WorkspaceDir).Msg("mock driver enabled")
	}
	mgr := sandbox.NewManager(driver)

	// ---------------- Postgres (workspace registry) ---------------------
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	var (
		store    workspaces.Store = workspaces.NewMemoryStore()
		pool     *pgxpool.Pool
	)
	if pgURL := strings.TrimSpace(os.Getenv("POSTGRES_URL")); pgURL != "" {
		ctx, cancel := context.WithTimeout(bgCtx, 10*time.Second)
		p, perr := pgxpool.New(ctx, pgURL)
		cancel()
		if perr != nil {
			logger.Warn().Err(perr).Msg("postgres connect failed; falling back to memory workspace store")
		} else {
			pool = p
			defer pool.Close()
			migCtx, mcancel := context.WithTimeout(bgCtx, 30*time.Second)
			if merr := rtmigrate.RunPool(migCtx, pool, migrations.FS); merr != nil {
				logger.Warn().Err(merr).Msg("workspace migrations failed; falling back to memory store")
			} else {
				store = workspaces.NewPostgresStore(pool)
				logger.Info().Msg("postgres workspace store enabled")
			}
			mcancel()
		}
	} else {
		logger.Warn().Msg("POSTGRES_URL empty — using in-memory workspace store (single-pod only)")
	}

	// ---------------- Redis pod registry --------------------------------
	registry, err := workspaces.New(workspaces.Config{
		RedisURL:          strings.TrimSpace(os.Getenv("RUNTIME_REDIS_URL")),
		PodIP:             strings.TrimSpace(os.Getenv("RUNTIME_POD_IP")),
		KeyTTL:            90 * time.Second,
		HeartbeatInterval: 30 * time.Second,
	})
	if err != nil {
		logger.Warn().Err(err).Msg("redis registry init failed; running single-pod")
		registry, _ = workspaces.New(workspaces.Config{})
	}
	if pingErr := registry.Ping(bgCtx); pingErr != nil {
		logger.Warn().Err(pingErr).Msg("redis ping failed")
	}
	registry.Heartbeat(bgCtx)

	// ---------------- S3 archiver + idle scanner ------------------------
	archiver, err := workspaces.NewArchiver(bgCtx, workspaces.ArchiverConfig{
		Bucket:      strings.TrimSpace(os.Getenv("WORKSPACE_S3_BUCKET")),
		Region:      strings.TrimSpace(os.Getenv("WORKSPACE_S3_REGION")),
		EFSRoot:     cfg.EFSMount,
		Concurrency: cfg.ArchiveConcurrency,
		Store:       store,
	}, logger)
	if err != nil {
		logger.Warn().Err(err).Msg("archiver init failed; archives disabled")
		archiver, _ = workspaces.NewArchiver(bgCtx, workspaces.ArchiverConfig{EFSRoot: cfg.EFSMount, Store: store}, logger)
	}
	idleScanner := workspaces.NewScanner(store, archiver, cfg.IdleArchiveAfter, 2*time.Minute, logger)
	superviseDaemon(bgCtx, logger, "idle-scanner", func(runCtx context.Context) error {
		idleScanner.Run(runCtx)
		return nil
	})

	// ---------------- Portability (state + snapshot) --------------------
	// The portable workspace stack is additive: the legacy single-pod
	// workspaces.* registry above keeps working, and the new state +
	// snapshot layer powers cross-pod handoff. When POD_NAME isn't set
	// (dev) the heartbeat/reaper goroutines short-circuit so we never
	// reclaim our own dev workspaces.
	var portState state.Store = state.NewMemoryStore()
	if pool != nil {
		bsCtx, bsCancel := context.WithTimeout(bgCtx, 10*time.Second)
		if err := state.BootstrapPostgres(bsCtx, pool); err != nil {
			logger.Warn().Err(err).Msg("portability: bootstrap postgres failed; using memory state store")
		} else {
			portState = state.NewPostgresStore(pool)
			logger.Info().Msg("portability: postgres state store enabled")
		}
		bsCancel()
	}
	snapMgr, err := snapshot.New(bgCtx, snapshot.Config{
		Bucket:    strings.TrimSpace(os.Getenv("WORKSPACE_BUCKET")),
		Region:    strings.TrimSpace(os.Getenv("AWS_REGION")),
		Prefix:    "workspaces",
		Retention: 5,
		KMSKeyID:  strings.TrimSpace(os.Getenv("WORKSPACE_KMS_KEY_ID")),
	}, logger)
	if err != nil {
		logger.Warn().Err(err).Msg("portability: snapshot manager init failed; portability snapshots disabled")
		snapMgr, _ = snapshot.New(bgCtx, snapshot.Config{}, logger)
	}
	if snapMgr.Enabled() {
		logger.Info().Str("bucket", os.Getenv("WORKSPACE_BUCKET")).Msg("portability: S3 snapshots enabled")
	}
	podID := strings.TrimSpace(os.Getenv("POD_NAME"))
	if podID == "" {
		podID = strings.TrimSpace(os.Getenv("HOSTNAME"))
	}
	workingDir := strings.TrimSpace(os.Getenv("WORKSPACE_LIVE_DIR"))
	if workingDir == "" {
		workingDir = "/var/lib/ironflyer/live"
	}

	var verifier *auth.Verifier
	if cfg.JWTSecret != "" {
		verifier = auth.NewVerifier([]byte(cfg.JWTSecret), cfg.JWTIssuer)
		logger.Info().Str("issuer", cfg.JWTIssuer).Msg("runtime JWT auth enabled")
	} else {
		logger.Warn().Msg("runtime auth disabled (IRONFLYER_JWT_SECRET empty)")
	}

	previewSecret := []byte(cfg.PreviewTokenSecret)
	if len(previewSecret) == 0 && cfg.JWTSecret != "" {
		previewSecret = []byte(cfg.JWTSecret)
	}

	portability := httpapi.Portability{
		State:          portState,
		Snapshots:      snapMgr,
		PodID:          podID,
		WorkingDir:     workingDir,
		StaleAfter:     60 * time.Second,
		HeartbeatEvery: 15 * time.Second,
	}
	handler := httpapi.New(mgr, httpapi.Options{
		CORSOrigin:      cfg.CORSOrigin,
		Verifier:        verifier,
		PreviewPrefix:   cfg.PreviewPrefix,
		AllowedPorts:    cfg.AllowedPreviewPorts,
		PreviewSecret:   previewSecret,
		PreviewTokenTTL: cfg.PreviewTokenTTL,
		MaxWorkspaces:   cfg.MaxWorkspaces,
		Lifecycle: httpapi.Lifecycle{
			Store:    store,
			Registry: registry,
			Archiver: archiver,
		},
		Portability: portability,
		// V22 Wave-2: every workspace create runs through the
		// allocator's admission funnel; the quota enforcer is also the
		// data source for the GET /quota/usage dashboard endpoint.
		Allocator:     scaleRes.Allocator,
		QuotaEnforcer: scaleRes.Quota,
	}, logger)
	httpapi.StartPortabilityWorkers(bgCtx, portability, logger)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		// PTY WebSockets + workspace upload streams are long-lived, so
		// IdleTimeout reclaims wedged keep-alives but WriteTimeout=0
		// lets streaming endpoints set per-request deadlines.
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error().Interface("panic", r).Bytes("stack", debug.Stack()).
					Msg("http listener panic recovered")
				sentryext.CaptureRecovered(bgCtx, r)
			}
		}()
		logger.Info().Str("addr", cfg.Addr).Msg("workspace runtime listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	sig := <-stop
	signal.Stop(stop)
	logger.Info().Str("signal", sig.String()).Msg("SIGTERM received — releasing owned workspaces")
	// Cancel the long-lived ctx so daemons (drainer, idle scanner,
	// portability workers) exit cleanly before we close the HTTP server.
	bgCancel()
	bgCancelScale()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Release Redis claims so the next request lands on whichever pod
	// the LB picks. PTY clients have already received their
	// `pty.shutdown_imminent` frame at this point.
	for _, id := range registry.OwnedIDs() {
		registry.Release(shutdownCtx, id)
	}
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn().Err(err).Msg("http server shutdown")
	}
	_ = registry.Close()
	logger.Info().Msg("shutdown complete")
}

// superviseDaemon launches fn in a goroutine, recovers any panic so a
// single daemon crash never tears the runtime down, reports the panic
// through Sentry + zerolog tagged by daemon name, and exits when ctx
// is cancelled. Used by the warm-pool drainer, idle scanner, and the
// portability workers.
func superviseDaemon(ctx context.Context, logger zerolog.Logger, name string, fn func(context.Context) error) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error().
					Str("daemon", name).
					Interface("panic", r).
					Bytes("stack", debug.Stack()).
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
	lvl, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	if cfg.LogFormat == "console" {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
			With().Timestamp().Str("svc", "runtime").Logger()
	}
	return zerolog.New(os.Stderr).With().Timestamp().Str("svc", "runtime").Logger()
}
