# Ironflyer V22 — Architecture Overhaul Plan

> Source of truth for the V22 transition. Every agent working on this overhaul
> reads this file first and follows it. If the code conflicts with V22, the code
> changes — not the plan.

## V22 in one paragraph

Ironflyer becomes a **paid AI execution engine** that ships finished products
end-to-end on prepaid wallet credits, with hard economic enforcement at every
step. The North Star is **Profitable Completed Execution Rate**: how many paid
executions complete successfully *with* positive gross margin. Every execution
is a measured economic unit — revenue and cost attributed per-execution,
recorded in an append-only ledger, gated by **Profit Guard** before any
expensive call.

Read the proof pack at
[`docs/ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/`](ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/)
for the full economic model. The README, executive section, and unit economics
documents are the spec — this V22_PLAN is the implementation contract.

## Hard architectural laws (from the proof pack)

1. **No execution starts without budget.** Wallet balance ≥ reservation, or
   the API returns 402 Payment Required with a `top_up_url`.
2. **No expensive reasoning runs without expected ROI.** Profit Guard gates
   every premium model call, sandbox allocation, mobile build, Vercel deploy,
   retry loop, long verification, and large artifact write.
3. **No scale is considered healthy unless gross margin stays protected.**
   Profit dashboards surface margin first; scale dashboards only matter when
   margin is healthy.

## Economic objects (the new domain)

| Object | Purpose |
| --- | --- |
| `Wallet` | Per-tenant prepaid credit balance with holds for active executions |
| `LedgerEntry` | Append-only debit/credit record per tenant/execution |
| `Execution` | One paid run of the finisher; tracks revenue, cost, completion score, margin |
| `Blueprint` | Reusable starter that drives cost down; tracked per-blueprint stats |
| `RepairRecipe` | Failure signature → known fix; reduces repeat repair cost |
| `PatchMemory` | Past patches keyed by intent; reused when intent matches |
| `ProfitGuardDecision` | One of: continue / degrade / pause_for_budget / stop / kill_branch / switch_provider / reuse_blueprint / reuse_repair |

## Hot path

```
user → /graphql topUp(amount)       → Wallet.Credit, Ledger.Write
user → /graphql createExecution(...) → ProfitGuard.Admit (reserve hold)
                                       Execution.Start, Ledger.Reserve
worker iteration:
  finisher gate → ProfitGuard.BeforeStep(estCost, expectedDelta)
                  → continue | degrade | switch_provider | reuse_blueprint
                    | reuse_repair | pause | stop | kill_branch
  provider call → BillingGuard.Charge(usage) → Ledger.Debit(provider_cost)
  sandbox tick → Ledger.Debit(sandbox_cost)
  patch apply → PatchMemory.Record, RepairRecipe.MaybeRecord
  completion scorer → Execution.SetCompletionScore(delta)
Execution.Commit:
  release unused hold → Ledger.CreditBack
  Ledger.Write platform_margin entry
  Blueprint.RecordOutcome(success, cost, margin)
```

## Code that goes away (does not serve V22)

The V22 plan is about closing the loop on paid execution + profit proof. The
following packages do not directly serve that loop and become noise we have to
maintain. Delete them and their wiring (GraphQL schema, resolvers, HTTP
handlers, store wiring, migrations, env vars, CLAUDE.md mentions):

| Package | Reason for removal |
| --- | --- |
| `internal/affiliates/` | Marketing/growth feature; not on 30/60/90 plan |
| `internal/leads/` | Pre-paid signup capture; V22 starts at paid wallet |
| `internal/brainstorm/` | Free reasoning burner; replaced by Blueprint selection |
| `internal/collab/` | Multi-user collab; not on milestone plan |
| `internal/prreview/` | PR-review SaaS; orthogonal to finisher |
| `internal/sharelinks/` | Shareable demo links; not on milestone plan |
| `internal/figma/` | Figma import is Milestone 4 (90-day plan) |
| `internal/imagegen/` | Image generation; not on milestone plan |
| `internal/context7/` | External docs tool; cost without measured ROI |
| `internal/integrations/` | Generic integrations; nothing on the milestone plan needs it |
| `internal/projectgraph/` | Knowledge-graph experiment; SurrealDB-only, no V22 callers |
| `internal/taskgraph/` | DAG explorer; finisher engine owns step graph already |
| `internal/domains/` | Custom domain mapping; Milestone 4+ |
| `internal/sentryext/` | Sentry extension; replace with simple structured errors |
| `internal/webhooks/` | Generic outbound webhooks; not on plan |
| `internal/chats/` | Chat history persistence; non-essential for paid execution proof |
| `internal/auth/saml.go` | Enterprise SSO; Milestone 4 |
| `internal/auth/ipallowlist.go` | Enterprise governance; Milestone 4 |
| `internal/auth/mfa.go` | Hold for Milestone 3+ |
| `internal/budget/dunning.go` | Wallet model has no dunning — no balance = no execution |
| `internal/finisher/figma_translator.go` | Figma import deferred |
| `internal/finisher/mobile_scaffolder.go` | Mobile build is 90-day |
| `internal/finisher/{kotlin_android,swift_ios,dotnet,java_spring,laravel,rails,rust,supabase,shadcn,hono,bun,express,python_fastapi,go_http,vite_react,remix,social,stripe}_scaffold*.go` | 30+ scaffolders collapse to a 3-blueprint data-driven registry |
| `apps/inference/` | ONNX inference service is reserved/unused; deferred until Milestone 3 |
| `apps/mobile/` | PWA mobile shell; not on 30/60-day plan |

## Code that gets built (the new V22 surface)

### Packages to create

```
core/orchestrator/internal/
  business/wallet/        — Per-tenant prepaid balance + holds (top-up, reserve, release, debit)
  business/ledger/        — Append-only ledger; debit/credit; reconcile; query
  business/execution/     — Execution entity + FSM + cost attribution + completion score
  business/profitguard/   — Decide(ctx, ExecState) → Decision; hooks for provider/runtime/finisher
  business/blueprints/    — Data-driven blueprint registry (YAML + Go executor); per-blueprint stats
  ai/completion/          — Completion scorer (gate progress → score delta)
  ai/repair/              — Repair genome (failure_signature → fix recipe) + patch memory
```

> Historical note: the table below and the rest of this document still
> reference the original flat `internal/<pkg>/` paths. Those packages
> now live under the five-domain layout — see
> [ARCHITECTURE_DOMAIN_MODULES.md](ARCHITECTURE_DOMAIN_MODULES.md) for
> the current home of each package.

### Migrations to add

```
migrations/00024_wallets.sql              — wallets(tenant_id, balance_usd, hold_usd, …)
migrations/00025_ledger.sql               — ledger_entries from V22 proof pack
migrations/00026_executions.sql           — executions(id, tenant_id, budget, spent,
                                            reserved, completion_score, gross_margin, status)
migrations/00027_blueprints.sql           — blueprint_runs, blueprint_stats
migrations/00028_repair_genome.sql        — repair_recipes(signature, fix_json, hits)
migrations/00029_profitguard_decisions.sql — profit_guard_decisions audit
migrations/00030_purge_legacy.sql         — DROP tables for removed packages
```

### GraphQL operations to add

```
mutation topUp(amountUSD: Float!) → CheckoutSession { url }
mutation createPaidExecution(input: CreatePaidExecutionInput!) → Execution
query wallet → Wallet { balanceUSD, holdUSD, ledgerCursor }
query execution(id) → Execution { …costs…, completionScore, grossMargin, decisions }
subscription executionFeed(id) → ExecutionEvent
query profitDashboard(window) → ProfitDashboard
query scaleDashboard(window)  → ScaleDashboard
query cohortDashboard(window) → CohortDashboard
query blueprintDashboard(window) → BlueprintDashboard
```

## Agent assignments (one prompt per agent)

The foundation agent runs first and must finish before the parallel agents
start (they all need a clean tree to build against). The parallel agents own
disjoint package paths so they cannot collide.

| Agent | Owns | Depends on |
| --- | --- | --- |
| 1. Foundation | top-level docs + purge + migration 00030 + restore `go build` | nothing |
| 2. Wallet     | `internal/wallet/` + Stripe top-up + GraphQL wallet ops      | Foundation |
| 3. Ledger     | `internal/ledger/` + migration 00025 + writer hooks          | Foundation |
| 4. Execution  | `internal/execution/` + migration 00026 + executor FSM       | Foundation |
| 5. ProfitGuard| `internal/profitguard/` + migration 00029 + enforcement hooks| Foundation |
| 6. Blueprints | `internal/blueprints/` + migration 00027 + 3 starter blueprints | Foundation |
| 7. Completion + Repair + Dashboards | `internal/completion/` + `internal/repair/` + migration 00028 + 4 dashboard GraphQL ops | Foundation |
| 8. Integration loop | cross-wire ProfitGuard hooks; regenerate gqlgen; `go build ./...` green | Agents 2-7 |

## Acceptance for the overhaul (from proof pack `06-acceptance`)

- [ ] Wallet exists, top-up via Stripe writes ledger credit
- [ ] No execution starts unless wallet hold succeeds
- [ ] Provider cost lands as ledger debit on every charged token
- [ ] Sandbox cost ticks lands as ledger debit
- [ ] ProfitGuard.Decide called at every enforcement point listed in proof pack
- [ ] Stop-loss, kill_branch, switch_provider, reuse_blueprint, reuse_repair all exercised
- [ ] Blueprint stats recorded per execution
- [ ] Completion score recorded per execution; completion-per-dollar derivable
- [ ] Profit, scale, cohort, blueprint dashboards return data via GraphQL
- [ ] `go build ./...` passes in both Go modules
- [ ] CLAUDE.md / ARCHITECTURE.md / README.md reflect V22 model

## Out of scope for this overhaul

- Web UI implementation was originally out of scope for this overhaul,
  but any follow-on web work is now bound to
  `design-reference/2026-05-25-private-ironflyer/` and
  `clients/web/DESIGN_REFERENCE.md`.
- Tests (per repository policy)
- VSCode extension changes beyond what GraphQL schema breakage requires
