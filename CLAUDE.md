# Ironflyer — guidance for AI assistants

This file is the contract for AI coding agents (Claude Code, Cursor, Aider,
Replit Agent, etc.) working on this repo. Read it before you touch code.

## What Ironflyer is

An **AI Product Finisher**. Not a chatbot, not a no-code toy, not a VSCode
clone. The product takes an idea and ships it through enforced gates:
**Spec → UX → Architecture → Code → Lint → Tests → Security → Deploy**.
The differentiator is that the gates *block* until they pass.

If a change makes a generated app feel easier to fake-ship, it works
against the product. If a change tightens what "finished" means, it
works for it.

## Layout

```
ironflyer/
├── apps/
│   ├── orchestrator/       Go — finisher engine, gates, providers, budget,
│   │                       auth, leads, runtime client, metrics
│   ├── runtime/            Go — per-user workspace sandboxes (mock/docker)
│   ├── web/                Next.js 15 + MUI 6 — marketing + dashboard +
│   │                       project workspace
│   └── vscode-extension/   TS — chat + gates + patches inside VSCode
├── packages/
│   ├── design-tokens/      Output.com-inspired tokens (alabaster + lime)
│   ├── sdk/                @ironflyer/sdk — TS client for both APIs
│   └── agents/             (reserved; canonical prompts live in
│                           apps/orchestrator/internal/agents/agents.yaml)
├── infra/
│   ├── compose/            docker-compose.dev.yml
│   ├── docker/             Dockerfiles + Ironflyer-branded code-server
│   ├── k8s/                Raw k8s manifests + kustomization
│   └── helm/ironflyer/     Production Helm chart
├── scripts/                smoke.sh — post-deploy verification
└── DEPLOY.md               End-to-end production runbook
```

Both Go modules use their own `go.mod`. Web is a single Next app under
`apps/web`. The SDK + design tokens are imported from web via
`../../../packages/*`.

## Conventions

- **Patches are mandatory.** The AI never writes files directly — go through
  `patch.Engine.Propose` so the lifecycle gates approve before apply.
- **Gates take `(ctx, *GateEnv)`.** When you add a gate, register it in
  `finisher.DefaultGates()`, document it in `domain.GateName` constants,
  and add it to the web `GATE_ORDER` + SDK union.
- **Streaming first.** Every provider implements `CompleteStream`;
  non-streaming `Complete` is a wrapper. Tokens go through the
  `BillingGuard` so cost lands in the ledger.
- **Budget is a hard contract.** Every paid model call must go through
  `Billing.Admit` then `Billing.Charge`. The Vault snapshot is the source
  of truth for `revenue − providerCost = margin`.
- **Per-user isolation.** Workspaces, projects, leads, tokens — every
  store has an owner check. Use `requireProjectAccess` in HTTP handlers.

## Quality bar

- `go build`, `go vet`, and `go test ./...` MUST pass in both Go modules.
- `npx tsc --noEmit` MUST pass in `apps/web` and `packages/sdk`.
- `npm test` MUST pass in `apps/vscode-extension`.
- CI ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) runs all of
  the above plus builds + pushes 4 Docker images on `main`.

## Style

- **Go**: zerolog for logs (`a.d.Logger.Info().Str("k", v).Msg(...)`).
  Errors propagate; only `Fatal()` at startup. No global state outside the
  metrics registry.
- **TS**: server components by default in `apps/web`; only mark
  `'use client'` when you reach for state, refs, or events.
- **CSS**: MUI `sx` prop with the `tokens` from `packages/design-tokens`.
  Lime (`tokens.color.accent.lime`) is the single CTA accent — used with
  restraint.

## Pricing + the finisher promise

Pricing is aligned with Base44's published ladder so price-shopping
visitors don't see arbitrage: Free / $20 / $40 / Enterprise custom.
Margin floor: subscription − `CostCapUSD`. Don't change pricing without
recomputing the cap so margin stays positive.

The differentiation argument is in
[apps/web/app/marketing.tsx](apps/web/app/marketing.tsx) (`comparisonRows`).
When a competitor ships something we don't, mark it honestly — fake checks
poison the table.

## When in doubt

- Re-read `ARCHITECTURE.md`. It's the locked spec.
- Check `MEMORY.md`-style notes in [project_*.md](/Users/moshemoran/.claude/projects/-Users-moshemoran-workspace-ironflyer-copilot/memory/) if you have access — they capture decisions that don't belong in the codebase.
- Ask before adding a new top-level dep. The repo is intentionally light;
  every dep is a future maintenance bill.
