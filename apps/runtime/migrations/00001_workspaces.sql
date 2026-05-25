-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS workspaces (
    id              TEXT        PRIMARY KEY,
    owner_id        TEXT        NOT NULL,
    project_id      TEXT        NULL,
    driver          TEXT        NOT NULL,
    status          TEXT        NOT NULL,
    efs_path        TEXT        NULL,
    s3_archive_key  TEXT        NULL,
    active_pod      TEXT        NULL,
    last_active_at  TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_workspaces_owner ON workspaces(owner_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_workspaces_status_active ON workspaces(status, last_active_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workspaces;
-- +goose StatementEnd
