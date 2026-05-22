'use client';

/**
 * ResponsiveContainer + breakpoint gates.
 *
 * Three thin wrappers so callers can avoid sprinkling `useBreakpoint`
 * conditionals through their JSX:
 *
 *   <MobileOnly>  ...renders only when isMobile.
 *   <TabletAndUp> ...renders when isTablet OR isDesktop.
 *   <DesktopOnly> ...renders only when isDesktop.
 *
 * `ResponsiveContainer` itself is a layout primitive that applies the
 * 16px edge padding from the design system (matches --edge-pad in
 * globals.css) and clamps to a comfortable max width on desktop. Use it
 * inside any new mobile-first surface.
 *
 * All four are client components because they branch on `useBreakpoint`.
 */
import type { ReactNode } from 'react';
import { Box } from '@mui/material';
import { useBreakpoint } from './useBreakpoint';

export function MobileOnly({ children }: { children: ReactNode }) {
  const { isMobile } = useBreakpoint();
  return isMobile ? <>{children}</> : null;
}

export function TabletAndUp({ children }: { children: ReactNode }) {
  const { isMobile } = useBreakpoint();
  return isMobile ? null : <>{children}</>;
}

export function DesktopOnly({ children }: { children: ReactNode }) {
  const { isDesktop } = useBreakpoint();
  return isDesktop ? <>{children}</> : null;
}

type ResponsiveContainerProps = {
  children: ReactNode;
  /** When true, applies safe-area-inset padding on top + bottom + sides. */
  safeArea?: boolean;
  /** Max width clamp on desktop; defaults to 1200. */
  maxWidth?: number;
};

export function ResponsiveContainer({
  children,
  safeArea = false,
  maxWidth = 1200,
}: ResponsiveContainerProps) {
  return (
    <Box
      className={safeArea ? 'safe-area-x' : undefined}
      sx={{
        width: '100%',
        maxWidth,
        mx: 'auto',
        px: { xs: 2, sm: 3, md: 4 },
      }}
    >
      {children}
    </Box>
  );
}
