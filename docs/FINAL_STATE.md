# Ironflyer — Final State (2026-05-26)

> Closing snapshot of the production-readiness sprint. 9 commits over
> ~12 hours of parallel agent work. This document is the single
> source of truth for what's in the codebase right now and what
> remains as operator action.

## Commit timeline (feat/vscode-extension)

```
33f13faf  feat: closure — 10/10 Anti-Bloat gates + templates tracked + final state
e168d0b8  feat: closure pass 7 — health backend, coder Atlas hook, 4 more tools, mobile audit, cleanup
210d7f75  feat: closure pass — Atlas boot, ProfitGuard final 3, Health UI, VSCode -52%, real tools
8df3ea01  feat: data layer end-to-end + Anti-Bloat Engine MVP + ProfitGuard coverage + cold-start
e099a734  perf+hardening: web bundle, Apollo cache, GraphQL caps (complexity/depth/APQ)
d11289c7  perf(core): deep hot-path + SQL + stability optimization round
bea1913e  perf(core): hot-path allocation reductions across orchestrator + runtime
3c3a5465  refactor(monorepo): core/ + clients/ split + 5-domain internal layout + prod closure
b70cee23  feat: close frontend↔core wiring, ship Studio cloud builder, sync constitution  (pre-sprint baseline)
```

## What got built

### Monorepo restructure
- `apps/` → `core/` (orchestrator, runtime, cli) + `clients/` (web,
  vscode-extension, scrcpy-bridge). 984 git renames preserved.
- `core/<service>/internal/` regrouped into 5 domains:
  `business/`, `ai/`, `operations/`, `customer/`, `suppliers/` —
  80 packages mapped (see `docs/ARCHITECTURE_DOMAIN_MODULES.md`).
- `internal/pkg/{env,httputil,httpclient}` mirrored on both Go
  modules for cross-cutting helpers (env parser, JSON response,
  HTTP client factory).

### Economic engine (V22 hard laws enforced)
- **Wallet + Ledger**: Postgres source-of-truth; `event_outbox` with
  FOR UPDATE SKIP LOCKED; transactional outbox via
  `outboxhooks.WriteEventInTx`.
- **ProfitGuard**: 100% of expensive call sites gated. Closed the
  3 final deferred sites: EAS retry, domain registrar Purchase,
  ideaparser describeIdea (synthetic `pre_execution:<tenant>` band).
  Audit chain emits one `EventProfitGuardDecision` row per Decide.
- **Two payment providers** behind `PaymentProvider` interface:
  Stripe (existing) + Paddle (new at
  `core/orchestrator/internal/business/budget/payments/paddle.go`).
  Both can be active simultaneously.

### Data plane verified end-to-end
| Store | Status |
|---|---|
| Postgres | Source of truth; 7 new perf indexes (00043); AddCost uses static query map (pgx auto-prepare hits) |
| ClickHouse | 10 `fact_*` + `rollup_*` tables; `async_insert=1` enabled; consumer wireup subscribed to `audit.security`, `memory.indexing`, `runtime.lifecycle` |
| SurrealDB | Production-default backend (`IRONFLYER_MEMORY_BACKEND=surreal` in `values-prod.yaml`); `SurrealGraph` (523 LOC) wired via `wireup/memorygraph.go` |
| Redis | Lock = SETNX + random token + Lua CAS unlock; rate limiter = atomic TxPipeline with `EXPIRE NX` |
| Redpanda | `RequiredAcks=All`; capped exp backoff (5min max / 12 attempts); DLQ topic per source; `outbox_oldest_unpublished_age_seconds` gauge |

End-to-end trace verified: `outboxhooks.WriteEventInTx` → `Claim FOR
UPDATE SKIP LOCKED` → `Publish` → `MarkPublished` (after broker ack)
→ `Consumer.handle` → `INSERT INTO raw_*_events`.

### Anti-Bloat Engine (Ironflyer's competitive moat)
| Component | Path |
|---|---|
| Capability Atlas | `core/orchestrator/internal/ai/atlas/` — 2,549 caps indexed at boot in 301ms |
| Architecture Manifest | `.ironflyer/architecture.json` — 7 layers, 34 rules, 13 owners |
| Reuse-First Preflight | `core/orchestrator/internal/operations/patch/preflight.go` — every `OpCreate` searches Atlas; ≥ 0.85 cosine → reuse warning |
| Refactor Proposer | `core/orchestrator/internal/ai/refactor/` — given a dup finding, emits unified-diff Proposal extracting to shared util |
| Coder agent prompt | Reuse-First + Diff Economy + Boundary Honesty instructions in `agents.yaml` |
| Health Dashboard | `clients/web/app/cockpit/health/` UI + `dashboards.healthDashboard` GraphQL resolver — live |

**Gate status — 10/10 functional:**
- Structural: `reuse_check`, `dep_graph`, `arch_boundary`
- Tool-backed:
  - `vuln_scan` (govulncheck)
  - `dedup` (jscpd)
  - `deadcode` (knip)
  - `complexity` (gocognit)
  - `mem_leak` (goleak via `/debug/leak/snapshot` endpoint)
  - `bundle_size` (size-limit)
  - `perf_budget` (Lighthouse CI)

Tool scripts at `scripts/lint/run-*.sh`. Pre-deploy wrapper at
`scripts/health/run-health.sh`. CI integration in
`.github/workflows/ci.yml` `lint-health` job.

### Production hardening
- **Observability**: Sentry across orchestrator + runtime + web +
  vscode-extension. OTel spans on 5 worker daemons + per-request HTTP
  middleware. New Prometheus metrics: `executions_started_total`,
  `executions_completed_total{outcome}`, `wallet_holds_active`,
  `provider_cost_dollars_total{provider}`.
- **GraphQL caps**: complexity (1000), depth (15), APQ-locked mode
  with registry at `clients/web/src/lib/gql/operations`.
- **Security**: PII redaction in audit chain, constant-time bearer
  auth on `/metrics`, security headers middleware (HSTS/CSP/XFO/
  Referrer/Permissions), operator bypass preserved for sandbox.
- **Graceful shutdown**: 30s deadline, all HTTP timeouts set,
  listener panic recovery. 10 daemons wrapped with `superviseDaemon`
  (Sentry capture + zerolog Error + debug.Stack).
- **Runtime tuning**: env-driven `IRONFLYER_GOMAXPROCS`,
  `IRONFLYER_GOMEMLIMIT`. Helm prod pins GOMEMLIMIT=1638MiB,
  GOMAXPROCS=2, GOGC=100. terminationGracePeriodSeconds=45.
- **HPA cost-leak guards**: orchestrator 3-20, web 2-30, runtime
  1-50. ProfitGuard `PauseForBudget` as per-execution mirror.

### Performance optimizations
- **SSE chat hot path**: pooled `*bytes.Buffer` per frame — ~0
  allocations/frame in steady state.
- **Provider hot paths**: `streamingHTTPClient` memoized via
  `sync.Once`; Anthropic `isClaude4Family` no longer allocates per
  turn; `contains` delegates to tuned `strings.Contains`.
- **DB**: `DrainRefinements` rewritten as `pgx.Batch` (N → 1 round
  trips); 5 unbounded subscription replay scans bounded with LIMIT.
- **Cold start**: parallel `errgroup` inits for Sentry+OTel+Postgres+
  Surreal+Redis. Lazy modes via `IRONFLYER_SKIP_MIGRATE`,
  `IRONFLYER_PG_LAZY`, `IRONFLYER_REDIS_LAZY`.
- **Web bundle**: 9 components moved client→server; `JSZip`
  lazy-loaded; 3 unused swiper/css imports removed; Apollo cache
  keyFields for Project/Patch/LedgerEntry/GateVerdict;
  `nextFetchPolicy: cache-first`.
- **VSCode extension**: bundle 1.24MB → 597KB (-52%). Replaced
  `@sentry/node` (900KB of OpenTelemetry cruft) with 140-line
  envelope sender. 4 commands lazy-loaded.

### Deploy targets
- **DigitalOcean Pulumi** at `infra/pulumi/` — `Pulumi.prod.yaml`
  configured for `ironflyer.ai` (registrar DO → DNS Cloudflare).
- **Helm chart** at `infra/helm/ironflyer/` — `values-prod.yaml`
  enables ClickHouse + Redpanda + KEDA + OPA by default; goRuntime
  block exposes GOMEMLIMIT/GOMAXPROCS/GOGC.

### Templates
- 6 mobile + web starters now tracked in git:
  - `react-native-expo` (Expo SDK 53)
  - `android-kotlin` (AGP 8.7.2 + Kotlin 2.0.21)
  - `ios-swift` (Swift 5.10 + xcodegen + `PrivacyInfo.xcprivacy`)
  - `nextjs-ts`, `vite-react-ts`, `static-html`, `go-chi`, `astro`
- `templates/sites/` and `templates/deploy/` also tracked.

## Documentation (43+ docs)

Key entry points:
- `CLAUDE.md` — contract for AI assistants
- `ARCHITECTURE.md` — V22 locked spec
- `DEPLOY.md` — production deploy runbook
- `docs/CLOSEOUT_CHECKLIST.md` — operator-actionable preflight
- `docs/ANTI_BLOAT_ENGINE.md` — Anti-Bloat MVP architecture
- `docs/ARCHITECTURE_PROFITGUARD.md` — coverage matrix
- `docs/PERF_BUDGETS.md` — SLO contract
- `docs/SECURITY_HARDENING_2026-05-26.md` — 14 findings, 13 closed
- `docs/MOBILE_STARTERS_AUDIT_2026-05-26.md`
- `docs/ARCHITECTURE_DOMAIN_MODULES.md`
- `docs/RUNBOOKS/{cold-start,upgrade,rollback,cost-spike,workspace-saturation,graphql-incident}.md`

## Build verification (current)

```
core/orchestrator    go build ./... + go vet ./...    clean
core/runtime         go build ./... + go vet ./...    clean
core/cli             go build ./...                   clean
clients/scrcpy-bridge go build ./...                  clean
clients/web          npx tsc --noEmit                 clean
clients/vscode-extension npm run typecheck + build    clean
scripts/v22_smoke.sh against live orchestrator        PASS
```

## Definition of Done — final tally

Per `docs/PROJECT_CLOSEOUT_PLAN.md` §10:

| # | DoD item | Status |
|---|---|---|
| 1 | User can top up / run / stop / refund / inspect | ✓ |
| 2 | Every cost lands in ledger | ✓ |
| 3 | Every execution emits events to Redpanda | ✓ (prod default) |
| 4 | ClickHouse dashboards show margin in near real time | ✓ (async inserts + facts wired) |
| 5 | SurrealDB improves reuse/retrieval | ✓ (prod default) |
| 6 | Temporal resumes interrupted workflows | partial (opt-in) |
| 7 | Runtime workers scale by demand / shrink when idle | partial (HPA wired, KEDA enabled) |
| 8 | GraphQL hardened for production | ✓ (complexity/depth/APQ) |
| 9 | One synthetic paid execution passes in CI/CD | ✓ (v22_smoke.sh PASS) |
| 10 | Operators can deploy / rollback / restore / explain | ✓ |

**8 ✓ / 2 partial / 0 ✗**. Both partials are opt-in features that
activate when production profiles enable them.

## Operator actions before `pulumi up`

See `docs/CLOSEOUT_CHECKLIST.md` §2b for the full preflight. The
short version:

1. Domain delegation: NS records for `ironflyer.ai` at DO registrar
   → Cloudflare nameservers.
2. Pulumi config: ~20 secrets per `DEPLOY.md §4` table.
3. Vendor accounts: Paddle merchant account, Stripe live keys,
   Sentry org/project, GitHub App, Resend domain verified.
4. `pulumi login` + `pulumi config set` + `pulumi preview` +
   `pulumi up`.
5. Post-deploy smoke: `IRONFLYER_API_URL=https://api.ironflyer.ai
   bash scripts/v22_smoke.sh`.

## Explicit deferrals (post-launch backlog)

These are not in scope for the launch sprint; they need their own
dedicated work:

1. **Time partitioning** for `ledger_entries`, `audit_log`,
   `execution_events`. Multi-week schema work with backfill plan.
2. **Federated Capability Atlas** — cross-tenant Atlas (anonymous)
   for blueprint sharing.
3. **Diff Economy public metric** — "LOC per Resolved Capability"
   as a competitive comparison surface.

## Closing word

Ironflyer is at the production-ready bar this sprint targeted.
Every change preserves the constitutional rules (no tests, design
reference is law, viz-first, V22 hard economic laws). The remaining
work to reach live traffic is operator action — credentials,
domain delegation, and `pulumi up`.
