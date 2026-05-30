import { useRef, useState } from 'react';
import { Box, Container, Stack } from '@mui/material';
import { useNavigate } from 'react-router-dom';
import { useThemeMode } from '../theme';
import { useStudio } from '../store';
import { STARTERS, matchStarter } from '../lib/starters';
import { AmbientBackdrop } from './home/AmbientBackdrop';
import { TopNav } from './home/TopNav';
import { Hero } from './home/Hero';
import { PromptComposer } from './home/PromptComposer';
import { TemplateRail } from './home/TemplateRail';
import { FeatureGrid } from './home/FeatureGrid';
import { TrustRow } from './home/TrustRow';

export function StudioHome() {
  const navigate = useNavigate();
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);
  const { mode, toggle } = useThemeMode();
  const startFromTemplate = useStudio((s) => s.startFromTemplate);
  const [prompt, setPrompt] = useState('');
  const [planFirst, setPlanFirst] = useState(true);

  const startWith = (value: string) => {
    const v = value.trim() || 'Build a production-ready app from this idea';
    startFromTemplate(v, matchStarter(v).files, {
      workMode: planFirst ? 'plan' : 'execute',
      preflight: planFirst,
    });
    navigate('/build');
  };

  const startTemplate = (id: string) => {
    if (id === 'more-templates') {
      navigate('/templates');
      return;
    }
    const starter = STARTERS.find((s) => s.id === id);
    if (starter) {
      startFromTemplate(starter.prompt, starter.files, {
        workMode: planFirst ? 'plan' : 'execute',
        preflight: planFirst,
      });
      navigate('/build');
      return;
    }
    const promptById: Record<string, string> = {
      'admin-panel': 'Build an admin panel with roles, audit logs, and secure workflows.',
      'mobile-app': 'Build a mobile app with React Native screens, auth, and an API.',
      'internal-tool': 'Build an internal tool with tables, roles, and workflow automations.',
    };
    startWith(promptById[id] ?? id);
  };

  return (
    <Box
      sx={(theme) => ({
        minHeight: '100dvh',
        position: 'relative',
        overflow: 'hidden',
        bgcolor: 'background.default',
        color: 'text.primary',
        transition: `background-color ${theme.studio.motion.base}, color ${theme.studio.motion.base}`,
      })}
    >
      <AmbientBackdrop />
      <Container maxWidth={false} sx={{ position: 'relative', zIndex: 1, maxWidth: 1500, px: { xs: 2, md: 3.5 }, py: { xs: 2, md: 2.25 } }}>
        <TopNav
          mode={mode}
          onThemeToggle={toggle}
          onLogin={() => navigate('/projects?auth=1')}
          onStart={() => {
            inputRef.current?.focus();
          }}
        />

        <Stack
          alignItems="center"
          spacing={{ xs: 2.75, md: 2.9 }}
          sx={{
            minHeight: { md: 'calc(100dvh - 92px)' },
            pt: { xs: 4.5, md: 5.5 },
            pb: { xs: 4, md: 3.5 },
          }}
        >
          <Hero />
          <PromptComposer
            inputRef={inputRef}
            value={prompt}
            onChange={setPrompt}
            planFirst={planFirst}
            onPlanFirstChange={setPlanFirst}
            onSubmit={() => startWith(prompt)}
          />
          <TemplateRail onSelect={startTemplate} />
          <FeatureGrid />
          <TrustRow />
        </Stack>
      </Container>
    </Box>
  );
}
