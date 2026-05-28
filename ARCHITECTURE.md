# Ironflyer Architecture — V22 Locked Spec

> Ironflyer is a **paid AI execution engine** that ships finished
> products end-to-end on prepaid wallet credits, with hard economic
> enforcement at every step.

The implementation contract is [`docs/V22_PLAN.md`](docs/V22_PLAN.md);
the full economic proof pack lives in
[`docs/ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/`](docs/ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/).
The ClickHouse analytics contract is
[`docs/ARCHITECTURE_ANALYTICS.md`](docs/ARCHITECTURE_ANALYTICS.md).
The policy/security/trust plane is specified in
[`docs/ARCHITECTURE_POLICY_SECURITY.md`](docs/ARCHITECTURE_POLICY_SECURITY.md).
The analytics plane is specified in
[`docs/ARCHITECTURE_ANALYTICS.md`](docs/ARCHITECTURE_ANALYTICS.md).
This document is the architectural truth — what the system is, the
objects it operates over, and the invariants that hold across every
release.

## Locked Decisions

1. **Core language**: Go (orchestrator, runtime, patch lifecycle).
2. **Web**: Next.js 15 + MUI 6 + React 19, governed by the locked
   design reference in `design-reference/2026-05-25-private-ironflyer/`.
3. **VS Code cloud workspace.** The Studio cockpit is the web product
   shell; real workspaces expose a VS Code-compatible cloud IDE through
   the runtime's code-server path and the VSCode extension remains a
   thin native-client surface.
4. **AI never mutates files directly.** Patch lifecycle is mandatory.
5. **Finisher Gates** block the loop. Spec → UX → Arch → Code → Test
   → Security → Deploy.
6. **Multi-provider** by capability + cost + tenant policy.
7. **Wallet-prepaid execution.** No execution starts without budget.
8. **ProfitGuard** decides before every expensive step.
9. **Append-only ledger.** Every economic event is recorded.

## Hot path (V22)

```
user → /graphql topUp(amount)        → Wallet.Credit, Ledger.Write
user → /graphql createExecution(...) → ProfitGuard.Admit (reserve hold)
                                       Execution.Start, Ledger.Reserve
worker iteration:
  finisher gate → ProfitGuard.BeforeStep(estCost, expectedDelta)
                  → continue | degrade | switch_provider
                    | reuse_blueprint | reuse_repair | pause
                    | stop | kill_branch
  provider call → BillingGuard.Charge(usage) → Ledger.Debit(provider_cost)
  sandbox tick → Ledger.Debit(sandbox_cost)
  patch apply  → PatchMemory.Record, RepairRecipe.MaybeRecord
  completion scorer → Execution.SetCompletionScore(delta)
Execution.Commit:
  release unused hold → Ledger.CreditBack
  Ledger.Write platform_margin entry
  Blueprint.RecordOutcome(success, cost, margin)
```

## Economic objects

| Object | Purpose |
| --- | --- |
| `Wallet` | Per-tenant prepaid credit balance with holds for active executions |
| `LedgerEntry` | Append-only debit/credit record per tenant/execution |
| `Execution` | One paid run of the finisher; tracks revenue, cost, completion score, margin |
| `Blueprint` | Reusable starter that drives cost down; tracked per-blueprint stats |
| `RepairRecipe` | Failure signature → known fix; reduces repeat repair cost |
| `PatchMemory` | Past patches keyed by intent; reused when intent matches |
| `ProfitGuardDecision` | One of: continue / degrade / pause_for_budget / stop / kill_branch / switch_provider / reuse_blueprint / reuse_repair |

## V22 Layout

```
ironflyer/
├── core/
│   ├── orchestrator/       Go — finisher + economic enforcement
│   │                       (wallet, ledger, execution, profitguard,
│   │                        blueprints, completion, repair)
│   ├── runtime/            Go — workspace runtime (containers, FS, PTY)
│   └── cli/                Go — operator CLI
├── clients/
│   ├── web/                Next.js + MUI dashboard (profit + scale +
│   │                       cohort + blueprint dashboards)
│   ├── vscode-extension/   TS — chat + gates + patches inside VSCode
│   └── scrcpy-bridge/      scrcpy WebSocket bridge for mobile mirroring
├── packages/
│   ├── design-tokens/      IronFlyer locked-reference tokens
│   └── sdk/                Client SDK (TS)
├── infra/                  compose / dockerfiles / k8s / helm
└── docs/
    ├── V22_PLAN.md         Implementation contract
    └── ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/
                            Full economic model + acceptance gates
```

`core/inference/` (ONNX private AI) and `clients/mobile/` (PWA shell)
have been retired for V22 — both are deferred to Milestone 3+.

> Domain ownership view: [docs/ARCHITECTURE_DOMAIN_MODULES.md](docs/ARCHITECTURE_DOMAIN_MODULES.md).

## Core Loop (gates)

```
Idea
  → Product Spec       (Spec gate)
  → UX Flow            (UX gate)
  → Architecture       (Arch gate)
  → Code Generation    (Code gate)
  → Validation         (Test gate)
  → Security Review    (Security gate)
  → Deployment Ready   (Deploy gate)
  → Completion Score   (per-execution score delta written to ledger)
```

Every gate implements `Check(project) []Issue`. Failed gates dispatch
a targeted repair task to the matching agent. The loop terminates only
when all gates pass, max iterations is hit, the wallet runs out, the
user intervenes, or ProfitGuard stops the run.

## Agent Runtime Model

- Stateless functions with typed input/output JSON Schemas.
- Orchestrator owns state. Agents are pure.
- Every call routed through Provider Router (capability + cost +
  policy + ProfitGuard verdict).
- All outputs validated against schema. Retry-with-feedback on
  failure (counted against the wallet).

## Provider Router

- **Capability tags** per request: `reasoning`, `code`, `json`,
  `vision`, `cheap`, `fast`.
- **Per-tenant policy**: wallet balance, ProfitGuard reservation,
  preferred provider, data residency.
- **Fallback chain** on provider error.
- **Streaming first.** Every provider implements `CompleteStream`;
  the BillingGuard charges through that stream so every token lands
  in the ledger.

## Patch Lifecycle

```
propose → validate (syntax, types, security, scope)
        → preview diff
        → approve (auto if low-risk policy met, manual otherwise)
        → apply (atomic, transactional FS)
        → snapshot (git-backed)
        → verify (tests + gates)
        → rollback on verification failure
```

PatchMemory records every apply keyed by intent so subsequent
executions can reuse a known-good patch instead of regenerating it.

## Workspace

- Phase 1: Docker rootless container per user; PTY exposed via WS to
  xterm.js.
- Phase 2: Firecracker microVMs for stronger isolation.
- All file operations go through the runtime FS API (audit +
  versioning).
- Runtime scale, sandbox isolation, snapshotting, quotas, warm pools,
  autoscaling, scale-to-zero, and sandbox billing ticks are specified in
  [`docs/ARCHITECTURE_RUNTIME_SCALE.md`](docs/ARCHITECTURE_RUNTIME_SCALE.md).

## Communication

- GraphQL for the public API (queries, mutations, subscriptions via
  `graphql-transport-ws` on the same path).
- REST is reserved for k8s probes, Prometheus, and the Stripe
  webhook only.
- gRPC between Go services when latency demands it.
- Temporal is the durable command runner for long executions; it does
  not replace the event backbone. See
  [`docs/ARCHITECTURE_WORKFLOWS.md`](docs/ARCHITECTURE_WORKFLOWS.md).
- Durable asynchronous events use the Postgres outbox -> Redpanda
  backbone defined in [`docs/ARCHITECTURE_EVENTS.md`](docs/ARCHITECTURE_EVENTS.md);
  Redis remains the ephemeral bus for live fan-out, locks, rate limits,
  and short-lived state.

## Storage

- **Postgres** — wallets, ledger, executions, blueprints, repair
  recipes, ProfitGuard decisions, projects, users, gates, patches; the
  source of truth for wallet, ledger, and execution state.
- **ClickHouse** *(opt-in)* — analytics read model for profit, cost, margin,
  cohort, blueprint, and scale dashboards; never authoritative for wallet,
  ledger, or execution state. The lean deployment omits ClickHouse +
  Redpanda entirely (`--profile analytics` enables them): the orchestrator
  skips the event publisher and the dashboards fall back to live Postgres
  reads via their adapter sources. ClickHouse is a performance tier for high
  event volume, not a correctness dependency.
- **SurrealDB** — AI Memory Graph for project/code graph, agent memory,
  repair-genome context, vector/hybrid retrieval, and spec → files →
  patches → failures relations. It stores derived retrieval context only;
  never wallet, ledger, or execution truth. See
  [`docs/ARCHITECTURE_MEMORY_GRAPH.md`](docs/ARCHITECTURE_MEMORY_GRAPH.md).
- **Redis** — distributed locks, rate limit budgets, ephemeral state.
- **ClickHouse** — replayable analytics projections for cost, margin,
  cohorts, provider performance, gate failures, and scale dashboards.
  See [`docs/ARCHITECTURE_ANALYTICS.md`](docs/ARCHITECTURE_ANALYTICS.md).
- **MinIO / R2 / S3** — snapshots, exports.
- **pgvector** — optional development/compatibility fallback for simple
  semantic search; not the canonical V22 memory graph.

## Learning Plane

Every execution emits typed `OutcomeEvent`s (execution_complete,
gate_outcome, patch_applied, repair_triggered, provider_chosen,
blueprint_used, profitguard_decision, completion_score) through the
Postgres outbox → Redpanda → ClickHouse `fact_outcome_events` path.
A hourly Pattern Miner (`internal/ai/learning`) projects per-tenant
provider / blueprint / gate statistics; the Strategy Adapter
rewrites bandit priors, blueprint priors, and ProfitGuard floors so
the next execution starts smarter. Closure Intelligence (Scope ×
Quality × Integration × Margin) is the single operator-readable
score. Full contract:
[`docs/FEEDBACK_BRAIN.md`](docs/FEEDBACK_BRAIN.md).

## Observability

- OpenTelemetry traces across all Go services.
- Per-execution agent trace tree (planner → coder → reviewer →
  completion scorer).
- Ledger entries flow into the profit / scale / cohort / blueprint
  dashboards exposed via GraphQL.
- ProfitGuard decisions are an audit-log row each.
