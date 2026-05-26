# Ironflyer — Domain Modules Map

> The orchestrator and runtime internal trees are physically grouped
> by business domain. This document describes the on-disk layout and
> the one-line purpose of each package. The five domains are real
> directories, not just an ownership lens.
>
> The implementation contract lives in
> [V22_PLAN.md](V22_PLAN.md); the locked architecture spec lives in
> [ARCHITECTURE.md](../ARCHITECTURE.md).

The two Go services in scope are:

- `core/orchestrator/internal/{business,ai,operations,customer,suppliers,pkg}/*`
- `core/runtime/internal/{operations,customer,suppliers,pkg}/*`

`pkg/` sits at the top of each `internal/` tree on purpose: it is a
cross-cutting shared-utility layer (env, httputil, httpclient) that
all five domains may import. It is not itself a domain.

`suppliers/` in the orchestrator is intentionally lean: the
overwhelming majority of vendor adapters live as files *inside*
domain packages — Anthropic / OpenAI / Gemini under `ai/providers`,
Stripe under `business/budget`, Vercel under `operations/deploy`, S3
under `operations/storage`. Moving those out would split the domain
that owns the contract. Suppliers is reserved for adapter packages
that exist purely as third-party integration surface with no first-
class domain home (e.g. the runtime's mobile device-cloud bridges).

---

## 1. Business — `core/orchestrator/internal/business/`

Owns: revenue, cost, margin, and the append-only financial truth.
Anything that moves money against a tenant lives here.

- `business/wallet` — prepaid per-tenant credit balance with holds, debits, releases; serialized per-tenant via row locks.
- `business/budget` — billing facade (plans, rates, optimizer, enforcer, Stripe metered, refunds, tokencap, vault); keeps a `payments/` subdir.
- `business/ledger` — append-only debit/credit entries; structural invariants enforced both in code and in Postgres.
- `business/blueprints` — starter catalogue with cost priors and per-blueprint stats; ProfitGuard ranks against these.
- `business/profitguard` — verdict engine (continue / degrade / switch / pause / stop / reuse) before every expensive step.
- `business/profitguardbridge` — adapts `execution.State` + provider quotes into `profitguard.ExecState`.
- `business/profitguardctx` — propagates execution-id + tenant through context so cost attribution and Guard land.
- `business/forecast` — tenant-vs-global percentile estimator that feeds budget reservations and ProfitGuard.
- `business/lastprovider` — bounded tracker mapping execution → last (provider, capability); feeds bandit on gate verdicts.
- `business/execution` — execution lifecycle, cost buckets (provider / sandbox / storage / deployment / premium), FSM, settler.
- `business/dashboards` — profit / scale / cohort / blueprint dashboard builders over Ledger + Execution sources.
- `business/clickhouse` — fact-table client + adapters that back the Profit/Scale dashboards when ClickHouse is wired.
- `business/wowloop` — assembled "wow loop" snapshot (execution + ledger + next-action) for the operator surface.
- `business/outboxhooks` — atomic event_outbox writes inside wallet/ledger/execution/patch/gate transactions.

## 2. AI — `core/orchestrator/internal/ai/`

Owns: model orchestration, prompts, retrieval, memory, completion
scoring, patch authoring, and gate-driven repair.

- `ai/agents` — YAML-driven role registry (Planner, Architect, Critic, Coder, …) with capability + thinking flags. Includes the `PreflightDecision` contract the coder/architect agents emit before patches land (Anti-Bloat Engine, see `docs/ANTI_BLOAT_ENGINE.md`).
- `ai/atlas` — Capability Atlas: live index of every reusable Go func / TS hook / React component / blueprint with embeddings, surfaced via `atlas.Search` to the Reuse-First Preflight gate. In-process + pgvector + surreal backends; the latter two reuse the existing `ai/memory` store so no second vector DB is needed.
- `ai/refactor` *(reserved)* — Refactor Proposer (playbook §8.6); needs `ts-morph` / `comby` codemod tooling and ships as a follow-up package.
- `ai/providers` — Anthropic / OpenAI / Gemini / HuggingFace / DeepSeek / Vercel AI Gateway routers + quality registry.
- `ai/completion` — per-execution completion scorer (latest-pass-by-gate) and per-dollar value tracking.
- `ai/embeddings` — embedder client + LRU cache; ONNX-capable build tag for local inference.
- `ai/retriever` — pure-Go BM25 with structure-aware chunking and symbol boost; in-process RAG for the Coder.
- `ai/memory` — owner-scoped memory federation; cross-project recall, never cross-user.
- `ai/memorygraph` — Surreal/pgvector node+edge graph with cascade delete and IntentGateRepair retrieval.
- `ai/ideaparser` — turns a free-form idea into a structured spec + blueprint pick (rules-first, LLM fallback).
- `ai/finisher` — auth-scaffold step + finisher-loop bits that own framework-matched starter recipes.
- `ai/scaffold` — StackSpec → starter template → patch.Patch through the standard apply pipeline.
- `ai/repair` — repair genome: failure signature → fix recipe with hits/successes and patch memory.
- `ai/domain` — canonical artifact + gate name constants the finisher pipeline and gates agree on.

## 3. Operations

Owns: execution-time machinery and platform hygiene — patches, gates,
deploys, runtime glue, observability, policy, secrets, configuration,
admission, and the workspace runtime itself.

### Orchestrator-side — `core/orchestrator/internal/operations/`
- `operations/arch` — Architecture Manifest reader: parses `.ironflyer/architecture.json` and exposes `Manifest.Validate(pkg, importPath)` for the `dep_graph` + `arch_boundary` Anti-Bloat gates.
- `operations/patch` — patch lifecycle engine: validate → preview → apply → snapshot → verify → rollback. AI never writes files directly.
- `operations/deploy` — provider-agnostic deploy adapter contract (Vercel today) with approvals, domains, sweeper.
- `operations/runtime` — bridge from finisher to the runtime service: applies patches via the File API.
- `operations/diagnostics` — WARN+ ring buffer + zerolog hook + HTTP surface for live operator triage.
- `operations/audit` — Moat #4: append-only, content-addressed, tamper-resistant audit chain (SHA-256 of canonical JSON).
- `operations/auditexport` — chain-proof + signed CSV/JSON exporter for SIEM / WORM destinations.
- `operations/securityreport` — Standard security-report builder over execution + finding + policy sources.
- `operations/abuse` — tenant-scoped abuse scoring with window, cache TTL, hard floor.
- `operations/policy` — PDP that returns allow/deny + audited reason; leaf package wired by integration agent.
- `operations/secrets` — V22 secret broker: SecretRef → Capability → Resolve, with TTL, redaction, audit.
- `operations/appsec` — secret scanner + SBOM + OSV + config/dependency-health gates with blocking verdicts.
- `operations/ratelimit` — per-key token-bucket limiter (per-user / per-IP) backing GraphQL hot paths.
- `operations/gqlhardening` — complexity / depth / CSRF / origin / persisted-query / introspection guards for gqlgen.
- `operations/operator` — `IsOperator(ctx)` canonical check; role plane + transitional Plan shortcut.
- `operations/metrics` — Prometheus counters/histograms + chi middleware; helpers for agent / gate / billing events.
- `operations/logctx` — ctx-keyed request_id / tenant_id / execution_id for canonical structured logs.
- `operations/tracing` — OTel TracerProvider wiring; degrades gracefully to no-op on misconfig.
- `operations/config` — orchestrator runtime config sourced + validated from env at startup.
- `operations/storage` — S3-compatible client config + periodic per-tenant storage-cost metering.
- `operations/store` — project persistence layer (memory now; same interface for Postgres later).
- `operations/migrate` — goose wrapper for orchestrator SQL migrations (idempotent boot).
- `operations/bus` — multi-pod event fan-out (MemoryBus / RedisBus + Multiplexer) for subscriptions and SSE.
- `operations/redisbus` — distributed lock (per-project finisher lease) + cross-pod rate-limit window; nil-safe.
- `operations/events` — typed event envelopes, topics, schemas, DLQ helpers, publisher/pump, Redpanda + registry.
- `operations/temporalworker` — Temporal activities + workflow with port adapters; idempotent retries via op-keys.
- `operations/graph` — gqlgen runtime wiring (HTTP + WS + sandbox) with JWT-on-connection_init.
- `operations/httpapi` — V22-minimal HTTP surface: probes, metrics, GraphQL, Stripe webhook. Nothing else.
- `operations/wireup` — integration glue (e.g., audit-export wiring) projected from env without code changes.
- `operations/mobile` — device-cloud gateway (BrowserStack, AWS Device Farm stub) for Pro-tier mobile QA.
- `operations/sentryext` — orchestrator Sentry init (complements OTel).

### Runtime-side — `core/runtime/internal/operations/`
- `operations/allocator` — admission funnel: wallet hold → ProfitGuard → quota → warm slot / cold start.
- `operations/quota` — hard per-tenant ceilings (sandboxes, CPU, mem, egress, snapshot GB, spend/day).
- `operations/runtimeclass` — per-tenant allowlist (docker / gvisor / kata / firecracker) + forced overrides.
- `operations/warmpool` — bounded warm inventory (image / sandbox / microvm / hot workspace) to absorb cold starts.
- `operations/sandbox` — Docker driver (CLI-based) for per-user code-server containers on EFS-mounted host paths.
- `operations/drivers` — driver factory + mock / docker selection.
- `operations/snapshot` — gzip-tar workspace content layer in S3 with SSE-KMS and LATEST pointer.
- `operations/snapshots` — tar+zstd streaming primitives used by the snapshot subsystem.
- `operations/state` — Postgres workspace metadata + Claim/Reclaim atomics for portability across pods.
- `operations/workspaces` — workspace registry / manager bridging metadata, content, and live layers.
- `operations/patcher` — unified-diff applier on top of the per-driver File API.
- `operations/preview` — preview JSON helpers shared across runtime surfaces.
- `operations/sentryext` — runtime Sentry init (complements OTel).
- `operations/httpapi` — runtime HTTP surface incl. allocator-mounted create/destroy + quota usage.
- `operations/config` — runtime service config (addr, driver, image, JWT, CORS, workspace dir).
- `operations/migrate` — goose wrapper for runtime SQL migrations.
- `operations/wireup` — runtime integration glue: snapshots + quota + warmpool + allocator + runtimeclass funnel.

## 4. Customer Lifecycle

Owns: signup, authentication, identity changes, plan/role assignment,
operator notifications. Everything the human-facing account touches
before and after an execution runs.

- `customer/auth` (orchestrator) — JWT-minting + email-change flow (verify-new-address, revoke-all-sessions-on-confirm) and role plane.
- `customer/notify` (orchestrator) — email sender abstraction (Resend / SendGrid / Noop) with engine, templates, prefs.
- `customer/auth` (runtime, at `core/runtime/internal/customer/auth`) — verifies orchestrator-minted JWTs; runtime never owns a user DB.

Cross-cutting slices live in their primary domain:

- `business/budget/plan.go` — pricing plans + entitlement bits used at signup and upgrade.
- `operations/operator` — operator role check (lifecycle owns the role-assignment side).
- `operations/abuse` — protects the lifecycle surface from credential stuffing / signup floods.

## 5. Suppliers

Owns: every external vendor surface. Most adapters live *inside* the
domain that owns the contract (see notes below); `suppliers/` is
reserved for adapters that have no first-class domain home.

### Orchestrator — `core/orchestrator/internal/suppliers/`

Intentionally empty for now. Adapter slices that would otherwise sit
here already live next to their domain contract:

- `ai/providers` — AI vendors: Anthropic, OpenAI, Gemini, HuggingFace, DeepSeek, Vercel AI Gateway.
- `business/budget` (stripe.go, metered.go, portal.go) — Stripe Checkout + metered usage + customer portal.
- `operations/deploy` (vercel.go, domain_providers.go) — Vercel deploy adapter and registrar/DNS purchase paths.
- `operations/storage` (s3client.go) — S3-compatible object store (AWS S3 / Cloudflare R2 / MinIO) via `S3_BACKEND`.
- `customer/notify` (email.go) — Resend / SendGrid transactional email.
- `operations/mobile` — BrowserStack App Live + AWS Device Farm device-cloud session brokers.
- `business/clickhouse` — ClickHouse fact-table client for analytics dashboards.
- `operations/events` (redpanda.go, schemaregistry.go) — Kafka / Redpanda transport + schema registry adapter.

### Runtime — `core/runtime/internal/suppliers/`
- `suppliers/mobile` — Appetize / EAS / BrowserStack mobile adapters (per-workspace lifecycle + device-cloud entry points). The lifecycle pieces are split out of the runtime operations plane because they belong to third-party device-farm vendors.

Workspace primitives that *are* vendor-shaped but are first-class
runtime contracts stay in operations:

- `core/runtime/internal/operations/sandbox` — Docker (and future gVisor / Kata / Firecracker) workspace driver.
- `core/runtime/internal/operations/snapshot` — AWS S3 (SSE-KMS) for workspace tarballs.
- `core/runtime/internal/operations/sentryext` — Sentry SDK init for the runtime service.

---

## Cross-cutting

Some packages legitimately serve more than one domain. The primary
owner is listed first; secondary lenses follow in parentheses.

- `operations/audit` — Operations (audit chain + tamper detection); Customer Lifecycle (per-user export).
- `operations/auditexport` — Operations (SIEM destinations); Business (signed CSV / chain proofs for refund disputes).
- `operations/operator` — Operations (capability check on resolvers); Customer Lifecycle (role assignment).
- `operations/abuse` — Operations (incident-mode floor); Customer Lifecycle (signup / login throttling).
- `operations/secrets` — Operations (broker + audit); Suppliers (resolution at the moment of vendor call).
- `operations/bus` + `operations/events` — Operations (transport); Business (carries outbox-bound economic events).
- `business/outboxhooks` — Business (writes are inside wallet/ledger transactions); Operations (outbox publish loop).
- `business/profitguardctx` — Business (cost attribution carrier); AI (every provider call reads it).
- `business/lastprovider` — Business (cost attribution); AI (bandit per-provider EMA updates).
- `operations/runtime` (orchestrator side) — Operations (patch apply bridge); AI (the finisher loop's RuntimeApplier).
- `business/clickhouse` — Suppliers (vendor adapter); Business (LedgerSource for dashboards).
- `core/runtime/internal/suppliers/mobile` — Operations (workspace lifecycle); AI (build artifact gate inputs).

---

## Adding a new package

1. Decide which of the five domains the package primarily serves.
2. Create it under `core/<service>/internal/<domain>/<pkg>` and add a
   one-line entry (under 100 chars) to this file under that section.
3. If it cross-cuts, list secondary domains in **Cross-cutting** with
   the primary owner first.
4. Reference this file in [ARCHITECTURE.md](../ARCHITECTURE.md) only
   if the new package is load-bearing for V22.
5. Update [V22_PLAN.md](V22_PLAN.md) only if the package changes the
   implementation contract.

## Related documents

- [ARCHITECTURE.md](../ARCHITECTURE.md) — V22 locked spec, hot path, economic objects.
- [V22_PLAN.md](V22_PLAN.md) — implementation contract.
- [ARCHITECTURE_POLICY_SECURITY.md](ARCHITECTURE_POLICY_SECURITY.md) — policy / secret / trust plane.
- [ARCHITECTURE_ANALYTICS.md](ARCHITECTURE_ANALYTICS.md) — ClickHouse + dashboards contract.
- [ARCHITECTURE_RUNTIME_SCALE.md](ARCHITECTURE_RUNTIME_SCALE.md) — runtime allocator + warm pools.
- [ARCHITECTURE_EVENTS.md](ARCHITECTURE_EVENTS.md) — event taxonomy + DLQ shape.
- [ARCHITECTURE_MEMORY_GRAPH.md](ARCHITECTURE_MEMORY_GRAPH.md) — memorygraph schema + cascades.
- [ARCHITECTURE_DEPLOY_DOMAINS.md](ARCHITECTURE_DEPLOY_DOMAINS.md) — deploy domain purchase + DNS.
- [ARCHITECTURE_APPSEC_CORE.md](ARCHITECTURE_APPSEC_CORE.md) — appsec scanners + verdicts.
- [ARCHITECTURE_CLOUD_IDE.md](ARCHITECTURE_CLOUD_IDE.md) — code-server studio surface.
- [ARCHITECTURE_WORKFLOWS.md](ARCHITECTURE_WORKFLOWS.md) — Temporal workflows.
