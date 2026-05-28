# Clients Layer — Status (2026-05-28)

End-to-end summary of the rebuilt `clients/` layer. Companion to the locked
plan in [`docs/ARCHITECTURE_CLIENTS.md`](ARCHITECTURE_CLIENTS.md) and the brand
in [`docs/BRAND_SYSTEM_2026-05-28.md`](BRAND_SYSTEM_2026-05-28.md). This is a
status snapshot of what exists in the tree, not a plan.

---

## 1. Overview

The client surfaces were rebuilt as a pnpm + Nx monorepo (root
`@ironflyer/clients`, `pnpm@10.33.2`, Node ≥20, React 19 pinned via overrides).
Four product surfaces — **studio**, **backoffice**, **marketing**, **mobile** —
sit on shared packages (`design-tokens`, `ui-web`, `ui-native`, `data`, `core`,
`assets`, `sdk`). They share one design language (cobalt → cyan brand), one
GraphQL client, and one set of business logic.

This replaces the legacy Next.js `clients/web` app (explicitly disliked, being
retired). The new MUI surfaces use **Vite + React + React Router 6**, not
Next.js; marketing is statically generated with `vite-react-ssg`. `clients/web`,
`clients/vscode-extension`, and the Go `clients/scrcpy-bridge` are intentionally
**not** members of this workspace (`pnpm-workspace.yaml`).

Workspace members: `clients/studio`, `clients/backoffice`, `clients/marketing`,
`clients/mobile`, and `packages/{design-tokens,core,data,ui-web,ui-native,assets,sdk}`.

---

## 2. Apps

| App | Path | Stack | Role |
| --- | --- | --- | --- |
| Studio | `clients/studio` | Vite + React + MUI + Zustand + React Router | Authed product cockpit / finisher |
| Backoffice | `clients/backoffice` | Vite + React + MUI + React Router | Internal operator admin |
| Marketing | `clients/marketing` | Vite + React + MUI, `vite-react-ssg` (static SSG) | Public site |
| Mobile | `clients/mobile` | Expo + expo-router + `@ironflyer/ui-native` | Native iOS/Android |

### Studio — the cockpit

Routes (`src/App.tsx`): the `AppShell` (persistent sidebar) wraps `/` (StudioHome
prompt composer), `/projects`, `/templates`, `/integrations`, `/agents`, `/plans`;
`/build` renders the full-screen `Editor`.

`StudioHome` is prompt-first: a composer ("What are we finishing today?"), a
"Plan first" toggle, category chips (Import a build, Finish auth, Wire payments,
Harden security, Ship to prod), recent projects, and a template carousel.
Submitting `startFromPrompt` (Zustand `store.ts`) seeds the project + constitution
and navigates to `/build`.

The `Editor` is a three-region layout: `ChatPanel` (left) + a tabbed work pane +
`GateInspector` drawer, driven by `EditorTopBar`. The pane tabs:

- **Top toggle tabs:** `Preview`, `Map` (GateMap), `Security`, `Code`.
- **Dashboard dropdown** (the "Finisher" group): `Dashboard`, `Intelligence`,
  `Code quality`, `Performance`, `Usage`, `Logs`, `Goals`.

Deploy enforces hard economic law 1 in the UI: if wallet remaining ≤ 0 or
ProfitGuard verdict is `block`, it shows a "Top up required (402)" dialog;
open gates trigger a "ship anyway?" confirm.

### Backoffice — admin

Routes (`src/App.tsx`) inside `AppShell`: `/` **Overview** (revenue), `/projects`
**Projects**, `/wallet` **Wallet** (spend), `/audit` **Audit**.

### Marketing — public site

Vite + React + MUI rendered to static HTML per route via `vite-react-ssg`
(was Astro, dropped 2026-05-28 because it cannot consume the MUI theme). Routes
(`src/routes.tsx`) under a `RootLayout` (Nav + Footer + ThemeToggle): `/` **Home**,
`/product` **Product**, `/studio` **Studio**, `/pricing` **Pricing**,
`/manifesto` **Manifesto**.

### Mobile — Expo

`expo-router` Stack in `app/_layout.tsx`, themed via `makeNativeTheme()` following
the OS color scheme. Screens: `(tabs)/index` (Home), `(tabs)/projects` (Projects
list), and the pushed `project/[id]` detail.

---

## 3. Shared packages

| Package | Provides |
| --- | --- |
| `design-tokens` | Brand source of truth — `./brand` export (`brand`, `modes`), `languages`. The only place colors/gradients/type live. Cobalt `#2F6BFF` primary, cyan `#18C8E6` secondary, amber signal, emerald success, rose danger; signature `linear-gradient(100deg, #2F6BFF, #18C8E6)`. |
| `ui-web` | MUI CSS-variables theme (`extendTheme`, `cssVarPrefix: 'if'`, dark+light schemes) built from tokens, with brand extras on `theme.brand.*` (gradient, accent, font, motion, shadow). `ThemeModeProvider`, `InitColorSchemeScript`, `DataGrid` (AG Grid), `fonts.css`. |
| `ui-web/fx` | Heavy/interactive layer, all lazy + theme-mapped: `Chart` (echarts), `DataGrid` (AG Grid via `ag-grid-react`/`ag-grid-community`), `CodeEditor` (CodeMirror via `@uiw/react-codemirror` + `@codemirror/lang-*`), `FlowCanvas` (React Flow `@xyflow/react`), `Scene3D` (three.js), `motion`/`Reveal`/`presets` (framer-motion), `confirmAction`/`toast` (sweetalert2), `Carousel` (swiper), `Lightbox` (fancybox). |
| `data` | React data layer over the orchestrator: `createGraphQLClient` (APQ-ready), `IronflyerDataProvider`/`QueryProvider` (TanStack Query), `useGraphQLQuery`, `AuthProvider`/`useAuth` (SignIn/SignUp/Me/SignOut), `useChatStream` (SSE chat), `useRunProjectFeed` (graphql-ws), `useEventStream` (SSE), `operations` (GraphQL strings), `queryKeys`. |
| `core` | Framework-agnostic logic: `types`, `format` (`formatUSD`, …), `copy` (`bannedPhrases` + `lintCopy`). No React/DOM. |
| `ui-native` | RN theme (`makeNativeTheme(mode)`) from the same `design-tokens/brand`. |
| `assets` | Brand `logo.svg` / `mark.svg` and index. |
| `sdk` | Existing typed GraphQL/graphql-ws client; `data` mirrors the relevant operations. |

---

## 4. Live orchestrator wiring (studio)

Studio is fully usable **offline** (sample data, no login). It goes live when
`VITE_GRAPHQL_ENDPOINT` is set; `main.tsx` passes it to `IronflyerDataProvider`
with `getToken` reading `localStorage['if-token']`.

- **Auth (`data/auth.tsx`):** JWT in `localStorage` (`if-token`). On boot, `Me`
  restores the session; `SignIn`/`SignUp` store the token; `LoginGate` shows the
  login screen when online and unauthenticated. Bearer token attaches to every
  GraphQL request and SSE/ws connection.
- **Chat (`data/chat.ts` → `ChatPanel`):** `createPaidExecution(projectID,
  budgetUSD: 50)` (reused per project) → `POST {base}/executions/{id}/chat/stream`
  SSE, parsing `event: delta|thinking|tool_call|finish|error`. On a budget /
  ProfitGuard pause before any text, it transparently starts a fresh execution
  and **retries once**, then surfaces a "top up" message. The first turn is
  grounded with the project constitution + uploaded research (`buildFocusContext`).
- **Live gates:** `DashboardPane` resolves the first real project via
  `useLiveProjectId` (`Projects` query) and runs the `Gates` query through
  `useGraphQLQuery`, mapping verdicts to the studio gate shape; shows a
  `live` / `sample data` chip and falls back to sample when offline/empty.
- **Live logs:** `useRunProjectFeed` opens one `graphql-ws` socket on the
  `/graphql` path (http→ws) and subscribes to `runProject` (gate/run/done/error
  events). `useEventStream` covers other SSE feeds via the `?token=` fallback.

> The orchestrator's in-memory DB driver resets on every restart — sign-ups,
> projects, and executions do not persist across an `IRONFLYER_DB_DRIVER=memory`
> restart.

---

## 5. How to run locally

1. **Orchestrator** (Go) from `core/orchestrator`:
   ```sh
   cd core/orchestrator
   IRONFLYER_DB_DRIVER=memory IRONFLYER_ADDR=:8080 ANTHROPIC_API_KEY=sk-... \
     go run ./cmd/...   # build + run the orchestrator on :8080
   ```
2. **Studio env:** copy `clients/studio/.env.example` → `clients/studio/.env` and
   set `VITE_GRAPHQL_ENDPOINT=http://localhost:8080/graphql`. (SSE/chat/ws URLs
   are derived by stripping the `/graphql` suffix.)
3. **Run studio:** `pnpm --filter @ironflyer/studio dev` (or `pnpm dev:studio`).
   The sidebar shows "connected"; the login screen appears — sign in and
   chat / dashboard / security / logs go live.

Root scripts (`package.json`): `dev:studio`, `dev:backoffice`, `dev:marketing`,
`dev:mobile`, `build:packages`, `build:web`, `typecheck`.

---

## 6. Constitutional rules honored

- **Style maps through the theme.** Every color/font/radius/spacing/transition
  comes from the MUI theme (`@ironflyer/ui-web`) or `theme.brand.*`; native reads
  `makeNativeTheme()`. No raw hex / inline literals in components; values
  originate in `design-tokens`.
- **English only** across UI copy; `core` ships a `bannedPhrases` / `lintCopy`
  guard against AI-tells.
- **No tests** — none written, run, or scaffolded.
- **Heavy libraries lazy.** echarts, AG Grid, CodeMirror, React Flow, three.js
  load via `React.lazy` + `Suspense` (`ui-web/fx`), kept off the cold bundle.
- **AppSec emphasized.** Studio carries a dedicated `Security` tab / `SecurityPane`
  and surfaces gate findings; deploy honors budget (402) + ProfitGuard verdicts.
- **Viz-first.** Default panes are visual (GateMap, gate cards, charts, meters)
  and name what is unclosed end-to-end; the Code editor is opt-in.

---

## 7. Remaining / next

- **Postgres persistence** — replace in-memory reset-on-restart with durable
  storage so sign-ups / projects / executions survive.
- **Patch diff + approve** — render proposed patches and wire approve/apply
  (`applyPatch`) through the gate lifecycle.
- **Persist Code & Goals to core** — `CodePane` is read-only ("save lands with
  the runtime File API"); Goals editing is local-only.
- **Wire Fix-all buttons** to `runFinisher` so gate-fix actions trigger real runs.
- **Real Performance / Code-quality metrics** — replace sample Lighthouse / load
  numbers with live measurements.
- **Meters / activity** still sample in `DashboardPane` pending a project-snapshot
  mapping.
