# Ironflyer Project Closeout Plan

Ironflyer must become an AI Execution Cloud, not another prompt-to-app
demo. The product promise is simple:

```text
Describe the outcome. Ironflyer finishes the product, proves it works,
keeps it secure, and shows the execution economics.
```

The company-level constraint is equally simple:

```text
No unprofitable execution path ships.
```

## Product North Star

Primary metric:

```text
Profitable Completed Execution Rate =
completed paid executions with positive gross margin
/
total paid executions
```

Supporting metrics:

- time to first preview
- cost to first preview
- completion per dollar
- gross margin per execution
- repeat execution rate
- recovery success rate
- refund rate
- support minutes per execution
- security gate pass rate

## Non-Duplicated Technology Map

Each technology owns one job. If two systems can own the same durable
truth, one of them is wrong.

| Layer | Technology | Owns | Must Not Own |
| --- | --- | --- | --- |
| Source of truth | Postgres | tenants, users, wallets, ledger, executions, auth, permissions, billing state, deployment state | analytics, AI graph, event history |
| Durable workflow | Temporal | long-running execution workflows, retries, compensation, approval waits, deploy lifecycle | event fan-out, analytics |
| Event backbone | Redpanda | immutable business/infra events, replay, DLQ, consumer fan-out, lag-based scaling | request cache, source-of-truth state |
| Analytics | ClickHouse | cost, margin, provider latency, cohorts, gate failure analysis, scale dashboards | transactional decisions |
| AI context graph | SurrealDB | project graph, code graph, memory, semantic context, repair/blueprint relation discovery | wallet, ledger, execution truth |
| Ephemeral coordination | Redis | locks, rate limits, short-lived pub/sub, session revocation cache | durable event log |
| Artifacts | S3/R2/MinIO | workspace snapshots, build artifacts, exports, logs, deployment bundles | relational records |
| Runtime isolation | Docker -> gVisor/Kata -> Firecracker | user workspaces, test runs, preview sandboxes | billing truth |
| Policy | OPA/Cedar | command permissions, deploy approvals, tenant/data/model policy | business state |
| Observability | OpenTelemetry + Prometheus/Grafana/ClickHouse | traces, metrics, logs, execution cost visibility | product workflow state |

## Target Architecture

```text
Clients
  Web / VSCode / CLI / SDK
      |
GraphQL API Gateway
      |
Control Plane
  Auth -> Tenant -> Policy -> Wallet -> ProfitGuard -> Orchestrator
      |
Temporal Workflows
      |
Execution Plane
  Agent Workers -> AI Gateway -> Provider Router
  Sandbox Workers -> Runtime Isolation
  Test Workers -> Gate Verification
  Deploy Workers -> Vercel/Fly/K8s targets
      |
Data Plane
  Postgres -> Outbox -> Redpanda -> ClickHouse / SurrealDB / Notifications
  S3/R2 for snapshots and artifacts
```

## Immediate Code Closure

These items are required before calling the backend commercially ready:

1. Force all agent model calls through `providers.BillingGuard`.
   `agents.Registry` must never call `providers.Router` directly.
2. Keep `ProfitGuard` enforcement points live for:
   before model call, before sandbox allocation, before premium
   reasoning, before retry loop, before long verification, before
   production deploy, before large artifact writes.
3. Add a Postgres `event_outbox` table and publisher.
   Durable writes create outbox rows inside the same transaction.
4. Add Redpanda as the production bus.
   Redis remains only for ephemeral fan-out and coordination.
5. Add ClickHouse ingestion from Redpanda.
   Dashboards should read ClickHouse, not production Postgres.
6. Move execution lifecycle to Temporal for production mode.
   Embedded mode remains for local dev.
7. Make SurrealDB the explicit AI memory graph backend.
   It enriches retrieval; it does not decide money or permissions.
8. Harden GraphQL: complexity limit, depth limit, production
   introspection gate, safe error masking, APQ policy.
9. Build the new web surface against GraphQL and the SDK, under the
   locked design reference in `design-reference/2026-05-25-private-ironflyer/`.
   First screen is the execution cockpit, not a landing page.
10. Add release gates: full tests, smoke, migration status, helm lint,
    Pulumi preview, security scan, synthetic paid execution.

## 30-Day Plan: Paid Execution Proof

Goal: prove that a customer can pay, run an execution, get a useful
preview, and that Ironflyer can reconstruct the economics.

Build:

- BillingGuard-only agent calls.
- Wallet top-up and reservation happy path.
- Execution cockpit UI matching the locked private IronFlyer reference.
- First Redpanda outbox topics.
- ClickHouse profit dashboard v1.
- SurrealDB memory graph v1.
- Production GraphQL hardening.
- 3 blueprints that reliably reach preview.
- Security/test gates visible to the customer.

Acceptance:

- 10 paying users.
- 50 paid executions.
- first preview success above 70%.
- gross margin above 45%.
- ledger can reconstruct every execution.
- no model call without ledger and ProfitGuard path.

## 60-Day Plan: Scale Proof

Goal: prove repeat usage under elastic infrastructure.

Build:

- Temporal production workflow path.
- Redpanda topics with schema registry and DLQ.
- KEDA scaling for worker groups by topic lag.
- ClickHouse cohort, provider, and blueprint dashboards.
- runtime snapshot/rehydration across pods.
- abuse scoring v1.
- OPA/Cedar policy plane v1.
- 5-7 production blueprints.

Acceptance:

- 50 paying users.
- 300 paid executions.
- preview success above 75%.
- gross margin above 45%.
- repeat usage above 25%.
- support minutes per execution decreasing.
- workers scale up and down from real queue pressure.

## 90-Day Plan: Trust And Expansion

Goal: become the trusted AI execution platform for real businesses.

Build:

- Firecracker or Kata/gVisor isolation path.
- customer-visible security report.
- deploy approvals and production data isolation.
- enterprise audit export.
- custom domains and production deploy targets.
- partner/import path for existing apps.
- cost forecast before execution starts.
- customer success loop: failed execution -> repair recipe -> cheaper
  future run.

Acceptance:

- 200 paying users or 20 high-value workspaces.
- 1,500 paid executions.
- gross margin above 50%.
- refund rate below 5%.
- security gate blocks unsafe deploys.
- one-click support bundle for every failed execution.

## Pricing

Start with prepaid execution credits:

- Free: demo credits only, no production deploy.
- Builder: monthly plan + execution credits.
- Business: higher credit limits, private apps, staging, audit.
- Enterprise: SSO, SCIM, custom policies, dedicated regions, custom
  rate limits.

Credits must map to real cost buckets:

- model tokens
- sandbox minutes
- build/test minutes
- deployment operations
- storage/artifact volume
- premium integrations

Every customer-facing credit burn must map to ledger entries.

## Customer Wow Loop

Every execution must leave the user with something concrete:

1. preview URL
2. changed files
3. gate report
4. security report
5. cost report
6. next best action

The differentiator is not that Ironflyer generates code. The
differentiator is that Ironflyer finishes the work and proves it.

## What Would Kill The Project

- model calls that bypass billing
- relying on Postgres for all analytics
- using Redis as a durable event log
- storing financial truth in SurrealDB
- shipping generated apps without security gates
- letting agents touch production data or secrets directly
- selling unlimited AI usage without margin guardrails
- building a beautiful UI before the paid execution loop is closed

## Definition Of Done

Ironflyer is closed for commercial launch when:

- a user can top up, run, stop, refund, and inspect an execution
- every cost lands in the ledger
- every execution emits events into Redpanda
- ClickHouse dashboards show margin in near real time
- SurrealDB improves reuse/retrieval without owning durable truth
- Temporal can resume interrupted workflows
- runtime workers scale by demand and shrink when idle
- GraphQL is hardened for production
- one synthetic paid execution passes in CI/CD
- operators can deploy, rollback, restore, and explain costs
