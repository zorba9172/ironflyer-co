import { Outlet } from 'react-router-dom';
import { Box } from '@mui/material';
import { ThemeModeProvider, InitColorSchemeScript } from '@ironflyer/ui-web';
import { Nav } from './components/Nav';
import { Footer } from './components/Footer';

export function RootLayout() {
  return (
    <ThemeModeProvider>
      <InitColorSchemeScript defaultMode="system" modeStorageKey="if-theme" />
      <Box sx={{ minHeight: '100vh', bgcolor: 'background.default', color: 'text.primary' }}>
        <Nav />
        <Box component="main">
          <Outlet />
        </Box>
        <Footer />
      </Box>
    </ThemeModeProvider>
  );
}
