# Ironflyer Feedback Brain — Closure Intelligence & Learning Plane

> Source spec: `docs/FIGMA_TO_PRODUCT_UNIFIED_PLAYBOOK_2026-05-26.md`
> §15 (Closure Intelligence) and §8 (Anti-Bloat Engine). Upstream
> contract: `docs/ARCHITECTURE_EVENTS.md` (outbox → Redpanda) and
> `docs/ARCHITECTURE_ANALYTICS.md` (ClickHouse fact tables).
> Status: **MVP wired end-to-end.** OutcomeEvent stream + ClickHouse
> consumer + Pattern Miner + Closure Intelligence + Weakness
> identification + `/cockpit/learning` (6 panels).

## Why this matters

Lovable, Base44, Bolt, Replit Agent, v0, and Cursor's agent mode all
ship the same anti-pattern: every paid run starts from zero. The
provider was picked the same way it was picked last week. The
blueprint was either ignored or guessed. The repair that fixed an
identical failure two days ago is re-invented. The gate that fails
60 % of the time on `mobile_build` is treated as a fresh surprise.

These are stateless systems wearing the costume of intelligence.

Ironflyer is the opposite. Every execution emits typed economic and
quality events into an append-only outcome stream. A miner reads that
stream on a clock, computes per-(provider, capability, path) and
per-(blueprint, intent) statistics, and rewrites the strategy the
next execution will see — which provider the bandit picks, which
blueprint is suggested, which ProfitGuard floor applies, which
repair recipe is consulted first. The Anti-Bloat Engine (§8) gave
us the static enforcement (reuse, layering, complexity); the
Feedback Brain gives us the **measured** moat. Anti-Bloat says "do
not duplicate"; the Feedback Brain says "the duplication you got
last week cost you $3.40 of margin and triggered four repair loops
— here is the patch class you should preempt."

The product law underneath is V22 Hard Law #3: **no scale is
considered healthy unless gross margin stays protected.** The
Feedback Brain is how the platform proves, per execution and per
cohort, whether scale was actually healthy.

## The 5-beat loop

```
   ┌────────────────────────────────────────────────────────────┐
   │                                                            │
   ▼                                                            │
  Run ──► Fix ──► Check ──► Learn ──► Improve ──► (loop, smarter)
   │       │        │         │         │
   │       │        │         │         └─ Strategy Adapter rewrites
   │       │        │         │            bandit / blueprint priors
   │       │        │         │            / ProfitGuard floors
   │       │        │         │
   │       │        │         └─ Pattern Miner consumes
   │       │        │            fact_outcome_events hourly
   │       │        │
   │       │        └─ Gates run, verdicts emit
   │       │           gate_outcome events
   │       │
   │       └─ patch.Engine applies, emits patch_applied;
   │          repair.Engine emits repair_triggered
   │
   └─ finisher.Engine starts an execution, emits
      execution_complete on commit
```

| Beat   | Owner                                                              | New surface |
| ------ | ------------------------------------------------------------------ | --- |
| Run    | `ai/finisher.Engine`, `business/execution`                         | emits `execution_complete`, `provider_chosen`, `blueprint_used` |
| Fix    | `operations/patch.Engine`, `ai/repair.Engine`                      | emits `patch_applied`, `repair_triggered` |
| Check  | `ai/finisher` gates (incl. Anti-Bloat lane)                        | emits `gate_outcome` |
| Learn  | `ai/learning.Miner` (hourly Temporal activity)                     | reads `fact_outcome_events`, writes `learning_*` projections |
| Improve| `ai/learning.Adapter` → bandit / blueprints / ProfitGuard         | mutates priors consulted by the next Run |

The loop closes inside one tenant and inside one workload class. We
do not cross-pollinate strategy between tenants — every metric is
scoped per `(tenant_id, workload, capability)`.

## OutcomeEvent contract

Universal event shape. One Go type, one Avro schema in the registry,
one ClickHouse fact table.

```go
// core/orchestrator/internal/ai/learning/event.go (reserved)
type OutcomeEvent struct {
    EventID      string         // ULID; idempotency key
    OccurredAt   time.Time      // canonical UTC
    TenantID     string
    ExecutionID  string         // empty for pre-execution events
    ProjectID    string
    Kind         Kind           // see table below
    Workload     string         // "standard_web" | "mobile_build" | "regulated" | ...
    Capability   string         // "reasoning" | "code" | "vision" | "cheap" | ...
    Provider     string         // "anthropic:sonnet-4.6" etc.; empty when irrelevant
    GateName     string         // domain.GateName; empty when irrelevant
    Outcome      Outcome        // "pass" | "fail" | "warn" | "skip" | "error"
    Severity     string         // "info" | "warning" | "error" | "critical"
    CostUSD      decimal.Decimal
    MarginPct    float64        // signed; negative is a regression
    DurationMS   int64
    PatchID      string
    BlueprintID  string
    RepairID     string
    Attrs        map[string]any // gate-specific; never PII
}
```

### Kinds — every emission documented

| Kind                  | Emitted by                                       | Why it matters |
| --------------------- | ------------------------------------------------ | --- |
| `execution_complete`  | `business/execution.settler` on commit           | Anchor row: revenue, total cost, completion score, margin |
| `gate_outcome`        | `ai/finisher.Engine` per `Gate.Check`            | Per-gate pass/fail/warn with severity + evidence path |
| `patch_applied`       | `operations/patch.Engine.Apply`                  | Net LOC, target paths, intent, time-to-apply |
| `repair_triggered`    | `ai/repair.Engine.Match`                         | Failure signature → recipe; reuse vs. new |
| `provider_chosen`     | `ai/providers.Router.Pick`                       | Pre-call: which provider/model the router picked + why |
| `blueprint_used`      | `business/blueprints.Use`                        | Per-execution: which starter, which version, which match score |
| `profitguard_decision`| `business/profitguard.Guard.Record`              | Verdict + reason; the Adapter learns when ProfitGuard was right |
| `completion_score`    | `ai/completion.Scorer`                           | Per-iteration score delta; feeds Scope Completion factor |

Every emission is **mandatory** for the surface that owns it. Adding
a new mutating endpoint without an `OutcomeEvent.Publish` is a
regression — the system cannot learn from what it cannot see (see
`CLAUDE.md → Conventions`).

## Data flow

```
business event (gate verdict, patch apply, provider pick, ...)
        │
        ▼
outboxhooks.WriteEventInTx(ctx, tx, OutcomeEvent) ── same Postgres tx
                                                     as the underlying
                                                     business write
        │
        ▼
event_outbox table (ACID-attached to the business mutation)
        │
        ▼  (operations/events publisher pump, at-least-once)
        ▼
Redpanda topic: ironflyer.outcome.v1
        │
        ▼  (operations/events consumer → business/clickhouse client)
        ▼
ClickHouse fact_outcome_events  ── append-only, partition by toDate(occurred_at)
        │
        ▼  (learning.Miner — Temporal activity, hourly cadence)
        ▼
learning_provider_perf  /  learning_blueprint_perf  /  learning_gate_stats
                                  │
                                  ▼  (learning.Adapter — pure function)
                                  ▼
           Bandit (lastprovider EMA)
           Blueprints (per-blueprint priors)
           ProfitGuard (margin floor per workload)
                                  │
                                  ▼
                          next execution runs smarter
```

The Postgres outbox row is the durability anchor: if Redpanda or
ClickHouse is down, the business mutation still committed and the
event is replayable. The miner is **idempotent over `(EventID,
window)`** so a replay never double-counts.

## Closure Intelligence formula

Playbook §15. The single operator-readable score in the cockpit
gauge:

```
Closure = ScopeCompletion × QualityConfidence × IntegrationStability × MarginHealth
```

| Factor                | Source                                                                                   | Acceptable | Red flag |
| --------------------- | ---------------------------------------------------------------------------------------- | ---------- | -------- |
| ScopeCompletion       | `ai/completion.Scorer`: validated work-packages / committed work-packages                | ≥ 0.80     | < 0.50   |
| QualityConfidence     | `learning_gate_stats`: pass-rate weighted by severity over last 24 h, current execution  | ≥ 0.85     | < 0.60   |
| IntegrationStability  | `learning_provider_perf`: provider + runtime healthy-ratio over rolling 5 min            | ≥ 0.95     | < 0.80   |
| MarginHealth          | `(revenue − cost_so_far − cost_remaining_est) / revenue` from ledger + forecast          | ≥ 0.15     | < 0.05   |

Reading rule for operators: **the gauge is multiplicative.** A single
factor near zero pulls the whole score down. The cockpit shows the
composite plus the four factor pills so a degraded score names what
collapsed. A run that says "running" without naming the weak factor
is a regression of the visual-first constitutional rule.

Drill-down: click the gauge → expanded panel surfaces the
per-factor sparkline (5-minute buckets) + the top three contributing
events. Code mode (VS Code / raw GraphQL) is one click further, never
the landing pane.

## GraphQL surface

The orchestrator exposes one new query and one new subscription:

```graphql
query LearningDashboard($tenantId: ID!, $window: LearningWindow! = LAST_7D) {
  learningDashboard(tenantId: $tenantId, window: $window) {
    closure {
      score
      scopeCompletion
      qualityConfidence
      integrationStability
      marginHealth
      updatedAt
    }
    banditConfidence    # 0..1; how decisive the bandit is
    gateFailureRates {  # ordered by failureRate desc
      gateName
      runs
      failureRate
      avgSeverity
    }
    blueprintSuccess {
      blueprintId
      runs
      winRate
      avgMarginPct
    }
    weaknesses(top: 5) {
      signature        # canonical "gate:mobile_build|provider:eas|workload:mobile_build"
      occurrences
      avgCostUSD
      lastSeen
      suggestedRemedy  # "add recipe", "switch provider", "raise floor", ...
    }
    recap {            # week-over-week
      runsDelta
      marginDelta
      closureDelta
      topWin           # OutcomeEvent.EventID of the best delta
      topRegression    # OutcomeEvent.EventID of the worst delta
    }
  }
  outcomeFeed(tenantId: $tenantId, kinds: [GATE_OUTCOME, PROVIDER_CHOSEN]) {
    eventId occurredAt kind outcome severity attrs
  }
}

subscription LearningPulse($tenantId: ID!) {
  learningPulse(tenantId: $tenantId) {
    eventId kind occurredAt outcome marginDelta
  }
}
```

Resolvers live in `internal/operations/graph/resolver/learning_*.go`;
sources are `business/clickhouse` (read model) + `ai/learning`
(projections). The schema fragment lands in
`schema/learning.graphql`.

## Web dashboard

Route: `/cockpit/learning`. Six panels, all dynamic-imported so the
heavy viz libs never hit the cold bundle (constitutional rule:
echarts / @xyflow/react are `next/dynamic` with `ssr: false`).

1. **LearningPulse** — top-row live counter (events/min) + sparkline
   of the last 60 minutes, bound to the `learningPulse`
   subscription. Tap → opens an event drawer with the underlying
   `OutcomeEvent` JSON (code mode, opt-in).
2. **BanditConfidence** — gauge `0..1`. Low = system is still
   exploring; high = it has converged on a provider mix. Subtitle
   names the top capability ("converged on `code` capability;
   exploring `vision`").
3. **GateFailureRate** — horizontal bar chart, ordered by failure
   rate desc, top 10 gates. Color encoded by severity. Clicking a
   bar drills to the offending OutcomeEvents in the last window.
4. **BlueprintSuccess** — scatter of (runs × win-rate), bubble size
   = avg margin. Outliers in the bottom-right (high-volume,
   low-win) are the first candidates for retirement.
5. **Weaknesses** — top-5 list of recurring failure signatures.
   Each row carries the suggested remedy and a "Propose recipe"
   action that opens a pre-filled repair-recipe draft (route to
   `ai/repair`).
6. **LearningRecap** — week-over-week deltas: runs, margin,
   closure score, plus the top win + top regression by margin
   delta. The recap is the operator's morning email rendered in
   the cockpit.

All charts pull from `chartPalette` in `components/charts/EChart.tsx`
on `tokens.color.*` — never raw hex, never lime as a primary series.

## How to query patterns

### GraphQL — operator-facing examples

```graphql
# 1. Executions that failed `security` gate in the last week.
query SecurityFailures($tenantId: ID!) {
  outcomeEvents(
    tenantId: $tenantId
    kinds: [GATE_OUTCOME]
    where: { gateName: "security", outcome: FAIL }
    window: LAST_7D
  ) {
    executionId occurredAt severity attrs
  }
}

# 2. Negative-margin providers on the `mobile_build` workload.
query MobileMarginByProvider($tenantId: ID!) {
  providerMargins(
    tenantId: $tenantId
    workload: "mobile_build"
    window: LAST_30D
  ) {
    provider runs avgMarginPct totalCostUSD
  }
}

# 3. Top 3 blueprints by win rate (≥ 10 runs).
query TopBlueprints($tenantId: ID!) {
  blueprintSuccess(
    tenantId: $tenantId
    minRuns: 10
    orderBy: WIN_RATE_DESC
    limit: 3
  ) { blueprintId runs winRate avgMarginPct }
}
```

### ClickHouse — for the operator who wants to dig

```sql
-- 1. Security gate failures, last 7 days.
SELECT execution_id, occurred_at, severity, attrs
FROM fact_outcome_events
WHERE tenant_id = {tenant:String}
  AND kind = 'gate_outcome'
  AND gate_name = 'security'
  AND outcome = 'fail'
  AND occurred_at >= now() - INTERVAL 7 DAY
ORDER BY occurred_at DESC
LIMIT 200;

-- 2. Negative-margin providers on mobile_build.
SELECT provider,
       count() AS runs,
       avgIf(margin_pct, margin_pct IS NOT NULL) AS avg_margin_pct,
       sum(cost_usd) AS total_cost_usd
FROM fact_outcome_events
WHERE tenant_id = {tenant:String}
  AND kind = 'execution_complete'
  AND workload = 'mobile_build'
  AND occurred_at >= now() - INTERVAL 30 DAY
GROUP BY provider
HAVING avg_margin_pct < 0
ORDER BY avg_margin_pct ASC;

-- 3. Top 3 blueprints by win-rate (>= 10 runs).
SELECT blueprint_id,
       count() AS runs,
       avgIf(1, outcome = 'pass') AS win_rate,
       avg(margin_pct) AS avg_margin_pct
FROM fact_outcome_events
WHERE tenant_id = {tenant:String}
  AND kind = 'execution_complete'
  AND blueprint_id != ''
  AND occurred_at >= now() - INTERVAL 30 DAY
GROUP BY blueprint_id
HAVING runs >= 10
ORDER BY win_rate DESC
LIMIT 3;
```

The GraphQL surface is the contract; the SQL is the escape hatch.
Both read the same fact table, so an operator can verify a panel
number with one SELECT.

## Operator runbook

The cockpit will surface one of four conditions. Each maps to a
fixed remediation.

### 1. `LearningPulse` drops to zero

**Likely cause:** the ingestion path is broken between
`event_outbox` and `fact_outcome_events`.

1. `kubectl -n ironflyer logs -l app=orchestrator --tail=200 | rg event_outbox` — confirm the publisher pump is alive.
2. `kubectl -n ironflyer get pods -l app=redpanda` — confirm the broker is healthy.
3. `clickhouse-client --query "SELECT max(occurred_at) FROM fact_outcome_events"` — confirm the consumer lag.
4. If publisher is alive but ClickHouse lag is growing, restart the consumer (`kubectl rollout restart deploy/clickhouse-consumer`); the events are durable in the outbox, so no data is lost.

### 2. `BanditConfidence` < 0.3 for > 24 h

**Likely cause:** not enough data, or workload is genuinely
heterogenous. **Expected on cold tenants** in the first week.

- If new tenant: leave alone. Confidence rises as `runs` clears
  ~50 across at least one (workload, capability) pair.
- If established tenant: query
  `learning_provider_perf` directly — a flat distribution across
  three providers means the routing policy is genuinely tied,
  which is fine. Force the bandit with
  `PUT /admin/learning/bandit/lock { workload, provider }` only
  if you have an out-of-band reason.

### 3. `Weaknesses` surfaces a repeating gate failure

**Action:** turn the weakness into a repair recipe.

1. Click the weakness row → opens the pre-filled draft in
   `ai/repair`.
2. The draft is keyed by the canonical signature
   (`gate:<name>|provider:<id>|workload:<class>`).
3. Approve the recipe; from the next execution on,
   `repair.Engine.Match` consults it first and the
   `repair_triggered` event will reference it.
4. The miner will pick up the new recipe automatically; no
   manual touch.

### 4. `MarginHealth` is red

**Action:** investigate cost variance with the existing
`docs/RUNBOOKS/cost-spike.md` runbook, and additionally:

1. Open `LearningRecap` → identify `topRegression`.
2. Trace the underlying `execution_id` in
   `business/dashboards` → confirm whether the regression was
   provider cost, sandbox cost, or repair churn.
3. If provider cost: the bandit will self-correct within the next
   miner window; if it doesn't (because `BanditConfidence` is
   high in the wrong direction), file a
   `learning_provider_perf` correction and rerun the adapter
   manually (`orchestrator learning adapter run --tenant ...`).
4. If sandbox cost: cross-check `ProfitGuard` decisions in
   `audit_log` — a missing `BeforeSandboxAllocation` is the
   regression, not the cost.

## Constitutional

`docs/FEEDBACK_BRAIN.md` is the source of truth for:

- The `OutcomeEvent` Kinds enumeration and Avro shape.
- The Closure Intelligence formula and per-factor sources.
- The miner cadence, idempotency contract, and adapter outputs.

`CLAUDE.md` references this document under **Conventions**: every
mutating business event emits an `OutcomeEvent` via
`learning.Publish(...)`. Adding a new mutating endpoint or resolver
without that emission is a regression even before the dashboard
shows it — the same discipline ProfitGuard expects per
`docs/ARCHITECTURE_PROFITGUARD.md`.

## File map

| Concept                       | Path |
| ----------------------------- | --- |
| OutcomeEvent type + Publish   | `core/orchestrator/internal/ai/learning/event.go` |
| Miner (Temporal activity)     | `core/orchestrator/internal/ai/learning/miner.go` |
| Adapter (pure function)       | `core/orchestrator/internal/ai/learning/adapter.go` |
| Closure scorer                | `core/orchestrator/internal/ai/learning/closure.go` |
| GraphQL resolvers             | `core/orchestrator/internal/operations/graph/resolver/learning_*.go` |
| GraphQL schema fragment       | `core/orchestrator/internal/operations/graph/schema/learning.graphql` |
| ClickHouse fact + projections | `business/clickhouse` (`fact_outcome_events`, `learning_*` views) |
| Web dashboard                 | `clients/web/src/app/cockpit/learning/` |
| Chart components              | `clients/web/src/components/learning/` |

## Related documents

- [ARCHITECTURE.md](../ARCHITECTURE.md) — V22 locked spec.
- [ARCHITECTURE_EVENTS.md](ARCHITECTURE_EVENTS.md) — outbox → Redpanda backbone.
- [ARCHITECTURE_ANALYTICS.md](ARCHITECTURE_ANALYTICS.md) — ClickHouse contract.
- [ARCHITECTURE_PROFITGUARD.md](ARCHITECTURE_PROFITGUARD.md) — the upstream economic guard that the Adapter tunes.
- [ANTI_BLOAT_ENGINE.md](ANTI_BLOAT_ENGINE.md) — static enforcement; the Feedback Brain is the measured counterpart.
- [FIGMA_TO_PRODUCT_UNIFIED_PLAYBOOK_2026-05-26.md](FIGMA_TO_PRODUCT_UNIFIED_PLAYBOOK_2026-05-26.md) §8, §15, §16.
