import type { ReactNode } from 'react';
import { CssVarsProvider, useColorScheme } from '@mui/material/styles';
import { CssBaseline } from '@mui/material';
import { studioTheme } from './studioTheme';

const MODE_KEY = 'ifs-theme';

// Wraps the studio in the product workspace CSS-variables theme. Light is the
// primary canvas; the choice persists to localStorage so toggles survive reloads.
export function StudioThemeProvider({ children }: { children: ReactNode }) {
  return (
    <CssVarsProvider theme={studioTheme} defaultMode="light" modeStorageKey={MODE_KEY}>
      <CssBaseline />
      {children}
    </CssVarsProvider>
  );
}

// Resolved light/dark + a one-call toggle, on top of MUI's color-scheme
// machinery. This is the single hook the theme-toggle button consumes.
export function useThemeMode() {
  const { mode, systemMode, setMode } = useColorScheme();
  const resolved = (mode === 'system' ? systemMode : mode) ?? 'light';
  return {
    mode: resolved as 'light' | 'dark',
    toggle: () => setMode(resolved === 'dark' ? 'light' : 'dark'),
    setMode,
  };
}
