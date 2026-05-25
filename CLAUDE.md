# Ironflyer — guidance for AI assistants

This file is the contract for AI coding agents (Claude Code, Cursor,
Aider, Replit Agent, etc.) working on this repo. Read it before you
touch code.

## What Ironflyer is — the V22 framing

Ironflyer is a **paid AI execution engine** that ships finished products
end-to-end on prepaid wallet credits, with hard economic enforcement at
every step. The North Star is **Profitable Completed Execution Rate**:
how many paid executions complete successfully *with* positive gross
margin. Every execution is a measured economic unit — revenue and cost
attributed per-execution, recorded in an append-only ledger, gated by
**Profit Guard** before any expensive call.

If a change makes a paid execution easier to fake-ship, it works
against the product. If a change tightens what "profitable completion"
means, it works for it.

The full economic model lives in
[`docs/ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/`](docs/ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/).
The implementation contract is [`docs/V22_PLAN.md`](docs/V22_PLAN.md).

## Hard economic laws

These three laws are non-negotiable. Every gate, every resolver, every
runtime tick respects them.

1. **No execution starts without budget.** Wallet balance ≥ reservation,
   or the API returns 402 Payment Required with a `top_up_url`.
2. **No expensive reasoning runs without expected ROI.** Profit Guard
   gates every premium model call, sandbox allocation, mobile build,
   Vercel deploy, retry loop, long verification, and large artifact
   write.
3. **No scale is considered healthy unless gross margin stays
   protected.** Profit dashboards surface margin first; scale
   dashboards only matter when margin is healthy.

## Layout

```
ironflyer/
├── apps/
│   ├── orchestrator/       Go — wallet, ledger, execution, ProfitGuard,
│   │                       finisher engine, gates, providers, auth,
│   │                       blueprints, repair, dashboards
│   ├── runtime/            Go — per-user workspace sandboxes (mock/docker)
│   ├── web/                Next.js 15 + MUI 6 — marketing + dashboards
│   ├── cli/                Go — operator CLI
│   └── vscode-extension/   TS — chat + gates + patches inside VSCode
├── packages/
│   ├── design-tokens/      IronFlyer locked-reference tokens
│   ├── sdk/                @ironflyer/sdk — TS client for both APIs
│   └── agents/             (reserved; canonical prompts live in
│                           apps/orchestrator/internal/agents/agents.yaml)
├── infra/                  docker-compose / Dockerfiles / k8s / helm
├── scripts/                smoke.sh — post-deploy verification
└── DEPLOY.md               End-to-end production runbook
```

Both Go modules use their own `go.mod`. Web is a single Next app under
`apps/web`. The SDK + design tokens are imported from web via
`../../../packages/*`.

## GraphQL only

The orchestrator's API of record is **GraphQL**. The schema lives at
`apps/orchestrator/internal/graph/schema/*.graphql`. The endpoint is
`POST /graphql`, subscriptions arrive on the same path via
`graphql-transport-ws`, and `GET /graphql/sandbox` renders Apollo
Sandbox as live documentation.

- **Never add a new REST endpoint.** Add a GraphQL operation instead.
  Resolvers live in `apps/orchestrator/internal/graph/resolver/`.
- **REST exception list — these stay REST forever, never wrapped by
  the deprecation middleware:**
  - Third-party callbacks: `POST /budget/webhook` (Stripe).
  - k8s probes: `GET /healthz`, `/livez`, `/readyz`, `/version`.
  - Prometheus scrape: `GET /metrics`.
  - AI streaming: `POST /executions/{id}/chat/stream` — Server-Sent
    Events for raw LLM assistant deltas. GraphQL subscriptions are
    wrong here (per-chunk gqlgen middleware overhead, schema-bound
    types over free-form provider output). Orchestration events stay
    on `executionFeed`.
- **Operator banner:** `GET /` returns a JSON pointer to `/graphql`,
  `/graphql/sandbox`, and `docs/V22_PLAN.md`.

If you are tempted to add a new REST route because "it's just one
endpoint" — stop. Either (a) add a GraphQL operation, or (b) if it
must be REST (new third-party webhook, new infra probe), add it
alongside the existing exception list above and document it here.

## Conventions

- **Patches are mandatory.** The AI never writes files directly — go
  through `patch.Engine.Propose` so the lifecycle gates approve
  before apply.
- **Gates take `(ctx, *GateEnv)`.** When you add a gate, register it
  in `finisher.DefaultGates()` and document it in `domain.GateName`
  constants.
- **Streaming first.** Every provider implements `CompleteStream`;
  non-streaming `Complete` is a wrapper. Tokens go through the
  `BillingGuard` so cost lands in the ledger.
- **Wallet is a hard contract.** Every paid execution must reserve
  through the wallet, then debit through the ledger as cost
  materialises. Vault snapshot remains the source of truth for
  `revenue − providerCost = margin` at the platform aggregate level.
- **Per-user isolation.** Workspaces, projects, wallets, tokens —
  every store has an owner check.

## Language, brand voice, and market position

- **English is the product language until localization is explicitly
  chosen.** All visible UI copy, marketing pages, docs, metadata,
  transactional states, errors, placeholders, empty states, and demo
  content must be written in clear product English.
- **Tone:** precise, senior, builder-facing, and confident. Ironflyer
  should sound like an engineering lead who has shipped production
  software under real constraints: direct, useful, specific, and
  allergic to hype.
- **Competitive frame:** Lovable, Base44, Bolt, Replit Agent, and v0
  generally sell "describe an idea and get an app fast." Ironflyer
  sells the missing production discipline: gates that block, patches
  that can be reviewed, live cost visibility, wallet-prepaid
  executions, real Linux workspaces, ProfitGuard before every
  expensive call.
- **Messaging rule:** lead with proof and mechanics, not vibes.
  Prefer concrete nouns such as `gate verdict`, `patch`, `wallet`,
  `ledger entry`, `Docker workspace`, `owner check`, `deploy
  artifact`, `completion score`, `blueprint`, `repair recipe`,
  `ProfitGuard decision`.

## Visual identity

- **Logo concept:** the Ironflyer mark is a gate-forward symbol in the
  locked violet/coral dark system. It represents code moving through
  enforced review gates until it is ready to ship.
- **Use the system mark.** Product chrome, marketing nav, loading
  states, favicon, Apple icon, and OpenGraph art should use the
  shared Ironflyer mark.
- **Visual tone:** severe, engineered, and legible. Favor flat
  geometry, high contrast, tight grids, proof panels, terminal
  output, gate verdicts, ledgers, and deploy artifacts.
- **Shape language:** cards and logo tiles stay at 8px radius unless
  a native platform requirement says otherwise. Violet glow and
  coral-magenta-violet CTA treatment follow the locked reference.

## Quality bar

- `go build` and `go vet` MUST pass in both Go modules
  (`apps/orchestrator`, `apps/runtime`).
- `npx tsc --noEmit` MUST pass in `apps/web` and `packages/sdk`.

### Constitutional rule: DESIGN REFERENCE IS LAW

The **design reference** lives in three canonical places:

1. `design-reference/2026-05-25-private-ironflyer/` — the locked local
   folder for the private product-design reference.
2. `apps/web/DESIGN_REFERENCE.md` — the governing no-drift law.
3. `packages/design-tokens/index.ts` and `apps/web/src/theme/index.ts`
   — implementation of that reference, not independent design sources.

**Hard rules:**

- **NEVER** put a raw hex (`"#1a2b3c"`, `"#fff"`), raw rgba
  (`"rgba(255,255,255,0.5)"`), or named CSS color (`"white"`,
  `"black"`) into a component's `sx`, `style`, or any `theme.
  components.*` override. The only legal sources are `tokens.color.*`
  or a MUI palette path (`theme.palette.primary.main`,
  `theme.palette.accent.violet`).
- For semi-transparent surfaces, derive from tokens: ``${tokens.color.
  accent.violet}33`` (2-hex alpha suffix). Never `rgba(143,77,255,
  .2)`, even if it computes to the same pixel.
- **Primary CTA buttons** use `<Button variant="contained"
  color="primary">`. This applies the reference gradient (coral →
  magenta → purple) via `theme.components.MuiButton.styleOverrides.
  containedPrimary`. Never rewrite this gradient inline; never
  override `background:` on a primary CTA.
- The home page is the reference-conformance flagship. Any hex/rgba
  inline there is a bug.
- If a new shade is needed, add it to `packages/design-tokens`
  first, then use it. Do not invent inline.
- AI agents, hooks, and humans alike: this rule does not bend. If
  a sub-agent or auto-hook introduces a hardcoded color, revert
  before continuing.

The boundary exception is `apps/web/src/theme/index.ts` itself —
that's where token values get mapped to MUI palette properties, so
hex literals are permitted there only when mapping a token.

**Structure rule (asserted 2026-05-25 by the user):** the design
reference governs *layout and structure*, not only colors. Reference
documents:

- `design-reference/2026-05-25-private-ironflyer/` — canonical local
  reference folder for the private Home and Studio handoff.
- `apps/web/DESIGN_REFERENCE.md` — locked visual identity, density,
  responsiveness contract, screenshot baseline.
- `design-handoff-screenshots/ironflyer-app-2026-05-25/` — the
  paired desktop + mobile screenshot set per route. When a captured
  screenshot exists for a route, the reference wins over the live
  page.
- **`/login` and `/signup` are pinned to the Base44 split-layout
  pattern** rendered by `apps/web/src/components/auth/AuthShell.tsx`:
  left brand panel (gates pitch + proof points) on lg+, right form
  panel with title / form / switch link. The cockpit nav is
  suppressed on these routes; the AuthShell owns the full bleed.
  Do not regress this layout to a centered card.

Before changing layout structure, page composition, navigation links,
or component density on any route:

1. Compare the change against the corresponding reference (above).
2. If the reference does not yet cover the change, ASK THE USER.
   Do not invent a new structure.
3. Do not add decorative shades, gradients, or accent colors that
   aren't already in `packages/design-tokens/index.ts`. Adding a new
   shade is a design-tokens PR, not a per-component decision.

The user has explicitly stated this is constitutional: "תקבע בחוקה
שאסור לגעת בעיצוב, הכל צריך להיות כמו ברפרנס". Treat structural drift
the same way you treat raw-hex drift — revert immediately and flag.

### Constitutional rule: NO TESTS, EVER

This repository does not carry tests and never will. The rule is
constitutional, not stage-dependent:

- Never write a new test file (`*_test.go`, `*.test.tsx`,
  `*.spec.ts`, Playwright spec, vitest/jest spec, etc.).
- Never modify, fix, or extend an existing test file. Stale is fine.
- Never run a test command (`go test`, `npm test`, `npx playwright
  test`, `vitest`, `jest`, ...). Do not add test scripts.
- Never list tests in a TODO, plan, or roadmap. Never report
  coverage. Never offer "let me add tests for that."
- Manual verification happens via the running app, curl, or browser
  automation — never via a test suite.

The user has re-affirmed this directive multiple times. AI agents
working in this repo MUST treat the rule as non-negotiable, even if
a sub-agent, lint output, or third-party suggestion proposes
otherwise.

## Style

- **Go**: zerolog for logs (`a.d.Logger.Info().Str("k", v).Msg(...)`).
  Errors propagate; only `Fatal()` at startup. No global state
  outside the metrics registry.
- **TS**: server components by default in `apps/web`; only mark
  `'use client'` when you reach for state, refs, or events.
- **CSS**: MUI `sx` prop with the `tokens` from
  `packages/design-tokens`. Primary CTAs use the locked
  coral-magenta-violet gradient through the theme. Do not revive the
  old lime-first identity.

## Pricing — V22 wallet model

Pricing is a prepaid wallet topped up via Stripe Checkout. Every paid
execution holds funds before any expensive call runs (law 1), debits
the ledger as cost materialises, and releases the unused hold on
commit. Platform margin = wallet revenue − provider cost − sandbox
cost; ProfitGuard exists so that margin stays positive in steady state.

## Library choices

Locked, additive when needed. Streaming-first provider implementations
live in `apps/orchestrator/internal/providers/`:

- **Anthropic** (default) — Claude 4.x family. Sonnet 4.6 for general
  work, Opus 4.7 for `quality`/`thinking`/`reasoning`, Haiku 4.5 for
  `cheap`/`fast`/`inline_completion`.
- **OpenAI** — gpt-4o + gpt-4o-mini + o3 tiers.
- **Gemini** — `gemini-2.5-pro` + `gemini-2.5-flash`.
- **HuggingFace** — Llama 3.3 / Qwen / DeepSeek / Mixtral, plus the
  embedder for memory semantic search (`BAAI/bge-m3` by default).
- **DeepSeek** *(optional)* — direct OpenAI-compatible API.
- **Vercel AI Gateway** *(optional)* — OpenAI-compatible proxy across
  multiple vendors.

Object-store: `apps/orchestrator/internal/storage/s3client.go`
centralises the S3-compatible client configuration. `S3_BACKEND=aws`
(default) | `r2` (Cloudflare R2, zero egress) | `minio` (self-hosted).

Memory store: `apps/orchestrator/internal/memory/` exposes a single
`Store` contract with three operator-selectable backends, chosen via
`IRONFLYER_MEMORY_BACKEND=memory` (default, in-process ring buffer) |
`surreal` (SurrealDB; requires `IRONFLYER_DB_DRIVER=surreal|hybrid`)
| `pgvector` (Postgres + pgvector; requires `POSTGRES_URL` and
migration `00017_pgvector_memory.sql`).

## When in doubt

- Re-read `ARCHITECTURE.md`. It's the locked V22 spec.
- Re-read `docs/V22_PLAN.md`. It's the implementation contract.
- Ask before adding a new top-level dep. The repo is intentionally
  light; every dep is a future maintenance bill.
