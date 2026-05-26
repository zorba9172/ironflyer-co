// Package config holds the runtime service config.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v10"
)

type Config struct {
	Addr         string `env:"IRONFLYER_RUNTIME_ADDR" envDefault:":8090"`
	Driver       string `env:"IRONFLYER_RUNTIME_DRIVER" envDefault:"mock"` // mock|docker
	WorkspaceDir string `env:"IRONFLYER_RUNTIME_WORKSPACE_DIR" envDefault:"/tmp/ironflyer-workspaces"`
	LogLevel     string `env:"IRONFLYER_LOG_LEVEL" envDefault:"info"`
	LogFormat    string `env:"IRONFLYER_LOG_FORMAT" envDefault:"console"`

	// Docker driver: which image to spin up per user. Defaults to our
	// Ironflyer-branded code-server build (see infra/docker/ironflyer-code.Dockerfile);
	// override to `codercom/code-server:latest` for a vanilla container.
	DockerImage string `env:"IRONFLYER_RUNTIME_DOCKER_IMAGE" envDefault:"ghcr.io/zorba9172/ironflyer-code:latest"`

	// CORS origin to allow (Next.js dev).
	CORSOrigin string `env:"IRONFLYER_RUNTIME_CORS_ORIGIN" envDefault:"*"`

	// Shared JWT secret with the orchestrator. Runtime verifies tokens it
	// receives via Bearer or ?token=. Empty => no auth (dev only).
	JWTSecret string `env:"IRONFLYER_JWT_SECRET"`
	JWTIssuer string `env:"IRONFLYER_JWT_ISSUER" envDefault:"ironflyer"`

	// Live preview reverse-proxy. PreviewPrefix is the URL prefix that
	// routes preview traffic — typically "/preview". The proxy strips the
	// `{prefix}/{workspaceID}/{port}` portion before forwarding.
	PreviewPrefix string `env:"IRONFLYER_RUNTIME_PREVIEW_PREFIX" envDefault:"/preview"`

	// AllowedPreviewPorts is the comma-separated list of internal ports
	// the proxy will dial. Wildcard "*" disables the allowlist (dev only).
	// Default covers Vite (5173), Next.js (3000/4000), Astro (4321),
	// generic http (8080), and the common 8000/8888/3001/5174 fallbacks.
	AllowedPreviewPorts string `env:"IRONFLYER_RUNTIME_PREVIEW_ALLOWED_PORTS" envDefault:"3000,3001,4000,4321,5173,5174,8000,8080,8888"`

	// MaxWorkspaces caps the number of concurrently registered workspaces
	// the runtime will accept. Zero means unlimited.
	MaxWorkspaces int `env:"IRONFLYER_RUNTIME_MAX_WORKSPACES" envDefault:"64"`

	// PreviewTokenSecret signs short-lived `?t=...` tokens for iframe
	// preview URLs. Empty means "reuse JWTSecret"; if both are empty the
	// runtime auto-generates an in-memory secret (dev only — tokens won't
	// survive restart).
	PreviewTokenSecret string `env:"IRONFLYER_RUNTIME_PREVIEW_TOKEN_SECRET"`

	// PreviewTokenTTL is the lifetime of a freshly minted preview token.
	PreviewTokenTTL time.Duration `env:"IRONFLYER_RUNTIME_PREVIEW_TOKEN_TTL" envDefault:"30m"`

	// EFSMount is the parent directory bind-mounted into every Docker
	// workspace container. In production this is an EFS-backed
	// PersistentVolume so any runtime pod can serve any workspace.
	EFSMount string `env:"RUNTIME_EFS_MOUNT" envDefault:"/var/lib/ironflyer/workspaces"`

	// IdleArchiveAfter is how long a workspace must sit untouched
	// before the archival scanner ships it to S3.
	IdleArchiveAfter time.Duration `env:"WORKSPACE_IDLE_ARCHIVE_AFTER" envDefault:"30m"`

	// ArchiveConcurrency is the per-pod cap on simultaneous archive
	// (and restore) operations. The S3 SDK does its own per-call
	// multipart parallelism; this knob bounds how many workspaces are
	// in-flight at once.
	ArchiveConcurrency int `env:"WORKSPACE_ARCHIVE_CONCURRENCY" envDefault:"4"`
}

func Load() (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return c, fmt.Errorf("env parse: %w", err)
	}
	return c, nil
}
