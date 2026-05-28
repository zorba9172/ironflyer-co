import { Box } from '@mui/material';
import { Outlet } from 'react-router-dom';
import { AppSidebar } from './components/AppSidebar';

// Persistent operator shell: fixed sidebar + scrollable content region.
export function AppShell() {
  return (
    <Box sx={{ display: 'flex', height: '100vh', bgcolor: 'background.default' }}>
      <AppSidebar />
      <Box sx={{ flex: 1, minWidth: 0, height: '100vh', overflowY: 'auto' }}>
        <Outlet />
      </Box>
    </Box>
  );
}
