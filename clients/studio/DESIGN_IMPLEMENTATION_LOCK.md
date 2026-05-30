# Studio Implementation Lock

This is the working checklist for implementing the Studio design constitution.
Review it before changing `clients/studio/src`.

## Required Home Composition

- The first screen must be the usable Studio product surface, not a marketing
  landing page.
- The left rail must remain visible on desktop and dark in both theme modes.
- The main content must expose a light operational mode and a dark cinematic
  mode through the shared theme toggle.
- The central prompt builder is the dominant element and must keep dark glass,
  neon rim lighting, and a gradient CTA.
- Recent projects and templates are secondary accelerators and must not exceed
  the visual weight of the prompt builder.

## Token Discipline

Use existing theme and token sources where possible:

- `@ironflyer/ui-web` for shared lazy primitives only when wrapped by Studio.
- `@ironflyer/design-tokens/brand` for text scale and brand foundations.
- MUI components for accessible controls.
- `src/theme/StudioThemeProvider` and `src/theme/useThemeMode` for Studio
  dark/light timing.
- `src/components/charts` for every ECharts surface.
- `src/components/tables` for every AG Grid or MUI DataGrid surface.

Exact Studio values live ONLY in `src/theme/tokens.ts` (the Aurora system).
Components read them through `theme.palette.*` / `theme.studio.*`; never inline.
The signature is indigo `#6366F1` → violet `#8B5CF6` → pink `#EC4899` on a
light `#F7F8FA` canvas (dark peer `#0A0F1C`). The retired orange and dark-neon
values must not reappear in any component.

## Interaction Rules

- Enter in the prompt submits unless Shift is held.
- Plan-first remains a visible switch.
- Theme timing is toggled by a real button using `useThemeMode().toggle`.
- Template cards call `startFromTemplate`.
- Recent project rows keep their live/open behavior.
- The editor route remains outside `AppShell`.

## Required Review Composition

- The Performance review page must lead with a visual production-readiness
  gauge, not a raw grid.
- The main audit summary must compare Frontend, Backend, Memory, and Security.
- Issue rows must show severity, current metric, target metric, impact, and a
  per-row fix action.
- The right column must explain the largest issue and show the AI optimization
  agent's progress.
- The primary fix CTA must be a gradient action that creates a safe reviewable
  change path, not an opaque destructive action.

## Required Preview Composition

- Preview stays a three-column IDE surface: chat, preview canvas, live-build
  status.
- The preview frame must be large, centered, softly bordered, and never buried
  inside decorative cards.
- Empty preview must show an inspectable AI-network placeholder, "No preview
  yet", explanatory copy, and a sample-screen action.
- The live-build column must show ProfitGuard, agents, and Definition of Done.
- The desktop/mobile selector and refresh action must stay visible above the
  preview frame.

## Required Data Visualization Composition

- Studio pages must import `StudioChart` and option builders from
  `src/components/charts`; direct `Chart` imports are only allowed inside that
  shared wrapper.
- Donuts, gauges, line trends, and horizontal bars must use the shared helpers
  so tooltip, legend, axis, grid, stroke, and neon series behavior stays
  consistent.
- Studio pages must import `StudioDataGrid` / `StudioDataTable` from
  `src/components/tables`; direct `@ironflyer/ui-web/data-grid` or
  `@ironflyer/ui-web/data-table` imports are only allowed inside those
  wrappers.
- A table-heavy screen must still lead with a summary visual or metric rail
  when the reference expects an operator-first read.

## Review Gate

Before merging any Studio visual change:

1. Compare against all locked reference images in `design_refernce/`.
2. Run `pnpm --filter @ironflyer/studio typecheck`.
3. Run `pnpm --filter @ironflyer/studio lint`.
4. Run `pnpm --filter @ironflyer/studio build`.
5. Manually verify dark and light timing from the button.
6. For review pages, compare against
   `design_refernce/ChatGPT Image May 30, 2026, 01_48_34 AM.png`.
7. For preview pages, compare against
   `design_refernce/ChatGPT Image May 30, 2026, 01_57_49 AM.png`.

## Drift Checks

Before merging, search for drift:

- `rg "useThemeMode.*@ironflyer/ui-web|from '@ironflyer/ui-web'.*useThemeMode" clients/studio/src`
- `rg "from '@ironflyer/ui-web/(data-grid|data-table)'|from '@ironflyer/ui-web/fx'.*Chart" clients/studio/src`
- `rg "theme\\.brand|t\\.brand|th\\.brand" clients/studio/src`

New matches must be justified as legacy migration work or moved to the Studio
theme/charts/tables wrappers.

If a change makes the home page look like a generic dashboard, it fails this
gate even if tests pass.
