# Runtime / Sandbox Scale Architecture

## Purpose

The runtime execution plane must scale paid workspaces without turning idle
capacity into margin loss. The design below keeps the local developer path
simple with Docker, then moves production through stronger sandbox isolation
and eventually to microVM-backed execution while preserving the V22 economic
invariants:

1. A sandbox is allocated only for an admitted paid execution or an explicitly
   budgeted preview session.
2. Runtime cost is measured as ledger-backed sandbox ticks.
3. Idle workspaces are checkpointed, archived, and removed from hot compute.
4. Scale decisions follow paid queue pressure and positive expected margin.

## Current Baseline

The runtime service already has a Docker driver for local workspaces, a mock
driver for dev/test, S3-style snapshot managers, and orchestrator-side sandbox
billing ticks. The current Kubernetes manifest keeps runtime state in one
pod/PVC, so it is intentionally not the long-term scale shape.

The cloud IDE path is part of the runtime plane: Docker workspaces can expose
an IronFlyer-branded code-server `ideUrl`, and the production target is a
signed workspace IDE route owned by runtime/edge. See
`docs/ARCHITECTURE_CLOUD_IDE.md`.

The scale target is to make runtime pods disposable schedulers for isolated
workspace sandboxes, with workspace state restored from object storage and
hot-cache volumes rather than bound to a specific pod.

## Target Execution Plane

```text
paid queue / execution FSM
  -> ProfitGuard Admit + wallet hold
  -> runtime allocator
  -> warm pool or cold sandbox boot
  -> workspace restore from S3/R2 + optional hot cache
  -> execute finisher steps / preview / tests
  -> periodic checkpoint + billing tick
  -> idle timeout
  -> archive snapshot to S3/R2
  -> sandbox destroy
```

Control plane ownership:

| Component | Owns |
| --- | --- |
| Orchestrator | Execution state, wallet holds, ProfitGuard decisions, billing ledger |
| Runtime API | Workspace lifecycle API, sandbox driver abstraction, health/probes |
| Runtime allocator | Queue-to-sandbox assignment, warm-pool use, quota enforcement |
| Sandbox driver | Docker/dev, gVisor/Kata containers, Firecracker/Kata microVMs |
| Snapshot/archiver | Restore/checkpoint/archive to S3/R2-compatible storage |
| KEDA/HPA/cluster autoscaler | Pod and node scale based on paid queue and saturation |

## Isolation Path

### Phase 0: Local Docker Dev

- Use the existing Docker driver on a developer machine.
- Rootless Docker is preferred; Docker Desktop is acceptable for local use.
- Workspace files live in a host path or Docker volume.
- Billing and ProfitGuard can be disabled in pure dev, but integration dev
  should run sandbox ticks against a test ledger.

### Phase 1: Hardened Container Runtime

- Move production sandboxes out of the runtime service pod and into dedicated
  sandbox pods/jobs.
- Use Kubernetes `RuntimeClass` with gVisor (`runsc`) for untrusted workspace
  processes.
- Apply per-sandbox pod security:
  - non-root user
  - read-only root filesystem where feasible
  - dropped Linux capabilities
  - seccomp/AppArmor profiles
  - no hostPath except controlled cache mounts
  - network policy default-deny with explicit egress allowlists
- Keep Docker only for local/dev and for transitional single-node installs.

### Phase 2: Kata Containers

- Use Kata when workloads need a VM boundary but still fit the Kubernetes pod
  scheduling model.
- Keep the same allocator and snapshot contract; only `RuntimeClass` changes.
- Prefer Kata for tenant code that runs package managers, build tools, or
  arbitrary test suites with broader syscall needs than gVisor tolerates.

### Phase 3: Firecracker MicroVM Pool

- Introduce a Firecracker-backed sandbox driver for high-risk or high-value
  paid executions.
- Preboot microVMs into a paused/ready state with base images already mounted.
- Attach workspace snapshots as restored block/file layers at assignment time.
- Use jailer namespaces, tap devices, and strict per-VM egress routing.
- Keep VM lifetime short: one execution branch or one interactive workspace
  lease, then checkpoint and destroy.

The driver interface should remain stable across these phases: create, exec,
PTY, file operations, preview target, checkpoint, and destroy. Isolation
strength becomes a scheduling decision, not a caller concern.

## Workspace State And Snapshotting

The hot filesystem should be treated as cache. Durable state belongs in object
storage.

Snapshot layout:

```text
s3://<bucket>/<prefix>/workspaces/<workspace_id>/<timestamp>.tar.zst
s3://<bucket>/<prefix>/workspaces/<workspace_id>/LATEST
s3://<bucket>/<prefix>/archives/<workspace_id>.tar.zst
```

Required behavior:

- Restore `LATEST` before assigning work to a cold sandbox.
- Checkpoint after patch apply, after successful gate verification, before idle
  teardown, and at a bounded periodic interval during long executions.
- Flip `LATEST` only after the full object upload succeeds.
- Keep a small retention window for recent checkpoints; archive older state.
- Support S3 and R2 using the S3 API surface.
- Encrypt snapshots with provider-managed or tenant-scoped KMS keys.
- Store metadata in Postgres: workspace id, tenant id, execution id, object key,
  checksum, size, created timestamp, restored timestamp, and last billing tick.

Cost controls:

- Compress with zstd for archive objects and choose gzip/zstd per runtime
  compatibility for hot checkpoints.
- Exclude transient directories by default: `.git/objects` may be retained only
  when needed, while `node_modules`, build caches, `.next`, `dist`, coverage,
  and package-manager caches should use cache layers or be regenerated.
- Use lifecycle rules to move old snapshots to cheaper storage and expire them
  after the tenant/project retention policy.

## Quotas And Admission

Quotas are enforced before allocation and rechecked during execution.

| Scope | Quota |
| --- | --- |
| Tenant | concurrent sandboxes, concurrent CPU, memory, egress, object storage GB |
| Execution | max wall time, max sandbox ticks, max restore/checkpoint bytes |
| Workspace | idle timeout, max disk, max open preview ports |
| Node | max sandbox pods, image pull bandwidth, restore concurrency |
| Region | max pending paid queue depth before new admissions degrade/pause |

Admission order:

1. Wallet hold exists.
2. ProfitGuard expected margin is positive after estimated sandbox cost.
3. Tenant concurrency and spend quotas have room.
4. A warm slot exists or a cold start can meet the execution SLA.
5. Node pool has capacity or the cluster autoscaler can add capacity within the
   maximum wait budget.

When any check fails, the allocator returns a typed reason:
`pause_for_budget`, `quota_exceeded`, `capacity_wait`, `degrade_runtime`,
or `stop_loss`.

## Warm Pools

Warm pools reduce latency, but they are inventory and must be bounded by margin.

Pool types:

| Pool | Contents | Use |
| --- | --- | --- |
| Image warm | Pulled base images on runtime nodes | Cheap default |
| Sandbox warm | Started gVisor/Kata pods waiting for assignment | Short interactive SLA |
| MicroVM warm | Booted Firecracker VMs paused at login shell | High-value tenants only |
| Workspace hot | Recently used workspace kept on node-local cache | Resume within idle window |

Warm-pool policy:

- Maintain a floor from recent paid arrival rate, not from total signups.
- Cap by reserved wallet dollars and expected gross margin.
- Drain warm slots when paid queue depth is zero for a cooldown window.
- Prefer image warming and node-local cache before keeping full sandboxes alive.
- Charge internal platform cost for warm inventory to a runtime overhead bucket;
  do not debit the tenant until a sandbox is assigned to an execution.

## Node Pools

Separate node pools keep expensive isolation from contaminating cheap workloads.

| Pool | Taints/labels | Workload |
| --- | --- | --- |
| `system` | `ironflyer.io/pool=system` | Orchestrator, web, databases clients |
| `runtime-general` | `ironflyer.io/pool=runtime-general` | gVisor/Kata sandbox pods |
| `runtime-build` | `ironflyer.io/pool=runtime-build` | CPU/memory-heavy builds/tests |
| `runtime-microvm` | `ironflyer.io/pool=runtime-microvm` | Firecracker/Kata VM workloads |
| `runtime-gpu` | `ironflyer.io/pool=runtime-gpu` | Optional future model/tool workloads |

Scheduling rules:

- Runtime pods tolerate only runtime taints.
- Untrusted sandbox pods never schedule on `system`.
- Build-heavy executions use `runtime-build` through ProfitGuard step metadata.
- High-risk tenants or enterprise policy can force `runtime-microvm`.
- Spot/preemptible nodes are allowed only for checkpoint-safe, retryable work.

## Kubernetes Scaling

Runtime scale uses two layers:

1. Pod scale with KEDA/HPA.
2. Node scale with the managed cluster autoscaler or Karpenter-equivalent.

KEDA triggers should be based on economic queue signals, not raw HTTP traffic:

- paid execution queue depth
- pending sandbox allocation requests
- warm pool deficit
- average restore latency
- active sandbox count per runtime pod
- CPU/memory saturation for runtime allocator pods

Suggested KEDA behavior:

```text
minReplicaCount: 0 for allocator workers in idle regions
minReplicaCount: 1 for the public runtime API while the product is online
cooldownPeriod: 300s
scaleUp: fast when paid_queue_depth > warm_capacity
scaleDown: conservative until all active sandbox leases are gone
```

HPA is still useful for runtime API pods on CPU/memory, but allocation workers
should primarily scale from queue metrics. Sandbox pods/jobs are created per
lease and should not be represented as long-lived runtime replicas.

Scale-to-zero rules:

- Allocator workers can scale to zero when no paid queue, no active restores,
  and no warm-pool floor.
- Sandbox pods always scale to zero after idle timeout or execution completion.
- Node pools may scale to zero except one small system pool if the cloud
  provider supports it.
- Public API scale-to-zero is optional and only acceptable if cold-start latency
  is product-approved.

## Security Boundaries

Tenant code is hostile by default.

Boundaries:

- Tenant identity is carried from orchestrator token to runtime allocation and
  attached to every workspace, snapshot, preview, log, and ledger event.
- Workspace credentials are short-lived and scoped to one workspace/execution.
- Preview URLs require signed, expiring tokens and route only to declared ports.
- Network egress defaults to deny; package registries and deployment providers
  are explicit allowlist entries by step type.
- Secrets are never written into snapshots. Runtime injects them as short-lived
  env vars/files from a secret broker and scrubs before checkpoint.
- Snapshots are tenant-scoped in object storage IAM and encrypted at rest.
- Logs redact tokens, clone URLs, env values, and package-manager auth files.
- Runtime service account can create/delete sandbox pods but cannot read wallet
  balances or mutate the ledger directly.
- Orchestrator writes ledger entries; runtime emits metered usage events.

Destructive actions:

- Destroy sandbox compute after checkpoint success or explicit discard.
- Delete hot caches only after archive durability is confirmed.
- Never delete object-store snapshots during request handling; use a retention
  worker with audit logs.

## Billing Ticks

Sandbox cost must be visible in the same ledger model as provider tokens.

Tick model:

```text
sandbox_cost =
duration_seconds
* runtime_class_rate_usd_per_second
* resource_multiplier(cpu, memory, disk, network)
```

Ledger events:

| Event | When | Notes |
| --- | --- | --- |
| `sandbox_allocated` | Lease assigned | No tenant debit yet unless minimum tick is configured |
| `sandbox_tick` | Every billing interval | Debit tenant/execution for elapsed compute |
| `sandbox_checkpoint` | Snapshot upload completes | Attribute storage/write cost |
| `sandbox_idle` | Idle timeout starts | Can trigger final tick + archive |
| `sandbox_destroyed` | Compute removed | Final partial tick |
| `runtime_overhead` | Warm pool inventory | Platform cost bucket, not tenant debit |

Default cadence:

- 60-second ticks for normal paid executions.
- Final partial tick on destroy so short runs are not free.
- Optional 10-second ticks for high-cost microVM/GPU pools.
- Tick writes are idempotent by `(execution_id, workspace_id, tick_start)`.

ProfitGuard uses the tick rate before allocation:

```text
estimated_runtime_cost =
estimated_duration_seconds
* selected_runtime_class_rate
* requested_resource_multiplier
```

If estimated runtime cost would push gross margin below threshold, the decision
must be `degrade_runtime`, `reuse_blueprint`, `pause_for_budget`, or `stop`.

## Failure Modes

| Failure | Response |
| --- | --- |
| Restore fails | retry with backoff, then fail allocation with typed error |
| Snapshot upload fails | keep sandbox alive within max grace, retry, then mark workspace at-risk |
| Node preempted | recover from last durable checkpoint on another node |
| Warm pool overfilled | drain oldest idle warm sandboxes first |
| Tenant quota exceeded | pause execution before new paid work starts |
| Runtime class unavailable | downgrade only if tenant policy and ProfitGuard allow it |
| Ledger tick write fails | retry outbox; do not keep unmetered sandbox alive past grace |

## Metrics

Required dashboards:

- paid queue depth
- active sandboxes by runtime class
- sandbox allocation latency p50/p95/p99
- restore/checkpoint latency and bytes
- warm pool hit rate and idle burn
- sandbox cost per completed execution
- completion-per-runtime-dollar
- node pool utilization and scale events
- failed restores/checkpoints
- unbilled sandbox seconds

The health target is not maximum utilization. The target is profitable completed
execution capacity with bounded cold-start latency.

## Implementation Milestones

1. Document and keep Docker/mock as the dev path.
2. Make workspace state object-store-first: restore on create, checkpoint on
   lifecycle events, archive on idle.
3. Split runtime API from sandbox workers/pods.
4. Add quota checks before allocation.
5. Add KEDA triggers from paid queue and allocation metrics.
6. Add gVisor `RuntimeClass` and runtime node pool scheduling.
7. Add warm image/cache pools before full warm sandboxes.
8. Add Kata for stronger isolation where gVisor compatibility hurts.
9. Add Firecracker for high-isolation/high-value executions.
10. Add scale-to-zero for allocator workers and sandbox pools.

## Non-Goals

- Free interactive compute without wallet reservation.
- Long-lived pet workspaces pinned to a single pod.
- Runtime service pods that directly run tenant code.
- Autoscaling on anonymous traffic or signup volume.
- Tenant-visible billing that hides runtime overhead or unbilled leakage.
