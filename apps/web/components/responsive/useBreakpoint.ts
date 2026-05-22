'use client';

/**
 * useBreakpoint — single source of truth for runtime breakpoint state.
 *
 * Mirrors the CSS custom properties in apps/web/app/globals.css so a
 * change in one place forces a change in the other on review:
 *   mobile  < 600px
 *   tablet  600-899px
 *   desktop >= 900px
 *
 * Returns `{ isMobile, isTablet, isDesktop }`. SSR-safe: during the
 * initial render (no `window`) it returns `isDesktop: true` so that
 * server-rendered desktop markup hydrates cleanly on desktop and the
 * mobile drawer never flashes for a tick on a wide viewport.
 *
 * Usage:
 *   const { isMobile } = useBreakpoint();
 *   return isMobile ? <MobileNav /> : <DesktopNav />;
 */
import { useEffect, useState } from 'react';

export const BREAKPOINTS = {
  mobileMax: 599,
  tabletMin: 600,
  tabletMax: 899,
  desktopMin: 900,
} as const;

export type BreakpointState = {
  isMobile: boolean;
  isTablet: boolean;
  isDesktop: boolean;
};

const DESKTOP_DEFAULT: BreakpointState = {
  isMobile: false,
  isTablet: false,
  isDesktop: true,
};

function readState(width: number): BreakpointState {
  if (width <= BREAKPOINTS.mobileMax) {
    return { isMobile: true, isTablet: false, isDesktop: false };
  }
  if (width <= BREAKPOINTS.tabletMax) {
    return { isMobile: false, isTablet: true, isDesktop: false };
  }
  return { isMobile: false, isTablet: false, isDesktop: true };
}

export function useBreakpoint(): BreakpointState {
  const [state, setState] = useState<BreakpointState>(DESKTOP_DEFAULT);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    const update = () => setState(readState(window.innerWidth));
    update();
    window.addEventListener('resize', update, { passive: true });
    window.addEventListener('orientationchange', update);
    return () => {
      window.removeEventListener('resize', update);
      window.removeEventListener('orientationchange', update);
    };
  }, []);

  return state;
}
