# Studio Design Constitution

This document is binding for `clients/studio`. New Studio UI must follow the
reference direction in `design_refernce/` and must not introduce a competing
visual language.

## References

- Primary home target:
  `design_refernce/ChatGPT Image May 30, 2026, 01_32_18 AM.png`
- Studio shell target:
  `design_refernce/ChatGPT Image May 30, 2026, 01_31_54 AM.png`
- IDE/workbench target:
  `design_refernce/ChatGPT Image May 30, 2026, 01_32_05 AM.png`
- Review-quality target:
  `design_refernce/ChatGPT Image May 30, 2026, 01_48_34 AM.png`
- Preview/live-build target:
  `design_refernce/ChatGPT Image May 30, 2026, 01_57_49 AM.png`
- Source notes:
  `design_refernce/mx.md`

## Design DNA

Ironflyer Studio is premium AI infrastructure: Gemini, OpenAI, Linear, Arc
Browser, and Apple Vision Pro, with soft cyber energy. It must feel calm,
expensive, intelligent, and production-grade.

Avoid hacker, crypto, gaming, Matrix green, heavy terminal, and generic MUI
dashboard aesthetics.

## Hierarchy

The prompt is the product.

Home page hierarchy:

1. Product headline.
2. Active AI system state.
3. Large prompt builder.
4. Primary build CTA.
5. Template and recent-project accelerators.

Nothing should visually compete with the prompt builder.

## Color

Dark foundation:

- Primary background: `#050816`
- Secondary background: `#0A1024`
- Surface: `#101936`
- Surface hover: `#152149`

Neon gradient:

- `#00D4FF -> #6B5CFF -> #FF4FD8`

Accents:

- AI blue: `#00D4FF`
- Intelligence purple: `#8B5CF6`
- Neon pink: `#FF4FD8`
- Success: `#00E6A7`
- Warning: `#FFB84D`
- Danger: `#FF5D73`

Light Studio screens use near-white backgrounds, cool blue-gray borders, dark
navy text, and restrained neon accents. A dark sidebar or dark prompt module is
allowed and encouraged as the visual anchor.

## Typography

- Font: Inter for UI and product text.
- Allowed weights: 500, 600, 700, 800.
- Headlines are bold, tight, and high-contrast.
- Body text is calm blue-gray.
- Mono fonts are only for code/editor contexts or tiny operational labels.
- Letter spacing is `0` for normal text; use tracked uppercase only for short
  section labels.

## Shape

- Compact controls: 10-14px radius.
- Operational panels: 16-24px radius.
- Prompt builder: 24-28px radius.
- Pills: fully rounded.
- Corners are soft and premium, never bubbly.

## Glow

Glow is reserved for:

- The prompt builder.
- Primary CTA buttons.
- Active navigation.
- AI status icons.
- Focus states.

Use cyan rim light plus purple/pink bloom. Do not put the full brand gradient
on large surfaces. Atmosphere may use radial light, but visible decorative
circles, blobs, or excessive neon are forbidden.

## Theme And Tokens Law (single source of truth)

The studio runs its **own** Neon Intelligence theme — NOT the shared
international brand. Everything maps through it:

- Values originate ONLY in `src/theme/tokens.ts`. That file is the sole place a
  raw hex/rgba may live. A literal color/gradient/blur/radius/shadow/font in any
  component `sx`/`style` is a bug — revert it.
- Mode-aware surfaces → `theme.palette.*` (`background.default`,
  `background.paper`, `text.primary/secondary`, `divider`, and the studio
  additions `surfaceRaised`, `surfaceHover`, `borderSubtle`, `cardBg`,
  `cardBorder`).
- Neon marks + effect recipes → `theme.studio.*`
  (`neon`, `gradient`, `effect`, `chart`, `radius`, `motion`), or import
  `studioTokens`/`neon`/`gradient`/`effect` from `../theme` for non-`sx`
  contexts (SVG fills, keyframes).
- Primary CTA → `<Button variant="contained" color="primary">`; the neon
  gradient comes from the theme. Never set `background` on a CTA.
- Dark/light is provided by `StudioThemeProvider` + `useThemeMode()` exported
  from `src/theme` (a CSS-variables theme carrying both schemes; default dark).
  The always-dark prompt builder legally reads `studioTokens.modes.dark.*`.

## Motion And Theme Timing

Required timing:

- Fast interactions: 120-220ms.
- Theme/background swaps: 180-420ms.
- Easing: `cubic-bezier(.22,.61,.36,1)` or the brand motion tokens.

No bounce, elastic, or playful game animation.

## Component Rules

- Use real buttons, inputs, switches, chips, and icon buttons for interaction.
- Icon-only controls need accessible labels and tooltips when meaning is not
  obvious.
- Use the existing data flow and routes before adding new app structure.
- Floating panels should use translucent surfaces, thin borders, blur, and
  soft depth.
- Cards are for repeated items and tool modules. Do not nest cards inside
  cards.
- The sidebar stays dark and visually connected to the Studio references in
  both light and dark timing.
- All ECharts work goes through `src/components/charts`. Components must not
  import `Chart` directly from `@ironflyer/ui-web/fx`.
- All app tables go through `src/components/tables`. Components must not import
  DataGrid/DataTable directly from `@ironflyer/ui-web`.

## Data Visualization Law

Graphs are product controls, not decoration.

- Colors come from `theme.studio.chart` and semantic palette values.
- Tooltips, legends, axes, split lines, empty states, gauges, and donut centers
  are standardized by the shared Studio chart helpers.
- Donut centers must name the operator decision: open, live, users, reviewed,
  grounding, clean, or a similarly concrete state.
- Gauges must use a quiet track plus one semantic neon arc.
- Horizontal bars use rounded bar ends and never invent a new palette.
- Tables use Studio chrome: soft border, mode-aware glass fill, compact rows,
  uppercase headers, and hover state tied to the active neon.

## Landing Lock (Neon Hero — APPROVED 2026-05-30)

The logged-out entry is the Neon Intelligence hero, owner-approved on
2026-05-30 ("מאוד יפה דף הבית שמור אותו"). Pixel target:
`design_refernce/…01_32_18….png`. Implementation: `pages/Landing.tsx` composing
seven self-contained sections under `pages/home/`, shown by `LoginGate` to
logged-out users (its CTAs reveal the sign-in form) and at `/welcome`.
Structure — do not regress:

1. `AmbientBackdrop` — radial blue/violet/pink glows (5–15%) + faint grid.
2. `TopNav` — logomark + wordmark, nav links, theme toggle, Log in, primary
   "Start a project free".
3. `Hero` — AI badge pill, headline with ONLY the final phrase gradient-filled,
   one-line subhead.
4. `PromptComposer` — the always-dark glowing centerpiece (28px radius, violet
   bloom, blur 24): multiline prompt, budget pill, Plan-first switch, "Build it"
   CTA. Nothing competes with it.
5. `TemplateRail` — quick-start chips.
6. `FeatureGrid` — four "doesn't feel like a card" feature cards.
7. `TrustRow` — monochrome wordmarks at 40% → 100% on hover.

This landing is locked. Edits go through the `pages/home/` sections + the studio
theme — never inline literals.

## Public Home Lock

The public `/` route is the neon product-builder hero. It must keep:

- Transparent top nav with `IronFlyer`, active Product pill, real destinations,
  log-in action, dark/light timing, and gradient start CTA.
- Centered AI-powered product-builder badge.
- Hero headline with controlled line breaks:
  `Build, review and ship` / `production apps` /
  `from a single prompt.`
- Wide always-dark glowing prompt builder with budget, plan-first, enhance, and
  build controls.
- Template chips, a single segmented feature band, and trust wordmarks visible
  as the first viewport resolves.
- Ambient neon atmosphere with engineered grid, side energy streaks, and bottom
  horizon depth.

## Authenticated Studio Home Lock

Authenticated product workspaces use the light dashboard reference. They must
keep:

- Dark left rail.
- Welcome headline: "What will we build today?"
- Active AI System row.
- Dark glowing prompt builder.
- Prompt actions under the composer.
- Recent projects panel.
- Template carousel with visual thumbnails.
- Theme timing button that toggles dark/light.

Any redesign must preserve this structure unless the reference files are
explicitly replaced.

`/welcome` is a preview-only legacy landing route and must not replace `/`.

## Drift Prevention

Forbidden in `clients/studio/src` except inside approved wrappers or theme
builders:

- New raw `#hex`, `rgba(...)`, `linear-gradient(...)`, or `radial-gradient(...)`
  values.
- New `theme.brand.*` usage. Existing legacy compatibility should be migrated
  toward `theme.studio.*`.
- `useThemeMode` from `@ironflyer/ui-web`; Studio uses `src/theme`.
- Direct chart/table primitive imports outside `src/components/charts` and
  `src/components/tables`.

## Review Quality Lock

The quality/performance review surface must keep:

- A bright operational canvas with soft blue-gray borders.
- Top project context and a clear review-mode signal.
- A primary AI fix CTA in the blue-purple-pink gradient.
- A production-readiness gauge as the main visual.
- Layer rows for Frontend, Backend, Memory, and Security.
- A scannable issue list with severity, current value, target value, impact,
  and Fix action.
- Right-side AI Analysis and AI Optimization Agent panels.

Tables alone are not enough for review-quality screens. The first read must be
visual, guided, and action-ready.

## Preview Live Build Lock

The preview workspace must keep:

- A light operational IDE shell with clear top project context.
- Left chat rail, central preview canvas, and right live-build status column.
- A large centered preview frame with desktop/mobile toggles.
- Empty preview state with calm AI-network visual and one sample-screen action.
- Right column showing ProfitGuard, active agents, and Definition of Done.
- Gate states that read as real blockers, not decorative status badges.

The preview area is the product proof surface. It must feel quiet, inspectable,
and trustworthy.
