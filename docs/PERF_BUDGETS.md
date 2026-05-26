# Ironflyer Performance Budgets

Ironflyer's competitive moat against Lovable / Bolt / v0 / Replit Agent
is speed — they're slow. This document is the **performance contract**
every PR is measured against. If a change pushes any number past the
budgets below, the PR is wrong even if the feature is correct.

## Cold start

| Surface       | p50    | p99    | Notes                                                                |
|---------------|--------|--------|----------------------------------------------------------------------|
| Orchestrator  | < 2s   | < 4s   | From process start to "listening" log line, with Postgres + Redis    |
| Runtime       | < 1.5s | < 3s   | From process start to "listening" log line, with S3 + Postgres       |
| Web (Next 15) | < 800ms| < 1.8s | Cold SSR first-byte for `/`, no client cache, prod build             |

### How orchestrator cold start hits budget

1. **Parallel init via `errgroup`.** Sentry, OTel, Postgres pool dial,
   Redis dial, and SurrealDB dial run in parallel in `main.go`. Total
   boot wait is `max(t_each)` instead of `sum(t_each)`.
2. **Lazy Postgres connect.** `pgxpool.NewWithConfig` is already lazy;
   the only blocking call is the explicit `pool.Ping(...)`. Setting
   `IRONFLYER_PG_LAZY=true` skips that ping and lets the first
   request pay the connect cost. Use this in preview / autoscale
   pods where boot SLO is tighter than first-request SLO.
3. **Skip in-process migrations in prod.** Set
   `IRONFLYER_SKIP_MIGRATE=true` when migrations are run as a
   separate helm pre-install hook
   (`infra/helm/ironflyer/templates/migrate-job.yaml`). Saves
   200-800 ms per boot.
4. **Per-step timing.** Each init leg logs its own `Dur("took", …)`
   so the operator can see where boot time actually goes without
   guessing.

### How runtime cold start hits budget

1. **Parallel init via `errgroup`.** Sentry and the scale plane
   (S3 snapshot manager, warm pool, allocator, quota) build in
   parallel.
2. **Idle scanner tick = 2 min.** Default tick interval in the
   workspace idle scanner. Don't crank this below 30s without a
   specific reason; the scanner walks the workspace store every
   tick.
3. **Warm pool drainer default = 60s.** Sane default in
   `warmpool.NewDrainer`. Bump higher when paid demand is bursty.

## p99 request latency

| Endpoint                                         | p50    | p99    |
|--------------------------------------------------|--------|--------|
| `POST /graphql` — simple query (dashboard tile)  | < 30ms | < 100ms|
| `POST /graphql` — mutation (no LLM)              | < 60ms | < 200ms|
| `POST /executions/{id}/chat/stream` — first byte | < 200ms| < 500ms|
| `POST /executions/{id}/chat/stream` — per token  | < 30ms | < 80ms |
| `GET /healthz`, `/livez`, `/readyz`              | < 5ms  | < 20ms |

### GraphQL latency knobs

- **APQ cache.** `GRAPHQL_APQ_LRU` defaults to **5000**. The old
  default of 100 forced LRU eviction on the dashboard hot-path
  during morning login waves, defeating the point of APQ. Bumped
  to 5000 so 40K-user installs stop thrashing.
- **Query parse cache.** `GRAPHQL_QUERY_CACHE_LRU` defaults to
  **5000** for the same reason — keep parsed `*ast.QueryDocument`
  in memory across requests instead of re-parsing.
- **Depth + complexity caps.** Always-on. `GRAPHQL_DEPTH_LIMIT` and
  `GRAPHQL_COMPLEXITY_LIMIT` reject fragment-cycle / quadratic-blowup
  DoS shapes before they hit a resolver.

### SSE / chat-stream knobs

- **Per-frame buffer pool.** `sseFrameBufPool` reuses the
  `bytes.Buffer` across frames. Verified live in
  `core/orchestrator/internal/operations/httpapi/chat_stream.go`.
- **Flush on every delta.** Every text / thinking / tool_call /
  finish / error frame ends with `flusher.Flush()`. No batching.
- **Provider channel buffer ≥ 32.** Every provider's `out` channel
  is buffered at 32 deltas. Mock at 16. Backpressure does not
  stall the provider's HTTP read loop.

## Per-request allocation budget

- GraphQL hot path (simple query, no resolver fan-out): **< 5 KB**
  per-request alloc (measured under `-benchmem`).
- SSE chat-stream per frame: **0 allocs** after the buffer pool
  warms.

## Web (Next 15)

- `experimental.optimizeCss: true` — critical CSS inlined.
- `experimental.optimizePackageImports`: MUI material / icons /
  lab / x-charts / x-data-grid, echarts(-for-react), @xyflow/react,
  lodash. Each one ships one named export per import instead of
  the whole barrel.
- Heavy viz libs (`echarts`, `@xyflow/react`, `three`) MUST land
  through `next/dynamic` with `ssr: false`. Never import them at
  module top-level.
- No `await import(...)` at page module top-level. Defer to the
  component body.

## Postgres pool

- `MaxConns=50`, `MinConns=5` (override via `POSTGRES_MAX_CONNS` /
  `POSTGRES_MIN_CONNS`).
- `MaxConnLifetime=30m`, `MaxConnIdleTime=5m`,
  `HealthCheckPeriod=30s` (override via the matching env vars).
- `IRONFLYER_PG_LAZY=true` skips the boot-time Ping when boot SLO
  beats first-request SLO.

## Where to look when a budget breaks

| Symptom                                  | First place to look                                          |
|------------------------------------------|--------------------------------------------------------------|
| Orchestrator boot > 2s p50               | Per-step `took=` logs in main.go boot block                  |
| GraphQL p99 spiking                      | APQ + query cache size; complexity + depth caps              |
| SSE first-byte > 500ms                   | Provider Pick chain; bandit cold-start; auth verify time     |
| SSE per-token > 80ms                     | Provider HTTP read; sseFrameBufPool eviction; flusher impl   |
| Runtime allocate > 100ms (warm hit)      | Warmpool `Lease` path; `runtimeClass` mismatch in pool       |
| Runtime allocate > 5s (cold start)       | Docker image pull cold; switch to alpine-based pre-pulled    |
| Web first paint > 1.8s                   | next.config experimental flags; lazy-loaded chart imports    |

## Constitutional rule

If a PR touches `main.go`, a hot-path resolver, the SSE writer, or
`next.config.mjs` and does not include a per-step timing line in the
description, ship a revert. Speed is the moat. We measure it.
