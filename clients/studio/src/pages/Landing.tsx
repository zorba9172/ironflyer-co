import { useRef, useState } from 'react';
import { Box, Stack } from '@mui/material';
import { useThemeMode } from '../theme';
import { AmbientBackdrop } from './home/AmbientBackdrop';
import { TopNav } from './home/TopNav';
import { Hero } from './home/Hero';
import { PromptComposer } from './home/PromptComposer';
import { TemplateRail } from './home/TemplateRail';
import { FeatureGrid } from './home/FeatureGrid';
import { TrustRow } from './home/TrustRow';

// ─────────────────────────────────────────────────────────────────────────
// THE STUDIO LANDING — Neon Intelligence hero.
// Pixel-faithful to clients/studio/design_refernce/…01_32_18….png.
// This is the logged-out entry: a prompt-first marketing surface whose every
// CTA hands off to authentication via `onEnter`. All color/effect/motion comes
// from the studio theme — see clients/studio/DESIGN_CONSTITUTION.md.
// ─────────────────────────────────────────────────────────────────────────
export function Landing({ onEnter }: { onEnter?: (prompt?: string) => void }) {
  const { mode, toggle } = useThemeMode();
  const inputRef = useRef<HTMLTextAreaElement | HTMLInputElement | null>(null);
  const [prompt, setPrompt] = useState('');
  const [planFirst, setPlanFirst] = useState(true);

  const enter = (seed?: string) => onEnter?.(seed ?? prompt);

  return (
    <Box
      sx={(theme) => ({
        position: 'relative',
        minHeight: '100dvh',
        overflowX: 'hidden',
        bgcolor: theme.palette.background.default,
        color: theme.palette.text.primary,
        transition: `background-color ${theme.studio.motion.base}, color ${theme.studio.motion.base}`,
      })}
    >
      <AmbientBackdrop />

      <Box sx={{ position: 'relative', zIndex: 1, maxWidth: 1200, mx: 'auto', px: { xs: 2, md: 4 }, pt: { xs: 2, md: 3 }, pb: { xs: 6, md: 9 } }}>
        <TopNav
          mode={mode}
          onThemeToggle={toggle}
          onLogin={() => enter()}
          onStart={() => enter()}
        />

        <Stack alignItems="center" sx={{ mt: { xs: 6, md: 10 } }}>
          <Hero />

          <Box sx={{ width: '100%', mt: { xs: 4, md: 5 } }}>
            <PromptComposer
              value={prompt}
              onChange={setPrompt}
              planFirst={planFirst}
              onPlanFirstChange={setPlanFirst}
              onSubmit={() => enter()}
              inputRef={inputRef}
            />
          </Box>

          <Box sx={{ width: '100%', mt: { xs: 3, md: 4 } }}>
            <TemplateRail onSelect={(id) => enter(`Start from the ${id} template`)} />
          </Box>
        </Stack>

        <Box sx={{ mt: { xs: 8, md: 12 } }}>
          <FeatureGrid />
        </Box>

        <Box sx={{ mt: { xs: 8, md: 12 } }}>
          <TrustRow />
        </Box>
      </Box>
    </Box>
  );
}
