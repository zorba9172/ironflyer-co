package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"

	"ironflyer/apps/runtime/internal/auth"
	"ironflyer/apps/runtime/internal/config"
	"ironflyer/apps/runtime/internal/httpapi"
	"ironflyer/apps/runtime/internal/sandbox"
)

func main() {
	_ = godotenv.Load(".env", ".env.local")
	cfg, err := config.Load()
	logger := buildLogger(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("config")
	}

	var driver sandbox.Driver
	switch cfg.Driver {
	case "docker":
		driver = sandbox.NewDockerDriver(cfg.DockerImage)
		logger.Info().Str("image", cfg.DockerImage).Msg("docker driver enabled")
	default:
		driver = sandbox.NewMockDriver(cfg.WorkspaceDir)
		logger.Info().Str("dir", cfg.WorkspaceDir).Msg("mock driver enabled")
	}
	mgr := sandbox.NewManager(driver)
	var verifier *auth.Verifier
	if cfg.JWTSecret != "" {
		verifier = auth.NewVerifier([]byte(cfg.JWTSecret), cfg.JWTIssuer)
		logger.Info().Str("issuer", cfg.JWTIssuer).Msg("runtime JWT auth enabled")
	} else {
		logger.Warn().Msg("runtime auth disabled (IRONFLYER_JWT_SECRET empty)")
	}

	previewSecret := []byte(cfg.PreviewTokenSecret)
	if len(previewSecret) == 0 && cfg.JWTSecret != "" {
		// Reuse the JWT secret so preview tokens survive restarts in any
		// deployment that has JWT configured. Still distinct domain via
		// the token payload, but the key material can safely be shared
		// because the signing protocols don't overlap.
		previewSecret = []byte(cfg.JWTSecret)
	}
	server := &http.Server{
		Addr: cfg.Addr,
		Handler: httpapi.New(mgr, httpapi.Options{
			CORSOrigin:      cfg.CORSOrigin,
			Verifier:        verifier,
			PreviewPrefix:   cfg.PreviewPrefix,
			AllowedPorts:    cfg.AllowedPreviewPorts,
			PreviewSecret:   previewSecret,
			PreviewTokenTTL: cfg.PreviewTokenTTL,
			MaxWorkspaces:   cfg.MaxWorkspaces,
		}, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info().Str("addr", cfg.Addr).Msg("workspace runtime listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
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
