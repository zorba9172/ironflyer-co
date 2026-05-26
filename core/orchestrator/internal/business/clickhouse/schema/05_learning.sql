-- Ironflyer Feedback Brain — learning fact tables.
--
-- Every OutcomeEvent published via `internal/ai/learning/publisher.go`
-- lands here through the Redpanda → ClickHouse consumer. Topics
-- starting with `ifly.<env>.learning.outcomes.v*` are routed to
-- raw_learning_events by tableForTopic; the materialized view below
-- projects each event into fact_outcome_events, and a SummingMergeTree
-- rolls the daily counts so the dashboard answers in O(1).
--
-- All JSON paths use JSONExtractString because the raw payload is
-- stored as a String column.
-- ---------------------------------------------------------------------

-- ---------------------------------------------------------------------
-- raw_learning_events — append-only mirror of the outbox stream for
-- the learning topic. ReplacingMergeTree(event_id) collapses any
-- redelivery duplicates on merge.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS raw_learning_events
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
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, occurred_at, event_id)
PARTITION BY toYYYYMM(occurred_at)
TTL toDateTime(occurred_at) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------
-- fact_outcome_events — one row per OutcomeEvent. attributes_json
-- carries the per-Kind body so the miner can read it with
-- JSONExtractString without re-parsing the full envelope.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fact_outcome_events
(
    event_id         UUID,
    tenant_id        String,
    execution_id     String,
    kind             LowCardinality(String),
    timestamp        DateTime64(3, 'UTC'),
    success          Int8,        -- -1 unknown, 0 fail, 1 success
    cost_usd         Decimal64(6),
    margin_usd       Decimal64(6),
    attributes_json  String
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, kind, timestamp, event_id)
PARTITION BY toYYYYMM(timestamp)
TTL toDateTime(timestamp) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_fact_outcome_events
TO fact_outcome_events AS
SELECT
    event_id,
    tenant_id,
    COALESCE(toString(JSONExtractString(payload, 'execution_id')), '')                   AS execution_id,
    JSONExtractString(payload, 'kind')                                                    AS kind,
    parseDateTime64BestEffortOrZero(JSONExtractString(payload, 'timestamp'), 3, 'UTC')    AS timestamp,
    multiIf(
        JSONHas(payload, 'success') AND JSONExtractBool(payload, 'success'), 1,
        JSONHas(payload, 'success'), 0,
        -1
    )                                                                                     AS success,
    toDecimal64OrZero(JSONExtractString(payload, 'cost_usd'), 6)                          AS cost_usd,
    toDecimal64OrZero(JSONExtractString(payload, 'margin_usd'), 6)                        AS margin_usd,
    JSONExtractRaw(payload, 'attributes')                                                 AS attributes_json
FROM raw_learning_events
WHERE event_type LIKE 'learning.outcome.%';

-- ---------------------------------------------------------------------
-- fact_pattern_observations — miner outputs. Strategy adapters and
-- the learning dashboard read from here.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS fact_pattern_observations
(
    event_id      UUID,
    tenant_id     String,
    pattern       LowCardinality(String),
    target        String,
    direction     LowCardinality(String),
    confidence    Float64,
    observed_at   DateTime64(3, 'UTC'),
    evidence_json String
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant_id, pattern, observed_at, event_id)
PARTITION BY toYYYYMM(observed_at)
TTL toDateTime(observed_at) + INTERVAL 6 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_fact_pattern_observations
TO fact_pattern_observations AS
SELECT
    event_id,
    tenant_id,
    JSONExtractString(payload, 'pattern')                                                  AS pattern,
    JSONExtractString(payload, 'target')                                                   AS target,
    JSONExtractString(payload, 'direction')                                                AS direction,
    toFloat64OrZero(JSONExtractString(payload, 'confidence'))                              AS confidence,
    parseDateTime64BestEffortOrZero(JSONExtractString(payload, 'observed_at'), 3, 'UTC')   AS observed_at,
    JSONExtractRaw(payload, 'evidence')                                                    AS evidence_json
FROM raw_learning_events
WHERE event_type = 'learning.pattern.v1';

-- ---------------------------------------------------------------------
-- rollup_learning_daily — SummingMergeTree on per-tenant-per-kind
-- counts so the dashboard answers in O(1) without scanning the fact
-- table. Aggregates: total count, success count, failure count, sum
-- of cost / margin.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rollup_learning_daily
(
    tenant_id    String,
    kind         LowCardinality(String),
    day          Date,
    total        UInt64,
    successes    UInt64,
    failures     UInt64,
    cost_usd     Decimal64(6),
    margin_usd   Decimal64(6)
)
ENGINE = SummingMergeTree
ORDER BY (tenant_id, kind, day)
PARTITION BY toYYYYMM(day)
TTL toDateTime(day) + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_rollup_learning_daily
TO rollup_learning_daily AS
SELECT
    tenant_id,
    kind,
    toDate(timestamp)                AS day,
    count()                          AS total,
    countIf(success = 1)             AS successes,
    countIf(success = 0)             AS failures,
    sum(cost_usd)                    AS cost_usd,
    sum(margin_usd)                  AS margin_usd
FROM fact_outcome_events
GROUP BY tenant_id, kind, day;
