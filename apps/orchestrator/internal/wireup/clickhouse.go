package wireup

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/clickhouse"
	"ironflyer/apps/orchestrator/internal/dashboards"
	"ironflyer/apps/orchestrator/internal/events"
)

// ClickHouseResult exposes the constructed analytics sources and the
// underlying client/consumer the orchestrator owns lifecycle for.
type ClickHouseResult struct {
	Client    *clickhouse.Client
	Consumer  *clickhouse.Consumer
	Ingester  *clickhouse.OutboxIngester
	Ledger    dashboards.LedgerSource
	Execution dashboards.ExecutionSource
	Blueprint dashboards.BlueprintSource
	Scale     dashboards.ScaleSource
}

// BuildClickHouse opens the ClickHouse client when the config is
// enabled, bootstraps the schema, and starts either a Redpanda
// consumer (preferred) or an outbox ingester fallback. The dashboard
// sources delegate to ClickHouse for analytics.
//
// When cfg.Enabled is false the returned ClickHouseResult is the
// zero value — callers should keep their existing PG-backed adapter
// stack in that case.
func BuildClickHouse(ctx context.Context, cfg clickhouse.Config, pool *pgxpool.Pool, redpandaBrokers string, log zerolog.Logger) (ClickHouseResult, error) {
	if !cfg.Enabled {
		return ClickHouseResult{}, nil
	}
	client, err := clickhouse.NewClient(cfg, log)
	if err != nil {
		return ClickHouseResult{}, err
	}
	if err := client.Ping(ctx); err != nil {
		_ = client.Close()
		return ClickHouseResult{}, err
	}
	if err := client.Bootstrap(ctx); err != nil {
		_ = client.Close()
		return ClickHouseResult{}, err
	}

	res := ClickHouseResult{
		Client:    client,
		Ledger:    clickhouse.NewLedgerSource(client),
		Execution: clickhouse.NewExecutionSource(client),
		Blueprint: clickhouse.NewBlueprintSource(client),
		Scale:     clickhouse.NewScaleSource(client),
	}

	if brokers := strings.TrimSpace(redpandaBrokers); brokers != "" {
		topics := []string{
			events.TopicExecutionLifecycle,
			events.TopicExecutionSteps,
			events.TopicGatesResults,
			events.TopicPatchesLifecycle,
			events.TopicBillingLedger,
			events.TopicProfitGuardDecisions,
			events.TopicDeployLifecycle,
		}
		res.Consumer = clickhouse.NewConsumer(client, strings.Split(brokers, ","), topics, "", log.With().Str("svc", "clickhouse-consumer").Logger())
		log.Info().Msg("clickhouse: Redpanda consumer enabled")
	} else if cfg.IngestFromOutbox && pool != nil {
		res.Ingester = clickhouse.NewOutboxIngester(client, pool, log.With().Str("svc", "clickhouse-ingester").Logger())
		log.Info().Msg("clickhouse: outbox ingester enabled (no Redpanda)")
	}
	return res, nil
}

// ErrClickHouseDisabled is returned by callers that explicitly require
// ClickHouse to be enabled but found the config disabled. Most
// callers should silently fall back to the PG adapters; this sentinel
// exists for the operator-facing paths that need to surface the
// state.
var ErrClickHouseDisabled = errors.New("clickhouse: disabled")
