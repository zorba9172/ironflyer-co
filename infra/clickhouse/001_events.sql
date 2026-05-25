CREATE DATABASE IF NOT EXISTS ironflyer;

CREATE TABLE IF NOT EXISTS ironflyer.raw_events
(
    event_id UUID,
    topic LowCardinality(String),
    key String,
    event_type LowCardinality(String),
    event_version UInt16,
    tenant_id String,
    execution_id String,
    occurred_at DateTime64(6, 'UTC'),
    payload String,
    ingested_at DateTime64(6, 'UTC') DEFAULT now64(6)
)
ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (topic, event_type, tenant_id, execution_id, event_id);

CREATE TABLE IF NOT EXISTS ironflyer.kafka_events
(
    raw String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda:9092',
    kafka_topic_list = 'ifly.dev.billing.ledger.v1,ifly.staging.billing.ledger.v1,ifly.prod.billing.ledger.v1',
    kafka_group_name = 'ironflyer-clickhouse-events',
    kafka_format = 'JSONAsString',
    kafka_num_consumers = 1;

CREATE MATERIALIZED VIEW IF NOT EXISTS ironflyer.mv_kafka_events
TO ironflyer.raw_events
AS
SELECT
    toUUIDOrZero(JSONExtractString(raw, 'id')) AS event_id,
    JSONExtractString(raw, 'topic') AS topic,
    JSONExtractString(raw, 'key') AS key,
    JSONExtractString(raw, 'eventType') AS event_type,
    toUInt16(JSONExtractUInt(raw, 'eventVersion')) AS event_version,
    JSONExtractString(JSONExtractRaw(raw, 'payload'), 'tenant_id') AS tenant_id,
    JSONExtractString(JSONExtractRaw(raw, 'payload'), 'execution_id') AS execution_id,
    parseDateTime64BestEffort(JSONExtractString(raw, 'createdAt')) AS occurred_at,
    JSONExtractRaw(raw, 'payload') AS payload
FROM ironflyer.kafka_events;

CREATE TABLE IF NOT EXISTS ironflyer.fact_ledger_entries
(
    event_id UUID,
    ledger_entry_id UUID,
    tenant_id String,
    execution_id String,
    entry_type LowCardinality(String),
    direction LowCardinality(String),
    amount_usd Decimal(18, 6),
    provider String,
    billable Bool,
    margin_relevant Bool,
    created_at DateTime64(6, 'UTC'),
    ingested_at DateTime64(6, 'UTC') DEFAULT now64(6)
)
ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, execution_id, entry_type, ledger_entry_id);

CREATE MATERIALIZED VIEW IF NOT EXISTS ironflyer.mv_ledger_entries
TO ironflyer.fact_ledger_entries
AS
SELECT
    event_id,
    toUUIDOrZero(JSONExtractString(payload, 'ledger_entry_id')) AS ledger_entry_id,
    JSONExtractString(payload, 'tenant_id') AS tenant_id,
    JSONExtractString(payload, 'execution_id') AS execution_id,
    JSONExtractString(payload, 'entry_type') AS entry_type,
    JSONExtractString(payload, 'direction') AS direction,
    toDecimal64(JSONExtractString(payload, 'amount_usd'), 6) AS amount_usd,
    JSONExtractString(payload, 'provider') AS provider,
    JSONExtractBool(payload, 'billable') AS billable,
    JSONExtractBool(payload, 'margin_relevant') AS margin_relevant,
    parseDateTime64BestEffort(JSONExtractString(payload, 'created_at')) AS created_at
FROM ironflyer.raw_events
WHERE topic LIKE 'ifly.%.billing.ledger.v1'
  AND startsWith(event_type, 'ledger.')
  AND JSONExtractString(payload, 'ledger_entry_id') != '';

CREATE VIEW IF NOT EXISTS ironflyer.rollup_profit_hourly AS
SELECT
    tenant_id,
    toStartOfHour(created_at) AS hour,
    sumIf(amount_usd, entry_type = 'wallet_topup') AS revenue_usd,
    sumIf(amount_usd, entry_type = 'provider_inference_cost') AS provider_cost_usd,
    sumIf(amount_usd, entry_type = 'sandbox_cost') AS sandbox_cost_usd,
    sumIf(amount_usd, entry_type IN ('storage_cost', 'deployment_cost', 'premium_reasoning_charge')) AS other_cost_usd,
    sumIf(amount_usd, entry_type = 'refund') AS refunds_usd,
    revenue_usd - provider_cost_usd - sandbox_cost_usd - other_cost_usd - refunds_usd AS gross_profit_usd,
    if(revenue_usd = 0, 0, gross_profit_usd / revenue_usd * 100) AS gross_margin_pct
FROM ironflyer.fact_ledger_entries FINAL
WHERE margin_relevant OR entry_type IN ('wallet_topup', 'refund')
GROUP BY tenant_id, hour;
