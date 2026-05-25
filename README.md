# Ironflyer

**Paid AI execution engine** — ships finished products end-to-end on
prepaid wallet credits, with hard economic enforcement at every step.

- Architecture: [ARCHITECTURE.md](ARCHITECTURE.md)
- Implementation contract: [docs/V22_PLAN.md](docs/V22_PLAN.md)
- Closeout plan: [docs/PROJECT_CLOSEOUT_PLAN.md](docs/PROJECT_CLOSEOUT_PLAN.md)
- Economic proof pack:
  [docs/ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/](docs/ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/)

## Hard economic laws

1. **No execution starts without budget.** Wallet balance ≥ reservation
   or the API returns 402 Payment Required with a `top_up_url`.
2. **No expensive reasoning runs without expected ROI.** ProfitGuard
   gates every premium model call, sandbox allocation, retry loop,
   long verification, and large artifact write.
3. **No scale is considered healthy unless gross margin stays
   protected.**

## What's in the box

- **Go orchestrator** with streaming-first provider router (Anthropic
  Claude, OpenAI, Gemini, HuggingFace, DeepSeek, Vercel AI Gateway),
  patch lifecycle, finisher gates, audit hash chain, append-only
  ledger, and wallet-aware billing guard. Wallet, execution,
  ProfitGuard, blueprints, repair recipes, completion scoring, and
  dashboards are owned by Agents 2-7 of the V22 overhaul.
- **Workspace runtime** — per-user sandboxes via Mock or Docker
  driver, File API, PTY WebSocket bridge for live terminals, and the
  code-server based cloud IDE path.
- **Next.js + MUI Studio cockpit** — wallet, execution, profit, scale,
  Studio, preview, deploy and cloud IDE entry points. Web UI changes
  are governed by [`apps/web/DESIGN_REFERENCE.md`](apps/web/DESIGN_REFERENCE.md)
  and [`design-reference/2026-05-25-private-ironflyer/`](design-reference/2026-05-25-private-ironflyer/).
- **VSCode extension** — thin client for chat + gates + patches.

## Quick start (dev)

```bash
# 1. Optional: spin up infra (Postgres, Redis, MinIO, code-server)
docker compose -f infra/compose/docker-compose.dev.yml up -d

# 2. Run orchestrator (defaults to mock provider; export ANTHROPIC_API_KEY for real)
cd apps/orchestrator && go run ./cmd/orchestrator
# → http://localhost:8080

# 3. Run workspace runtime
cd apps/runtime && go run ./cmd/runtime
# → http://localhost:8090

# 4. Run web
cd apps/web && npm install && npm run dev
# → http://localhost:3000
```

## Environment

The orchestrator + runtime are configured by env vars. The full list,
with secret tables for the production install, lives in
[`DEPLOY.md`](DEPLOY.md) § 4. For local-only dev the defaults work;
the most common dev overrides are:

- `ANTHROPIC_API_KEY` — enable real Claude streaming.
- `OPENAI_API_KEY` / `GEMINI_API_KEY` / `HF_API_KEY` /
  `DEEPSEEK_API_KEY` — enable additional providers in the bandit.
- `IRONFLYER_RUNTIME_DRIVER=docker` — switch the runtime off the mock
  driver to real Docker sandboxes.
- `STRIPE_SECRET_KEY` + `STRIPE_WEBHOOK_SECRET` + `STRIPE_PRICE_*` —
  enable the wallet top-up Checkout flow.

## Monorepo

| Path | Purpose |
| --- | --- |
| `apps/orchestrator` | Finisher engine + gates + wallet + ledger + ProfitGuard + auth |
| `apps/runtime`      | Workspace runtime — sandboxes, File API, PTY WS |
| `apps/web`          | Next.js + MUI dashboard (wallet, profit, scale views) |
| `apps/cli`          | Operator CLI |
| `apps/vscode-extension` | VSCode extension — thin client |
| `packages/design-tokens` | IronFlyer design tokens implementing the locked reference |
| `packages/sdk`      | TypeScript client SDK |
| `packages/agents`   | Agent prompts + JSON schemas (reserved) |
| `infra/`            | Compose / Docker / k8s / Helm |

## API documentation

The orchestrator's API of record is **GraphQL**. The schema is the
single source of truth — see
`apps/orchestrator/internal/graph/schema/*.graphql`.

| Surface                  | URL                       | Transport                   |
| ------------------------ | ------------------------- | --------------------------- |
| Queries / mutations      | `POST /graphql`           | HTTP                        |
| Persisted-query GETs     | `GET  /graphql`           | HTTP                        |
| Subscriptions            | `WS   /graphql`           | `graphql-transport-ws`      |
| Live schema + playground | `GET  /graphql/sandbox`   | HTML (Apollo Sandbox embed) |

Visit `/graphql/sandbox` in a browser to explore the schema, paste a
JWT into the HTTP Headers tab, and run live queries.

REST is reserved for k8s probes (`/healthz`, `/livez`, `/readyz`,
`/version`), Prometheus (`/metrics`), and the Stripe webhook
(`/budget/webhook`). New features land as GraphQL operations — see
[`CLAUDE.md`](CLAUDE.md).

## Operations

Everything operational — install, upgrade, rollback, runbooks, SLOs,
DR, scale, multi-region — lives behind a single index:

**[`docs/OPERATIONS.md`](docs/OPERATIONS.md)** — the page operators
bookmark.

If you're installing for the first time, start at
[`DEPLOY.md`](DEPLOY.md).

## Architecture Closeout

V22 now has explicit, non-overlapping architecture planes:

- [Events / Redpanda](docs/ARCHITECTURE_EVENTS.md)
- [Analytics / ClickHouse](docs/ARCHITECTURE_ANALYTICS.md)
- [Durable Workflows / Temporal](docs/ARCHITECTURE_WORKFLOWS.md)
- [AI Memory Graph / SurrealDB](docs/ARCHITECTURE_MEMORY_GRAPH.md)
- [Runtime Scale / Sandboxes](docs/ARCHITECTURE_RUNTIME_SCALE.md)
- [Policy, Security, Trust](docs/ARCHITECTURE_POLICY_SECURITY.md)
- [Project Closeout Plan](docs/PROJECT_CLOSEOUT_PLAN.md)

## Status

V22 is in commercial closeout. The backend foundation builds, the
wallet/ledger/execution/ProfitGuard surface is present, and the current
priority is closing the paid execution loop end-to-end: every agent call
through BillingGuard, durable events through Redpanda, margin analytics
through ClickHouse, long executions through Temporal, and AI memory
through SurrealDB. The web surface is intentionally next: rebuild it as
an execution cockpit against GraphQL and the SDK.
