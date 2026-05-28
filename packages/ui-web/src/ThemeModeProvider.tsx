import type { ReactNode } from 'react';
import { CssVarsProvider, useColorScheme } from '@mui/material/styles';
import { CssBaseline } from '@mui/material';
import { theme } from './theme';

const MODE_KEY = 'if-theme';

export function ThemeModeProvider({ children }: { children: ReactNode }) {
  return (
    <CssVarsProvider theme={theme} defaultMode="system" modeStorageKey={MODE_KEY}>
      <CssBaseline />
      {children}
    </CssVarsProvider>
  );
}

// Resolved light/dark + a toggle, on top of MUI's color-scheme machinery.
export function useThemeMode() {
  const { mode, systemMode, setMode } = useColorScheme();
  const resolved = (mode === 'system' ? systemMode : mode) ?? 'dark';
  return {
    mode: resolved as 'light' | 'dark',
    toggle: () => setMode(resolved === 'dark' ? 'light' : 'dark'),
    setMode,
  };
}
