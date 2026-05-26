-- +goose Up
-- +goose StatementBegin
-- A63: add workspace_id to executions so the wow-loop builder can resolve
-- the live runtime sandbox bound to an execution without proxying through
-- projectID. The finisher engine populates this column the moment a
-- workspace is resolved/allocated for the active execution (see
-- engine.go: e.executionService.SetWorkspaceID after FindWorkspaceForProject).
--
-- TEXT (not UUID) because the runtime's workspace identifier is an
-- arbitrary string in the workspace contract — keeping the column TEXT
-- avoids a forced UUID coercion on every future driver swap. Nullable so
-- legacy rows + executions that never allocate a sandbox stay valid.
ALTER TABLE executions ADD COLUMN IF NOT EXISTS workspace_id TEXT;
-- +goose StatementEnd

-- +goose StatementBegin
-- Partial index — most rows during the warm-up period will have
-- workspace_id NULL; index only the rows that carry a value so the
-- "find executions for this workspace" lookup stays cheap without
-- bloating the index with NULLs.
CREATE INDEX IF NOT EXISTS idx_executions_workspace
    ON executions(workspace_id)
    WHERE workspace_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_executions_workspace;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE executions DROP COLUMN IF EXISTS workspace_id;
-- +goose StatementEnd
