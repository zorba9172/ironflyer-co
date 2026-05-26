-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS budget_ledger (
    id            UUID        PRIMARY KEY,
    user_id       TEXT        NOT NULL,
    project_id    TEXT        NULL,
    provider      TEXT        NOT NULL,
    model         TEXT        NOT NULL,
    input_tokens  INT         NOT NULL DEFAULT 0,
    output_tokens INT         NOT NULL DEFAULT 0,
    cache_read    INT         NOT NULL DEFAULT 0,
    cache_create  INT         NOT NULL DEFAULT 0,
    cost_usd      NUMERIC(20,10) NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_budget_ledger_user_period ON budget_ledger(user_id, created_at);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS budget_vault (
    id         UUID        PRIMARY KEY,
    kind       TEXT        NOT NULL,
    user_id    TEXT        NULL,
    amount     NUMERIC(20,10) NOT NULL,
    note       TEXT        NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_budget_vault_kind ON budget_vault(kind);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS budget_vault;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS budget_ledger;
-- +goose StatementEnd
