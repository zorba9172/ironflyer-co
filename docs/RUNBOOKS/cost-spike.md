# Runbook — Cost spike

When the per-execution cost dashboard, the profit dashboard, or a paging
alert from ClickHouse says provider spend has crossed the daily budget,
this runbook is the response loop.

## 1. Confirm the spike is real

```bash
# Snapshot the orchestrator-side profit aggregate via GraphQL.
curl -s -X POST http://localhost:8080/graphql \
  -H 'content-type: application/json' \
  -H "authorization: Bearer $OPERATOR_JWT" \
  -d '{"query":"{ profitDashboard { revenueUSD providerCostUSD grossProfitUSD grossMarginPct activeExecutions } }"}'

# Compare against the previous hour's snapshot.
```

If `providerCostUSD` is materially above plan and `grossMarginPct` is
below the configured threshold, the spike is real.

## 2. Identify the source

The append-only ledger is the source of truth. Aggregate by execution
and by provider:

```bash
docker exec -it compose-postgres-1 psql -U ironflyer -d ironflyer <<'SQL'
SELECT tenant_id,
       execution_id,
       SUM(amount_usd) AS spend,
       COUNT(*)         AS entries
FROM   ledger_entries
WHERE  ts > now() - interval '1 hour'
  AND  entry_type = 'debit'
GROUP  BY tenant_id, execution_id
ORDER  BY spend DESC
LIMIT  20;
SQL
```

If a single tenant or execution is responsible, that's the lever to
pull first.

## 3. Stop the bleed

The platform has three escalating controls. Pick the smallest one that
solves the incident.

- **Per-execution stop** — call `stopExecution(id:)` for the runaway
  execution. ProfitGuard refuses the next step and the wallet hold is
  released:

  ```bash
  curl -s -X POST http://localhost:8080/graphql \
    -H 'content-type: application/json' \
    -H "authorization: Bearer $OPERATOR_JWT" \
    -d '{"query":"mutation { stopExecution(id: \"<id>\") { id status } }"}'
  ```

- **Per-tenant pause** — drop the tenant's wallet balance to a hard
  cap until they top up. This is the lever for a runaway customer.

- **Global ProfitGuard kill switch** — set
  `IRONFLYER_PROFITGUARD_DEFAULT=stop` and restart the orchestrator.
  Every gate then returns `stop` and no further provider spend
  accrues.

## 4. Postmortem

- Was a single blueprint responsible? If yes, mark it `at_risk` in
  blueprint storage so ProfitGuard down-weights it.
- Was a provider regression responsible (a model returning huge
  responses)? Open a ticket against the provider router to add a
  per-provider cap.
- Did the alert fire on time? If not, lower the threshold or shorten
  the aggregation window in ClickHouse.

## 5. Verification

After the incident:

```bash
# profitDashboard should show grossMarginPct back above the threshold.
curl -s -X POST http://localhost:8080/graphql \
  -H 'content-type: application/json' \
  -H "authorization: Bearer $OPERATOR_JWT" \
  -d '{"query":"{ profitDashboard { revenueUSD providerCostUSD grossMarginPct activeExecutions } }"}'

# v22_smoke.sh must still exit 0 (or PASS-WITH-WARN).
bash scripts/v22_smoke.sh
```
