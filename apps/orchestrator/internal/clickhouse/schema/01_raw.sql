-- Ironflyer analytics plane — raw event tables.
--
-- These tables receive the parsed event envelope plus a raw JSON
-- payload string for every domain event flowing through Redpanda.
-- ReplacingMergeTree(event_id) makes duplicate delivery a no-op so
-- consumers stay strictly at-least-once with no double-counting.
-- Partition by month and TTL at 90 days — long-term truth lives in
-- Postgres and S3, ClickHouse is the hot analytics window.

CREATE TABLE IF NOT EXISTS raw_execution_events
(
    event_id        UUID,
    event_type      LowCardinality(String),
    event_version   UInt32,
    tenant_id       String,
    execution_id    Nullable(String),
    occurred_at     DateTime64(3, 'UTC'),
    producer        LowCardinality(String),
    trace_id        Nullable(String),
    idempotency_key String,
    payload         String
)
ENGINE = ReplacingMergeTree(event_id)
ORDER BY (tenant_id, occurred_at, event_id)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS raw_ledger_events
(
    event_id        UUID,
    event_type      LowCardinality(String),
    event_version   UInt32,
    tenant_id       String,
    execution_id    Nullable(String),
    occurred_at     DateTime64(3, 'UTC'),
    producer        LowCardinality(String),
    trace_id        Nullable(String),
    idempotency_key String,
    payload         String
)
ENGINE = ReplacingMergeTree(event_id)
ORDER BY (tenant_id, occurred_at, event_id)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS raw_agent_events
(
    event_id        UUID,
    event_type      LowCardinality(String),
    event_version   UInt32,
    tenant_id       String,
    execution_id    Nullable(String),
    occurred_at     DateTime64(3, 'UTC'),
    producer        LowCardinality(String),
    trace_id        Nullable(String),
    idempotency_key String,
    payload         String
)
ENGINE = ReplacingMergeTree(event_id)
ORDER BY (tenant_id, occurred_at, event_id)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS raw_gate_events
(
    event_id        UUID,
    event_type      LowCardinality(String),
    event_version   UInt32,
    tenant_id       String,
    execution_id    Nullable(String),
    occurred_at     DateTime64(3, 'UTC'),
    producer        LowCardinality(String),
    trace_id        Nullable(String),
    idempotency_key String,
    payload         String
)
ENGINE = ReplacingMergeTree(event_id)
ORDER BY (tenant_id, occurred_at, event_id)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS raw_runtime_events
(
    event_id        UUID,
    event_type      LowCardinality(String),
    event_version   UInt32,
    tenant_id       String,
    execution_id    Nullable(String),
    occurred_at     DateTime64(3, 'UTC'),
    producer        LowCardinality(String),
    trace_id        Nullable(String),
    idempotency_key String,
    payload         String
)
ENGINE = ReplacingMergeTree(event_id)
ORDER BY (tenant_id, occurred_at, event_id)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS raw_deploy_events
(
    event_id        UUID,
    event_type      LowCardinality(String),
    event_version   UInt32,
    tenant_id       String,
    execution_id    Nullable(String),
    occurred_at     DateTime64(3, 'UTC'),
    producer        LowCardinality(String),
    trace_id        Nullable(String),
    idempotency_key String,
    payload         String
)
ENGINE = ReplacingMergeTree(event_id)
ORDER BY (tenant_id, occurred_at, event_id)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS raw_security_events
(
    event_id        UUID,
    event_type      LowCardinality(String),
    event_version   UInt32,
    tenant_id       String,
    execution_id    Nullable(String),
    occurred_at     DateTime64(3, 'UTC'),
    producer        LowCardinality(String),
    trace_id        Nullable(String),
    idempotency_key String,
    payload         String
)
ENGINE = ReplacingMergeTree(event_id)
ORDER BY (tenant_id, occurred_at, event_id)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;
