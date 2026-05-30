# Studio Design Constitution

This document is binding for `clients/studio`. New Studio UI must follow the
reference direction in `design_refernce/` and must not introduce a competing
visual language. **When a reference image exists for a surface, the reference
wins over any prose, any prior build, and any sub-agent suggestion.**

## References (source of truth — in priority order)

The **newest** references define the law. Older renders are kept for structural
guidance only where the newer set is silent.

1. `design_refernce/image.png` — **Home (light)**, the primary target. Left
   rail, friendly greeting ("Good morning, Meir 👋"), white prompt card with an
   indigo **Start building** CTA, Recent Builds gallery, Active Agents row, and
   a right **Live Build** column.
2. `design_refernce/image copy.png` — **Review (light)**. Indigo progress to
   "Deploy", a stat-card rail with sparklines, a multi-color **Provider spend**
   bar chart, and a teal/violet **donut** with a named center.
3. `design_refernce/ChatGPT…01_32_05….png` — workbench / IDE shell.
4. `design_refernce/ChatGPT…01_48_34….png` — Performance Review (readiness
   gauge + layer rows + issue list + AI panels).
5. `design_refernce/ChatGPT…01_57_49….png` — Preview / live-build three-column.

`design_refernce/mx.md` is **superseded** by this document and the tokens. Its
dark-neon-cyber and warm-orange directions are retired drift; do not revive them.

## Design DNA

Ironflyer Studio is a premium AI product-builder for non-engineers. It must feel
**clean, light, calm, expensive, and intelligent** — the love-child of Linear,
Lovable, Figma, and Vercel with a soft indigo→violet aurora signature.

- **Light-first.** The product canvas is light. Dark mode is a first-class peer
  reached by the toggle — not the default.
- **Focus zones may go dark.** The left rail, the workbench/code surface, and
  the dark prompt variant may use the dark canvas inside light mode for focus.
- Avoid: hacker / crypto / gaming, Matrix green, heavy terminal, neon-cyber
  overload, warm-orange identity, and generic MUI dashboard aesthetics.

## Hierarchy

The prompt is the product. Home page hierarchy:

1. Warm greeting + product headline.
2. The large prompt builder (the hero — nothing competes with it).
3. Primary build CTA.
4. Recent builds + template accelerators.
5. The ambient Live Build / system-state column as supporting context.

## Color

The studio runs its **own** theme. Every value originates in `src/theme/tokens.ts`
(the "Aurora" system) — the sole place a raw hex/rgba may live.

Signature (mode-independent):

- Primary indigo: `#6366F1` (CTA, active nav, focus, primary series)
- Violet: `#8B5CF6` · Blue: `#3B82F6` · Pink: `#EC4899` · Cyan: `#22CCEE`
- Aurora gradient: `#4F6BF5 → #7C5CF6 → #D96BD8` (CTAs, final headline phrase,
  active states, focus rims, thin energy edges — **never** large flat surfaces)
- Success `#16B364` · Warning `#F79009` · Danger `#F04438` · Info `#2E90FA`

Light canvas: bg `#F7F8FA`, surfaces `#FFFFFF`, hairlines `#EAECF0`, ink
`#101828 / #475467 / #98A2B3`.

Dark canvas: bg `#0A0F1C`, surfaces `#111827`, hairlines `#283142`, ink
`#F9FAFB / #CBD5E1 / #8B96A8`.

## Typography (editorial two-typeface system — the Base44 differentiator)

Inspired by output.com's refined editorial hierarchy — on WHITE, never beige.
Generic bold-Inter-everywhere is forbidden; that is the look we are leaving.

- **Display → Bricolage Grotesque** (`font.display`) for `h1`–`h3`: characterful,
  editorial, young. Tight negative tracking (`font.tracking.display` −0.022em on
  h1/h2, `tight` −0.014em on h3). Weights 600–700, NOT 800-everywhere.
- **Text → Inter** (`font.family`) for `h4`–`h6`, body, UI. Body at comfortable
  line-height (1.5–1.55). Section headers precise, not heavy.
- **Mono → Geist Mono** only for code/editor or tiny operational labels.
- **Overline/labels:** Inter 600, uppercase, `font.tracking.label` (0.08em), 11px.
- Precise modular scale lives in `studioTheme.ts` typography (h1 48 / h2 36 /
  h3 28 / h4 22 / h5 18 / h6 16 / body1 15 / body2 13 / caption 12 / overline 11).
  Use `variant=` — never an inline `fontSize`/`fontFamily`/`fontWeight`.
- All three faces load once via `@ironflyer/ui-web/fonts.css` (already imported
  in `main.tsx`). Headlines must render in Bricolage, not the system fallback.

## Shape

- Compact controls: 8–10px radius.
- Cards: 12–14px radius.
- Operational panels / prompt builder: 16–20px radius.
- Pills: fully rounded. Corners are soft and premium, never bubbly.

## Glow

Glow is reserved for: the prompt builder, primary CTAs, active navigation, AI
status icons, and focus states. Use a soft indigo/violet bloom. Never put the
full aurora gradient on a large surface. Atmosphere may use radial light
(`effect.ambient`); visible decorative circles or blobs are forbidden.

## Iconography Law (single source of truth)

There is **one** UI glyph system and **one** illustrated-asset registry. Both
live in `src/icons/`. No component imports `react-icons/*` directly anymore.

- **UI glyphs → `import { Icon } from 'src/icons'`** (relative, e.g.
  `../icons`; or the named exports it re-exports). The UI set is **Lucide**
  (`react-icons/lu`); the editor
  surface may use VS Code glyphs (`react-icons/vsc`) **only through the same
  barrel**. Brand/tech marks come from `src/lib/techIcons.tsx` (Simple Icons).
- **Illustrated / 3D / animated assets → `import { asset } from '../icons/assets'`**,
  the typed manifest mapping `public/icons/**` and `ironflyer/assets/market/**`
  (SVG line sets, 3D animated `.mp4/.gif`, illustration packs). Use these for
  feature tiles, empty states, onboarding, and marketing — never as control
  glyphs. Prefer SVG; reach for animated 3D only on hero/empty surfaces.
- A raw `<svg>` path, an inline emoji used as an icon, or a direct
  `react-icons` import in a component is drift — route it through `src/icons/`.

## Data Visualization Law

Graphs are product controls, not decoration. **Use the full range** — pie,
donut, vertical & horizontal bar, grouped/stacked bar, line/area, gauge, and
radar. Do not lock onto one chart type, and **not everything is 3D** — reserve
3D for at most one signature moment per surface.

- All ECharts work goes through `src/components/charts`. Never import `Chart`
  from `@ironflyer/ui-web/fx` outside that wrapper.
- Colors come from `theme.studio.chart` + semantic palette. Never an inline hex,
  never lime as a primary series.
- Tooltips, legends, axes, split lines, empty states, gauges, and donut centers
  are standardized by the shared Studio chart helpers.
- Donut centers must name the operator decision (open, live, users, reviewed,
  clean, …). Gauges use a quiet track + one semantic aurora arc. Bars use
  rounded ends.

## Component Law

- **Uniform card.** Repeated items and tool modules use the shared Studio card
  chrome (soft border, mode-aware fill, 12–14px radius, hover lift `-2px`).
  Never nest a card inside a card.
- **Uniform box/panel.** Operational panels use the shared GlassPanel /
  SectionHeader chrome — one panel language across every surface.
- **Uniform table.** Every table goes through `src/components/tables`
  (`StudioDataGrid` / `StudioDataTable` / `StudioTableShell`): soft border,
  mode-aware glass fill, compact rows, uppercase headers, hover tied to the
  active indigo. Never import DataGrid/DataTable directly.
- Use real buttons, inputs, switches, chips, and icon buttons. Icon-only
  controls need accessible labels + tooltips.
- The sidebar stays visually anchored across both modes.

## Theme And Tokens Law

- Values originate ONLY in `src/theme/tokens.ts`. A literal
  color/gradient/blur/radius/shadow/font in any component `sx`/`style` is a bug
  — revert it.
- Mode-aware surfaces → `theme.palette.*` (`background.default/paper`,
  `text.*`, `divider`, and the studio additions `surfaceRaised`,
  `surfaceHover`, `borderSubtle`, `cardBg`, `cardBorder`).
- Brand marks + recipes → `theme.studio.*` (`neon`, `gradient`, `effect`,
  `chart`, `radius`, `motion`), or import `studioTokens`/`neon`/`gradient`/
  `effect` from `../theme` for non-`sx` contexts (SVG fills, keyframes).
- Primary CTA → `<Button variant="contained" color="primary">`; the aurora
  gradient comes from the theme. Never set `background` on a CTA.
- Dark/light comes from `StudioThemeProvider` + `useThemeMode()` exported from
  `src/theme` (CSS-variables theme carrying both schemes; default **light**).

## Motion And Theme Timing (synchronized with type + color in one source)

Motion is a designed layer of the system, not an afterthought — and it lives in
the SAME single source of truth as color and typography (`src/theme/tokens.ts`
› `motion`). All three are read through the theme; never hand-roll a duration or
easing in a component.

- **Four intents, one ramp.** Easings: `standard` (symmetric), `decelerate`
  (enter), `accelerate` (exit), `emphasized` (hero/CTA, theme swap). Duration
  ramp: instant 80 · fast 180 · base 280 · slow 460 · hero 620 ms.
- **Use named transitions** from `theme.studio.motion`: `hover` (micro/lift/focus
  rim), `enter` (mount/reveal), `exit` (dismiss), `emphasis`/`theme` (canvas
  swap), `stagger` (56ms between list/grid items). Back-compat `fast/base/slow`
  strings remain.
- Fast interactions 120–220ms; theme/background swaps 180–460ms.
- No bounce, elastic, or game animation. Honor `prefers-reduced-motion`.
- **The IDE / workbench consumes this same system** — its chrome transitions,
  panel reveals, and gate-state changes use `theme.studio.motion`, so the editor
  feels continuous with the rest of the studio, not a foreign surface.

## Surface Locks

Structure to preserve unless the reference files are explicitly replaced.

**Home (`image.png`).** Left rail (collapsible, anchored). Warm greeting
headline. Dominant prompt builder (white card in light, dark glass in dark,
violet focus glow, gradient CTA). Recent Builds gallery + Active Agents row as
secondary accelerators. Right Live Build column showing real gate/agent state.

**Review (`image copy.png` + `…01_48_34…`).** Bright operational canvas. Lead
with a **visual** read, not a raw grid: a production-readiness **gauge**, a
stat-card rail with sparklines, the multi-color provider-spend **bar**, and a
named-center **donut**. Layer rows (Frontend, Backend, Memory, Security). A
scannable issue list with severity / current / target / impact / Fix. Right-side
AI Analysis + AI Optimization Agent panels. A primary AI-fix CTA in the aurora
gradient that opens a safe reviewable change path — never an opaque destructive
action.

**Preview (`…01_57_49…`).** Three-column IDE surface: chat rail, central
preview canvas (large, centered, softly bordered, desktop/mobile toggles), right
live-build column (ProfitGuard, active agents, Definition of Done). Empty state:
calm AI-network visual + "No preview yet" + one sample-screen action. Gate
states read as real blockers, not decorative badges.

**Workbench (`…01_32_05…`).** Chat rail + file tree + code center + right status,
dark functional canvas permitted inside light mode.

## Drift Prevention

Forbidden in `clients/studio/src` except inside the theme/chart/table/icon
wrappers:

- New raw `#hex`, `rgba(...)`, `linear-gradient(...)`, `radial-gradient(...)`.
- Direct `react-icons/*` imports in components (route through `src/icons/`).
- Direct chart/table primitive imports outside `src/components/{charts,tables}`.
- `useThemeMode` from `@ironflyer/ui-web` (Studio uses `src/theme`).
- New `theme.brand.*` usage (migrate toward `theme.studio.*`).

If a change makes the home page look like a generic dashboard, or revives the
orange / dark-neon identities, it fails this gate even if it typechecks.

## Review Gate (before merging a Studio visual change)

1. Compare against the locked references in `design_refernce/` (newest first).
2. `pnpm --filter @ironflyer/studio typecheck`
3. `pnpm --filter @ironflyer/studio lint`
4. `pnpm --filter @ironflyer/studio build`
5. Manually verify light **and** dark timing from the toggle.
6. Grep for drift: raw hex/rgba, direct `react-icons`/chart/table imports,
   `theme.brand.*`.
