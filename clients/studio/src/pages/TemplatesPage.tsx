import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, Chip, Stack, Typography } from '@mui/material';
import { LuLayoutGrid, LuSparkles } from 'react-icons/lu';
import { useStudio } from '../store';
import { AmbientBackdrop } from './home/AmbientBackdrop';
import { TemplateCard } from './templates/TemplateCard';
import { CATEGORIES, TEMPLATES } from './templates/data';

// ─────────────────────────────────────────────────────────────────────────
// Templates — the full-page starter gallery. Mirrors the home TemplateRail
// energy at scale: an ambient-backed header, glanceable proof stats, sticky
// category filter pills, and floating glass template cards with a readiness
// meter + clear "Use template" CTA.
//
// Functionality is preserved verbatim: useNavigate, useStudio.startFromPrompt,
// and the use(name) → startFromPrompt(`Start from the ${name} template`) +
// navigate('/build') contract. Only presentation/layout/UX changed; all colors,
// radii, blur, motion, and gradients flow through the studio theme.
// ─────────────────────────────────────────────────────────────────────────

export function TemplatesPage() {
  const navigate = useNavigate();
  const startFromPrompt = useStudio((s) => s.startFromPrompt);
  const [cat, setCat] = useState<string>('All');

  const use = (name: string) => {
    startFromPrompt(`Start from the ${name} template`);
    navigate('/build');
  };

  const list = useMemo(
    () => (cat === 'All' ? TEMPLATES : TEMPLATES.filter((t) => t.cat === cat)),
    [cat],
  );

  return (
    <Box sx={{ position: 'relative', minHeight: '100%' }}>
      <AmbientBackdrop />

      <Box
        sx={{
          position: 'relative',
          zIndex: 1,
          px: { xs: 3, md: 6 },
          py: { xs: 4, md: 6 },
          maxWidth: 1180,
          mx: 'auto',
        }}
      >
        {/* Header — AI badge → headline (final phrase gradient) → subhead. */}
        <Stack spacing={2.5} sx={{ mb: { xs: 4, md: 5 } }}>
          <Chip
            icon={<LuSparkles size={15} />}
            label="Production-ready starters"
            sx={(theme) => ({
              alignSelf: 'flex-start',
              height: 32,
              borderRadius: theme.studio.radius.pill,
              border: `1px solid ${theme.palette.divider}`,
              backgroundColor: theme.palette.cardBg,
              backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
              color: theme.palette.text.secondary,
              fontWeight: theme.typography.fontWeightMedium,
              '& .MuiChip-icon': { color: theme.studio.neon.blue, ml: 0.25 },
              '& .MuiChip-label': { px: 1 },
            })}
          />

          <Typography
            variant="h2"
            sx={{ fontSize: { xs: '2rem', md: '2.75rem' }, maxWidth: 760 }}
          >
            Start from a proven build and{' '}
            <Box
              component="span"
              sx={(theme) => ({
                backgroundImage: theme.studio.gradient.signature,
                WebkitBackgroundClip: 'text',
                backgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                color: 'transparent',
              })}
            >
              ship it to production.
            </Box>
          </Typography>

          <Typography color="text.secondary" sx={{ maxWidth: 640, fontSize: '1.0625rem' }}>
            Every template lands with real data, gates, and a deploy path. Pick one and
            the finisher takes it the rest of the way.
          </Typography>

          {/* Glanceable proof strip — mirrors the catalog state, not decoration. */}
          <Stack
            direction="row"
            divider={
              <Box
                sx={(theme) => ({ width: '1px', alignSelf: 'stretch', backgroundColor: theme.palette.borderSubtle })}
              />
            }
            spacing={2.5}
            sx={{ mt: 0.5, flexWrap: 'wrap', rowGap: 1.5 }}
          >
            {[
              { value: TEMPLATES.length, label: 'starter blueprints' },
              { value: CATEGORIES.length - 1, label: 'categories' },
              { value: 'Gated', label: 'before every deploy' },
            ].map((s) => (
              <Stack key={s.label} direction="row" alignItems="baseline" spacing={0.75}>
                <Typography
                  variant="h6"
                  sx={(theme) => ({ color: theme.palette.text.primary, fontWeight: 700 })}
                >
                  {s.value}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {s.label}
                </Typography>
              </Stack>
            ))}
          </Stack>
        </Stack>

        {/* Sticky category filter — pills follow the home rail treatment. */}
        <Box
          sx={(theme) => ({
            position: 'sticky',
            top: 0,
            zIndex: 2,
            py: 1.5,
            mb: 3,
            backgroundColor: `${theme.palette.background.default}D9`,
            backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
            WebkitBackdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
          })}
        >
          <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1 }}>
            {CATEGORIES.map((c) => {
              const active = c === cat;
              return (
                <Chip
                  key={c}
                  label={c}
                  onClick={() => setCat(c)}
                  icon={c === 'All' ? <LuLayoutGrid size={14} aria-hidden /> : undefined}
                  sx={(theme) => ({
                    height: 34,
                    px: 0.5,
                    borderRadius: theme.studio.radius.pill,
                    fontWeight: 600,
                    cursor: 'pointer',
                    border: `1px solid ${active ? 'transparent' : theme.palette.divider}`,
                    color: active ? theme.palette.common.white : theme.palette.text.secondary,
                    background: active ? theme.studio.gradient.cta : theme.palette.cardBg,
                    backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
                    transition: `transform ${theme.studio.motion.fast}, border-color ${theme.studio.motion.fast}, color ${theme.studio.motion.fast}, background-color ${theme.studio.motion.fast}`,
                    '& .MuiChip-icon': { color: 'inherit', ml: 0.75, mr: -0.25 },
                    '& .MuiChip-label': { px: 1.25 },
                    '&:hover': {
                      transform: 'translateY(-1px)',
                      borderColor: active ? 'transparent' : theme.studio.neon.violet,
                      color: active ? theme.palette.common.white : theme.palette.text.primary,
                      backgroundColor: active ? undefined : theme.palette.surfaceHover,
                    },
                  })}
                />
              );
            })}
          </Stack>
        </Box>

        {/* Card grid. */}
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', lg: 'repeat(3, 1fr)' },
            gap: { xs: 2, md: 2.5 },
          }}
        >
          {list.map((t) => (
            <TemplateCard key={t.name} template={t} onUse={() => use(t.name)} />
          ))}
        </Box>
      </Box>
    </Box>
  );
}
