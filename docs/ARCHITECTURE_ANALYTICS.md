# Analytics Plane — ClickHouse

ClickHouse is Ironflyer's real-time business and operations analytics
store. It answers the questions that must never burden production
Postgres:

- Are paid executions profitable?
- Which providers are burning margin?
- Which blueprints reduce cost?
- Which gates block customers?
- Which tenants are growing, stuck, or abusive?
- Which worker pools need to scale?

ClickHouse is not the source of truth. It is a replayable projection fed
from Redpanda.

## Ownership Boundary

| System | Owns |
| --- | --- |
| Postgres | transactional truth: wallets, ledger, executions, tenants |
| Redpanda | ordered event transport and replay |
| ClickHouse | analytical projections, rollups, dashboards |
| Redis | ephemeral counters and short-lived coordination |
| SurrealDB | AI context graph and retrieval |

If a dashboard can tolerate replay/rebuild semantics, it belongs in
ClickHouse. If a customer balance or execution state depends on it, it
belongs in Postgres.

## Ingestion Model

```text
Postgres transaction
  writes business row
  writes event_outbox row
      |
Outbox publisher
      |
Redpanda topic
      |
ClickHouse Kafka engine / sink connector
      |
Raw event table
      |
Materialized views
      |
Dashboard tables
```

Consumers must be idempotent. Every event carries:

- `event_id`
- `event_type`
- `event_version`
- `tenant_id`
- `execution_id` when applicable
- `occurred_at`
- `producer`
- `trace_id`
- `idempotency_key`

## Raw Tables

Start with append-only raw tables:

- `raw_execution_events`
- `raw_ledger_events`
- `raw_agent_events`
- `raw_gate_events`
- `raw_runtime_events`
- `raw_deploy_events`
- `raw_security_events`

Each table stores the parsed envelope plus `payload JSON`.

## Fact Tables

Materialized views project raw events into query-optimized fact tables:

- `fact_execution_costs`
- `fact_execution_completion`
- `fact_provider_usage`
- `fact_gate_outcomes`
- `fact_blueprint_runs`
- `fact_runtime_minutes`
- `fact_deploys`
- `fact_security_findings`
- `fact_wallet_topups`
- `fact_refunds`

Fact tables use `ReplacingMergeTree` keyed by `event_id` or
`idempotency_key` so duplicate delivery does not double-count revenue or
cost.

## Dashboard Rollups

The commercial cockpit reads rollups, not raw events:

- `rollup_profit_hourly`
- `rollup_profit_daily`
- `rollup_provider_daily`
- `rollup_blueprint_daily`
- `rollup_gate_daily`
- `rollup_cohort_monthly`
- `rollup_runtime_capacity_5m`
- `rollup_abuse_tenant_daily`

The first production dashboard must show:

- revenue
- provider cost
- sandbox cost
- deployment/storage cost
- gross margin
- profitable completed execution rate
- cost to first preview
- completion per dollar
- stuck execution count
- top margin leaks by provider/blueprint/gate

## Late And Duplicate Events

Late events are normal. Duplicates are normal. Incorrect margin is not.

Rules:

- Redpanda offsets are not business idempotency.
- `event_id` is globally unique.
- `idempotency_key` is stable for economic effects.
- ledger-derived rollups only count `margin_relevant=true` entries.
- dashboard rollups recompute a sliding correction window.
- raw event retention must exceed the largest expected replay window.

For daily financial rollups, keep a 14-day correction window. For
cohort rollups, recompute the active month.

## Retention

Recommended defaults:

- raw events: 90 days hot
- fact tables: 18 months
- daily rollups: 7 years
- security/audit analytical projections: 7 years

The legal audit chain remains in Postgres/object storage. ClickHouse is
for fast inspection and trend analysis.

## What Does Not Go Into ClickHouse

- wallet balance truth
- authorization decisions
- active execution FSM mutation
- secrets
- source code bodies
- full patch payloads unless explicitly redacted
- customer production data

Store references, hashes, sizes, costs, verdicts, and timings instead.

## Scale Signals

ClickHouse feeds the operational control loop:

- Redpanda consumer lag
- execution queue wait
- worker utilization
- sandbox cold-start time
- provider latency and error rate
- cost spike alerts
- margin floor breaches

KEDA scales workers from Redpanda lag. Operators inspect the result in
ClickHouse.

## Implementation Order

1. Add Postgres outbox schema.
2. Add Redpanda publisher.
3. Add raw ClickHouse ingestion for ledger and execution events.
4. Build `rollup_profit_hourly`.
5. Move dashboard resolvers from Postgres aggregation to ClickHouse.
6. Add provider/gate/blueprint rollups.
7. Add correction-window jobs and replay runbook.

## Current state (2026-05-26 audit)

- The canonical cost fact table is `fact_execution_costs`, not the
  legacy `profit_projection` name that some smoke steps once
  referenced. Schema files live under
  `core/orchestrator/internal/business/clickhouse/schema/`
  (01_raw / 02_facts / 03_rollups / 04_correction).
- The Redpanda → ClickHouse consumer subscribes to every domain stream
  that `tableForTopic()` maps to a `raw_*` table:
  `execution.lifecycle`, `execution.steps`, `gates.results`,
  `patches.lifecycle`, `billing.ledger`, `profitguard.decisions`,
  `deploy.lifecycle`, `audit.security`, `memory.indexing`,
  `runtime.lifecycle`. See `internal/operations/wireup/clickhouse.go`.
- The consumer uses one precompiled `INSERT INTO raw_<domain>_events`
  per table (no fmt.Sprintf) and the client enables server-side async
  insert (`async_insert=1, wait_for_async_insert=0,
  async_insert_busy_timeout_ms=1000`) so high-throughput writes batch
  on the ClickHouse side without per-row round-trip overhead. See
  `internal/business/clickhouse/{client.go, consumer.go}`.
- Dashboard reads are wired through the ClickHouse adapters
  (`LedgerSource`, `ExecutionSource`, `BlueprintSource`, `ScaleSource`
  in `internal/business/clickhouse/adapters.go`). The Postgres adapter
  stack remains as the fallback when `IRONFLYER_CLICKHOUSE_HOSTS` is
  empty.
- Honest gap: no `dim_tenant` / `dim_provider` tables exist yet. All
  joins use the raw string columns (`tenant_id`, `provider`) which is
  acceptable at current row counts but should evolve into proper dim
  tables when per-tenant attributes (plan, region) need to surface on
  the operator dashboard.
