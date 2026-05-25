-- Path: billing-ledger schema proposal
-- Role: Detailed financial ledger for proving profit.

CREATE TABLE ledger_entries (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  execution_id UUID REFERENCES executions(id),
  entry_type TEXT NOT NULL,
  direction TEXT NOT NULL CHECK (direction IN ('debit','credit')),
  amount_usd NUMERIC(18,6) NOT NULL,
  provider TEXT,
  billable BOOLEAN NOT NULL DEFAULT true,
  margin_relevant BOOLEAN NOT NULL DEFAULT true,
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ledger_tenant_created ON ledger_entries(tenant_id, created_at DESC);
CREATE INDEX idx_ledger_execution ON ledger_entries(execution_id);
CREATE INDEX idx_ledger_type_created ON ledger_entries(entry_type, created_at DESC);
