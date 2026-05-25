-- 00018_patch_stages.sql — staging area for grouped patch review.
--
-- A PatchStage bundles several patch IDs into one logical review unit
-- (e.g. "add an auth middleware" produces 5-15 patches across files).
-- Apply / reject the stage as a single transaction.
--
-- Schema mirrors patch.PatchStage in apps/orchestrator/internal/patch.
-- patch_ids is JSONB so the slice can grow without a join table —
-- staging is a low-cardinality feature, not a workload.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS patch_stages (
    id               TEXT         PRIMARY KEY,
    project_id       TEXT         NOT NULL,
    name             TEXT         NOT NULL,
    description      TEXT         NULL,
    patch_ids        JSONB        NOT NULL DEFAULT '[]'::jsonb,
    status           TEXT         NOT NULL DEFAULT 'open',
    rejection_reason TEXT         NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_patch_stages_project ON patch_stages(project_id, created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_patch_stages_status  ON patch_stages(status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS patch_stages;
-- +goose StatementEnd
