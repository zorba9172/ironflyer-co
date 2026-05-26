// scrcpy-bridge is a standalone Go service that wraps scrcpy + Pion
// WebRTC so the Ironflyer frontend can stream a live Android emulator
// into the cockpit. The orchestrator and the runtime authenticate via
// a shared bridge token; browsers attach to the signaling WebSocket
// after the runtime hands them the session URL.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/apps/scrcpy-bridge/internal/buildinfo"
	"ironflyer/apps/scrcpy-bridge/internal/server"
	"ironflyer/apps/scrcpy-bridge/internal/session"
)

func main() {
	logger := zerolog.New(os.Stdout).
		With().
		Timestamp().
		Str("service", buildinfo.Component).
		Str("version", buildinfo.Version).
		Logger()

	cfg := loadConfig(logger)

	manager := session.NewManager(cfg.ScrcpyPath, cfg.AdbServer, logger)
	srv := server.New(cfg, manager)

	addr := ":" + strconv.Itoa(cfg.Port)
	httpSrv := srv.HTTPServer(addr)

	go func() {
		logger.Info().Str("addr", addr).Str("scrcpy_path", cfg.ScrcpyPath).
			Str("adb_server", cfg.AdbServer).Msg("scrcpy-bridge listening")
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal().Err(err).Msg("http server crashed")
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info().Str("signal", sig.String()).Msg("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Warn().Err(err).Msg("http shutdown error")
	}
	manager.Shutdown()
	logger.Info().Msg("scrcpy-bridge stopped")
}

func loadConfig(logger zerolog.Logger) server.Config {
	port := envInt("BRIDGE_PORT", 9100)
	adbServer := envOr("ADB_SERVER", "localhost:5037")
	scrcpy := envOr("SCRCPY_PATH", "")
	if scrcpy == "" {
		if p, err := exec.LookPath("scrcpy"); err == nil {
			scrcpy = p
		} else {
			// Don't fatal — /healthz still answers and a deployer
			// may mount scrcpy via the Dockerfile.
			scrcpy = "scrcpy"
			logger.Warn().Msg("scrcpy not found on PATH; relying on SCRCPY_PATH or runtime PATH")
		}
	}
	token := strings.TrimSpace(os.Getenv("BRIDGE_SHARED_TOKEN"))
	if token == "" {
		logger.Warn().Msg("BRIDGE_SHARED_TOKEN unset — /v1 routes will return 503 until set")
	}
	return server.Config{
		Port:        port,
		ScrcpyPath:  scrcpy,
		AdbServer:   adbServer,
		SharedToken: token,
		Logger:      logger,
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
