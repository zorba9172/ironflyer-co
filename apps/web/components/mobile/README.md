# Mobile + Responsive System

Helpers for making Ironflyer's web app feel native on phones and installable as a PWA.

## What lives where

| Path | Purpose |
| --- | --- |
| `apps/web/app/globals.css` | Breakpoint CSS vars, safe-area utilities, tap-highlight reset, `.rtl` / `.ltr`, `.no-scrollbar`, `.tap-target`. |
| `apps/web/components/responsive/useBreakpoint.ts` | Runtime breakpoint hook: `{ isMobile, isTablet, isDesktop }`. |
| `apps/web/components/responsive/ResponsiveContainer.tsx` | `MobileOnly`, `TabletAndUp`, `DesktopOnly`, `ResponsiveContainer`. |
| `apps/web/components/mobile/MobileNavDrawer.tsx` | Bottom-sheet nav, opt-in for any shell. |
| `apps/web/components/PWAInstaller.tsx` | Install-prompt banner + service worker registration. |
| `apps/web/public/sw.js` | Cache-first assets, network-first HTML, ignores `/api/*` and `/app/projects/*`. |
| `apps/web/app/apple-icon.tsx` | 180x180 iOS home-screen icon. |

## Breakpoints

| Name | Range |
| --- | --- |
| `isMobile` | `width < 600px` |
| `isTablet` | `600px - 899px` |
| `isDesktop` | `>= 900px` |

Same values are emitted as CSS custom properties in `globals.css` (`--bp-mobile-max`, `--bp-tablet-min`, `--bp-tablet-max`, `--bp-desktop-min`) — use them in custom media queries instead of magic numbers.

## Touch ergonomics

- Tap targets must be at least **44 x 44 px**. Apply `className="tap-target"` to icon-only buttons or set `minHeight: 'var(--tap-target)'` via `sx`.
- Padding from the screen edge is **16 px** by default (`--edge-pad`). `ResponsiveContainer` already enforces this with MUI breakpoint padding.
- Body copy on mobile should be at least 17 px — use `.mobile-readable` for long-form text that was tuned for desktop.
- Honour the notch with `.safe-area`, `.safe-area-bottom`, `.safe-area-x`. The drawer + install banner already apply these.

## Recipes

### Conditionally render a mobile-only UI

```tsx
import { MobileOnly, DesktopOnly } from '@/components/responsive';

<MobileOnly><MobileNavDrawerTrigger /></MobileOnly>
<DesktopOnly><Sidebar /></DesktopOnly>
```

### Branch on the breakpoint inside one component

```tsx
import { useBreakpoint } from '@/components/responsive/useBreakpoint';

const { isMobile } = useBreakpoint();
return <Layout dense={isMobile} />;
```

### Wire the mobile nav drawer into a workspace shell

The workspace shell file (`components/workspace/WorkspaceSidebar.tsx` and similar) is owned by another agent. The drawer is additive — opt in like this:

```tsx
import { useState } from 'react';
import { useBreakpoint } from '@/components/responsive/useBreakpoint';
import { MobileNavDrawer } from '@/components/mobile/MobileNavDrawer';

const { isMobile } = useBreakpoint();
const [open, setOpen] = useState(false);

return (
  <>
    {isMobile && <HamburgerButton onClick={() => setOpen(true)} />}
    <MobileNavDrawer
      items={navItems}
      open={open}
      onClose={() => setOpen(false)}
      onNavigate={(href) => router.push(href)}
    />
  </>
);
```

Pass the same items the desktop sidebar already renders — the drawer mirrors them.

## PWA install flow

`<PWAInstaller />` is mounted once in `apps/web/app/layout.tsx`. It registers `/sw.js`, captures `beforeinstallprompt`, waits 15 seconds, and shows the install banner. Dismissals are stored in `localStorage` and never re-shown until the keys are cleared.

The service worker uses `cacheName: 'ironflyer-v1'` — bump the version when changing caching rules. Send `postMessage({ type: 'SKIP_WAITING' })` from the page to force an immediate activation after deploys.
