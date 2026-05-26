# ARCHITECTURE_PROFITGUARD

> V22 Hard Law #2: **no expensive reasoning runs without expected ROI.**
> Every premium model call, sandbox allocation, mobile build, production
> deploy, retry loop, long verification, and large artifact write passes
> through `profitguard.Guard.Decide` before it runs. This document is
> the locked coverage matrix — drift is a regression.

## Layout

```
core/orchestrator/internal/business/profitguard/
├── decision.go          Decision verdict shape (Action, Reason, Margin, ...)
├── errors.go            ErrInvalidState (snapshot adapter contract)
├── guard.go             Guard.Decide algorithm + Record + audit fan-out
├── metrics.go           ironflyer_profitguard_decisions_total
├── policy.go            DefaultPolicy + per-workload margin floors
├── state.go             ExecState — pure snapshot input
├── store.go             MemoryStore + PostgresStore + WithOutbox
└── types.go             Action / EnforcementPoint wire constants

core/orchestrator/internal/business/profitguardbridge/
└── bridge.go            execution.State + provider quotes → ExecState

core/orchestrator/internal/business/profitguardctx/
└── profitguardctx.go    WithExecution(ctx, execID, tenantID) propagation
```

The policy module is a **pure function of (Policy, EnforcementPoint,
ExecState)**: no store reads, no provider calls, no wallet writes. This
is what makes the layer cheap enough to run on every step.

## Enforcement-point coverage

| EnforcementPoint              | Call site                                                              | Guarded BEFORE | Notes |
| ----------------------------- | ---------------------------------------------------------------------- | -------------- | ----- |
| `BeforeModelCall`             | `providers.BillingGuard.CompleteStream`                                | ✅              | applyDecision(BeforeModelCall) |
| `BeforeModelCall`             | `providers.BillingGuard.CompleteStreamWithFailover`                    | ✅ (V22 fix)    | dominant agents path; previously bypassed ProfitGuard |
| `BeforePremiumReasoning`      | `providers.BillingGuard.CompleteStream` (when caps include premium)    | ✅              | second Decide pass after BeforeModelCall |
| `BeforePremiumReasoning`      | `providers.BillingGuard.CompleteStreamWithFailover` (V22 fix)          | ✅              | matches CompleteStream contract |
| `BeforeSandboxAllocation`     | `finisher.Engine.guardSandboxAllocation` (Run loop)                    | ✅              | runs before sandbox biller starts ticking |
| `BeforeSandboxAllocation`     | `runtime/internal/operations/allocator.Allocate`                       | ✅              | runtime admission funnel checks WithProfitGuard ctx |
| `BeforeRetryLoop`             | `finisher.Engine.runCoderRetryLoop`                                    | ✅              | ProfitGuardHook.BeforeRetry |
| `BeforeRetryLoop`             | `finisher.recovery.runRecoveryLoop`                                    | ✅              | one Decide per attempt |
| `BeforeRetryLoop`             | EAS retry loop (`eas/client.go`)                                       | ⚠️ deferred    | tight transient-error retry, max 3, bounded cost per attempt |
| `BeforeMobileBuild`           | `finisher.MobileBuildGate.runMobileBuilds` (V22 fix)                   | ✅              | per (kind, target); Stop/Kill skips gradlew/xcodebuild |
| `BeforeMobileBuild`           | GraphQL `mobileTriggerBuild` resolver (V22 fix)                        | ✅              | EAS build trigger |
| `BeforeMobileBuild`           | GraphQL `mobileSubmitToStore` resolver (V22 fix)                       | ✅              | EAS store submission |
| `BeforeMobileBuild`           | GraphQL `mobilePublishUpdate` resolver (V22 fix)                       | ✅              | EAS OTA update |
| `BeforeMobileBuild`           | `wireup.ReserveMobileBuild`                                            | ✅              | already in place (per-execution wallet reservation) |
| `BeforeVercelDeploy`          | `deploy.MemoryService.Promote` / `deploy.PostgresService.Promote`      | ✅              | GuardDeploy (production env only) |
| `BeforeVercelDeploy`          | `deploy.PostgresService.Open` (production env)                         | ✅              | GuardDeploy at Plan time |
| `BeforeArtifactStore`         | `patch.Engine.applyToWorkspace` (artifacts > 1 MiB)                    | ✅              | ArtifactStoreHookAdapter |
| `BeforeLongVerification`      | `finisher.Engine.runReviewer` (when est > 60s)                         | ✅              | LongVerificationHookAdapter |
| `BeforeExecutionAdmit`        | execution lifecycle admit                                              | ✅              | wallet hold + admit gate |

### Sandbox specifics — Mac pool refusal

Per playbook §10 the iOS native + bare-RN iOS Mac-host path is **Pro
tier only**: ProfitGuard refuses Mac allocations that would push the
user's wallet negative. This is enforced at three layers:

1. **Static refusal at the gate.** `MobileBuildGate` checks
   `IRONFLYER_MAC_POOL_ENABLED=1` AND `m.NeedsMacHost()` BEFORE asking
   the runtime to launch xcodebuild. Without the env var, the gate
   degrades to `SeverityInfo: deferred to EAS cloud` so the build is
   never attempted on this pod.
2. **ProfitGuard supply cap.** `Policy.MaxNextStepUSDByWorkload`
   includes `mobile_build → $5.00`, so a single Mac-minute step
   projected above $5 trips `supply_cap` Stop verdict.
3. **Runtime allocator.** `core/runtime/internal/operations/allocator`
   requires `X-Ironflyer-ProfitGuard: ok` on every workspace POST and
   refuses allocation when the header is missing — this is checked
   AFTER the orchestrator's BeforeSandboxAllocation hook fires.

## Audit chain

Every `Guard.Record` call now emits two durable rows:

1. **profit_guard_decisions table** — the canonical durable log
   (migration `00029_profitguard_decisions.sql`). Postgres-backed
   stores additionally emit a `profitguard.decisions.v1` outbox event
   when `WithOutbox()` is set, so dashboards can replay every decision
   offline.
2. **Audit chain (`audit.Store`)** — V22 introduces
   `profitguard.AuditSink` and `NewWithAuditSink(policy, store, sink)`.
   The wireup helper `wireup.NewProfitGuardAuditSink` lands one
   `EventProfitGuardDecision` row per Decide call via
   `audit.RecordProfitGuardDecision`. The row carries the canonical
   attrs:
   - `tenant_id`, `execution_id`, `enforcement_point`, `action`,
     `reason`
   - `est_cost_usd`, `spent_usd`, `reserved_usd`
   - `expected_margin_pct`, `risk_score` (when present)
   - `recommended_provider` (on SwitchProvider verdicts)
   - `model`, `provider`, `resolver_action`, `mobile_kind`,
     `mobile_target`, `project_id` (when stamped in decision metadata)

   Audit emission is best-effort: a flaky audit chain does not block
   the per-Record return or the underlying gated action.

Wire-up snippet (cmd/orchestrator/main.go style):

```go
auditSink := wireup.NewProfitGuardAuditSink(auditStore)
profitGuard := profitguard.NewWithAuditSink(policy, pgStore, auditSink)
```

## Open gaps (honestly deferred)

- **EAS client transient-error retry loop** (`eas/client.go:299`).
  Bounded at 3 retries per call, each retry only repeats the same HTTP
  request (no fresh provider tokens). Per-step cost cap means a single
  rate-limited request cannot exceed the workload budget. Adding a
  per-iteration Decide would require threading a guard into the HTTP
  client; the cost/benefit favours leaving this loop ungated and
  relying on the surrounding `BeforeMobileBuild` envelope to refuse
  the outer build.
- **Domain registrar purchase** (`deploy.domain_purchase.go`). Capped
  at $75 by `DomainPurchasePolicy.MaxPriceUSD`. Not on the
  EnforcementPoint enumeration. If we expand to bulk domain seizures
  we should add `BeforeDomainPurchase` to `profitguard.types.go` and
  gate `MemoryDomainService.PurchaseDomain` /
  `PostgresDomainService.PurchaseDomain` accordingly.
- **`ideaparser.LLMParser.Parse`** — calls `router.Complete` directly
  rather than going through `BillingGuard`. This is an unmetered
  internal call invoked once per `describeIdea` GraphQL mutation
  (token-cap-bounded by `budget.DefaultPromptCap`) and its tenant /
  executionID are not yet on ctx at that call site. Listed for a
  future pass that threads ProfitGuard through the parser.
- **Haiku-tier model calls.** Per cost policy, Haiku 4.5 is "cheap" —
  the BeforeModelCall hook still fires for it, but the margin floor
  for `standard_web` (45%) treats it as a freely-billable tier. No
  fix needed.

## Coverage discipline

When you add a new expensive surface (a new external API, a new
runtime command, a new build target), the V22 contract says **wire a
Decide BEFORE the call lands** AND update this table. The metrics
counter `ironflyer_profitguard_decisions_total{enforcement_point=...}`
is the operator-side pulse — a new surface that bypasses Decide will
not appear in the metric, which is a regression even before the
margin dashboard shows it.

The decision lifecycle is:

```
Snapshot → Decide → Record → metrics tick + audit row + (optional) outbox event
                  ↓
                  Action ∈ {Continue | Degrade | SwitchProvider |
                            ReuseBlueprint | ReuseRepair |
                            PauseForBudget | Stop | KillBranch}
```

The caller is responsible for honouring the verdict. Common patterns:
- `BillingGuard.applyDecision` (model calls) — handles SwitchProvider
  + Degrade automatically by mutating `req`.
- `wireup.MobileBuildHookAdapter` (mobile builds) — surfaces the
  verdict as a string; the gate translates it into a degraded issue
  on Stop/Kill/Pause.
- `deploy.GuardDeploy` — translates Stop/Kill/Pause into
  `ErrProfitGuardBlocked`.
