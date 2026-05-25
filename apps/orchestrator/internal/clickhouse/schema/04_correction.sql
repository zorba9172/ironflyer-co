-- Ironflyer analytics plane — correction state table.
--
-- The CorrectionJob (apps/orchestrator/internal/clickhouse/correction.go)
-- runs every hour and recomputes the trailing 14-day window of each
-- daily rollup so late-arriving raw events still influence the
-- dashboard. This table is the job's own bookkeeping: per (rollup,
-- window_day) it records the last run timestamp, the entry count, and
-- the terminal status.
--
-- ReplacingMergeTree(last_recomputed_at) collapses repeated runs of
-- the same window down to the most recent attempt — operators only
-- care about the latest state per window. Daily-grain ORDER BY so
-- range scans over "the last 14 days for rollup X" stay cheap.

CREATE TABLE IF NOT EXISTS rollup_correction_state
(
    rollup_name        LowCardinality(String),
    window_start       DateTime64(3, 'UTC'),
    last_recomputed_at DateTime64(3, 'UTC'),
    entries_recomputed UInt64,
    status             LowCardinality(String),   -- 'idle' | 'running' | 'failed'
    error_summary      String
)
ENGINE = ReplacingMergeTree(last_recomputed_at)
ORDER BY (rollup_name, window_start)
PARTITION BY toYYYYMM(window_start)
SETTINGS index_granularity = 8192;
