'use client';

import { CssBaseline, ThemeProvider } from '@mui/material';
import { ironflyerTheme } from '../lib/theme';
import { AuthProvider } from './auth-context';

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider theme={ironflyerTheme}>
      <CssBaseline />
      <AuthProvider>{children}</AuthProvider>
    </ThemeProvider>
  );
}
