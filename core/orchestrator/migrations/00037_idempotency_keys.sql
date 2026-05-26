-- +goose Up
-- V22 Wave 3 / Item 5 (WORKFLOWS.md "Idempotency open gaps").
-- Temporal gives at-least-once activity execution. Every economic
-- mutation that wallet / ledger / execution drive on behalf of a
-- workflow MUST be safe to retry without double-charging or doubling
-- a state transition. This migration adds the durable operation-key
-- columns + unique indexes that the new opKey-aware service methods
-- collide against on retry.
--
-- Wallet: a single row per tenant — we don't add columns to `wallets`
-- itself. Instead, `wallet_operations` is the per-op dedupe log; a
-- successful Hold/Release/Debit/TopUp/Refund inserts one row keyed by
-- op_key, and a retry collides on the PK and reads back the prior
-- outcome.
--
-- Ledger: append-only, so dedupe lives on `ledger_entries.op_key` —
-- a UNIQUE index where NOT NULL (partial index lets legacy rows with
-- NULL op_key remain valid).
--
-- Execution: the FSM already prevents double transitions, but
-- Temporal can retry Admit/Start/Settle (Succeed|Fail) with the same
-- intent. Storing the op_key per terminal-class transition lets the
-- idempotent wrappers no-op the second call without consulting the
-- FSM at all.

CREATE TABLE IF NOT EXISTS wallet_operations (
  op_key TEXT PRIMARY KEY,
  tenant_id UUID NOT NULL,
  op_type TEXT NOT NULL CHECK (op_type IN ('hold','release','debit','topup','refund')),
  amount_usd NUMERIC(18,6) NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('succeeded','failed')),
  error_code TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_wallet_ops_tenant
  ON wallet_operations(tenant_id, created_at DESC);

ALTER TABLE ledger_entries ADD COLUMN IF NOT EXISTS op_key TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_ledger_op_key
  ON ledger_entries(op_key) WHERE op_key IS NOT NULL;

ALTER TABLE executions ADD COLUMN IF NOT EXISTS admit_op_key TEXT;
ALTER TABLE executions ADD COLUMN IF NOT EXISTS start_op_key TEXT;
ALTER TABLE executions ADD COLUMN IF NOT EXISTS settle_op_key TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_exec_admit_op
  ON executions(admit_op_key) WHERE admit_op_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_exec_start_op
  ON executions(start_op_key) WHERE start_op_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_exec_settle_op
  ON executions(settle_op_key) WHERE settle_op_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS uniq_exec_settle_op;
DROP INDEX IF EXISTS uniq_exec_start_op;
DROP INDEX IF EXISTS uniq_exec_admit_op;
ALTER TABLE executions DROP COLUMN IF EXISTS settle_op_key;
ALTER TABLE executions DROP COLUMN IF EXISTS start_op_key;
ALTER TABLE executions DROP COLUMN IF EXISTS admit_op_key;
DROP INDEX IF EXISTS uniq_ledger_op_key;
ALTER TABLE ledger_entries DROP COLUMN IF EXISTS op_key;
DROP INDEX IF EXISTS idx_wallet_ops_tenant;
DROP TABLE IF EXISTS wallet_operations;
