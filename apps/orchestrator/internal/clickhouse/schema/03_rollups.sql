-- Ironflyer analytics plane — rollup tables.
--
-- Rollups are the dashboard data sources. SummingMergeTree on
-- decimal/uint columns means consumers can INSERT delta rows and the
-- background merge collapses them — no UPDATE statements, no race.
-- TTL 7 years for daily/hourly business rollups, 18 months for the
-- high-frequency 5-minute runtime capacity series.
--
-- All money columns are Decimal64(6); timestamps DateTime64(3, 'UTC').
-- Gross margin is computed in the dashboard SELECT (revenue - cost),
-- never stored, so a backfill of either column produces a correct
-- answer with no double-write.

-- ---------------------------------------------------------------------
-- rollup_profit_hourly — the production margin dashboard backbone.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rollup_profit_hourly
(
    hour                                  DateTime,
    tenant_id                             String,
    revenue_usd                           Decimal64(6),
    provider_cost_usd                     Decimal64(6),
    sandbox_cost_usd                      Decimal64(6),
    storage_cost_usd                      Decimal64(6),
    deployment_cost_usd                   Decimal64(6),
    refunds_usd                           Decimal64(6),
    completed_executions                  UInt64,
    profitable_completed_executions       UInt64
)
ENGINE = SummingMergeTree
ORDER BY (hour, tenant_id)
PARTITION BY toYYYYMM(hour)
TTL hour + INTERVAL 7 YEAR
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------
-- rollup_profit_daily — the same shape rolled to one row per day.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rollup_profit_daily
(
    day                                   Date,
    tenant_id                             String,
    revenue_usd                           Decimal64(6),
    provider_cost_usd                     Decimal64(6),
    sandbox_cost_usd                      Decimal64(6),
    storage_cost_usd                      Decimal64(6),
    deployment_cost_usd                   Decimal64(6),
    refunds_usd                           Decimal64(6),
    completed_executions                  UInt64,
    profitable_completed_executions       UInt64
)
ENGINE = SummingMergeTree
ORDER BY (day, tenant_id)
PARTITION BY toYYYYMM(day)
TTL day + INTERVAL 7 YEAR
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------
-- rollup_provider_daily — per-provider cost/latency rollups.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rollup_provider_daily
(
    day             Date,
    tenant_id       String,
    provider        LowCardinality(String),
    model           LowCardinality(String),
    calls           UInt64,
    cost_usd        Decimal64(6),
    input_tokens    UInt64,
    output_tokens   UInt64,
    latency_ms_sum  UInt64
)
ENGINE = SummingMergeTree
ORDER BY (day, tenant_id, provider, model)
PARTITION BY toYYYYMM(day)
TTL day + INTERVAL 7 YEAR
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------
-- rollup_blueprint_daily — feeds the Blueprint dashboard.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rollup_blueprint_daily
(
    day                    Date,
    tenant_id              String,
    blueprint_id           LowCardinality(String),
    executions             UInt64,
    preview_success_count  UInt64,
    refund_count           UInt64,
    repair_count_sum       UInt64,
    revenue_usd_sum        Decimal64(6),
    cost_usd_sum           Decimal64(6),
    completion_score_sum   Float64
)
ENGINE = SummingMergeTree
ORDER BY (day, tenant_id, blueprint_id)
PARTITION BY toYYYYMM(day)
TTL day + INTERVAL 7 YEAR
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------
-- rollup_gate_daily — gate pass/fail counts.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rollup_gate_daily
(
    day            Date,
    tenant_id      String,
    gate_name      LowCardinality(String),
    verdict        LowCardinality(String),
    count          UInt64,
    duration_ms    UInt64
)
ENGINE = SummingMergeTree
ORDER BY (day, tenant_id, gate_name, verdict)
PARTITION BY toYYYYMM(day)
TTL day + INTERVAL 7 YEAR
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------
-- rollup_cohort_monthly — cohort funnel rollup used by Cohort dash.
-- One row per (cohort_month, tenant) — the dashboard aggregates to
-- month-level with DISTINCT tenant counts.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rollup_cohort_monthly
(
    cohort_month       Date,
    tenant_id          String,
    first_paid_at      DateTime64(3, 'UTC'),
    second_paid_at     DateTime64(3, 'UTC'),
    total_runs         UInt64,
    spend_usd_sum      Decimal64(6),
    revenue_usd_sum    Decimal64(6),
    refund_count       UInt64,
    completed_count    UInt64
)
ENGINE = ReplacingMergeTree(total_runs)
ORDER BY (cohort_month, tenant_id)
PARTITION BY toYYYYMM(cohort_month)
TTL cohort_month + INTERVAL 7 YEAR
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------
-- rollup_runtime_capacity_5m — 5-minute cluster capacity snapshots.
-- High-frequency, 18-month TTL — operational, not financial.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rollup_runtime_capacity_5m
(
    bucket            DateTime,
    pool              LowCardinality(String),
    active_runs       UInt64,
    queued_runs       UInt64,
    capacity          UInt64,
    utilization_pct   Float64
)
ENGINE = ReplacingMergeTree(bucket)
ORDER BY (bucket, pool)
PARTITION BY toYYYYMM(bucket)
TTL bucket + INTERVAL 18 MONTH
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------
-- rollup_abuse_tenant_daily — abuse scoring for the abuse dashboard.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rollup_abuse_tenant_daily
(
    day               Date,
    tenant_id         String,
    score             Float64,
    failed_executions UInt64,
    refunds_usd       Decimal64(6),
    rate_limit_hits   UInt64
)
ENGINE = SummingMergeTree
ORDER BY (day, tenant_id)
PARTITION BY toYYYYMM(day)
TTL day + INTERVAL 7 YEAR
SETTINGS index_granularity = 8192;
