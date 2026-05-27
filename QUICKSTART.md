# Ironflyer Quickstart

The fastest path from clone to a paid execution that ships a real
artifact. Targets the V22 architecture: prepaid wallet, ProfitGuard
on every expensive call, blueprint-aware finisher loop, and a live
studio cockpit.

## Prerequisites

- Docker + Docker Compose (Desktop 4.27+ or compose v2)
- Go 1.23+
- Node 20+
- Optional: an Anthropic API key for real LLM execution (otherwise
  the orchestrator runs on the mock provider, which still completes
  every flow but doesn't emit interesting code)

## 1. Clone + env

```bash
git clone https://github.com/your-org/ironflyer-copilot.git
cd ironflyer-copilot/ironflyer
cp .env.example .env
# Minimum required for real LLM runs:
#   ANTHROPIC_API_KEY=sk-ant-...
# Leave Stripe blank for mock wallet flows.
```

## 2. Start infrastructure

The default profile brings up the **lean** stack — only what the
orchestrator + runtime hard-require (~300 MB combined RAM):

```bash
docker compose -f infra/compose/docker-compose.dev.yml up -d
# Brings up the lean default: postgres (pgvector), redis, surrealdb, minio.
```

Wait for healthchecks to settle (~30s on a warm machine):

```bash
docker compose -f infra/compose/docker-compose.dev.yml ps
```

`surrealdb` reports `unhealthy` in `docker compose ps` because its
bundled healthcheck queries an endpoint that returns 404 — the RPC at
`ws://localhost:8000/rpc` is live. Treat that as cosmetic.

Optional profiles (opt-in, not part of the lean default):

- `--profile analytics` — `redpanda` + `clickhouse` (~1.3 GB). Needed
  for the analytics pipeline; the orchestrator degrades gracefully
  when `REDPANDA_BROKERS` / `CLICKHOUSE_URL` are unset.
- `--profile temporal` — `temporal` + `temporal-ui` (~768 MB; UI on
  `:8233`). Needed for durable-workflow paths in production mode.
- `--profile stripe` — `stripe listen` forwarding to
  `host.docker.internal:8080/budget/webhook` (requires
  `STRIPE_SECRET_KEY` in `.env`).
- `--profile apps` — also builds + runs orchestrator/runtime/web in
  containers (slower iteration; usually better to keep them on host).
- `--profile full` — everything above.

## 3. Run database migrations

The orchestrator auto-applies migrations on boot, but you can also
run them explicitly. `POSTGRES_URL` is required by the migrate
binary:

```bash
cd core/orchestrator
POSTGRES_URL="postgres://ironflyer:ironflyer@localhost:5432/ironflyer?sslmode=disable" \
  go run ./cmd/migrate up
```

## 4. Start the orchestrator

```bash
cd core/orchestrator
go run ./cmd/orchestrator
# Listens on :8080. Logs the boot summary — confirm:
#   "V22 wallet: Postgres backend + durable outbox"
#   "ideaparser: llm backend wired" (if ANTHROPIC_API_KEY set)
#   "Stripe checkout + webhook enabled" (if Stripe set)
```

## 5. Start the runtime

```bash
cd core/runtime
go run ./cmd/runtime
# Listens on :8090. Mock driver by default — set
# IRONFLYER_RUNTIME_DRIVER=docker once you want real sandboxes.
```

## 6. Start the web cockpit

```bash
cd clients/web
npm install
npm run codegen
npm run dev
# Open http://localhost:3000
```

Web and Studio UI work must follow the locked reference in
`design-reference/2026-05-25-private-ironflyer/` and
`clients/web/DESIGN_REFERENCE.md`. Do not use older local captures as a
reason to drift from the private dark reference.

## 7. First execution — UI path

1. Open http://localhost:3000.
2. Sign up — creates the per-tenant wallet automatically.
3. Top up via Stripe test card `4242 4242 4242 4242` (any future
   expiry, any CVC). If Stripe is unwired, the dev wallet starts at
   zero — see step 8 for the CLI workaround.
4. Type a product idea in the hero input
   (`"A landing page for my coffee shop"`).
5. Hit **Build**. The Studio opens as the VS Code cloud-style builder:
   prompt, plan, code/files, preview, assistant, gates, and deploy
   context all stay connected to the same execution. The preview iframe
   lights up once the first artifact lands.

## 8. End-to-end smoke runner (CLI)

The smoke runner authenticates, walks the wallet → describeIdea →
poll → support bundle flow, and exits non-zero on any failure.

```bash
go run ./core/orchestrator/cmd/ironflyer-smoke \
  -graphql=http://localhost:8080/graphql \
  -email=demo@ironflyer.dev \
  -password=demo1234 \
  -prompt="A static landing for a yoga studio" \
  -budget=2 \
  -topup=25 \
  -timeout=10m
```

Flags:
- `-skip-signup` if the user already exists.
- `-auto-confirm-topup` to skip the manual checkout pause (use only
  when Stripe is wired up + you don't intend to actually top up).
- `-poll=5s` to change the execution status polling cadence.

The runner walks:
1. `signUp` (best-effort) → `signIn` (Bearer token).
2. `wallet` snapshot.
3. `walletCreateTopUp` (if `available < budget` and Stripe is wired).
4. `describeIdea` mutation — prints the chosen blueprint + cost
   estimate + execution id.
5. `execution(id:)` polled every `-poll` until terminal
   (`succeeded`/`failed`/`stopped`/`killed`).
6. `executionSupportBundle(executionID:)` — preview URL, changed
   files, cost report, gate verdicts.

## Architecture references

- `ARCHITECTURE.md` — locked V22 spec.
- `docs/V22_PLAN.md` — implementation contract.
- `docs/ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/` —
  full economic model.
- `DEPLOY.md` — production runbook.

## Troubleshooting

- **`describeIdea` returns `INSUFFICIENT_FUNDS`** — wallet is zero.
  Either complete a Stripe top-up or, in dev, seed credits by hand:
  ```sql
  -- inside docker exec -it <postgres-container> psql -U ironflyer
  INSERT INTO wallets(tenant_id, balance_usd, hold_usd) VALUES (
    '<your-tenant-id>', 100, 0
  ) ON CONFLICT (tenant_id) DO UPDATE
    SET balance_usd = EXCLUDED.balance_usd;
  ```
- **`go run ./cmd/orchestrator` fatals on Postgres** — check
  `docker compose ps`; the orchestrator dials Postgres at the URL in
  `POSTGRES_URL`. Default expects the compose container on
  `localhost:5432`.
- **`npm run codegen` fails to introspect the schema** — make sure
  the orchestrator is running and reachable at
  `http://localhost:8080/graphql`.
- **Web shows `NotConfigured` for V22 surfaces** — at least one V22
  service is unwired on the orchestrator. Confirm the boot logs say
  `V22 wallet`, `V22 execution`, `V22 profitguard store` etc. are
  Postgres-backed (need `IRONFLYER_DB_DRIVER=postgres` or `hybrid`).

## Studio surfaces (close-out)

Studio chat now streams agent reasoning events end-to-end. Each gate
emits `agent.stage.started.v1`, `agent.thinking.v1`,
`agent.tool.call.v1`, `agent.tool.result.v1`, and
`agent.stage.completed.v1` onto the execution event feed; the studio
chat consumes them via the existing executionEvents subscription. The
Files tab fetches a real file tree from the runtime sandbox (port
8090, `/workspaces/{id}/files`). The Preview iframe shows the live
workspace dev server — the runtime now allocates a preview port the
moment the workspace is created (default 3000, blueprint can override
via `X-Ironflyer-Preview-Port`) and the executionSupportBundle resolver
returns the live URL instead of waiting on a deploy. Publish walks the
five-step Plan → Build → Approve → Promote → Live stepper, with per-
phase progress driven by `planDeploy` / `buildDeployPreview` /
`decideApproval` / `promoteDeploy`.

## Studio ↔ Core integration map

```
User action in studio          →  Orchestrator surface                    →  Core path
─────────────────────────────────────────────────────────────────────────────────────────
Type prompt + Build            →  describeIdea mutation                   →  IdeaParser → Project.Create → Execution.Create+Admit+Start → engine.Run
Send chat message              →  refineIdea mutation                     →  Execution event (studio.refine.v1) → engine.consumeRefinements() at next gate → Project.Spec.Idea updated → next agent prompt sees it
View live preview              →  executionSupportBundle.previewURL       →  wowloop.RuntimeSource (runtimePreviewAdapter) → runtime.Client.FindWorkspaceForProject(projectID) → runtime.Client.PreviewURL(wsID) → workspace dev server
View files                     →  workspaceFiles query → GET /workspaces/{id}/files  →  runtime sandbox driver (mock/docker)
Publish                        →  planDeploy → buildDeployPreview → (decideApproval) → promoteDeploy  →  deploy.Service → Vercel adapter → workspace artifact
Agent reasoning stream         →  executionEvents subscription            →  finisher emits agent.{stage.started,thinking,tool.call,tool.result,stage.completed}.v1 onto execution_events
```

ExecutionSnapshot.WorkspaceID is populated from `execution.ProjectID`
(no schema migration) and the runtime adapter dereferences it via
`FindWorkspaceForProject`. The service-to-service bearer comes from
`IRONFLYER_RUNTIME_BEARER` (optional; unset = anonymous to the
runtime, which falls back to its "demo" user in dev).
