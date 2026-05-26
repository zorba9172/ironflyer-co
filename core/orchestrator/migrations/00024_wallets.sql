-- +goose Up
-- +goose StatementBegin
-- V22 wallet model — per-tenant prepaid credit balance.
--
-- balance_usd is the cash on hand; hold_usd is the portion of that
-- balance reserved by active executions (ProfitGuard.Admit holds funds
-- before the execution starts; commit either releases the unused hold
-- back to balance or debits it). available = balance_usd - hold_usd.
--
-- lifetime_topup_usd / lifetime_spend_usd are monotonically increasing
-- counters used by the profit dashboards so they don't have to scan the
-- ledger to render headline numbers.
--
-- Law 1 ("no execution starts without budget") is enforced at the SQL
-- layer by the CHECK constraints + the SELECT … FOR UPDATE flow in the
-- Hold/Debit code paths.
CREATE TABLE IF NOT EXISTS wallets (
    tenant_id          UUID           PRIMARY KEY,
    balance_usd        NUMERIC(18,6)  NOT NULL DEFAULT 0,
    hold_usd           NUMERIC(18,6)  NOT NULL DEFAULT 0,
    lifetime_topup_usd NUMERIC(18,6)  NOT NULL DEFAULT 0,
    lifetime_spend_usd NUMERIC(18,6)  NOT NULL DEFAULT 0,
    updated_at         TIMESTAMPTZ    NOT NULL DEFAULT now(),
    created_at         TIMESTAMPTZ    NOT NULL DEFAULT now(),
    CHECK (balance_usd >= 0),
    CHECK (hold_usd >= 0)
);
-- +goose StatementEnd

-- +goose StatementBegin
-- wallet_topups records every Stripe Checkout top-up attempt. The
-- stripe_session_id UNIQUE constraint is the idempotency anchor for the
-- webhook: a duplicate checkout.session.completed delivery becomes a
-- no-op INSERT instead of a double credit.
CREATE TABLE IF NOT EXISTS wallet_topups (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID         NOT NULL,
    stripe_session_id TEXT         UNIQUE,
    amount_usd        NUMERIC(18,6) NOT NULL,
    status            TEXT         NOT NULL CHECK (status IN ('pending','succeeded','failed','refunded')),
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    completed_at      TIMESTAMPTZ
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_wallet_topups_tenant
    ON wallet_topups(tenant_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS wallet_topups;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS wallets;
-- +goose StatementEnd
