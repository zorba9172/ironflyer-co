// Forecast wireup — V22 Wave-3 (Agent 34).
//
// Picks the in-memory or Postgres backend based on whether a pool is
// wired. Both backends satisfy forecast.Forecaster; main.go assigns
// the result onto resolver.Resolver.Forecaster.
package wireup

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/forecast"
)

// BuildForecaster returns a Postgres-backed forecaster when pgPool is
// non-nil and an in-memory one otherwise. The in-memory backend boots
// empty (no seeded samples) — the resolver will return wide bands with
// a low-confidence caveat until the first execution lands.
func BuildForecaster(pgPool *pgxpool.Pool, log zerolog.Logger) forecast.Forecaster {
	cfg := forecast.DefaultConfig()
	if pgPool == nil {
		log.Info().Msg("forecast: memory backend (no pgxpool wired)")
		return forecast.NewMemoryForecaster(cfg)
	}
	log.Info().Msg("forecast: postgres backend")
	return forecast.NewPostgresForecaster(pgPool, cfg)
}
