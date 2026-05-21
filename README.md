# Ironflyer

**AI Product Finisher** — combines Lovable + Base44 + Copilot on steroids,
engineered to actually finish real products end-to-end.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the locked spec.

## What's in the box (current state)

- **Go orchestrator** with streaming-first provider router, Anthropic Claude
  adapter (caching + extended thinking + tools), Temporal workflow engine,
  patch lifecycle, finisher gates, brainstorm strategist.
- **Self-managing budget** — subscription tiers, rate sheet, per-user
  ledger, company vault, optimizer (cheapest model that satisfies the
  required capabilities), enforcer (admit/downgrade/block).
- **Workspace runtime** — per-user sandboxes via Mock or Docker driver,
  File API, PTY WebSocket bridge for live terminals.
- **Next.js + MUI dashboard** — output.com aesthetic, lovable.dev flow,
  streaming chat with token deltas, Finisher gates panel, Budget meter,
  Brainstorm pane, PWA manifest for mobile take-away.

## Quick start (dev)

```bash
# 1. Optional: spin up infra (Postgres, Redis, MinIO, Temporal, code-server)
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

| Var | Default | Purpose |
| --- | --- | --- |
| `IRONFLYER_ADDR` | `:8080` | Orchestrator listen addr |
| `IRONFLYER_ENV` | `dev` | dev / staging / prod |
| `IRONFLYER_LOG_FORMAT` | `console` | `console` or `json` |
| `IRONFLYER_EXECUTOR` | `embedded` | `embedded` or `temporal` |
| `TEMPORAL_ADDR` | `localhost:7233` | when executor=temporal |
| `ANTHROPIC_API_KEY` | _(empty)_ | enable real Claude streaming |
| `ANTHROPIC_MODEL` | `claude-opus-4-7` | preferred Claude model |
| `OPENAI_API_KEY` | _(empty)_ | reserved |
| `IRONFLYER_RUNTIME_ADDR` | `:8090` | Workspace runtime listen addr |
| `IRONFLYER_RUNTIME_DRIVER` | `mock` | `mock` or `docker` |

## Monorepo

| Path | Purpose |
| --- | --- |
| `apps/api` | Public API gateway (reserved) |
| `apps/orchestrator` | Finisher engine + gates + repair loop + budget + brainstorm |
| `apps/inference` | ONNX private AI (reserved) |
| `apps/runtime` | Workspace runtime — sandboxes, File API, PTY WS |
| `apps/web` | Next.js + MUI dashboard |
| `apps/mobile` | PWA mobile shell (web is PWA-installable) |
| `packages/agents` | Agent prompts + JSON schemas (reserved) |
| `packages/design-tokens` | output.com-inspired tokens |
| `packages/ui`, `packages/sdk` | Reserved |
| `services/figma-slicer` | Figma → component tree (reserved) |
| `services/patch-engine`, `services/sandbox` | Reserved |
| `infra/` | Compose / Docker / k8s |

## Key endpoints

### Orchestrator
- `GET  /health` · `GET /agents`
- `GET  /projects` · `POST /projects` · `GET /projects/{id}`
- `POST /projects/{id}/run` — finisher loop (gates + repair)
- `GET  /projects/{id}/stream` — execution SSE
- `POST /projects/{id}/chat` — streaming chat SSE (POST)
- `POST /projects/{id}/brainstorm` — Strategist + Runner
- `GET  /projects/{id}/files` · `POST /projects/{id}/patches` · `POST /patches/{id}/apply`
- `GET  /budget/plans` · `/budget/rates` · `/budget/vault` · `/budget/users/{userId}`

### Runtime (workspace)
- `POST   /workspaces` — create per-user workspace
- `GET    /workspaces/{id}` — status
- `DELETE /workspaces/{id}` — teardown
- `GET    /workspaces/{id}/files` — list files
- `GET    /workspaces/{id}/files/*path` — read file
- `PUT    /workspaces/{id}/files/*path` — write file
- `DELETE /workspaces/{id}/files/*path` — delete file
- `GET    /workspaces/{id}/terminal` — WebSocket → PTY

## Status

Phase 1 — runnable foundation with streaming chat, multi-agent brainstorm,
self-managing budget, Temporal workflow, and cloud workspace runtime.
Mock providers/drivers make everything work without external credentials.

Next: real Postgres-backed store, Stripe webhooks, ONNX inference service,
Figma slicer.
