-- +goose NO TRANSACTION
-- 00043_perf_indexes — performance audit follow-up. Adds the indexes
-- the hot-path queries in executions / execution_events / deploys /
-- blueprint_runs were scanning around. Every CREATE is IF NOT EXISTS
-- and CONCURRENTLY so production runs do not lock the underlying
-- tables. The whole migration runs outside a transaction
-- (`-- +goose NO TRANSACTION`) because Postgres rejects
-- CREATE INDEX CONCURRENTLY inside an explicit transaction block.

-- +goose Up

-- executions: ListByTenantAndProject filters on (tenant_id, project_id)
-- and orders by created_at DESC. The existing idx_executions_tenant
-- only covers tenant_id; this composite keeps the project-scoped
-- listing index-only as execution volume grows.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_tenant_project_created
    ON executions(tenant_id, project_id, created_at DESC)
    WHERE project_id IS NOT NULL;

-- executions: ActiveCount / QueuedCount run COUNT(*) WHERE status =
-- 'running' (or status IN created/admitted). idx_executions_status
-- covers it but the partial below stays tiny and gives the metrics
-- pollers an index-only scan for the active/queued sets.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_status_running
    ON executions(status)
    WHERE status IN ('running','created','admitted');

-- executions: forecaster reads (tenant_id, created_at, status IN
-- ('succeeded','refunded')). Equality on tenant first, then status,
-- then created_at range — composite matches the column order in the
-- WHERE clause for an index-only path.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_tenant_status_created
    ON executions(tenant_id, status, created_at DESC);

-- execution_events: every per-execution feed filters by
-- (execution_id, event_type IN (...)) ORDER BY created_at. The
-- existing idx_execution_events_exec covers (execution_id,
-- created_at) so the IN-list has to be re-checked at scan time.
-- The composite below puts event_type in the index key so the IN
-- becomes a bitmap-merge over a few tight ranges.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_execution_events_exec_type_created
    ON execution_events(execution_id, event_type, created_at);

-- deploys: GetByExecution does WHERE execution_id = $1 ORDER BY
-- created_at DESC LIMIT 1 — currently a full-table scan. Index
-- covers the lookup directly.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_deploys_execution
    ON deploys(execution_id, created_at DESC)
    WHERE execution_id IS NOT NULL;

-- blueprint_runs: forecaster queries (blueprint_id, tenant_id,
-- created_at >= ...). idx_bruns_blueprint covers blueprint_id only;
-- this composite keeps the per-tenant window scan index-only.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bruns_blueprint_tenant_created
    ON blueprint_runs(blueprint_id, tenant_id, created_at DESC);

-- ledger_entries: ListByExecution orders by created_at ASC.
-- idx_ledger_execution(execution_id) covers the equality but the sort
-- still spills once per-execution row count grows. Composite is small
-- and removes the sort node.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ledger_execution_created
    ON ledger_entries(execution_id, created_at)
    WHERE execution_id IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_ledger_execution_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_bruns_blueprint_tenant_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_deploys_execution;
DROP INDEX CONCURRENTLY IF EXISTS idx_execution_events_exec_type_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_executions_tenant_status_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_executions_status_running;
DROP INDEX CONCURRENTLY IF EXISTS idx_executions_tenant_project_created;
