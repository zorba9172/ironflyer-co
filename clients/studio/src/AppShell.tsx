import { Box } from '@mui/material';
import { Outlet, useNavigate } from 'react-router-dom';
import { AppSidebar } from './components/AppSidebar';
import { useStudio } from './store';

// Persistent shell for the workspace pages (home, projects, templates,
// integrations, agents). The editor route renders full-screen without it.
export function AppShell() {
  const navigate = useNavigate();
  const startFromPrompt = useStudio((s) => s.startFromPrompt);
  return (
    <Box sx={{ display: 'flex', height: '100vh', bgcolor: 'background.default' }}>
      <AppSidebar onNewProject={() => { startFromPrompt('Finish my product'); navigate('/build'); }} />
      <Box sx={{ flex: 1, minWidth: 0, height: '100vh', overflowY: 'auto' }}>
        <Outlet />
      </Box>
    </Box>
  );
}
