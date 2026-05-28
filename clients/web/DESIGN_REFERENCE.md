# Ironflyer Web Design Reference

Last locked: 2026-05-25

Prompt-first amendment locked: 2026-05-27

Restored Home rebaseline locked: 2026-05-27

Hero timing rebaseline locked: 2026-05-27

Clean hero and timing enforcement locked: 2026-05-27

This document codifies the private dark IronFlyer screenshots supplied on 2026-05-25 as the mandatory visual reference for `clients/web`. Use it as the source of truth before changing web UI, CSS, theme tokens, layout primitives, or component density.

## Source Of Truth

- Canonical local reference folder: `design-reference/2026-05-25-private-ironflyer/`
- Restored Home reference folder: `design-reference/2026-05-27-restored-prompt-home/`
- Home Hero timing reference folder: `design-reference/2026-05-27-hero-timing-reference/`
- Studio target spec: `design-reference/2026-05-25-private-ironflyer/references/STUDIO_VSCODE_CLOUD_TARGET.md`
- Studio target board: `design-reference/2026-05-25-private-ironflyer/references/studio-vscode-cloud-target.html`
- Primary reference: the private dark Home and Studio screenshots supplied in the product-design handoff conversation on 2026-05-25.
- Local handoff bundle: `design-handoff-screenshots/ironflyer-app-2026-05-25/`
- Contact sheet: `design-handoff-screenshots/ironflyer-app-2026-05-25/index.html`
- Route mapping: `design-handoff-screenshots/ironflyer-app-2026-05-25/manifest.json`
- Archived handoff: `design-handoff-screenshots/ironflyer-app-2026-05-25.zip`

The canonical folder is the stable pointer all docs and implementation work must follow. The local handoff bundle contains 17 route captures in paired viewports, but it is secondary to the private dark screenshots when they disagree:

- Desktop: `desktop-1440/*.png`, captured at `1440x1100`, full page
- Mobile: `mobile-390/*.png`, captured at `390x844`, full page

## 2026-05-27 Restored Home Rebaseline

The Home route `/` now follows the restored prompt-first reference requested by the product owner after rejecting the orbital redesign:

- Dark primary: `design-reference/2026-05-27-restored-prompt-home/references/home-dark-restored-reference.png`
- Light timing: same restored structure with light palette treatment
- Hero light timing: `design-reference/2026-05-27-hero-timing-reference/references/home-hero-light-reference.png`
- Hero dark timing: `design-reference/2026-05-27-hero-timing-reference/references/home-hero-dark-reference.png`

This reference is binding for the Home route. It supersedes the rejected orbital Home redesign when they disagree on structure, density, hero composition, textures, builder panel, pricing/FAQ arrangement, CTA treatment, and footer behavior.

Dark and light timings must share the same structure and interaction model. Dark is the primary pixel baseline; light inherits its layout and converts palette, surfaces and shadow treatment without inventing a new page.

The Home Hero first viewport is now locked to the centered prompt-builder composition: nav, centered eyebrow, three-line headline, centered prompt composer, template chips, four-value capability rail, and trusted company logos. The light and dark timing toggle must preserve this exact hierarchy.

The clean Hero amendment is binding: no descriptive paragraph is allowed between the headline and prompt composer, and no decorative ellipse, ring, blob, planet, egg-shaped glow, or abstract panel may sit behind the headline text. The headline must remain crisp on the page background; atmosphere may live in the surrounding section or composer glow only when it does not compete with the typography.

Light and dark timing are product states, not alternate designs. `/`, public inner pages, `/login`, `/signup`, and `/login/reset` must respect `?theme=light|dark` with matching structure, spacing, copy density, CTA treatment, and footer behavior. A component that renders only in dark mode while the route is in light timing is a design drift bug.

During active design review, route guards must not block visual inspection of Studio, wallet, deploy, execution, settings, and project routes. Auth redirects may be re-enabled only as an explicit product/security step after visual review is complete.

The required route set is: `/`, `/product`, `/solutions`, `/resources`, `/enterprise`, `/login`, `/signup`, `/dashboard`, `/projects`, `/templates`, `/pricing`, `/studio`, `/studio/demo`, `/p/demo`, `/executions`, `/execution/demo`, `/execution/demo/security`, `/deploy`, `/deploy/demo`, `/wallet`, and `/wallet/topup`.

If any local screenshot bundle, implementation detail, theme comment, token name, generated asset, or existing page disagrees with the private screenshots, the private screenshots win. Future dated screenshot bundles do not supersede this baseline unless this document records product-owner approval and a new rebaseline.

## 2026-05-27 Marketing Texture Amendment

The product owner supplied a new dark marketing reference on 2026-05-27 for the public website system. It preserves the locked palette and density, but clarifies the marketing texture: near-black base, purple cosmic field, subtle star/noise texture, orbital/planetary accents, small 3D geometric elements, compact builder-preview panels, trusted-logo strip, flow panel, feature grid, template rail, testimonial band, pricing cards, FAQ/code panel, and final CTA/footer.

This amendment applies to `/` and to public inner marketing routes (`/product`, `/solutions`, `/mobile`, `/security`, `/developers`, `/enterprise`, `/pricing`, `/templates`, `/resources`, `/showcase`, `/blog`, `/changelog`) as a shared visual language. Inner pages must continue the same texture and 3D motif instead of reverting to flat dark cards.

## Prompt-First Home Law

The Home route is a product entry surface, not a passive landing page. The natural-language prompt composer is locked to the top of the first viewport, immediately after the global navigation and before any hero sales copy or decorative media.

Do not move the Home prompt below the headline, below a product preview, into a modal, into a later section, or behind a CTA. A first-time visitor must be able to start a project within seconds without scrolling. The composer may be visually integrated with the 2026-05-27 texture, but the interaction model and HeroPromptInput affordances from the last committed prompt design are the baseline.

## Locked Reference Contract

The private reference is locked. Do not change the visual direction, palette, layout structure, density, radius system, shadows, imagery style, product-panel composition, navigation hierarchy, or CTA treatment unless the product owner explicitly approves a new reference.

Do not add decorative silhouettes, random shadows, new accent colors, new page structures, stock-like imagery, softened card systems, enlarged marketing copy, or unrelated visual ideas just to "improve" the page. Improvement means making the implementation closer to the private reference, more responsive, more interactive, and more connected to the product flow.

Every new page must inherit the same reference system: dark space base, violet product glow, orange-magenta-violet primary CTA, compact product surfaces, 8px cards, real UI previews, and no page-level horizontal scroll.

Any intentional deviation must be recorded here with: date, route, reason, approval source, and new screenshot baseline. Without that record, drift is a bug.

## Visual Identity

The locked direction is premium dark-space SaaS: severe, engineered, dense, glossy enough to feel valuable, but still legible and product-led. Preserve the high-contrast near-black/violet system with orange-to-magenta primary CTAs and violet product glow.

Avoid visual drift toward light alabaster marketing pages, beige palettes, lime-first identity, pastel tints, generic stock visuals, oversized soft cards, or explanatory in-app copy. Gradients are allowed only when they match the private reference: CTA fills, violet edge glows, cosmic imagery, and focused surface highlights.

## Palette

Use the existing design tokens as the named palette. Do not introduce nearby substitute colors without updating this reference and the screenshot baseline.

- Base dark: `#050612`
- Dark surface: `#0c0d20`
- Raised dark surface: `#11132a`
- Dark hover surface: `#191538`
- Dark inset: `#080918`
- Alabaster: `#f4f0e8`
- Deep alabaster: `#e7dfd2`
- Primary text on dark: `#f7f4ff`
- Secondary text on dark: `#b9b2d3`
- Muted text: `#777096`
- Inverse text: `#090816`
- Primary violet: `#8f4dff`
- Violet highlight: `#b56cff`
- Warm CTA orange: `#ff7848`
- CTA gradient: `#ff7848 -> #e149c9 -> #8f4dff`
- Supporting accents: `#ffb457`, `#7eb7ff`, `#7fe28a`, `#ff4f6d`
- Borders on dark: `rgba(178, 133, 255, 0.16)` and `rgba(187, 147, 255, 0.34)`
- Light dividers: `rgba(7,16,17,0.08)`

## Typography

- Body/display family: `Inter` through `var(--font-body)`, with system sans fallbacks.
- Mono family: `Geist Mono`, `JetBrains Mono`, then system monospace fallbacks.
- Body copy stays compact: roughly `14.5px` for primary body and `13.5px` for secondary body, with `1.5` line height.
- Captions use `12px`; overlines use mono uppercase around `11px`, `600`, with deliberate tracking.
- Headings are heavy and tight. Preserve the locked scale and visual weight from screenshots; do not inflate dashboard or tool headings into hero type.
- Buttons use sentence case, `700` weight, and no letter spacing.

## Layout Density

- Default product screens are dense, scan-friendly, and grid-led.
- Public Home must follow the private dark landing reference: top nav, hero copy on the left, product-builder preview on the right, trusted logos, flow panel, feature grid, templates, testimonial, pricing, FAQ, and bottom CTA.
- Studio must follow the private full-viewport builder reference: left rail, top action bar, status strip, prompt/code/preview workbench, and mobile collapse with no horizontal scroll.
- Keep app chrome, toolbars, tabs, filters, stat blocks, and repeated cards compact enough to match the screenshots.
- Prefer direct work surfaces over explanatory panels.
- Do not nest cards inside cards. Page sections should be bands or unframed constrained layouts; cards are for repeated items, modals, and genuinely framed tools.
- Preserve stable dimensions for boards, tiles, toolbar controls, counters, and repeated cards so hover and loading states do not shift layout.

## Absolute Responsiveness Contract

No route may create page-level horizontal overflow at any supported viewport. The invariant is: `document.documentElement.scrollWidth <= document.documentElement.clientWidth + 1`.

All app shells, marketing sections, grids, carousels, resizable panes, media blocks, tables, code panels, and decorative assets must use bounded layout: `min-width: 0`, `max-width: 100%`, responsive grid tracks, clipped internal overflow where needed, and transforms that cannot expand the page canvas.

`/` is a full-bleed public route and must not rely on shell padding hacks or `100vw` negative-margin tricks. `CockpitFrame` owns the full-width canvas for the Home and Studio workspace routes.

`overflow-x: clip` on `html`, `body`, or shells is a final safety guard after the actual overflowing component has been fixed. It must never be used as permission to ship broken responsive layout.

## Interactive Product Visuals

Product UI shown in the Home hero, Studio workbench, templates, pricing, dashboards, and deep product pages must be built from real interactive frontend components, not shipped as one flattened product screenshot.

Static or generated images may be used for atmosphere, background texture, decorative cosmic elements, thumbnails, or non-product artwork. Any product surface that implies controls, tabs, panels, cards, code, previews, carousels, resizing, hover, drag, selection, or live state must be implemented as layered UI components connected to the frontend flow.

Gallery and carousel experiences use component-level interaction, preferably Swiper where appropriate, with touch, drag, keyboard, and bounded internal overflow. A carousel must never create page-level `overflow-x`.

## Studio Flow Contract

The Studio is not a static mock. A user who starts from `/studio`, enters a natural-language app idea, and clicks Generate must receive a created project and a live execution, then land on `/p/{projectID}` with an explicit `executionID` in the URL.

The composer must call the backend idea bootstrap flow with `startImmediately: true`. Creating a project without starting or resolving an execution is a product bug because it strands the user outside the builder loop.

The project workspace must load the execution directly from `executionID` when present, without relying on a broad recents/executions list. Preview, dashboard, code, patches, wallet context, and support-bundle surfaces must remain connected to the same execution.

Every Studio change must pass an automated user-journey check that covers prompt-to-project creation, live workspace load, refinement through the chat composer, responsive no-overflow behavior, and the interactive Studio panes.

The core post-build automation must also walk the operational surfaces that prove the product loop is connected: execution detail, cost, ledger, ProfitGuard, support bundle, wallet, deploy list, and deploy detail. A mock that returns an empty success path is not enough; it must include realistic projects, executions, support bundles, patches, deploys, wallet top-ups, ledger entries, and ProfitGuard decisions.

## Visualization-First Contract

Operator-facing surfaces (studio workbench, execution detail, profit
dashboard, wallet, deploy detail, security review) must lead with a
visual that mirrors the AI's current technical state — workflow DAG,
spend bars, cost-breakdown bar, gate verdict graph, revenue/cost area —
before exposing raw tables, JSON payloads, or text-only status. The
visualization is the first thing the operator sees per route and is
the answer to "what is happening right now".

Every visual must bind to live orchestrator state. Nodes, bars, chips,
gauges, and edges map one-to-one to a phase, gate, patch, cost line,
ledger entry, deploy artifact, or finding. Decorative charts with no
backing series are a bug, not chartjunk.

The "what is not closed end-to-end" surface is mandatory: phase nodes,
gate chips, and breakdown legends must explicitly name a blocking item
(pending gate, missing build artifact, unresolved patch, headroom or
overage on budget) so an operator can read open work without expanding
the timeline.

Information graphs default to a compact glanceable form and expand on
hover, click, or toggle. The studio Workbench primary pane defaults to
`preview` — never `code` — so the first paint reflects the build, not
the source.

Heavy visualization libraries (echarts, @xyflow/react, future
three.js views) load through `next/dynamic({ ssr: false })`. Cold
routes that do not render a chart must not pay the chunk cost.

Charts pull every color from `chartPalette` and `tokens.color.*`
(see `clients/web/src/components/charts/EChart.tsx`). No raw hex, no
lime-first chrome.

## Code-Editor-For-Pros Contract

VS Code, the cloud IDE iframe, raw event payload viewers, GraphQL
Sandbox, ledger CSV export, and any future code-grade pane are the
professional layer of Ironflyer — reachable in one click from every
relevant surface, but never the default landing experience.

These panes must read as power tools rather than as primary product:
mono fonts allowed, terse labels allowed, density tighter than the
visual layer, and they must announce themselves as the lifted hood
(`Open code`, `Open in IDE`, `Open ledger`, `Open sandbox`). Replacing
a visualization with a code editor as the default for any route is a
regression against the Visualization-First Contract above.

## VS Code Cloud Contract

Studio is a VS Code cloud-style product builder. The visual target is `design-reference/2026-05-25-private-ironflyer/references/STUDIO_VSCODE_CLOUD_TARGET.md` and the private Studio screenshot it records: global nav, left rail, breadcrumb/action bar, mode/status row, prompt panel, file tree, code editor, preview, status cards, assistant strip, and bounded internal scrolling.

The cloud IDE layer is based on the repo's open-source code-server path unless a documented rebaseline chooses a different project. See `docs/ARCHITECTURE_CLOUD_IDE.md`.

Do not ship a light IDE skin, lime-first legacy theme, centered marketing composer, or flattened editor screenshot as the Studio. The Studio may use mock/demo data for guests, but it must feel and behave like the real IDE shell and must not block viewing when the user is not authenticated.

## Radius And Shadows

- Default radius is `8px`; this applies to cards, papers, buttons, icon buttons, inputs, and tooltips.
- Larger radii are exceptions only when already visible in the locked screenshots.
- Pills are reserved for chips/badges only.
- Shadows are restrained but present on premium panels. Prefer violet-tinted borders, dark raised surfaces, and focused glow only around hero/workbench/CTA elements.

## Mobile Rules

- The `390x844` screenshots are mandatory references, not secondary checks.
- Every visual or layout change must be checked at minimum on `390x844`, `430x932`, `768x1024`, and `1440x1100`.
- Mobile should preserve the same product density while avoiding horizontal overflow.
- Navigation, action bars, data cards, form controls, and route-specific panels must remain readable without overlapping.
- Text must wrap or reduce hierarchy before it clips. Do not use viewport-width font scaling.
- Desktop-only composition must collapse into clear vertical rhythm matching the mobile screenshot set.
- Carousels must drag inside their own bounds, resizable or collapsible panes must remain usable, and affected route screenshots must be captured.

## No-Drift Checklist

Before shipping a web visual change:

- Compare the affected route against the matching 2026-05-25 desktop and mobile screenshots.
- Confirm palette values come from `packages/design-tokens` or this document.
- Confirm orange-to-magenta gradients are limited to primary CTAs and key brand accents.
- Confirm default radius remains `8px` and cards are not softened.
- Confirm shadows remain absent or restrained.
- Confirm typography weight, scale, and density match the screenshot family.
- Confirm mobile at `390x844` has no clipped text, overlapping controls, or horizontal scroll.
- Confirm the browser invariant: `document.documentElement.scrollWidth <= document.documentElement.clientWidth + 1`.
- Confirm carousel and gallery scroll is internal only.
- Confirm the Studio app-creation journey still calls `describeIdea(startImmediately: true)`, opens `/p/{projectID}` with `executionID`, accepts a refinement message, and walks the core post-build pages.
- Confirm no light-theme drift, lime-first accents, decorative blobs, nested cards, or generic stock visuals were introduced.
- If intentional drift is required, update this document and capture a new dated screenshot baseline in the same handoff format.
