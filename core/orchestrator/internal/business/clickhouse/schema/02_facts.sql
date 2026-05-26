-- Ironflyer analytics plane — fact tables and materialized views.
--
-- Each raw_* table feeds one or more fact_* tables through a
-- materialized view. Fact tables use ReplacingMergeTree (no version
-- column) so duplicate consumer deliveries collapse on merge — the
-- dedup key is the ORDER BY tuple, and "newest insert wins" is the
-- right policy when the same event is reprojected. event_id is UUID
-- and therefore cannot serve as the explicit version column.
-- TTL is 18 months — the analytical hot window for cost/margin trends.
--
-- Payload field names follow the V22 taxonomy from
-- core/orchestrator/internal/outboxhooks/outboxhooks.go and
-- internal/ledger/postgres.go. Each MV pins the exact event_type
-- strings produced by the outbox constructors so a future event_type
-- rename is a one-place edit.
--
-- All JSON paths use JSON_VALUE because the raw payload is stored as
-- a String column. JSON_VALUE returns '' for missing fields, which
-- the toXxxOrZero helpers tolerate. If a TODO(payload) marker is
-- present, the most-likely path is in place but the producer has not
-- yet been pinned by Schema Registry — re-validate when the registry
-- subject lands.

-- ---------------------------------------------------------------------
-- fact_execution_costs — per-execution cost attribution.
-- Source: billing.ledger.*.v1 from BillingLedgerEvent constructor +
-- ledger.*.v1 from ledger/postgres.go.ledgerEvent (mirror writer).
-- cost_type is derived from the V22 entry_type taxonomy in
-- internal/ledger/ledger.go.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fact_execution_costs
(
    event_id     UUID,
    tenant_id    String,
    execution_id String,
    cost_type    LowCardinality(String),
    amount_usd   Decimal64(6),
    provider     LowCardinality(String),
    occurred_at  DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, execution_id, cost_type, occurred_at)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_fact_execution_costs
TO fact_execution_costs AS
SELECT
    event_id,
    tenant_id,
    COALESCE(toString(JSON_VALUE(payload, '$.execution_id')), '')          AS execution_id,
    CASE JSON_VALUE(payload, '$.entry_type')
        WHEN 'provider_inference_cost'   THEN 'provider'
        WHEN 'sandbox_cost'              THEN 'sandbox'
        WHEN 'storage_cost'              THEN 'storage'
        WHEN 'deployment_cost'           THEN 'deployment'
        WHEN 'premium_reasoning_charge'  THEN 'premium_reasoning'
        ELSE 'other'
    END                                                                    AS cost_type,
    toDecimal64OrZero(JSON_VALUE(payload, '$.amount_usd'), 6)              AS amount_usd,
    -- TODO(payload): provider lives under metadata.provider for billing.ledger.*;
    -- ledger.*.v1 mirror writes it as a top-level field. Try both.
    COALESCE(
        toString(JSON_VALUE(payload, '$.provider')),
        toString(JSON_VALUE(payload, '$.metadata.provider')),
        ''
    )                                                                      AS provider,
    occurred_at
FROM raw_ledger_events
WHERE event_type IN (
    'billing.ledger.provider_inference_cost.v1',
    'billing.ledger.sandbox_cost.v1',
    'billing.ledger.storage_cost.v1',
    'billing.ledger.deployment_cost.v1',
    'billing.ledger.premium_reasoning_charge.v1',
    'ledger.provider_inference_cost.v1',
    'ledger.sandbox_cost.v1',
    'ledger.storage_cost.v1',
    'ledger.deployment_cost.v1',
    'ledger.premium_reasoning_charge.v1'
);

-- ---------------------------------------------------------------------
-- fact_execution_completion — terminal lifecycle event per execution.
-- Source: ExecutionLifecycleEvent constructor — execution.settled.v1
-- is the canonical terminal in lifecycle.go; completed/failed/cancelled
-- variants are kept to absorb older producers and Temporal worker emits.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fact_execution_completion
(
    event_id          UUID,
    tenant_id         String,
    execution_id      String,
    blueprint_id      LowCardinality(String),
    status            LowCardinality(String),
    completion_score  Float64,
    spent_usd         Decimal64(6),
    revenue_usd       Decimal64(6),
    refunded_usd      Decimal64(6),
    duration_sec      Float64,
    queue_wait_sec    Float64,
    occurred_at       DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, execution_id, occurred_at)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_fact_execution_completion
TO fact_execution_completion AS
SELECT
    event_id,
    tenant_id,
    COALESCE(toString(JSON_VALUE(payload, '$.execution_id')), '')          AS execution_id,
    COALESCE(toString(JSON_VALUE(payload, '$.blueprint_id')), '')          AS blueprint_id,
    -- Lifecycle event_type carries the canonical status verb
    -- ("settled", "completed", "failed", "cancelled"). Fall back to
    -- payload.status when an older producer stamps it inline.
    multiIf(
        event_type = 'execution.settled.v1',   'settled',
        event_type = 'execution.completed.v1', 'completed',
        event_type = 'execution.failed.v1',    'failed',
        event_type = 'execution.cancelled.v1', 'cancelled',
        event_type = 'execution.refunded.v1',  'refunded',
        event_type = 'execution.stopped.v1',   'stopped',
        event_type = 'execution.killed.v1',    'killed',
        COALESCE(toString(JSON_VALUE(payload, '$.status')), '')
    )                                                                      AS status,
    toFloat64OrZero(JSON_VALUE(payload, '$.completion_score'))             AS completion_score,
    toDecimal64OrZero(JSON_VALUE(payload, '$.spent_usd'), 6)               AS spent_usd,
    toDecimal64OrZero(JSON_VALUE(payload, '$.revenue_usd'), 6)             AS revenue_usd,
    toDecimal64OrZero(JSON_VALUE(payload, '$.refunded_usd'), 6)            AS refunded_usd,
    toFloat64OrZero(JSON_VALUE(payload, '$.duration_sec'))                 AS duration_sec,
    toFloat64OrZero(JSON_VALUE(payload, '$.queue_wait_sec'))               AS queue_wait_sec,
    occurred_at
FROM raw_execution_events
WHERE event_type IN (
    'execution.settled.v1',
    'execution.completed.v1',
    'execution.failed.v1',
    'execution.cancelled.v1',
    'execution.refunded.v1',
    'execution.stopped.v1',
    'execution.killed.v1'
);

-- ---------------------------------------------------------------------
-- fact_provider_usage — per-provider call cost, latency, tokens.
-- Source: ledger.provider_inference_cost.v1 (and the billing.ledger.*
-- mirror). Provider/model/tokens/latency live under metadata.* in the
-- ledger writer; provider may also be top-level in the V22 mirror.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fact_provider_usage
(
    event_id      UUID,
    tenant_id     String,
    execution_id  String,
    provider      LowCardinality(String),
    model         LowCardinality(String),
    input_tokens  UInt64,
    output_tokens UInt64,
    cost_usd      Decimal64(6),
    latency_ms    UInt64,
    occurred_at   DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, provider, model, occurred_at)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_fact_provider_usage
TO fact_provider_usage AS
SELECT
    event_id,
    tenant_id,
    COALESCE(toString(JSON_VALUE(payload, '$.execution_id')), '')          AS execution_id,
    COALESCE(
        toString(JSON_VALUE(payload, '$.provider')),
        toString(JSON_VALUE(payload, '$.metadata.provider')),
        ''
    )                                                                      AS provider,
    -- TODO(payload): model is provider-specific; ledger metadata uses
    -- "model" today, BillingGuard pumps it through metadata.model.
    COALESCE(
        toString(JSON_VALUE(payload, '$.model')),
        toString(JSON_VALUE(payload, '$.metadata.model')),
        ''
    )                                                                      AS model,
    toUInt64OrZero(
        COALESCE(
            JSON_VALUE(payload, '$.metadata.input_tokens'),
            JSON_VALUE(payload, '$.input_tokens')
        )
    )                                                                      AS input_tokens,
    toUInt64OrZero(
        COALESCE(
            JSON_VALUE(payload, '$.metadata.output_tokens'),
            JSON_VALUE(payload, '$.output_tokens')
        )
    )                                                                      AS output_tokens,
    toDecimal64OrZero(JSON_VALUE(payload, '$.amount_usd'), 6)              AS cost_usd,
    toUInt64OrZero(
        COALESCE(
            JSON_VALUE(payload, '$.metadata.latency_ms'),
            JSON_VALUE(payload, '$.latency_ms')
        )
    )                                                                      AS latency_ms,
    occurred_at
FROM raw_ledger_events
WHERE event_type IN (
    'billing.ledger.provider_inference_cost.v1',
    'ledger.provider_inference_cost.v1'
);

-- ---------------------------------------------------------------------
-- fact_gate_outcomes — gate verdicts per execution/project.
-- Source: GateResultEvent — payload pins project_id, execution_id,
-- gate, verdict, issues. We retain `gate_name` as the canonical column
-- name and source it from payload.gate.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fact_gate_outcomes
(
    event_id      UUID,
    tenant_id     String,
    execution_id  String,
    project_id    String,
    gate_name     LowCardinality(String),
    verdict       LowCardinality(String),
    duration_ms   UInt64,
    occurred_at   DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, gate_name, occurred_at)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_fact_gate_outcomes
TO fact_gate_outcomes AS
SELECT
    event_id,
    tenant_id,
    COALESCE(toString(JSON_VALUE(payload, '$.execution_id')), '')          AS execution_id,
    COALESCE(toString(JSON_VALUE(payload, '$.project_id')), '')            AS project_id,
    COALESCE(
        toString(JSON_VALUE(payload, '$.gate')),
        toString(JSON_VALUE(payload, '$.gate_name')),
        ''
    )                                                                      AS gate_name,
    COALESCE(toString(JSON_VALUE(payload, '$.verdict')), '')               AS verdict,
    -- TODO(payload): GateResultEvent does not emit duration_ms today;
    -- the audit gate.verdict.v1 emitter stamps duration_ms when present.
    toUInt64OrZero(JSON_VALUE(payload, '$.duration_ms'))                   AS duration_ms,
    occurred_at
FROM raw_gate_events
WHERE event_type IN (
    'gate.result.v1',
    'gates.result.v1',
    'gate.verdict.v1'
);

-- ---------------------------------------------------------------------
-- fact_blueprint_runs — per-execution blueprint outcome row.
-- Source: execution.settled.v1 (V22 canonical terminal) and the
-- legacy execution.completed.v1. Blueprint id is stamped by the
-- finisher engine when the execution belongs to a blueprint run.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fact_blueprint_runs
(
    event_id          UUID,
    tenant_id         String,
    execution_id      String,
    blueprint_id      LowCardinality(String),
    status            LowCardinality(String),
    completion_score  Float64,
    revenue_usd       Decimal64(6),
    cost_usd          Decimal64(6),
    repair_count      UInt32,
    preview_success   UInt8,
    refunded          UInt8,
    occurred_at       DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, blueprint_id, execution_id, occurred_at)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_fact_blueprint_runs
TO fact_blueprint_runs AS
SELECT
    event_id,
    tenant_id,
    COALESCE(toString(JSON_VALUE(payload, '$.execution_id')), '')          AS execution_id,
    COALESCE(toString(JSON_VALUE(payload, '$.blueprint_id')), '')          AS blueprint_id,
    multiIf(
        event_type = 'execution.settled.v1',   'settled',
        event_type = 'execution.completed.v1', 'completed',
        event_type = 'execution.failed.v1',    'failed',
        COALESCE(toString(JSON_VALUE(payload, '$.status')), '')
    )                                                                      AS status,
    toFloat64OrZero(JSON_VALUE(payload, '$.completion_score'))             AS completion_score,
    toDecimal64OrZero(JSON_VALUE(payload, '$.revenue_usd'), 6)             AS revenue_usd,
    toDecimal64OrZero(JSON_VALUE(payload, '$.spent_usd'), 6)               AS cost_usd,
    toUInt32OrZero(JSON_VALUE(payload, '$.repair_count'))                  AS repair_count,
    toUInt8OrZero(JSON_VALUE(payload, '$.preview_success'))                AS preview_success,
    toUInt8OrZero(JSON_VALUE(payload, '$.refunded'))                       AS refunded,
    occurred_at
FROM raw_execution_events
WHERE event_type IN (
    'execution.settled.v1',
    'execution.completed.v1'
);

-- ---------------------------------------------------------------------
-- fact_runtime_minutes — sandbox minute consumption.
-- TODO(payload): no outboxhooks constructor exists yet for runtime
-- minute roll-ups. The runtime sweeper emits raw rows directly via the
-- workspace timer; field names below mirror the runtime accounting
-- spec in core/runtime. amount_usd is the cost the sandbox pricer
-- attributes per session.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fact_runtime_minutes
(
    event_id      UUID,
    tenant_id     String,
    execution_id  String,
    workspace_id  String,
    driver        LowCardinality(String),
    minutes       Float64,
    cost_usd      Decimal64(6),
    occurred_at   DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, workspace_id, occurred_at)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_fact_runtime_minutes
TO fact_runtime_minutes AS
SELECT
    event_id,
    tenant_id,
    COALESCE(toString(JSON_VALUE(payload, '$.execution_id')), '')          AS execution_id,
    COALESCE(toString(JSON_VALUE(payload, '$.workspace_id')), '')          AS workspace_id,
    COALESCE(toString(JSON_VALUE(payload, '$.driver')), '')                AS driver,
    toFloat64OrZero(JSON_VALUE(payload, '$.minutes'))                      AS minutes,
    toDecimal64OrZero(JSON_VALUE(payload, '$.amount_usd'), 6)              AS cost_usd,
    occurred_at
FROM raw_runtime_events
WHERE event_type IN (
    'runtime.minutes_consumed.v1',
    'runtime.session_closed.v1'
);

-- ---------------------------------------------------------------------
-- fact_deploys — deploy lifecycle terminal events.
-- Source: deploy lifecycle topic (TopicDeployLifecycle). The audit
-- constants (deploy.smoke_result.v1, deploy.rollback.v1) cover the
-- terminal states; the V22 publisher additionally emits
-- deploy.succeeded.v1 / deploy.failed.v1 from the sweeper.
-- TODO(payload): target field is the deploy target (vercel, fly,
-- internal) — confirm once the deploy publisher is stamped.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fact_deploys
(
    event_id     UUID,
    tenant_id    String,
    execution_id String,
    deploy_id    String,
    project_id   String,
    target       LowCardinality(String),
    status       LowCardinality(String),
    duration_sec Float64,
    cost_usd     Decimal64(6),
    occurred_at  DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, deploy_id, occurred_at)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_fact_deploys
TO fact_deploys AS
SELECT
    event_id,
    tenant_id,
    COALESCE(toString(JSON_VALUE(payload, '$.execution_id')), '')          AS execution_id,
    COALESCE(toString(JSON_VALUE(payload, '$.deploy_id')), '')             AS deploy_id,
    COALESCE(toString(JSON_VALUE(payload, '$.project_id')), '')            AS project_id,
    COALESCE(toString(JSON_VALUE(payload, '$.target')), '')                AS target,
    multiIf(
        event_type = 'deploy.succeeded.v1',  'succeeded',
        event_type = 'deploy.failed.v1',     'failed',
        event_type = 'deploy.rolled_back.v1','rolled_back',
        event_type = 'deploy.rollback.v1',   'rolled_back',
        event_type = 'deploy.smoke_result.v1', COALESCE(toString(JSON_VALUE(payload, '$.status')), 'smoke'),
        COALESCE(toString(JSON_VALUE(payload, '$.status')), '')
    )                                                                      AS status,
    toFloat64OrZero(JSON_VALUE(payload, '$.duration_sec'))                 AS duration_sec,
    toDecimal64OrZero(JSON_VALUE(payload, '$.amount_usd'), 6)              AS cost_usd,
    occurred_at
FROM raw_deploy_events
WHERE event_type IN (
    'deploy.succeeded.v1',
    'deploy.failed.v1',
    'deploy.rolled_back.v1',
    'deploy.rollback.v1',
    'deploy.smoke_result.v1'
);

-- ---------------------------------------------------------------------
-- fact_security_findings — gate-fed security/audit projections.
-- Source: TopicAuditSecurity (audit.security.v1) plus the legacy
-- security.finding.v1. TODO(payload): finding_id / category / rule are
-- the audit envelope keys per docs/ARCHITECTURE_EVENTS.md; severity
-- is one of {info, low, medium, high, critical}.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fact_security_findings
(
    event_id    UUID,
    tenant_id   String,
    project_id  String,
    finding_id  String,
    severity    LowCardinality(String),
    category    LowCardinality(String),
    rule        LowCardinality(String),
    occurred_at DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, severity, occurred_at)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_fact_security_findings
TO fact_security_findings AS
SELECT
    event_id,
    tenant_id,
    COALESCE(toString(JSON_VALUE(payload, '$.project_id')), '')            AS project_id,
    COALESCE(toString(JSON_VALUE(payload, '$.finding_id')), '')            AS finding_id,
    COALESCE(toString(JSON_VALUE(payload, '$.severity')), 'info')          AS severity,
    COALESCE(toString(JSON_VALUE(payload, '$.category')), '')              AS category,
    COALESCE(toString(JSON_VALUE(payload, '$.rule')), '')                  AS rule,
    occurred_at
FROM raw_security_events
WHERE event_type IN (
    'audit.security.v1',
    'security.finding.v1'
);

-- ---------------------------------------------------------------------
-- fact_wallet_topups — Stripe-driven credit purchases.
-- Source: wallet.topup.v1 (wallet/postgres.go.emitWalletEvent) plus
-- the billing.ledger.wallet_topup.v1 mirror. payment_ref lives in
-- metadata.payment_ref (Stripe checkout session id).
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fact_wallet_topups
(
    event_id      UUID,
    tenant_id     String,
    topup_id      String,
    amount_usd    Decimal64(6),
    payment_ref   String,
    occurred_at   DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, occurred_at, topup_id)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_fact_wallet_topups
TO fact_wallet_topups AS
SELECT
    event_id,
    tenant_id,
    COALESCE(
        toString(JSON_VALUE(payload, '$.topup_id')),
        toString(JSON_VALUE(payload, '$.ledger_entry_id')),
        ''
    )                                                                      AS topup_id,
    toDecimal64OrZero(JSON_VALUE(payload, '$.amount_usd'), 6)              AS amount_usd,
    COALESCE(
        toString(JSON_VALUE(payload, '$.payment_ref')),
        toString(JSON_VALUE(payload, '$.metadata.payment_ref')),
        ''
    )                                                                      AS payment_ref,
    occurred_at
FROM raw_ledger_events
WHERE event_type IN (
    'wallet.topup.v1',
    'billing.ledger.wallet_topup.v1',
    'ledger.wallet_topup.v1'
);

-- ---------------------------------------------------------------------
-- fact_refunds — operator-issued or auto-refunds.
-- Source: billing.ledger.refund.v1 + ledger.refund.v1 mirror, plus
-- the execution.refunded.v1 lifecycle marker for tenant-visible refunds.
-- reason is operator-supplied free text; clamp to LowCardinality via
-- the materialized projection.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fact_refunds
(
    event_id      UUID,
    tenant_id     String,
    execution_id  String,
    refund_id     String,
    amount_usd    Decimal64(6),
    reason        LowCardinality(String),
    occurred_at   DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, occurred_at, refund_id)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_fact_refunds
TO fact_refunds AS
SELECT
    event_id,
    tenant_id,
    COALESCE(toString(JSON_VALUE(payload, '$.execution_id')), '')          AS execution_id,
    COALESCE(
        toString(JSON_VALUE(payload, '$.refund_id')),
        toString(JSON_VALUE(payload, '$.ledger_entry_id')),
        ''
    )                                                                      AS refund_id,
    toDecimal64OrZero(JSON_VALUE(payload, '$.amount_usd'), 6)              AS amount_usd,
    COALESCE(
        toString(JSON_VALUE(payload, '$.reason')),
        toString(JSON_VALUE(payload, '$.metadata.reason')),
        ''
    )                                                                      AS reason,
    occurred_at
FROM raw_ledger_events
WHERE event_type IN (
    'billing.ledger.refund.v1',
    'ledger.refund.v1',
    'execution.refunded.v1'
);
