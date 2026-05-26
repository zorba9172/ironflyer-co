-- +goose Up
-- +goose StatementBegin
-- workspaces_state owns the metadata layer of the portable runtime.
-- Every workspace has a stable identity (id), an owner (owner_id), a
-- driver-specified container image, and an explicit current_pod_id that
-- identifies which runtime pod (StatefulSet ordinal — e.g. runtime-0)
-- holds the live Docker container right now.
--
-- The interesting verb is "claim": an UPDATE ... WHERE current_pod_id
-- IS NULL OR current_pod_id = '' that lets exactly one pod take over a
-- homeless workspace. The companion verb is "reap": a periodic UPDATE
-- that frees workspaces whose owner stopped heartbeating.
--
-- The runtime service itself runs a thin idempotent BootstrapPostgres
-- on startup (core/runtime/internal/state.BootstrapPostgres) so it
-- works even when the orchestrator hasn't yet migrated. This file is
-- the canonical schema; the bootstrap mirror is "good enough until
-- goose runs."
CREATE TABLE IF NOT EXISTS workspaces_state (
    id                TEXT        PRIMARY KEY,
    owner_id          TEXT        NOT NULL,
    project_id        TEXT        NULL,
    driver            TEXT        NOT NULL,
    image             TEXT        NULL,
    status            TEXT        NOT NULL,
    current_pod_id    TEXT        NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_heartbeat_at TIMESTAMPTZ NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_workspaces_state_owner
    ON workspaces_state(owner_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- Pod lookup index: the orchestrator runtime client uses this to
-- resolve current_pod_id for PTY routing.
CREATE INDEX IF NOT EXISTS idx_workspaces_state_pod
    ON workspaces_state(current_pod_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- Heartbeat index: the reaper scans rows whose heartbeat is older than
-- the staleness cutoff. Partial index keeps the working set tiny.
CREATE INDEX IF NOT EXISTS idx_workspaces_state_heartbeat
    ON workspaces_state(last_heartbeat_at)
    WHERE current_pod_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workspaces_state;
-- +goose StatementEnd
