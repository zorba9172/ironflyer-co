// Package config holds the runtime service config.
package config

import (
	"fmt"

	"github.com/caarlos0/env/v10"
)

type Config struct {
	Addr         string `env:"IRONFLYER_RUNTIME_ADDR" envDefault:":8090"`
	Driver       string `env:"IRONFLYER_RUNTIME_DRIVER" envDefault:"mock"` // mock|docker
	WorkspaceDir string `env:"IRONFLYER_RUNTIME_WORKSPACE_DIR" envDefault:"/tmp/ironflyer-workspaces"`
	LogLevel     string `env:"IRONFLYER_LOG_LEVEL" envDefault:"info"`
	LogFormat    string `env:"IRONFLYER_LOG_FORMAT" envDefault:"console"`

	// Docker driver: which image to spin up per user.
	DockerImage string `env:"IRONFLYER_RUNTIME_DOCKER_IMAGE" envDefault:"codercom/code-server:latest"`

	// CORS origin to allow (Next.js dev).
	CORSOrigin string `env:"IRONFLYER_RUNTIME_CORS_ORIGIN" envDefault:"*"`

	// Shared JWT secret with the orchestrator. Runtime verifies tokens it
	// receives via Bearer or ?token=. Empty => no auth (dev only).
	JWTSecret string `env:"IRONFLYER_JWT_SECRET"`
	JWTIssuer string `env:"IRONFLYER_JWT_ISSUER" envDefault:"ironflyer"`
}

func Load() (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return c, fmt.Errorf("env parse: %w", err)
	}
	return c, nil
}
