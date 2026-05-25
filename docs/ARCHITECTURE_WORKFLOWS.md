# Ironflyer Temporal / Durable Workflows Plan

This document defines how Ironflyer should use Temporal for long-running paid
executions without replacing Redpanda or changing the V22 economic model.

## Boundary

Temporal is the durable command runner. It owns execution state transitions,
timers, retries, cancellation, approval waits, and compensation.

Redpanda remains the event backbone. It carries execution events, UI fan-out,
analytics, notifications, audit mirrors, and downstream dashboard streams.
Temporal history is not an analytics log and should not be used as a pub/sub
bus.

The bridge between them is an outbox:

1. Workflow activities mutate Postgres under the existing domain services.
2. The same activity writes an `execution_events` / outbox row with an
   idempotency key.
3. A publisher emits that event to Redpanda.
4. External events that must affect an execution, such as approval, budget
   top-up, deploy webhook, or sandbox lifecycle callback, are consumed from
   GraphQL/Redpanda and delivered to Temporal as workflow signals.

This keeps Redpanda as the observable event stream while Temporal handles the
long-lived execution command.

## Workflow Model

Primary workflow:

```text
FinisherExecutionWorkflow(executionID, tenantID, projectID, budget, policy)
```

Workflow id is `execution:{executionID}`. Starting the same execution twice
must return the existing workflow handle instead of launching another run.
Temporal run id is operational only; Ironflyer domain identity is always the
execution id.

The workflow is deterministic and contains no direct HTTP, SQL, filesystem,
provider, or clock calls. Those are activities. The workflow decides ordering:

```text
AdmitExecution
EnsureSandbox
StartExecution
for each finisher gate / repair iteration:
  ProfitGuardBeforeStep
  RunGate or RunAgentStep
  RecordProviderCost / RecordSandboxCost
  ProposePatch
  ValidatePatch
  WaitForHumanApproval when required
  ApplyPatch
  VerifyPatch
DeployCandidate
WaitForDeployApproval when required
PromoteDeploy
SettleExecution
CleanupSandbox
```

Long waits are workflow timers or signal waits, not sleeping goroutines in an
orchestrator pod. A workflow can wait hours or days for a user approval,
budget top-up, deploy callback, or sandbox resume while preserving its exact
state.

## Activities

Activities wrap the existing Go services; Temporal should not introduce a
second implementation of wallet, ledger, ProfitGuard, patch lifecycle, or
runtime clients.

| Activity | Existing owner reused | Notes |
| --- | --- | --- |
| `AdmitExecutionActivity` | wallet, execution, ledger, ProfitGuard | Places wallet hold, writes reservation entry, transitions created -> admitted. |
| `StartExecutionActivity` | execution | Transitions admitted -> running. Idempotent if already running. |
| `ProfitGuardBeforeStepActivity` | profitguard, execution, provider router | Returns continue/degrade/pause/stop/kill/switch/reuse decision. |
| `RunGateActivity` | finisher gates | Runs one gate with a bounded timeout and returns structured issues. |
| `RunAgentStepActivity` | agents registry, providers BillingGuard | Performs provider call; provider cost is attributed by existing BillingGuard hooks. |
| `ProposePatchActivity` | patch engine | Creates a patch candidate; no direct AI filesystem mutation. |
| `ValidatePatchActivity` | patch engine, security gates | Syntax/types/security/scope validation. |
| `ApplyPatchActivity` | runtime applier | Atomic runtime FS apply plus snapshot/commit stamp. |
| `RollbackPatchActivity` | runtime applier | Compensation for failed verification after apply. |
| `EnsureSandboxActivity` | runtime client | Creates or reattaches workspace by idempotency key. |
| `SuspendSandboxActivity` | runtime client, snapshot store | Used during long human waits to stop compute burn. |
| `CleanupSandboxActivity` | runtime client | Always scheduled during terminal workflow cleanup. |
| `DeployCandidateActivity` | deploy adapter | Builds preview/candidate deploy with idempotency key. |
| `PromoteDeployActivity` | deploy adapter | Production promotion after policy/manual approval. |
| `RollbackDeployActivity` | deploy adapter | Compensation for failed promotion or explicit cancel. |
| `SettleExecutionActivity` | execution Settler, wallet, ledger, blueprint stats | Releases unused hold, records platform margin, records blueprint outcome. |
| `EmitExecutionEventActivity` | execution events/outbox | Writes durable event for Redpanda publisher and GraphQL backfill. |

Task queues should split resource classes once volume requires it:
`ironflyer-finisher`, `ironflyer-provider`, `ironflyer-runtime`,
`ironflyer-deploy`, and `ironflyer-billing`. Phase 1 can keep one queue,
`ironflyer-finisher`, because the current config already names it.

## Retry Policy

Temporal retries infrastructure failures; business decisions remain explicit
workflow branches.

Default activity retry:

```text
initial interval: 2s
backoff coefficient: 2.0
maximum interval: 2m
maximum attempts: 6
```

Use narrower policies by category:

| Category | Retry | Non-retryable |
| --- | --- | --- |
| Postgres transaction conflict, network timeout, 5xx | Yes | Schema/validation errors |
| Provider transient failure | Yes, low attempts, then router fallback | ProfitGuard stop, tenant policy violation |
| Patch validation failure | No automatic retry of same patch | Invalid patch, forbidden path, security failure |
| Runtime sandbox create/exec | Yes, with heartbeat and reattach | Owner check failure, invalid workspace |
| Deploy provider 5xx/webhook timeout | Yes | Invalid deploy config, missing secret |
| Billing/settlement write | Retry until settled or operator intervention | Invalid amount/programming error |
| Human approval wait | Timer/signal, not activity retry | Approval rejected, approval expired |

Provider calls that may charge money must use short, explicit attempts. A
failed provider call can still be billable; BillingGuard records actual cost
when usage is known. The retry path must re-enter ProfitGuard before the next
expensive attempt.

## Compensation

Ironflyer uses saga-style compensation. We never mutate ledger history to
"undo" money; corrections are new compensating entries.

| Completed step | Compensation |
| --- | --- |
| Wallet hold placed | Release unused hold on stop/fail/kill. |
| Provider call charged | No reversal; keep provider cost and let settlement compute true margin. |
| Sandbox allocated | Snapshot if useful, then suspend/cleanup; bill actual lifetime. |
| Patch applied | Roll back to the prior git/runtime snapshot, then record repair failure. |
| Deploy preview created | Delete preview or mark inactive when provider supports it. |
| Production deploy promoted | Roll back to prior deployment/version when available; otherwise mark manual incident. |
| Top-up credited | Never compensate automatically; refunds are explicit ledger entries. |
| Blueprint stats recorded | Write idempotently by execution id so duplicate settlement cannot double-count. |

Workflow cancellation, timeout, or worker crash must run terminal cleanup:
stop/kill execution, cleanup or suspend sandbox, settle wallet/ledger, and emit
terminal event. Cleanup activities need their own retry policy and should use a
disconnected context in Temporal terms so cancellation of the main workflow
does not skip settlement.

## Idempotency

Temporal gives at-least-once activity execution. Every activity that mutates
state must be safe to run more than once.

Idempotency keys:

```text
execution:{executionID}:admit
execution:{executionID}:start
execution:{executionID}:gate:{gate}:{iteration}
execution:{executionID}:agent:{stage}:{iteration}:{attempt}
execution:{executionID}:patch:{patchID}:apply
execution:{executionID}:sandbox:{workspaceID}:tick:{bucketStart}
execution:{executionID}:deploy:{target}:{candidateID}
execution:{executionID}:settlement
execution:{executionID}:event:{eventType}:{sequence}
```

Required storage guarantees before production Temporal rollout:

- `ledger_entries` needs a stable operation/idempotency key or a unique
  caller-supplied id per economic leg. Current append-only semantics are
  correct, but settlement retries can duplicate entries unless writes collapse
  by key.
- Wallet top-up is already idempotent by Stripe session id. Holds, debits, and
  releases need execution-scoped operation keys for Temporal retries.
- Execution transitions are already FSM-guarded; retrying `Start`, `Succeed`,
  `Fail`, `Stop`, and `Kill` should treat the desired current/terminal state
  as success.
- Sandbox create/attach should accept an idempotency key and return an
  existing workspace when the same execution already owns one.
- Deploy adapters must pass provider idempotency keys where supported and
  persist provider deployment ids before waiting on webhooks.
- Outbox/Redpanda publishing should dedupe by event id; consumers must remain
  idempotent because Redpanda delivery is still at-least-once.

## Human Approval

Manual approval is a workflow state, not a blocked HTTP request.

Approval points:

- High-risk patch apply.
- Security gate exception.
- Production deploy promotion.
- Budget top-up / continuation after `pause_for_budget`.
- ProfitGuard stop-loss override.

Flow:

1. Workflow emits `approval_requested` to the outbox/Redpanda stream.
2. GraphQL exposes the pending approval with execution id, gate, risk, diff,
   cost impact, expiry, and allowed actions.
3. Workflow waits for `ApprovalReceived`, `ApprovalRejected`, `BudgetToppedUp`,
   or timeout.
4. During long waits the workflow should suspend the sandbox unless a live
   preview is explicitly being billed.
5. Approval/rejection is recorded as an execution event and audit row.

Timeout policy is per approval type. Patch approvals can expire quickly
(minutes/hours). Budget and production deploy approvals can wait longer, but
the sandbox should not keep billing unless the tenant explicitly chooses that.

## Sandbox Lifecycle

Temporal should make workspace lifetime explicit:

```text
EnsureSandbox -> lease heartbeat -> optional suspend/resume -> cleanup
```

The workflow stores `workspaceID` in workflow state after
`EnsureSandboxActivity`. Activity heartbeats include the workspace id so a
retry can reattach instead of creating a second workspace.

Billing rules:

- Sandbox billing starts only after a workspace is allocated for an execution.
- Ticks are bucketed, for example one ledger row per minute, using an
  idempotency key that includes execution id, workspace id, and bucket start.
- On suspend, emit a final partial tick and stop compute billing.
- On resume, create a new lease span and continue ticking.
- On cleanup failure, retry cleanup and keep the execution in
  `cleanup_pending` / `settlement_pending` operational state until finalization
  succeeds.

The runtime service remains money-agnostic. Worker activities own the span and
call the existing `TickReporter` / ledger path.

## Deploy Lifecycle

Deploy is part of the paid workflow because it can consume provider, sandbox,
storage, and hosting cost.

Deploy stages:

1. `DeployCandidateActivity` builds and publishes a preview/candidate.
2. `DeployVerificationActivity` runs health checks, smoke tests, security
   checks, and URL capture.
3. If policy allows low-risk auto deploy, continue. Otherwise wait for human
   approval.
4. `PromoteDeployActivity` promotes to production.
5. `RecordDeployCostActivity` writes deployment/storage/egress cost.
6. `RollbackDeployActivity` compensates on failed promotion or explicit cancel.

Deploy webhooks should not update execution state directly. They write an
event/outbox row and signal the workflow, so Temporal remains the state machine.

## Billing Settlement

The settlement leg is mandatory and must be retried harder than normal work.
No execution is financially complete until settlement succeeds.

Settlement activity:

```text
SettleExecutionActivity(executionID, finalStatus, idempotencyKey)
```

Responsibilities:

- Read final execution costs and reserved amount.
- Debit actual spend from the wallet hold.
- Release unused hold.
- Write `credit_release` when applicable.
- Write `platform_margin`, including negative margin honestly.
- Record blueprint outcome.
- Emit `settlement_succeeded` or `settlement_failed`.

If settlement fails after the workflow has reached a product terminal state,
the execution should be visible as `settlement_pending` for operators and
dashboards. Retrying settlement must not double-debit the wallet, double-write
margin, or double-count blueprint stats.

## Embedded Dev vs Worker Runtime

The current dev path stays embedded until the Temporal path is fully proven.

| Concern | Embedded dev | Temporal worker path |
| --- | --- | --- |
| GraphQL API | Orchestrator handles mutations, queries, subscriptions. | Orchestrator remains API/control plane. |
| Finisher loop | Runs in-process through `finisher.Engine`. | Worker runs `FinisherExecutionWorkflow` and activities. |
| Wallet/ledger/execution services | Same in-process services, memory or Postgres. | Same services, called by worker activities against Postgres. |
| ProfitGuard | In-process hook before provider/retry/sandbox. | Activity before every expensive step; same policy and store. |
| Provider router/BillingGuard | In-process. | Worker activity uses same router; BillingGuard still writes cost attribution. |
| Patch lifecycle | In-process engine + runtime applier. | Activities call same patch engine/applier. |
| Sandbox billing | Existing `SandboxBiller` wrapper in orchestrator. | Worker owns lease span; calls same TickReporter with idempotent buckets. |
| Approval wait | Request/response or short-lived in-process state. | Workflow signal wait with timers; UI sees Redpanda/outbox events. |
| Deploy | In-process gate/activity style. | Dedicated deploy activities, optional separate task queue. |
| Redpanda events | Embedded publisher/outbox when available. | Activities write outbox; publisher emits to Redpanda; workflow receives signals for relevant events. |
| Temporal dependency | Not required. `IRONFLYER_TEMPORAL_HOST` empty keeps embedded. | Required. Worker connects to Temporal host/namespace/task queue. |

Phase 1 can run worker registration in the orchestrator binary when
`IRONFLYER_TEMPORAL_HOST` is set. Phase 2 should split a dedicated
`orchestrator-worker` deployment so API pods can scale separately from
long-running execution workers.

## Deployment Plan

1. Keep default `IRONFLYER_EXECUTOR=embedded` for local dev and lightweight
   review apps.
2. Normalize Temporal env names before production rollout. The repo currently
   has both legacy `TEMPORAL_*` keys and `IRONFLYER_TEMPORAL_*` deployment
   keys; production should use one canonical set.
3. Add worker registration for `FinisherExecutionWorkflow` and activities.
4. Start workflows from `createPaidExecution` after wallet admission succeeds,
   or move admission into the first workflow activity with a single
   idempotency key. Do not do both without dedupe.
5. Add operation keys for ledger, wallet hold/debit/release, sandbox ticks,
   deploy calls, and settlement.
6. Add outbox publisher to Redpanda and keep GraphQL subscriptions reading
   from event storage / Redpanda fan-out rather than Temporal history.
7. Canary with a tenant or feature flag: embedded and Temporal paths must
   produce equivalent execution events, costs, and settlement rows.
8. Split workers by task queue when provider/runtime/deploy load starts
   starving API pods.
9. Use Temporal worker versioning/build ids for deploys; drain old workers
   before removing activity code that live workflows may still reference.

## Open Implementation Gaps

- No workflow implementation exists yet; the Temporal SDK is present in module
  metadata and infra has dev/prod configuration hooks.
- Ledger writes are append-only but do not yet expose a domain operation key.
- `Settler.Close` is documented as idempotent but currently needs storage-level
  dedupe before Temporal can retry it safely.
- Runtime sandbox create/attach needs an execution idempotency contract.
- Deploy activities need persisted provider deployment ids and rollback hooks.
- Human approval needs durable approval records plus workflow signals.
- Redpanda outbox/publisher should be introduced as the integration boundary;
  Temporal must not become the event bus.
