# Studio VS Code Cloud Target

Status: TARGET. This is the locked Studio reference supplied by the product owner in the 2026-05-25 handoff chat.

Do not use `studio-entry-current-gap.png` as the target. That file is only an implementation-progress capture. The target is the private screenshot and the structured reference below.

## Target Screenshot Identity

- Route family: `/studio`, `/p/[projectID]`
- Visual mode: dark VS Code cloud builder
- Desktop reference viewport: wide desktop, approximately 1600 x 947
- Required first impression: full product workspace, not marketing, not empty start page
- Reference ownership: private product-design handoff

## Locked Layout

1. Top global nav, full width, height about 58px.
   - Left: IronFlyer logo and wordmark.
   - Center: Product, Templates, Solutions with chevron, Pricing, Resources with chevron, Enterprise.
   - Right: Log in, primary `Start a project` gradient button.
   - Border bottom: subtle violet/dark line.

2. Left app rail starts under the global nav.
   - Width about 250px.
   - Background: near-black with subtle violet edge.
   - Top CTA: `+ New app`, outlined violet, height about 37px.
   - Primary nav: Home, All apps, Templates, Integrations, Studio.
   - Studio item is active with violet filled surface.
   - Recents list: MathQuest, ClientFlow active, Fit booking, InvoicePro, TeamHub.
   - Lower cards: Upgrade your plan, Settings, What's new, Pro plan usage card.

3. Studio main top bar.
   - Row 1: breadcrumb `Studio > ClientFlow`, right actions `GitHub`, `Preview live`, `Publish`.
   - Row 2: mode tabs left: Preview, Mobile, Code active.
   - Row 2 status cards right: Plan Locked, Web Live, Mobile Queued, Gate 92/100 active, Deploy Preview.

4. Workbench grid.
   - Column 1: AI Prompt panel, about 320px wide.
   - Column 2: Code panel, dominant middle area.
   - Column 3: Preview panel, about 430px wide.
   - Bottom: Studio Assistant spans full main width below the three panels.

## Panel Details

### AI Prompt

- Header: `AI Prompt`, right button `Improve prompt`.
- Textarea content: "Build a client operations portal with projects, invoices, approvals, role-based access and team activity dashboard."
- Context chips row: Add context, Business goals active/warm, Data models.
- Primary gradient button: `Generate`, full width, violet gradient, sparkle icon.
- Suggestions title.
- Four suggestion cards in 2x2 grid:
  - Add subscription billing flow with stripe
  - Add team chat and mentions
  - Add client portal notifications
  - Add advanced permissions

### Code

- Header: `Code`.
- Left files tree visible inside panel:
  - src
  - components
  - pages
  - hooks
  - lib
  - styles
  - .env.local
  - package.json
  - README.md
- Editor tab: `Dashboard.tsx`.
- Code visible with line numbers and colored syntax.
- Footer: TypeScript, Saved 2m ago, cursor position, Format.

### Preview

- Header: `Preview`.
- Device toggles: desktop active, tablet/mobile icons, refresh.
- Preview app header: `Client operations portal`, Live badge, menu icon.
- Four metric cards:
  - Revenue `$18.2k`
  - Open approvals `27`
  - Files `1.4k`
  - Deploy health `99.9%`
- Projects table with rows:
  - Website redesign / Maya P. / Live / 2m ago
  - Mobile app / Noah K. / Preview / 10m ago
  - CRM integration / Liam T. / Live / 1h ago
  - Analytics dashboard / Emma R. / Queued / 2h ago
- Bottom link: `View all projects ->`

### Studio Assistant

- Header: `Studio Assistant`.
- Four completed cards:
  - Added project table with filters and search
  - Connected Stripe billing flow
  - Added role-based access control
  - Added activity feed and notifications
- Right composer: placeholder `Ask anything... (e.g. "Add invoice approvals flow")`
- Context chips: ClientFlow, src/dashboard.tsx, Data models.
- Send icon button, violet circular.

## Locked Colors

- Page base: `#050612`
- Panel background: `#0c0d20`, `#11132a`, `#080918`
- Text primary: `#f7f4ff`
- Text secondary: `#b9b2d3`
- Text muted: `#777096`
- Border subtle: `rgba(178, 133, 255, 0.16)`
- Border strong: `rgba(187, 147, 255, 0.34)`
- Active violet: `#8f4dff`, `#b56cff`
- CTA gradient: `#ff7848 -> #e149c9 -> #8f4dff`
- Success: `#7fe28a`
- Warning: `#ffb457`

## Forbidden Drift

- No centered Studio hero.
- No missing global nav on the Studio desktop reference.
- No replacing the interactive panels with one image.
- No light theme.
- No lime-first theme.
- No custom palette additions without updating `apps/web/DESIGN_REFERENCE.md`.
- No page-level horizontal scroll.
- No hiding the prompt panel on mobile; it stacks above code/preview.

## Acceptance

Desktop implementation must show the full shell: global nav, left rail, top action/status bars, prompt/code/preview columns, and bottom assistant in the first viewport. Mobile stacks the same surfaces without horizontal overflow.
