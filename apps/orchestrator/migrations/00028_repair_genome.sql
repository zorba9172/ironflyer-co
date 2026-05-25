-- +goose Up
-- +goose StatementBegin
-- V22 repair genome + patch memory + completion scores (Agent 7).
--
-- repair_recipes: failure_signature → known fix. A failure signature is
-- the SHA-256 of a normalised failure string; the genome learns which
-- fix shape recovered the build/gate so the next occurrence can short
-- circuit the expensive reasoning loop.
--
-- patch_memory: past patches keyed by an intent signature (prompt +
-- gate context). Lets the finisher re-apply or re-rank a known patch
-- shape when the same intent recurs.
--
-- completion_scores: append-only log of completion-score events per
-- execution + gate. The dashboards aggregate (score, delta) over this
-- table to compute completion-per-dollar.
CREATE TABLE IF NOT EXISTS repair_recipes (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    failure_signature   TEXT NOT NULL UNIQUE,
    category            TEXT NOT NULL,
    fix_json            JSONB NOT NULL,
    hits                INT NOT NULL DEFAULT 0,
    successes           INT NOT NULL DEFAULT 0,
    last_hit_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_repair_category ON repair_recipes(category);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS patch_memory (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    intent_signature    TEXT NOT NULL,
    patch_json          JSONB NOT NULL,
    affected_paths      TEXT[] NOT NULL,
    cost_usd            NUMERIC(18,6) NOT NULL DEFAULT 0,
    applied_count       INT NOT NULL DEFAULT 0,
    success_count       INT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_applied_at     TIMESTAMPTZ
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_patch_memory_intent ON patch_memory(intent_signature);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS completion_scores (
    execution_id    UUID NOT NULL,
    gate_name       TEXT NOT NULL,
    score           NUMERIC(5,4) NOT NULL,
    delta           NUMERIC(6,4) NOT NULL,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (execution_id, gate_name, recorded_at)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS completion_scores;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS patch_memory;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS repair_recipes;
-- +goose StatementEnd
