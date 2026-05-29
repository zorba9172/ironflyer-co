import { useState } from 'react';
import { Box } from '@mui/material';
import { Outlet, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { operations, useRequest } from '@ironflyer/data';
import { toast } from '@ironflyer/ui-web/fx';
import { AppSidebar } from './components/AppSidebar';
import { useStudio } from './store';

// Persistent shell for the workspace pages (home, projects, templates,
// integrations, agents). The editor route renders full-screen without it.
export function AppShell() {
  const navigate = useNavigate();
  const request = useRequest();
  const qc = useQueryClient();
  const startFromPrompt = useStudio((s) => s.startFromPrompt);
  const setLiveProjectId = useStudio((s) => s.setLiveProjectId);
  const [creating, setCreating] = useState(false);
  const createNewProject = async () => {
    const prompt = 'Finish my product';
    if (!request) {
      startFromPrompt(prompt);
      navigate('/build');
      return;
    }
    setCreating(true);
    try {
      const created = await request<{ createProject: { id: string } }>('CreateProject', operations.CREATE_PROJECT, {
        input: { name: 'Untitled project', idea: prompt },
      });
      startFromPrompt(prompt);
      setLiveProjectId(created.createProject.id);
      void qc.invalidateQueries({ queryKey: ['projects'] });
      navigate('/build');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Could not create a live project. Starting locally.', 'error');
      startFromPrompt(prompt);
      navigate('/build');
    } finally {
      setCreating(false);
    }
  };
  return (
    <Box sx={{ display: 'flex', height: '100vh', bgcolor: 'background.default' }}>
      <AppSidebar onNewProject={createNewProject} newProjectBusy={creating} />
      <Box sx={{ flex: 1, minWidth: 0, height: '100vh', overflowY: 'auto' }}>
        <Outlet />
      </Box>
    </Box>
  );
}
