import { useRef, useState } from 'react';
import { Box, Stack, Typography } from '@mui/material';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@ironflyer/data';
import { useStudio } from '../store';
import { STARTERS, matchStarter } from '../lib/starters';
import { AppSidebar } from '../components/AppSidebar';
import { LogoMark } from '../components/LogoMark';
import type { StudioProject } from '../studioData';
import { HomeTopBar } from './home/HomeTopBar';
import { PromptComposer } from './home/PromptComposer';
import { TemplateRail } from './home/TemplateRail';
import { RecentBuilds } from './home/RecentBuilds';
import { FeatureGrid } from './home/FeatureGrid';
import { LiveBuild } from './home/LiveBuild';

// Derive a friendly first name from the signed-in account; falls back warmly.
function firstNameOf(email?: string | null): string {
  if (!email) return 'there';
  const handle = email.split('@')[0] ?? '';
  const part = handle.split(/[._-]/)[0] ?? '';
  return part ? part.charAt(0).toUpperCase() + part.slice(1) : 'there';
}

export function StudioHome() {
  const navigate = useNavigate();
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);
  const startFromTemplate = useStudio((s) => s.startFromTemplate);
  const openProject = useStudio((s) => s.openProject);
  const { user } = useAuth();
  const [prompt, setPrompt] = useState('');
  const [planFirst, setPlanFirst] = useState(true);

  const launch = { workMode: (planFirst ? 'plan' : 'execute') as 'plan' | 'execute', preflight: planFirst };

  const startWith = (value: string) => {
    const v = value.trim() || 'Build a production-ready app from this idea';
    startFromTemplate(v, matchStarter(v).files, launch);
    navigate('/build');
  };

  const startTemplate = (id: string) => {
    if (id === 'more-templates') {
      navigate('/templates');
      return;
    }
    const starter = STARTERS.find((s) => s.id === id);
    if (starter) {
      startFromTemplate(starter.prompt, starter.files, launch);
      navigate('/build');
      return;
    }
    const promptById: Record<string, string> = {
      shell: 'Start a new app from scratch and let the agents scaffold it.',
      'admin-panel': 'Build an admin panel with roles, audit logs, and secure workflows.',
      'mobile-app': 'Build a mobile app with React Native screens, auth, and an API.',
      'internal-tool': 'Build an internal tool with tables, roles, and workflow automations.',
    };
    startWith(promptById[id] ?? id);
  };

  const openBuild = (p: StudioProject) => {
    openProject(p);
    navigate('/build');
  };

  return (
    <Box
      sx={(theme) => ({
        minHeight: '100dvh',
        display: 'flex',
        bgcolor: 'background.default',
        color: 'text.primary',
        transition: `background-color ${theme.studio.motion.base}, color ${theme.studio.motion.base}`,
      })}
    >
      <AppSidebar onNewProject={() => inputRef.current?.focus()} />

      <Box component="main" sx={{ flex: 1, minWidth: 0, height: '100dvh', overflow: 'auto' }}>
        {/* Mobile masthead — the sidebar is hidden under md. */}
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={(theme) => ({
            display: { xs: 'flex', md: 'none' },
            px: 2,
            py: 1.25,
            borderBottom: `1px solid ${theme.palette.borderSubtle}`,
            bgcolor: 'background.paper',
            position: 'sticky',
            top: 0,
            zIndex: 2,
          })}
        >
          <Stack direction="row" alignItems="center" spacing={1}>
            <LogoMark size={24} />
            <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>Ironflyer Studio</Typography>
          </Stack>
        </Stack>

        <Box
          sx={(theme) => ({
            position: 'relative',
            minHeight: '100%',
            background: theme.studio.effect.ambient.light,
          })}
        >
          <Box
            sx={{
              maxWidth: 1320,
              mx: 'auto',
              px: { xs: 2, sm: 3, md: 4 },
              py: { xs: 3, md: 4 },
            }}
          >
            <Box
              sx={{
                display: 'grid',
                gap: { xs: 3, lg: 4 },
                gridTemplateColumns: { xs: '1fr', lg: 'minmax(0, 1fr) 320px' },
                alignItems: 'start',
              }}
            >
              {/* ── Primary column: greeting, prompt, recent builds, agents ── */}
              <Stack spacing={{ xs: 3, md: 4 }} sx={{ minWidth: 0 }}>
                <HomeTopBar
                  name={firstNameOf(user?.email)}
                  onNewProject={() => inputRef.current?.focus()}
                  onSearch={() => inputRef.current?.focus()}
                />

                <Stack spacing={2}>
                  <PromptComposer
                    inputRef={inputRef}
                    value={prompt}
                    onChange={setPrompt}
                    planFirst={planFirst}
                    onPlanFirstChange={setPlanFirst}
                    onTool={(t) => { if (t === 'From template') navigate('/templates'); }}
                    onSubmit={() => startWith(prompt)}
                  />
                  <TemplateRail onSelect={startTemplate} />
                </Stack>

                <RecentBuilds onOpen={openBuild} onViewAll={() => navigate('/projects')} />
                <FeatureGrid />
              </Stack>

              {/* ── Ambient right column: real gate/agent state ── */}
              <Box sx={{ display: { xs: 'none', lg: 'block' }, position: 'sticky', top: 16 }}>
                <LiveBuild />
              </Box>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
