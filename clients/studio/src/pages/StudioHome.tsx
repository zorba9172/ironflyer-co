import { useRef, useState } from 'react';
import { Box, Button, Stack, Typography } from '@mui/material';
import { useNavigate } from 'react-router-dom';
import { LuArrowRight, LuBot, LuExternalLink } from 'react-icons/lu';
import { text } from '@ironflyer/design-tokens/brand';
import { useStudio } from '../store';
import { STARTERS, matchStarter } from '../lib/starters';
import { AppSidebar } from '../components/AppSidebar';
import { LogoMark } from '../components/LogoMark';
import { Hero } from './home/Hero';
import { PromptComposer } from './home/PromptComposer';
import { TemplateRail } from './home/TemplateRail';
import { FeatureGrid } from './home/FeatureGrid';

export function StudioHome() {
  const navigate = useNavigate();
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);
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
        display: 'flex',
        bgcolor: 'background.default',
        color: 'text.primary',
        transition: `background-color ${theme.studio.motion.base}, color ${theme.studio.motion.base}`,
      })}
    >
      <AppSidebar onNewProject={() => inputRef.current?.focus()} />
      <Box component="main" sx={{ flex: 1, minWidth: 0, height: '100dvh', overflow: 'auto', p: { xs: 1.2, md: 2 } }}>
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={(theme) => ({
            display: { xs: 'flex', md: 'none' },
            mb: 1,
            px: 1,
            py: 0.8,
            borderRadius: `${theme.studio.radius.lg}px`,
            border: `1px solid ${theme.palette.cardBorder}`,
            bgcolor: 'background.paper',
          })}
        >
          <Stack direction="row" alignItems="center" spacing={1}>
            <LogoMark size={24} />
            <Typography sx={{ fontWeight: 900, fontSize: '1rem' }}>Ironflyer</Typography>
          </Stack>
          <Typography sx={{ color: 'text.secondary', fontSize: '0.78rem', fontWeight: 800 }}>Build studio</Typography>
        </Stack>
        <Box
          sx={(theme) => ({
            minHeight: { xs: 'calc(100dvh - 78px)', md: 'calc(100dvh - 32px)' },
            borderRadius: { xs: `${theme.studio.radius.lg}px`, md: 4 },
            border: `1px solid ${theme.palette.cardBorder}`,
            overflow: 'hidden',
            background: theme.studio.effect.ambient.light,
            display: 'flex',
            flexDirection: 'column',
          })}
        >
          <Stack alignItems="center" spacing={{ xs: 2, md: 3 }} sx={{ flex: 1, px: { xs: 1.5, sm: 2.5, md: 4 }, pt: { xs: 3.5, sm: 4.5, md: 7 }, pb: { xs: 4, md: 7 } }}>
            <Stack
              direction="row"
              alignItems="center"
              justifyContent="center"
              useFlexGap
              spacing={{ xs: 0.7, sm: 1.5 }}
              sx={(theme) => ({
                width: 'fit-content',
                maxWidth: '100%',
                flexWrap: 'wrap',
                px: { xs: 1.05, sm: 2.4 },
                py: { xs: 0.75, sm: 1.15 },
                borderRadius: theme.studio.radius.pill,
                bgcolor: `${theme.palette.primary.main}cc`,
                color: theme.palette.primary.contrastText,
                boxShadow: '0 12px 34px rgba(242,103,46,0.18)',
              })}
            >
              <Typography sx={{ display: { xs: 'none', md: 'block' }, fontWeight: 900, fontSize: { xs: text.s82, md: text.s98 } }}>
                Limited time welcome offer
              </Typography>
              <Button
                size="small"
                color="inherit"
                endIcon={<LuExternalLink size={15} />}
                sx={(theme) => ({
                  minHeight: { xs: 30, sm: 34 },
                  px: { xs: 1.15, sm: 1.55 },
                  borderRadius: theme.studio.radius.pill,
                  bgcolor: 'rgba(255,255,255,0.56)',
                  color: theme.palette.text.primary,
                  fontWeight: 900,
                  fontSize: { xs: text.s76, sm: text.s82 },
                  '&:hover': { bgcolor: 'rgba(255,255,255,0.72)' },
                })}
              >
                <Box component="span" sx={{ display: { xs: 'none', sm: 'inline' } }}>
                  Get 40% off select yearly plans
                </Box>
                <Box component="span" sx={{ display: { xs: 'inline', sm: 'none' } }}>
                  40% off yearly
                </Box>
              </Button>
              <Typography sx={(theme) => ({ fontFamily: theme.brand.font.mono, fontSize: { xs: text.s86, md: text.s130 }, fontWeight: 900, color: theme.palette.common.black })}>
                47:59:34
              </Typography>
            </Stack>

            <Button
              variant="outlined"
              color="inherit"
              startIcon={<LuBot size={18} />}
              endIcon={<LuArrowRight size={17} />}
              onClick={() => navigate('/agents')}
              sx={(theme) => ({
                mt: { xs: 0.25, md: 1.25 },
                minHeight: 48,
                px: 2.3,
                borderRadius: theme.studio.radius.pill,
                bgcolor: 'background.paper',
                borderColor: `${theme.palette.secondary.main}33`,
                boxShadow: '0 1px 2px rgba(24,22,20,0.04)',
                fontWeight: 900,
                '&:hover': { bgcolor: 'background.paper', borderColor: theme.palette.secondary.main },
              })}
            >
              Go to your Superagent
            </Button>

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
          </Stack>

          <Box
            sx={(theme) => ({
              mx: { xs: 1.5, sm: 3, md: 10 },
              mt: 'auto',
              bgcolor: 'background.paper',
              border: `1px solid ${theme.palette.cardBorder}`,
              borderBottom: 0,
              borderRadius: { xs: `${theme.studio.radius.xl}px ${theme.studio.radius.xl}px 0 0`, md: '28px 28px 0 0' },
              minHeight: { xs: 150, md: 178 },
              p: { xs: 2, md: 3 },
              boxShadow: '0 -10px 36px rgba(24,22,20,0.04)',
            })}
          >
            <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }}>
              <Stack direction="row" spacing={1}>
                <Typography sx={(theme) => ({ px: 1.4, py: 0.85, borderRadius: `${theme.studio.radius.sm}px`, border: `1px solid ${theme.palette.divider}`, bgcolor: theme.palette.surfaceHover, fontWeight: 800 })}>Recent apps</Typography>
                <Typography sx={{ px: 1.4, py: 0.85, color: 'text.primary', fontWeight: 800 }}>Templates</Typography>
              </Stack>
              <Typography sx={{ color: 'text.primary', fontWeight: 700 }}>View all  ›</Typography>
            </Stack>
            <FeatureGrid />
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
