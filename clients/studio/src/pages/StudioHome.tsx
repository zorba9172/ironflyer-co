import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, Button, Card, Chip, IconButton, InputBase, Stack, Switch, Typography } from '@mui/material';
import { Carousel } from '@ironflyer/ui-web/fx';
import { useStudio } from '../store';
import { mockProject } from '../studioData';

const templates = [
  { name: 'SaaS dashboard', meta: 'Auth · billing · admin' },
  { name: 'Marketplace', meta: 'Listings · payments · payouts' },
  { name: 'AI chatbot', meta: 'Streaming · memory · usage' },
  { name: 'Booking app', meta: 'Calendar · reminders · Stripe' },
  { name: 'Internal tool', meta: 'Tables · roles · audit log' },
];

const categories = ['Import a build', 'Finish auth', 'Wire payments', 'Harden security', 'Ship to prod', 'More'];
const recents = [
  { name: 'Northwind Checkout', meta: '2 gates open · imported from lovable', tone: 'warning.main' },
  { name: 'MathQuest', meta: 'shipped · 0 gates open', tone: 'success.main' },
];

export function StudioHome() {
  const navigate = useNavigate();
  const { startFromPrompt, openProject } = useStudio();
  const [prompt, setPrompt] = useState('');
  const [planMode, setPlanMode] = useState(false);

  // composer / chips → create a project from the prompt and open the editor
  const start = () => {
    startFromPrompt(prompt || 'Finish my product');
    navigate('/build');
  };
  // recents → open the existing project (mock for now)
  const open = () => {
    openProject(mockProject);
    navigate('/build');
  };

  return (
    <Box sx={{ minHeight: '100%', display: 'flex', flexDirection: 'column' }}>
      <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', px: 3, py: 8, maxWidth: 920, mx: 'auto', width: '100%' }}>
        <Typography variant="h2" sx={{ fontSize: { xs: '2.25rem', md: '3.25rem' }, textAlign: 'center', mb: 4 }}>
          What are we finishing today?
        </Typography>

        {/* Composer */}
        <Box
          sx={{
            width: '100%',
            border: 1,
            borderColor: 'divider',
            borderRadius: 4,
            bgcolor: 'background.paper',
            p: 2,
            transition: (t) => `border-color ${t.brand.motion.fast}`,
            '&:focus-within': { borderColor: 'primary.main' },
          }}
        >
          <InputBase
            multiline
            minRows={2}
            fullWidth
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder="Paste a repo or Lovable/Bolt link to import — or describe the product you want to finish…"
            onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); start(); } }}
            autoFocus
            sx={{ fontSize: '1rem', px: 1 }}
          />
          <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mt: 1.5 }}>
            <Stack direction="row" spacing={0.5}>
              {['+', '⌥', '⚙'].map((g) => (
                <IconButton key={g} size="small" sx={{ border: 1, borderColor: 'divider', borderRadius: 1.5, color: 'text.secondary', width: 34, height: 34 }}>{g}</IconButton>
              ))}
            </Stack>
            <Stack direction="row" alignItems="center" spacing={1}>
              <Typography sx={{ fontSize: '0.85rem', color: 'text.secondary' }}>Plan first</Typography>
              <Switch size="small" checked={planMode} onChange={(e) => setPlanMode(e.target.checked)} />
              <IconButton
                onClick={start}
                aria-label="Start"
                sx={(t) => ({ color: '#fff', backgroundImage: t.brand.gradient.signature, width: 36, height: 36, '&:hover': { boxShadow: t.brand.shadow.glow } })}
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
              </IconButton>
            </Stack>
          </Stack>
        </Box>

        {/* Category chips */}
        <Stack direction="row" spacing={1} sx={{ mt: 3, flexWrap: 'wrap', justifyContent: 'center', gap: 1 }}>
          {categories.map((c) => (
            <Chip key={c} label={c} onClick={start} variant="outlined" sx={{ borderColor: 'divider', '&:hover': { bgcolor: 'action.hover' } }} />
          ))}
        </Stack>
      </Box>

      {/* Recents */}
      <Box sx={{ borderTop: 1, borderColor: 'divider', px: 3, py: 3 }}>
        <Box sx={{ maxWidth: 920, mx: 'auto' }}>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Recent projects</Typography>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 1.5 }}>
            {recents.map((r) => (
              <Button key={r.name} onClick={open} sx={{ justifyContent: 'flex-start', p: 2, border: 1, borderColor: 'divider', borderRadius: 3, textAlign: 'left', '&:hover': { borderColor: 'text.disabled', bgcolor: 'action.hover' } }}>
                <Stack direction="row" alignItems="center" spacing={1.5}>
                  <Box sx={{ width: 10, height: 10, borderRadius: 99, bgcolor: r.tone }} />
                  <Box>
                    <Typography sx={{ fontWeight: 600, color: 'text.primary' }}>{r.name}</Typography>
                    <Typography sx={{ fontSize: '0.8rem', color: 'text.disabled' }}>{r.meta}</Typography>
                  </Box>
                </Stack>
              </Button>
            ))}
          </Box>

          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mt: 3, mb: 1.5 })}>Start from a template</Typography>
          <Carousel slidesPerView="auto" gap={14} pagination={false}>
            {templates.map((tpl) => (
              <Card key={tpl.name} onClick={start} sx={{ width: 220, p: 2, cursor: 'pointer', '&:hover': { borderColor: 'text.disabled' } }}>
                <Typography sx={{ fontWeight: 600 }}>{tpl.name}</Typography>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.72rem', color: 'text.disabled', mt: 0.5 })}>{tpl.meta}</Typography>
              </Card>
            ))}
          </Carousel>
        </Box>
      </Box>
    </Box>
  );
}
