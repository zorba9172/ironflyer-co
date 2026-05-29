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
├── core/
│   ├── orchestrator/       Go — wallet, ledger, execution, ProfitGuard,
│   │                       finisher engine, gates, providers, auth,
│   │                       blueprints, repair, dashboards
│   ├── runtime/            Go — per-user workspace sandboxes (mock/docker)
│   └── cli/                Go — operator CLI
├── clients/
│   ├── web/                Next.js 15 + MUI 6 — marketing + dashboards
│   ├── vscode-extension/   TS — chat + gates + patches inside VSCode
│   └── scrcpy-bridge/      scrcpy WebSocket bridge for mobile mirroring
├── packages/
│   ├── design-tokens/      IronFlyer locked-reference tokens
│   ├── sdk/                @ironflyer/sdk — TS client for both APIs
│   └── agents/             (reserved; canonical prompts live in
│                           core/orchestrator/internal/ai/agents/agents.yaml)
├── infra/                  docker-compose / Dockerfiles / k8s / helm
├── scripts/                smoke.sh — post-deploy verification
└── DEPLOY.md               End-to-end production runbook
```

Both Go modules use their own `go.mod`. Web is a single Next app under
`clients/web`. The SDK + design tokens are imported from web via
`../../../packages/*`.

> **`clients/web` is LEGACY and disliked.** It is being retired in favor of the
> new `clients/` surfaces (studio / backoffice / marketing / mobile — see
> `docs/ARCHITECTURE_CLIENTS.md`). The owner explicitly dislikes this app,
> *especially how it presents the Studio*. Do NOT copy its layout, flows,
> component structure, or UX ideas into the new surfaces. Reusing
> framework-agnostic libraries is fine; borrowing its design/presentation
> decisions requires **asking the owner first**.

## GraphQL only

The orchestrator's API of record is **GraphQL**. The schema lives at
`core/orchestrator/internal/operations/graph/schema/*.graphql`. The endpoint is
`POST /graphql`, subscriptions arrive on the same path via
`graphql-transport-ws`, and `GET /graphql/sandbox` renders Apollo
Sandbox as live documentation.

- **Never add a new REST endpoint.** Add a GraphQL operation instead.
  Resolvers live in `core/orchestrator/internal/operations/graph/resolver/`.
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
- **Every business event that materially changes state should emit an
  `OutcomeEvent`** via `learning.Publish(...)`. The system uses these
  to evolve its own strategy via the Pattern Miner (Feedback Brain).
  New mutating endpoints/resolvers should add the emission as part
  of the change. Source of truth for the contract:
  [`docs/FEEDBACK_BRAIN.md`](docs/FEEDBACK_BRAIN.md).

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
  (`core/orchestrator`, `core/runtime`).
- `npx tsc --noEmit` MUST pass in `clients/web` and `packages/sdk`.

### Constitutional rule: DESIGN REFERENCE IS LAW

The **design reference** lives in three canonical places:

1. `design-reference/2026-05-25-private-ironflyer/` — the locked local
   folder for the private product-design reference.
2. `clients/web/DESIGN_REFERENCE.md` — the governing no-drift law.
3. `packages/design-tokens/index.ts` and `clients/web/src/theme/index.ts`
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
- **Prompt-first home law:** on `/`, the natural-language project
  composer must remain at the top of the first viewport, immediately
  after the global navigation and before hero sales copy or decorative
  media. Do not move it lower, hide it behind a CTA, or replace it
  with a static product preview. The product promise is that a visitor
  can start building within seconds.
- **2026-05-27 public marketing texture:** public pages inherit the
  near-black cosmic texture, violet orbital glow, compact builder
  previews, and recurring 3D geometric/planetary accents from the
  owner-supplied reference. Inner marketing routes must feel like the
  same world as Home, not flat standalone pages.
- If a new shade is needed, add it to `packages/design-tokens`
  first, then use it. Do not invent inline.
- AI agents, hooks, and humans alike: this rule does not bend. If
  a sub-agent or auto-hook introduces a hardcoded color, revert
  before continuing.

The boundary exception is `clients/web/src/theme/index.ts` itself —
that's where token values get mapped to MUI palette properties, so
hex literals are permitted there only when mapping a token.

**Structure rule (asserted 2026-05-25 by the user):** the design
reference governs *layout and structure*, not only colors. Reference
documents:

- `design-reference/2026-05-25-private-ironflyer/` — canonical local
  reference folder for the private Home and Studio handoff.
- `clients/web/DESIGN_REFERENCE.md` — locked visual identity, density,
  responsiveness contract, screenshot baseline.
- `design-handoff-screenshots/ironflyer-app-2026-05-25/` — the
  paired desktop + mobile screenshot set per route. When a captured
  screenshot exists for a route, the reference wins over the live
  page.
- **`/login` and `/signup` are pinned to the Base44 split-layout
  pattern** rendered by `clients/web/src/components/auth/AuthShell.tsx`:
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

### Constitutional rule: STYLE MAPS THROUGH THE THEME, NEVER INLINE

Asserted 2026-05-28 by the user: "אתה לא יכול לכתוב ולדרוס סטייל בקוד —
הכל צריך להיות מיפוי לפי MUI theme."

Styling is **never** written or overridden ad-hoc inside a component. It is
declared once in the theme and *consumed* by components. This applies to the
new `clients/` surfaces as much as to `clients/web`.

**Hard rules:**

- **MUI surfaces (`clients/studio`, `clients/backoffice`, `clients/marketing`,
  any React+MUI app):** every color, font, radius, spacing, shadow, and
  transition comes from the **MUI theme** in `@ironflyer/ui-web` (the
  `extendTheme` CSS-variables `theme`, derived from
  `@ironflyer/design-tokens/brand`; dark/light via `CssVarsProvider`).
  Brand-specific values (gradients, mono font, motion) are carried on
  `theme.brand.*`. Components reference
  `theme.palette.*`, `theme.typography.*`, `theme.shape.*`, `theme.spacing()`,
  and theme variants. Reusable visual changes go into the theme's
  `components.*` overrides — **never** a one-off `sx`/`style` with a literal
  value (no hardcoded `fontFamily`, hex, px font-size, or color).
- **No raw values in `sx`/`style`.** Spacing uses the theme scale
  (`sx={{ p: 3 }}`), not `px`. Typography uses `variant=`, not inline
  `fontSize`/`fontFamily`. A literal style value in a component is a bug —
  promote it to the theme.
- **Non-MUI surface exception (`clients/mobile`, React Native).** MUI is
  web-only, so native consumes the parallel theme from `@ironflyer/ui-native`
  `makeNativeTheme()` — itself derived from the same `@ironflyer/design-tokens/brand`.
  Same law: no hardcoded colors/fonts in screens; read them from the native
  theme. (Marketing is React+MUI and follows the MUI rule above — it is NOT an
  exception. It is statically generated with `vite-react-ssg`, so the MUI theme
  is rendered to static HTML at build with no client framework cost.)
- **One source of truth.** Whether the engine is MUI or CSS variables, the
  values originate in `@ironflyer/design-tokens`. Changing a brand value is a
  tokens edit, never a per-component edit.

Treat inline-style drift exactly like raw-hex drift: revert immediately and move
the value into the theme/tokens.

### Constitutional rule: VISUALIZATION-FIRST, CODE-FOR-PROS

Ironflyer is a paid AI execution engine — and the AI's technical
state is the product. Every operator-facing surface must mirror that
state as a **quick-readable visual graph** before falling back to
raw text or tables. Code-grade tooling (the Monaco editor, the
cloud IDE, raw GraphQL, ledger CSVs) is the **professional layer**
that lives behind a toggle; it is not the default.

**Hard rules:**

- **The default view of every cockpit, studio, execution, profit,
  and wallet surface is visual.** A workflow DAG, chart, gauge,
  stacked bar, timeline, or status graph must surface what the
  orchestrator is doing right now and what is still open end-to-end
  before the operator scrolls past raw tables.
- **Visualizations are mirrors, not decoration.** Every node, bar,
  or chip must map to a concrete piece of orchestrator state
  (phase, gate verdict, cost line, patch, deploy artifact, gate
  finding). No charts that exist to look impressive without a live
  data binding.
- **The "what is not closed end-to-end" surface is mandatory.**
  Phase nodes, gate chips, and cost panels must expose what is
  blocking the next transition: a pending gate, a missing build
  artifact, an unresolved patch, an unbudgeted cost line. A run
  that says "running" without naming what is open is a regression.
- **Collapsible information graphs.** Information graphs must
  default to a compact, glanceable form and expand on hover, click,
  or toggle. Operators must be able to skim five surfaces in ten
  seconds and dive into one of them in two clicks.
- **Code editor is opt-in, not opt-out.** Monaco / cloud IDE /
  Apollo Sandbox / raw timeline JSON are reachable in one click but
  never the landing pane. They are positioned as the "open the
  hood" path for professionals.
- **Charts honor the locked palette.** Every chart pulls from
  `chartPalette` in `clients/web/src/components/charts/EChart.tsx` and
  `tokens.color.*`. No raw hex; no lime as a primary chart series.
- **Heavy libraries lazy-load.** echarts, @xyflow/react, three.js
  and any future viz lib MUST be imported through `next/dynamic`
  with `ssr: false` so they never land in the cold initial bundle.

If a feature ships a new operator surface without a default visual
that mirrors the underlying technical state, it works against the
product even if the GraphQL plumbing is correct.

### Constitutional rule: PERFORMANCE — LAZY, VIRTUALIZED, STABLE

Asserted 2026-05-29 by the user: every surface must be optimized for
performance, virtualization, and resilience under load. Performance is
not a follow-up — it ships with the feature. These laws are
non-negotiable and apply to every client surface (studio / backoffice /
marketing / web / mobile).

**Hard rules:**

- **Heavy libraries always load behind a lazy boundary.** echarts,
  `@xyflow/react`, three.js, `ag-grid`, the Monaco editor, Sandpack,
  and any comparably heavy lib MUST be reached only through a lazy
  wrapper + inner split (Next: `next/dynamic` `ssr:false`; Vite/SPA:
  `React.lazy` + `Suspense`; the canonical pattern is
  `packages/ui-web/src/fx/*` and `packages/ui-web/src/data-grid/`,
  wrapper re-exports + a lazily-imported `*Inner`). They must NEVER be
  statically imported on a cold path. **Type-only imports are fine**
  (erased at build) — runtime values (components, enums like
  `Position`/`MarkerType`, `Handle`) are not, and pull the lib into
  the bundle. A build that puts one of these libs in the entry chunk
  is a regression; verify with the chunk report.
- **Route + pane code-splitting.** Editor panes, route screens, and
  any tab whose content carries heavy deps load on demand via
  `React.lazy` + `Suspense` with a non-blocking fallback. The app
  shell (nav, chat, the always-visible chrome) is the only thing in
  the initial chunk.
- **Virtualize unbounded lists.** Any list that can grow without a
  hard cap — chat transcripts, logs, ledgers, findings, file trees,
  tables — MUST be virtualized. Use `ag-grid` (via the lazy `DataGrid`)
  for tabular surfaces and `@tanstack/react-virtual` for custom lists.
  Never render thousands of DOM nodes eagerly.
- **Live-polled visualizations must be signature-memoized.** A surface
  that re-fetches on an interval (gates, economics, forecasts) MUST
  derive a content **signature** of exactly what it renders and gate
  the expensive rebuild (React Flow node/edge graphs, chart options) on
  that signature — so identical polls don't rebuild the graph or churn
  the viewport. A polling surface that flickers or re-fits on every
  tick is a regression. Callbacks passed into memoized builders MUST be
  stable (`useCallback`); an unstable handler silently defeats the memo.
- **Every lazy boundary needs a fallback** that holds layout (skeleton
  / sized box / spinner), never a flash of collapsed content.

Treat a perf regression — a heavy lib in the cold bundle, an
un-virtualized unbounded list, a flickering polled graph — the same way
you treat raw-hex drift: fix it before continuing.

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
- **TS**: server components by default in `clients/web`; only mark
  `'use client'` when you reach for state, refs, or events.
- **CSS**: MUI `sx` prop with the `tokens` from
  `packages/design-tokens`. Primary CTAs use the locked
  coral-magenta-violet gradient through the theme. Do not revive the
  old lime-first identity.

## Mobile support — Expo, Android native, iOS native, Flutter

Ironflyer ships native mobile builds as a first-class surface, not a
hybrid PWA wrap. The contract is the same as for web — gates block,
patches review, ProfitGuard meters every minute — only the targets
and the tooling differ.

**Supported `StackDecision.Mobile.Kind` values:**
- `expo` (recommended) — Expo Router + EAS Build. No Mac in our pool
  required for iOS (EAS cloud handles signing).
- `react-native-bare` — Ejected RN. Native folders on disk;
  Android builds in Linux, iOS requires Mac pool.
- `android-native` — Kotlin + Jetpack Compose. Linux sandbox only.
- `ios-native` — Swift + SwiftUI. **Pro tier required** (mac pool;
  Scaleway/MacStadium/AWS mac2.metal hosts).
- `flutter` — Dart + Flutter. Android Linux-only; iOS needs Mac pool.

**Gate:** `domain.GateMobileBuild` runs after Budget, before Deploy.
It validates the manifest (Expo `app.json` / Android `build.gradle` /
iOS `xcodegen.yml` + xcodeproj / Flutter `pubspec.yaml`), checks the
reverse-DNS bundle id against `domain.AppIDPattern`, verifies signing
secrets exist in `Project.Secrets` (never serialised), then — when a
workspace runtime is attached — drives a real `gradlew assembleDebug`
or `xcodebuild build` or `eas build` and confirms the artifact lands
at the expected path. `IRONFLYER_MAC_POOL_ENABLED=1` enables the iOS
native path; absence forces a degraded SeverityInfo "deferred to EAS
cloud" or SeverityWarning when no fallback exists.

**Runtime:** `core/runtime/internal/suppliers/mobile/` owns per-workspace mobile
lifecycle — Metro server start/stop, Android emulator allocation
(KVM passthrough required on the host), iOS xcodebuild dispatch.
Routes live under `/v1/workspaces/{id}/mobile/...` on the runtime
service (same auth + per-user isolation as the existing routes; the
GraphQL-only rule applies to the **orchestrator**, not the runtime).

**Image:** `infra/Dockerfile.mobile-runtime` adds Android SDK 35 +
emulator + Expo/EAS CLIs + optional Flutter (`--build-arg
WITH_FLUTTER=1`). Mac pool provisioning lives in
`infra/Dockerfile.mobile-runtime-mac.md` (markdown, not a Dockerfile
— macOS cannot be containerised).

**Templates:** Real, runnable starters live at
`templates/starters/react-native-expo/`,
`templates/starters/android-kotlin/`,
`templates/starters/ios-swift/`. Each one obeys the same constraints
as the rest of the repo (English UI copy; design-token-derived
colors in user-facing UI; no test files).

**Ledger:** Mobile is metered separately so the cost panel can split
build minutes from emulator minutes from Mac workspace minutes —
see `core/orchestrator/internal/business/ledger/mobile.go` (`EntryMobileBuildMin`,
`EntryEmulatorMin`, `EntryMacWorkspaceMin`, `EntryEASBuildCredit`,
`EntryAppetizeMin`). ProfitGuard reservation lives at
`core/orchestrator/internal/operations/wireup/profitguard_mobile.go`. A
follow-up migration must extend the `ledger_entries.entry_type`
CHECK constraint to allow the new values (currently the in-memory
backend accepts them; Postgres rejects them).

**Agents:** Two new roles in `agents.yaml` — `mobile-coder` and
`mobile-deployer`. The first owns Expo/RN/Kotlin/Swift/Flutter
patches; the second owns `eas.json`, fastlane, and the mobile
release GitHub Actions workflow.

**Pricing tier boundary:** Expo + Android-native are Free tier
eligible. iOS native (any `NeedsMacHost()` path) is Pro tier only —
the cost floor on Apple-licensed hardware is ~$130–500/month per
concurrent workspace and ProfitGuard refuses Mac allocations that
would push the user's wallet negative.

## Pricing — V22 wallet model

Pricing is a prepaid wallet topped up via Stripe Checkout. Every paid
execution holds funds before any expensive call runs (law 1), debits
the ledger as cost materialises, and releases the unused hold on
commit. Platform margin = wallet revenue − provider cost − sandbox
cost; ProfitGuard exists so that margin stays positive in steady state.

## Library choices

Locked, additive when needed. Streaming-first provider implementations
live in `core/orchestrator/internal/ai/providers/`:

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

Object-store: `core/orchestrator/internal/operations/storage/s3client.go`
centralises the S3-compatible client configuration. `S3_BACKEND=aws`
(default) | `r2` (Cloudflare R2, zero egress) | `minio` (self-hosted).

Memory store: `core/orchestrator/internal/ai/memory/` exposes a single
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
