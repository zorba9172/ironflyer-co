# Architecture — Clients (web · mobile · backoffice · marketing)

Status: locked plan, 2026-05-28. Authoritative map for everything under
`clients/`. Supersedes the Next.js `clients/web` app, which is retired.

The goal: four product surfaces that share one design language, one API
client, one set of business logic, and one optimization discipline — so a
solo operator can move fast without rebuilding the same thing four times.

---

## 0. Locked decisions

| Decision | Choice | Why |
| --- | --- | --- |
| Web framework | **React + Vite** (SPA), **no Next.js** | Operator preference; SPA fits an authed cockpit + admin. |
| Marketing framework | **Vite + React + MUI, statically generated with `vite-react-ssg`** | Next is out; user requires MUI on every web surface (constitutional). `vite-react-ssg` renders the MUI theme to static HTML per route at build for SEO, then hydrates. (Astro was the first cut, dropped 2026-05-28 because it cannot consume the MUI theme.) |
| Mobile | **Expo (React Native), New Architecture** | One codebase → iOS + Android; SDK 53 baseline already validated in `templates/`. |
| Monorepo tool | **Nx + pnpm** | Best Expo/RN generators + affected-graph caching. **Bun not used as the RN package manager** — Metro is not reliable on Bun yet. Bun is fine for scripts. |
| Cross-platform UI | **Shared tokens + shared logic; platform-native component libs** (MUI on web, RN lib on native) | Astro + the existing echarts/xyflow/xterm/MUI investment make a single component lib (Tamagui) high-friction. Tokens + data + core capture ~70% of the sharing benefit at far lower risk. |
| Parallel build? | **No.** Sequence the work. | Solo operator; shared foundation must stabilize before surfaces fork off it. |

Tamagui (one component set compiling to web + native) remains the fallback
if literal component sharing becomes a hard requirement — revisit only after
`studio` ships.

---

## 1. App inventory

| Surface | Path | Stack | State |
| --- | --- | --- | --- |
| **Studio** (product cockpit) | `clients/studio` | Vite + React + React Router | NEW |
| **Backoffice** (admin) | `clients/backoffice` | Vite + React + React Router | NEW |
| **Marketing** (תדמית: product + studio pages) | `clients/marketing` | Vite + React + MUI, `vite-react-ssg` (static) | NEW — **one app, route segments, NOT split in two** |
| **Mobile** | `clients/mobile` | Expo + expo-router | NEW |
| VSCode extension | `clients/vscode-extension` | esbuild + TS | KEEP |
| scrcpy bridge | `clients/scrcpy-bridge` | Go (WebRTC) | KEEP (infra, not a frontend) |
| ~~web~~ | `clients/web` | Next.js 15 | **RETIRE** → replaced by `studio` + `marketing` |

> **`clients/web` is legacy and disliked — do not mine it for ideas.** The owner
> explicitly does not like this app, *especially how it presents the Studio*. Do
> NOT copy its layout, flows, component structure, or UX into the new surfaces.
> Reusing framework-agnostic *libraries* (echarts, xyflow, xterm, virtuoso) is
> fine; reusing its *design/presentation* decisions requires **asking the owner
> first**. The new brand + theme are the source of truth instead.

Why not split marketing into two apps: a product page and a studio page are
route segments, not separate deployments. Two apps = two pipelines, two
domains, duplicated header/footer/SEO config, for zero benefit. Keep one app;
split later only if they ever need independent release cadences.

---

## 2. Target layout

```
ironflyer/
  nx.json · pnpm-workspace.yaml · tsconfig.base.json   # NEW root tooling
  clients/
    studio/            # Vite + React + MUI  (authed cockpit)
    backoffice/        # Vite + React + MUI  (admin)
    marketing/         # Vite + React + MUI, vite-react-ssg (static HTML)
    mobile/            # Expo
    vscode-extension/  # (existing)
    scrcpy-bridge/     # (existing, Go)
  packages/
    design-tokens/     # EXISTS — source of truth, framework-agnostic
    sdk/               # EXISTS — @ironflyer/sdk, GraphQL/graphql-ws
    core/              # NEW — pure TS: types, zod, formatters, budget math, i18n
    data/              # NEW — TanStack Query hooks over sdk (web-shared)
    ui-web/            # NEW — React + MUI theme built from tokens
    ui-native/         # NEW — RN components built from tokens
    assets/            # NEW — logos, fonts, icons, lottie
    tsconfig/          # NEW — shared tsconfig presets
    eslint-config/     # NEW — shared lint
```

---

## 3. Shared packages — the contract

Every package is ESM, `"sideEffects": false`, and tree-shakeable. Apps depend
on packages; packages never depend on apps. Dependency direction is strictly
one-way: `tokens → ui-* / core → data → apps`.

| Package | Purpose | Consumed by |
| --- | --- | --- |
| `design-tokens` | Colors, radii, spacing, type scale, i18n language list. **The only place colors live.** | every UI package + app |
| `sdk` | Typed GraphQL client + graphql-ws subscriptions against the orchestrator. | `data`, mobile, vscode-extension |
| `core` | Framework-agnostic logic: domain types, zod schemas, money/decimal math (budget), date/format helpers, he/en strings. No React, no DOM. | every app + package |
| `data` | React hooks: TanStack Query wrappers over `sdk`, query keys, SSE/subscription bindings, auth-token plumbing. | studio, backoffice, marketing islands. Mobile reuses the fetchers + query keys; RN-specific provider wiring lives in `mobile`. |
| `ui-web` | React component library: MUI theme generated from `tokens`, plus app primitives (Button, Card, DataTable, charts wrappers). | studio, backoffice, marketing |
| `ui-native` | React Native components: tokens → RN StyleSheet/Paper theme; same component vocabulary as `ui-web` (Button/Card/List) so screens read the same. | mobile |
| `assets` | Brand logos (SVG), fonts (Inter, Geist Mono), icon set, lottie. Web imports via Vite; native via Metro resolver + `expo-font`. | all apps |
| `tsconfig` / `eslint-config` | One TS + lint baseline; apps extend. | all apps |

`ui-web` and `ui-native` deliberately expose the **same component names and
props** where it's sensible (Button, Card, List, EmptyState). Logic and data
are literally shared; the rendering layer is parallel-but-mirrored, not unified.

---

## 4. Data & state

- **Server state:** TanStack Query in `data`, backed by `sdk`. One pattern
  everywhere; no Apollo (the retired web app used Apollo — not carried over).
- **Realtime:** single graphql-ws connection per app, surfaced as TanStack
  subscription hooks in `data`. SSE token fallback uses `?token=` per the
  existing auth model.
- **Client state:** Zustand, scoped per app. No shared global store across
  surfaces — they are different users (operator vs admin vs visitor).
- **Auth:** JWT in SecureStore (mobile) / cookie (web); token plumbing lives
  in `data` so every surface authenticates identically.

---

## 5. Optimization & virtualization

**Web (studio, backoffice):**
- Route-level code splitting via `React.lazy` + React Router lazy routes.
- Vite `manualChunks` to isolate heavy libs (`echarts`, `@xyflow/react`,
  `@xterm/xterm`) so they load only on the screens that use them.
- **Virtualization:** `react-virtuoso` for long lists/log streams/tables
  (already proven in the old web app).
- TanStack Query cache + dedupe; suspense boundaries per panel.
- Honor `docs/PERF_BUDGETS.md`; CI fails on budget regressions.

**Marketing (Vite + React + MUI, `vite-react-ssg`):**
- Static HTML generated per route at build (SEO), then hydrated.
- MUI theme rendered to static CSS at build; SSR-safe dark/light via
  `CssVarsProvider` + `InitColorSchemeScript` (no theme flash).
- React Router v6 (vite-react-ssg requires it); studio/backoffice match.

**Mobile (Expo):**
- **Virtualization:** `@shopify/flash-list` (not stock FlatList).
- Hermes + New Architecture (already enabled in the starter).
- `react-native-reanimated` for 60fps interactions; `expo-image` for caching;
  `react-native-mmkv` for fast local storage.

**Shared:**
- Nx affected graph + local/remote cache — only changed projects rebuild.
- All packages ESM + `sideEffects:false` for tree-shaking.
- One graphql-ws socket per app, never per-component.

---

## 6. Inherited constraints (non-negotiable)

- **Tokens only.** No hardcoded hex/rgba anywhere in `clients/`. Colors come
  from `design-tokens` (via MUI theme on web, RN theme on native). This is
  constitutional.
- **No lime-first identity.** Primary = violet; CTAs = coral→magenta→purple
  gradient; live/success = mint. Lime banned from CTA/chrome.
- **No tests.** Do not write, run, or scaffold tests for these apps.
- **Prod is APQ-locked.** Any web/mobile client hitting `/graphql` in prod must
  send `extensions.persistedQuery.sha256Hash`. `sdk` owns this.
- **Viz-first.** Every operator surface lands on a visual mirror of AI state
  and names what's unclosed end-to-end; code editor is opt-in.

---

## 7. Build sequence (no parallel work)

1. **Root tooling** — Nx + pnpm workspace, `tsconfig` + `eslint-config`,
   wire existing `design-tokens` + `sdk` into the graph.
2. **`core`** — domain types, zod, budget math, i18n, formatters.
3. **`data`** — TanStack Query layer over `sdk`.
4. **`ui-web`** — MUI theme from tokens + shared primitives.
5. **`studio`** — the product cockpit (the heart). Migrate the good parts of
   `clients/web` here; drop Next-specific code.
6. **`backoffice`** — admin, reusing `ui-web` + `data`.
7. **`marketing`** — React+MUI static site (product + studio pages).
8. **`ui-native` + `mobile`** — Expo app last, once the API surface and the
   component vocabulary are stable.

Retire `clients/web` only after `studio` + `marketing` cover its surface.

---

## 8. Migration from `clients/web`

Carry over (framework-agnostic, reuse as-is): `react-virtuoso`, `echarts`,
`@xyflow/react`, `@xterm/*`, `framer-motion`, `zustand`, `graphql-ws`, the MUI
theme derived from tokens.

Drop: `next`, `@mui/material-nextjs`, `@sentry/nextjs` (swap for
framework-agnostic `@sentry/react`), Apollo Client (replaced by `sdk` + `data`),
`critters`, `next.config.mjs`, the `app/` router tree.
