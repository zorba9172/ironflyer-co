# Ironflyer Bug Hunt Report — Closure Agent R

Generated 2026-05-26 from `tmp/logs/orchestrator.log`, `runtime.log`,
`scenario_*.json`, `noarg_query_results.txt`, `docs/PROJECT_CLOSEOUT_PLAN.md`
DoD checklist, `docs/V22_PLAN.md` hard laws, and direct source inspection.
`go vet ./...` clean in both Go modules.

## Executive summary — ranked top 10

Each item is ordered by money / trust / functional impact, not by ease.

1. **Wow-loop promise is empty**: V22 customer-facing report contains
   `previewURL=null`, `changedFiles=[]`, `patchCount=0`,
   `completionScore=0`, `costReport=0` after a successful `describeIdea`
   + `createPaidExecution`. The whole "Ironflyer finishes the product
   and proves it" pitch returns blanks. (Bug #1)
2. **Seven GraphQL "internal server error" surfaces are literal
   `panic("not implemented")`** in zbase/agents/budget/audit resolvers —
   any caller of `agents / myBudget / plans / rates / vault / verifyAudit
   / version` crashes the request goroutine. (Bug #2)
3. **Outbox publishes events under `ifly.dev.*` but schemas are only
   registered for `ifly.prod.*`** → every wallet topup / hold / release
   / ProfitGuard decision is inserted into the outbox **without schema
   validation** (10+ warns per execution). Silent contract drift to any
   downstream consumer. (Bug #3)
4. **DoD #3 + #4 not met in lean default**: Redpanda + ClickHouse only
   start under `--profile analytics`, so events are written to the
   Postgres outbox but never fan out, and `profitDashboard` is served
   from in-process aggregator returning a hard-coded looking
   `revenueUSD: 150 / margin 79.2%` synthetic. (Bug #4)
5. **Runtime reaper crashes every 30s** with
   `failed to encode args[0]: unable to encode 60 into text format for
   text (OID 25)` — `PostgresStore.Reap` passes `int64` into a
   `text`-typed SQL parameter. Stale workspaces are never freed. (Bug #5)
6. **DoD #1 stop/refund missing**: GraphQL has `stopExecution` but no
   refund mutation; v22 smoke does not exercise stop/refund. A paying
   user cannot recover funds from a stuck execution. (Bug #6)
7. **Execution Postgres FSM crash with empty UUID**:
   `tx_select_for_update execution_id="" SQLSTATE 22P02` —
   `txTransition` is called with `id=""` somewhere in the create-paid
   path, returning a 500 to the user. (Bug #7)
8. **Hardening law #8 not met**: introspection is on, CSRF off, APQ
   unlocked in dev banner; depth/complexity caps are wired but
   production-only and not active for the surface used by web. (Bug #8)
9. **DoD #6 Temporal disabled by default** — `embedded finisher
   executor active`, so interrupted runs cannot be resumed across pods.
   Combined with #5, an OOM kill loses real money. (Bug #9)
10. **`ping` schema/resolver type mismatch**: smoke probe surfaces
    `Field "ping" must not have a selection since type "String" has no
    subfields.` The harness expects an object; schema returns scalar.
    Every `gqlhealth`-style probe is broken. (Bug #10)

Full report at `tmp/logs/bug_hunt_report.md` (24 bugs total).

---

## Detailed bugs

### Bug #1 — Wow-loop / executionSupportBundle returns empty proof for a "successful" execution
- **Severity**: BLOCKER
- **Where**: `apps/orchestrator/internal/graph/resolver/wowloop.resolver.go`
  + upstream finisher executor that never lands a patch or a preview in
  the mock/embedded path used by `scenario_describeIdea + scenario_wowloop`.
- **Evidence** (`tmp/logs/scenario_wowloop.json`):
  ```json
  {"data":{"executionSupportBundle":{
    "previewURL":null,"changedFiles":[],"patchCount":0,
    "gateReport":{"completionScore":0,"stages":[]},
    "securityReport":{"passRate":1,"findings":[]},
    "costReport":{"revenueUSD":0,"providerCostUSD":0,
                  "sandboxCostUSD":0,"grossMarginPct":0},
    "nextBestAction":{"kind":"review_patch","title":"Review the patches that landed"}
  }}}
  ```
  Yet `scenario_describeIdea.json` shows the same execution
  (`b734b862-…`) returned `status: "running"` with `budgetUSD: 2`.
- **Fix sketch**: the embedded finisher must (a) actually run at least
  one gate iteration in dev mode without `ANTHROPIC_API_KEY`, (b) write
  a synthetic patch via the mock provider so `changedFiles` and
  `patchCount` are non-zero, (c) compute `costReport` from the ledger
  even when zero-cost mock tokens were burned. Also: `nextBestAction`
  contradicts the empty `patchCount` — fix the action selector to gate
  on `patchCount > 0`.
- **Blast radius**: every V22 demo / paid execution / customer-facing
  proof. This is the differentiator the closeout plan explicitly names
  ("Customer Wow Loop"). Without this, Ironflyer ships nothing.

### Bug #2 — Seven panic-only resolvers crash live queries
- **Severity**: BLOCKER (customer-visible 500s)
- **Where**:
  - `internal/graph/resolver/zbase.resolver.go:22` (`Ping`)
  - `internal/graph/resolver/zbase.resolver.go:27` (`Version`)
  - `internal/graph/resolver/agents.resolver.go:16` (`Agents`)
  - `internal/graph/resolver/agents.resolver.go:21` (`AgentTelemetry`)
  - `internal/graph/resolver/agents.resolver.go:26` (`BanditRanking`)
  - `internal/graph/resolver/agents.resolver.go:31` (`CostStream` subscription)
  - `internal/graph/resolver/audit.resolver.go:16` (`Audit`)
  - `internal/graph/resolver/audit.resolver.go:21` (`VerifyAudit`)
  - `internal/graph/resolver/audit.resolver.go:26` (`AuditExportCSVURL`)
  - `internal/graph/resolver/audit.resolver.go:31` (`AuditExportPDFURL`)
  - `internal/graph/resolver/budget.resolver.go:16` (`StartCheckout` mutation)
  - `internal/graph/resolver/budget.resolver.go:21` (`Plans`)
  - `internal/graph/resolver/budget.resolver.go:26` (`Rates`)
  - `internal/graph/resolver/budget.resolver.go:31` (`Vault`)
  - `internal/graph/resolver/budget.resolver.go:36` (`MyBudget`)
- **Evidence** (`tmp/logs/orchestrator.log` 7× `graphql: panic in resolver`
  + `tmp/logs/noarg_query_results.txt`):
  ```text
  agents                         ERR  internal server error
  myBudget                       ERR  internal server error
  plans                          ERR  internal server error
  rates                          ERR  internal server error
  vault                          ERR  internal server error
  verifyAudit                    ERR  internal server error
  version                        ERR  internal server error
  ```
  Each emits a 60+ frame goroutine stack with `panic_value:
  "not implemented: <Name>"`.
- **Fix sketch**: each resolver must either (a) return a typed empty
  value plus the wired service from `Resolver`, or (b) return
  `gqlNotConfigured("<field>")` like the wallet/ledger resolvers do.
  Never `panic` from a resolver — that turns a missing wiring into a
  500. `StartCheckout` in particular is on the paid-execution hot path.
- **Blast radius**: any client that hits these fields. The web cockpit
  almost certainly calls `myBudget`, `plans`, `version`, and `agents`
  for the dashboards. `StartCheckout` blocks Stripe top-up entirely
  when STRIPE_SECRET_KEY is set.

### Bug #3 — Outbox env prefix mismatch ⇒ schema validation silently bypassed
- **Severity**: HIGH (silent contract drift; trust-killer in
  production once Redpanda is on)
- **Where**:
  - producer side: `internal/wallet/postgres.go:259`
    (`events.TopicFor("", "billing", "ledger", 1)` → `ifly.dev.*`
    when `IRONFLYER_ENV` unset)
  - registry side: `internal/events/topics.go:22-30` (all `TopicXxx`
    constants hardcoded to `ifly.prod.*`)
  - boot wiring: `internal/events/schemaregistry.go:579+` registers
    schemas under the `prod` constants
  - hit: `internal/outboxhooks/outboxhooks.go:164-168` (the warn
    "schema subject not registered; inserting without validation")
- **Evidence** (orchestrator.log boot lines 5-14 register
  `ifly.prod.billing.ledger.v1-default`; lines 56-58, 61, 64-68 emit
  rows for `ifly.dev.billing.ledger.v1-wallet.topup.v1` etc., 10 warns
  in a 90-second window):
  ```text
  registered: ifly.prod.billing.ledger.v1-default
  outbox emitted subject: ifly.dev.billing.ledger.v1-wallet.topup.v1
    → "outboxhooks: schema subject not registered;
       inserting without validation"
  ```
- **Fix sketch**: register schemas under every allowed env
  (`dev/staging/prod`), or have the registry resolve subjects through
  the same env-stripping path the ClickHouse consumer uses
  (`internal/clickhouse/consumer.go:264`), or change the producers to
  pass the prod-pinned topic constants. Today the bypass means any
  payload-shape regression ships unnoticed and lands in ClickHouse /
  Redpanda mid-flight.
- **Blast radius**: every paid-execution event is unvalidated. As soon
  as a downstream consumer is wired (DoD #3/#4), shape drift hits
  production silently.

### Bug #4 — Redpanda + ClickHouse off in lean default ⇒ DoD #3, #4 unmet
- **Severity**: HIGH
- **Where**: `infra/compose/docker-compose.dev.yml` (analytics profile);
  `apps/orchestrator/internal/dashboards/` (in-process aggregator path)
- **Evidence**: PROJECT_CLOSEOUT_PLAN DoD table marks both ✗ for
  2026-05-26. `profitDashboard` in `scenario_v22_smoke.log` returns
  `{"revenueUSD":150,"providerCostUSD":24.5,"grossProfitUSD":118.8,
  "grossMarginPct":79.2,"activeExecutions":14}` from a fresh signup
  whose only history is one $50 topup and one $1 hold — meaning the
  numbers are seeded/synthetic, not measured.
- **Fix sketch**: either (a) make Redpanda + ClickHouse on by default
  in dev compose (the brokers line already reads
  `Redpanda event publisher enabled` so the env is set, but the
  containers are absent), or (b) clearly route the dashboard to the
  in-process aggregator and label the values "estimated" in the
  resolver so reviewers don't mistake demo data for measured truth.
- **Blast radius**: DoD #3 + #4 + the "Profit dashboards surface margin
  first" hard law #3 from V22.

### Bug #5 — Runtime reaper loop crashes every 30s on Postgres encode
- **Severity**: HIGH
- **Where**: `apps/runtime/internal/state/store.go:465-483`
  (`PostgresStore.Reap` — `($1 || ' seconds')::interval` with `int64`
  arg)
- **Evidence** (`tmp/logs/runtime.log` lines 9-17):
  ```text
  "reaper: scan" error="failed to encode args[0]:
    unable to encode 60 into text format for text (OID 25):
    cannot find encode plan"
  ```
  Repeats every 30s for the entire uptime; zero successful reap calls.
- **Fix sketch**: pass `strconv.FormatInt(secs, 10)` as the bound
  arg, or rewrite the SQL to use
  `make_interval(secs => $1::int)` (no string concat). The current
  expression forces a `text` cast on `$1` because of the `||`
  operator, and pgx v5 refuses to silently coerce `int64` → `text`.
- **Blast radius**: any pod that crashes mid-execution leaves its
  workspaces marked owned forever; a replacement pod can never `Claim`
  them. Direct blocker for DoD #6 + #7 (resume-across-pods + idle
  shrinkage).

### Bug #6 — No refund mutation; stop path not exercised
- **Severity**: HIGH (DoD #1 unmet)
- **Where**: `internal/graph/schema/*.graphql` (no refund mutation);
  `scripts/v22_smoke.sh` (does not invoke stop)
- **Evidence**: PROJECT_CLOSEOUT_PLAN DoD #1 marked "partial".
  Schema dump (`tmp/logs/schema_dump.json`) shows no field starting
  with `refund`.
- **Fix sketch**: add `refundExecution(executionID: ID!)` mutation
  that releases the wallet hold AND credits the wallet for any
  already-debited cost the user is owed (per Stripe semantics), then
  add `stopExecution + refundExecution` legs to the v22 smoke.
- **Blast radius**: every paying customer who hits a broken run has
  no automated recovery path — a real-money trust killer.

### Bug #7 — Execution Postgres SELECT FOR UPDATE on empty UUID
- **Severity**: HIGH
- **Where**: `apps/orchestrator/internal/execution/postgres.go:276`
  via `txTransition(ctx, id, ...)`; caller passes `id = ""` somewhere
  in the create / admit / start path.
- **Evidence** (orchestrator.log):
  ```text
  "execution postgres op failed"
    error="ERROR: invalid input syntax for type uuid: \"\" (SQLSTATE 22P02)"
    op="tx_select_for_update" execution_id=""
  ```
  Happened during signup → first execution for tenant
  `e6c510d4-…` (deepdive scenario).
- **Fix sketch**: guard `txTransition` with `if id == "" { return
  ErrNotFound }` before opening the tx, and audit the caller — the
  most likely site is `Admit` being called with the not-yet-assigned
  execution ID. The real root cause may be a missing `RETURNING id`
  step in `Create` that leaves the in-memory struct's ID blank.
- **Blast radius**: any paid-execution attempt that triggers this
  path returns a 500 to the user; ProfitGuard reservation may already
  have happened, leaving an orphaned wallet hold (compounds Bug #6).

### Bug #8 — GraphQL production hardening (DoD #8) not active in dev
- **Severity**: MEDIUM
- **Where**: `internal/graph/server.go` + `internal/graph/hardening/*`
  (gated by `IRONFLYER_PROD=true`)
- **Evidence** (orchestrator.log line 45):
  ```text
  "V22 GraphQL hardening enabled" prod=false max_depth=10
   complexity_limit=1000
  ```
  Note `prod=false`. CSRF/APQ middleware mounts only when
  `IRONFLYER_PROD=true`; depth+complexity caps load but introspection
  is on and persisted-query enforcement is off.
- **Fix sketch**: keep depth+complexity always-on in every env, gate
  only the harder bits (CSRF, persisted-query enforcement, masked
  errors) behind `IRONFLYER_PROD`. Add a `graphql: hardening profile`
  log line that explicitly names what is on so an operator can audit
  it without grepping source.
- **Blast radius**: dev surface accepts any query depth/complexity;
  any leak of dev token → arbitrary-cost reasoning, since BillingGuard
  is the only remaining cap.

### Bug #9 — Temporal disabled by default ⇒ DoD #6 unmet
- **Severity**: MEDIUM
- **Where**: `infra/compose/docker-compose.dev.yml` (temporal profile);
  `apps/orchestrator/internal/finisher/executor` selection on boot.
- **Evidence** (orchestrator.log line 39):
  ```text
  executor=embedded "Temporal worker disabled;
    embedded finisher executor active"
  ```
- **Fix sketch**: same approach as Bug #4 — Temporal on by default in
  dev, OR a clear startup warning that says "executions will not
  survive an orchestrator restart in this profile". Currently the
  banner is informational and easy to miss.
- **Blast radius**: any orchestrator crash mid-execution loses the
  workflow state. Combined with Bug #5, real money is at risk.

### Bug #10 — `ping` resolver/schema type mismatch surfaces in probe
- **Severity**: LOW (but trust-eroding — health probe broken)
- **Where**: `internal/graph/schema/*.graphql` declares `ping: String!`,
  but the smoke harness sends `{ ping { … } }` selection set.
- **Evidence** (`tmp/logs/noarg_query_results.txt`):
  ```text
  ping  ERR  Field "ping" must not have a selection since type
              "String" has no subfields.
  ```
- **Fix sketch**: either change `ping` to return a `PingResult { ok:
  Boolean!, version: String! }` object (preferred — gives operators
  something useful), or fix the probe harness to call `{ ping }`.
  Today the resolver also `panic`s (zbase.resolver.go:22) so the
  scalar path would crash anyway — fix both.
- **Blast radius**: any operator probe / k8s synthetic / sandbox
  ad-hoc check that hits `ping` fails.

---

### Bug #11 — `outboxhooks` schema-not-registered floods dev log
- **Severity**: MEDIUM
- **Where**: same root cause as Bug #3, but called out separately
  because it's the single noisiest line in the orchestrator log
  (10 occurrences in 90 s) and drowns out other signal.
- **Evidence**: counts grouped by subject:
  ```text
  4× ifly.dev.profitguard.decisions.v1-profitguard.decision.v1
  4× ifly.dev.billing.ledger.v1-wallet.topup.v1
  2× ifly.dev.billing.ledger.v1-wallet.hold.v1
  1× ifly.dev.execution.lifecycle.v1-execution.settled.v1
  1× ifly.dev.billing.ledger.v1-wallet.release.v1
  1× ifly.dev.billing.ledger.v1-ledger.credit_reservation.v1
  1× ifly.dev.billing.ledger.v1-ledger.credit_release.v1
  ```
- **Fix sketch**: register schemas for every `AllowedEnvs` value
  (`dev`, `staging`, `prod`) — see `events/topics.go:37`. The bulk
  registration call at `schemaregistry.go:579+` only walks the
  `Topic*` constants.
- **Blast radius**: log noise + the silent-bypass risk in Bug #3.

### Bug #12 — `IRONFLYER_AUDIT_EXPORT_HMAC_SECRET unset` ⇒ audit export disabled
- **Severity**: MEDIUM
- **Where**: boot wiring in `cmd/orchestrator/main.go` (env read) +
  `internal/audit/export/*`
- **Evidence** (orchestrator.log line 49):
  ```text
  "IRONFLYER_AUDIT_EXPORT_HMAC_SECRET unset or too short;
   audit export downloads disabled"
  ```
- **Fix sketch**: generate a per-process dev secret automatically when
  unset (with a "DEV ONLY" log), so the audit export GraphQL fields
  return signed URLs in dev too. Pair with Bug #2 — the resolvers
  `AuditExportCSVURL` / `AuditExportPDFURL` currently `panic` even
  when the secret is set.
- **Blast radius**: any feature that hangs on audit export is dark in
  dev; enterprise audit DoD line item is blocked.

### Bug #13 — Stripe disabled in lean default, no path to top-up
- **Severity**: MEDIUM
- **Where**: boot wiring + `internal/budget/stripe/*`
- **Evidence** (orchestrator.log line 17):
  ```text
  "Stripe disabled (set STRIPE_SECRET_KEY + STRIPE_WEBHOOK_SECRET)"
  ```
  Pairs with the resolver panic on `StartCheckout` (Bug #2).
- **Fix sketch**: ship a documented dev mode that uses
  `IRONFLYER_DEV_WALLET_SEED_USD` end-to-end (the v22 smoke already
  relies on it) and gate `StartCheckout` with a clear `gqlNotConfigured`
  rather than a panic. Production must surface a clear
  "Stripe required" boot error, not a runtime panic.

### Bug #14 — `ideaparser` LLM path always fails with mock provider
- **Severity**: LOW
- **Where**: `internal/ideaparser/llm_parser.go:113`
- **Evidence** (orchestrator.log line 65):
  ```text
  component=ideaparser
    error="ideaparser: llm response invalid: no JSON object in response"
    "ideaparser: llm path failed; falling back to rules"
  ```
  Happens because `ANTHROPIC_API_KEY` is unset and the mock provider
  doesn't honour the JSON-only system prompt.
- **Fix sketch**: detect mock provider in `llmParser.parse` and skip
  the LLM path silently (or have the mock provider emit a valid JSON
  envelope for the canonical idea-parse system prompt).
- **Blast radius**: every dev describeIdea call burns one wasted
  provider round-trip per request.

### Bug #15 — `pendingDeployApprovals` returns `ok` with no work
- **Severity**: LOW
- **Where**: `internal/graph/resolver/deploy.resolver.go` +
  `internal/deploy/sweeper`
- **Evidence**: `pendingDeployApprovals ok` in noarg_query_results.txt
  with no fixtures. The "approval expiry sweeper" ticks every 60s
  with nothing to do. Not a bug per se — but the DoD #10 cost-explain
  / deploy approval surface is unverified by any smoke.
- **Fix sketch**: add a `deploy approval pending → expired → reaped`
  smoke fixture to `scripts/v22_smoke.sh`.

### Bug #16 — Synthetic `profitDashboard` values are misleadingly large
- **Severity**: LOW (UX / trust)
- **Where**: `internal/dashboards/*`
- **Evidence**: fresh signup with $50 topup + $1 hold returns
  `revenueUSD: 150, providerCostUSD: 24.5, activeExecutions: 14`.
  The numbers are clearly leftover seed/demo data being summed across
  tenants.
- **Fix sketch**: scope the in-process aggregator by tenant; never
  cross-add another tenant's ledger into a user's profit dashboard.
  Verify with a multi-tenant pair in the smoke.
- **Blast radius**: any customer who sees their dashboard before
  ClickHouse is wired will get someone else's revenue number — direct
  cross-tenant leak risk.

### Bug #17 — DoD #5 SurrealDB cosmetic-unhealthy noise
- **Severity**: LOW
- **Where**: `infra/compose/docker-compose.dev.yml` SurrealDB
  healthcheck.
- **Evidence**: PROJECT_CLOSEOUT_PLAN DoD #5 notes container reports
  `unhealthy` because the bundled probe hits a 404 endpoint;
  orchestrator log line 15 confirms RPC is live.
- **Fix sketch**: change the healthcheck to `curl -fsS http://surrealdb:8000/health`
  or `wget -q --spider ws://localhost:8000/rpc`. Cosmetic noise
  pollutes any operator dashboard.

### Bug #18 — `scripts/smoke.sh` is wedged against lean dev stack
- **Severity**: LOW
- **Where**: `scripts/smoke.sh`
- **Evidence**: PROJECT_CLOSEOUT_PLAN operability table:
  ```text
  GET /projects → 404 (REST removed)
  plans / providersHealth → 422 (V22 stores unwired in lean)
  verifyAudit → 422 (operator JWT missing)
  ```
- **Fix sketch**: either delete `scripts/smoke.sh` in favour of
  `v22_smoke.sh`, or rewrite it as a thin GraphQL probe. Today it's a
  trip-wire that fires on every operator inspection.

### Bug #19 — `operatorScaleSnapshot` is operator-only with no operator dev path
- **Severity**: LOW
- **Where**: `internal/graph/resolver/operator.resolver.go:51`
- **Evidence** (noarg_query_results.txt): `operatorScaleSnapshot ERR
  operator role required`. Expected — but there is no documented dev
  way to mint an operator JWT for the smoke harness, so any
  operator-only surface is unverified.
- **Fix sketch**: add `IRONFLYER_DEV_OPERATOR_BOOTSTRAP=email@x` env
  that promotes that user to operator on signup, and add an
  operator-leg to the smoke.

### Bug #20 — Mock provider only ⇒ DoD-bench math is unrepresentative
- **Severity**: LOW
- **Where**: `internal/providers/mock/*`
- **Evidence** (orchestrator.log line 20):
  ```text
  "ANTHROPIC_API_KEY not set — running on mock provider only"
  ```
- **Fix sketch**: separate "dev profile" (mock OK) from "perf profile"
  (real Anthropic with $5 cap) so the wow-loop output is verified
  against real provider latencies before any closeout claim.

### Bug #21 — `outboxhooks: schema subject not registered` is a WARN but really a contract-drift detector
- **Severity**: LOW (severity-bump candidate — current level
  understates risk)
- **Where**: same as Bugs #3 and #11.
- **Fix sketch**: in production, escalate to ERROR (or refuse the
  insert) when a producer publishes to a subject with no registered
  schema. Today the warn is easy to miss in dashboards.

### Bug #22 — Deep code dup: 3 places re-derive "strip ifly.<env> prefix"
- **Severity**: LOW (dead-code / dup signal asked for)
- **Where**:
  - `internal/clickhouse/consumer.go:264`
  - `internal/events/topic_registration.go:118-123`
  - `internal/events/dlq.go:31`
- **Fix sketch**: extract a single `events.StripEnvPrefix(topic)
  string` helper and have the three callers use it. Reduces drift
  risk on the next topic taxonomy change.

### Bug #23 — `wireup/clickhouse.go:65` carries TODO comment about pinned topic constants
- **Severity**: LOW
- **Where**: `apps/orchestrator/internal/wireup/clickhouse.go:65`
- **Fix sketch**: resolve the TODO by routing the ClickHouse consumer
  through `events.TopicFor` so it tracks the same env as the producer
  — closes the loop for Bug #3 from the consumer side.

### Bug #24 — No SLO / on-call surface for the panic recovery counter
- **Severity**: LOW
- **Where**: `internal/graph/server.go` panic recovery handler
- **Evidence**: `graphql: panic in resolver` log line 7× in a 90 s
  smoke, but no Prometheus counter is documented to alert on it.
- **Fix sketch**: emit `graphql_resolver_panic_total{field=...}`
  counter and add a Grafana alert. Bugs #2 + #7 would have fired
  this alarm immediately.

---

## DoD vs reality (cross-link)

| DoD line | Status | Bugs |
| --- | --- | --- |
| 1. top up / run / stop / refund / inspect | partial | #6, #7 |
| 2. every cost lands in ledger | partial | (verified write path; doc note re. read mismatch may be stale) |
| 3. every execution emits to Redpanda | ✗ | #4, #3 |
| 4. ClickHouse dashboards near-real-time | ✗ | #4, #16 |
| 5. SurrealDB enrichment, not source of truth | partial | #17 |
| 6. Temporal can resume interrupted workflows | ✗ | #9, #5 |
| 7. Runtime workers scale & shrink | ✗ | #5 |
| 8. GraphQL hardened for prod | partial | #8 |
| 9. Synthetic paid execution in CI/CD | ✓ (PASS-WITH-WARN) | — |
| 10. Operators deploy / rollback / restore / explain costs | partial | #15, #18, #19 |

## Sanity baselines (verified during this pass)
- `apps/orchestrator: go vet ./...` → clean (no findings)
- `apps/runtime: go vet ./...` → clean (no findings)
- `v22 smoke` end-to-end → PASS (wallet seed + hold law-1 verified)
