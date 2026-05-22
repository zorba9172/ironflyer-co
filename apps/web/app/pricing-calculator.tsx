'use client';

import { useMemo, useState } from 'react';
import { Bolt, TrendingUp } from '@mui/icons-material';
import {
  Box, Chip, Slider, Stack, ToggleButton, ToggleButtonGroup, Typography,
} from '@mui/material';
import { tokens } from '../../../packages/design-tokens';

// PricingCalculator estimates a builder's monthly provider cost using
// rough Anthropic + OpenAI list prices (Q2 2026). The output is intentionally
// directional, not invoice-accurate — the actual workspace ledger is the
// source of truth. We pin "Lite / Economy / Power" to model picks so the
// number reflects the effort-dial behaviour of the real router.

// Rough per-1M-token prices as of Q2 2026. Real rates fluctuate; this is
// the band the marketing site should set expectations around.
const MODEL_RATES = {
  lite:    { name: 'claude-haiku-4.5 / gpt-4o-mini', inUsdPerM:  0.35, outUsdPerM: 1.40 },
  economy: { name: 'claude-sonnet-4.6',              inUsdPerM:  3.00, outUsdPerM: 15.00 },
  power:   { name: 'claude-opus-4.7',                inUsdPerM: 15.00, outUsdPerM: 75.00 },
} as const;

type Effort = keyof typeof MODEL_RATES;

const effortBlend = {
  lite:    { lite: 1.00, economy: 0.00, power: 0.00 },
  economy: { lite: 0.55, economy: 0.40, power: 0.05 },
  power:   { lite: 0.30, economy: 0.45, power: 0.25 },
};

// Plans the calculator can compare against.
const PLANS = [
  { name: 'Starter',    monthly: 0,   cap: 3   },
  { name: 'Pro',        monthly: 20,  cap: 15  },
  { name: 'Team (5)',   monthly: 200, cap: 80  },
  { name: 'Enterprise', monthly: null, cap: null },
];

// Heuristic: a "build session" averages ~120K input / ~30K output tokens
// across all gates (spec + ux + code + tests + security). Calibrated against
// real Ironflyer workspaces; the calculator multiplies by sessions/month.
const SESSION_INPUT_K  = 120;
const SESSION_OUTPUT_K = 30;

export function PricingCalculator() {
  const [sessions, setSessions] = useState(40);
  const [effort, setEffort]     = useState<Effort>('economy');
  const [billing, setBilling]   = useState<'monthly' | 'yearly'>('monthly');

  const monthlyProviderCost = useMemo(() => {
    const blend = effortBlend[effort];
    const inputTokensM  = (sessions * SESSION_INPUT_K)  / 1000;
    const outputTokensM = (sessions * SESSION_OUTPUT_K) / 1000;
    let cost = 0;
    (Object.keys(blend) as Effort[]).forEach((k) => {
      const r = MODEL_RATES[k];
      const share = blend[k];
      cost += inputTokensM  * share * r.inUsdPerM;
      cost += outputTokensM * share * r.outUsdPerM;
    });
    return cost;
  }, [effort, sessions]);

  const recommended = useMemo(() => {
    const fit = PLANS.find((p) => p.cap !== null && monthlyProviderCost <= p.cap);
    return fit ?? PLANS[PLANS.length - 1];
  }, [monthlyProviderCost]);

  const yearlyDiscount = 0.2;
  const subscriptionShown = (() => {
    const plan = PLANS.find((p) => p.name === recommended.name);
    if (!plan || plan.monthly === null) return null;
    return billing === 'yearly'
      ? Math.round(plan.monthly * (1 - yearlyDiscount))
      : plan.monthly;
  })();

  return (
    <Box sx={{
      borderRadius: { xs: 3, md: 5 },
      bgcolor: '#0d0e0f',
      color: tokens.color.bg.alabaster,
      overflow: 'hidden',
      border: '1px solid rgba(244,240,232,0.08)',
    }}>
      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', md: '1.05fr 0.95fr' },
        gap: 0,
      }}>
        {/* Inputs */}
        <Box sx={{ p: { xs: 3, md: 4.5 }, borderRight: { md: '1px solid rgba(244,240,232,0.08)' } }}>
          <Typography variant="overline" sx={{ color: tokens.color.accent.lime, fontWeight: 900, letterSpacing: '0.14em' }}>
            Your usage
          </Typography>
          <Typography sx={{ mt: 0.8, fontFamily: tokens.font.display, fontWeight: 400, fontSize: { xs: '1.9rem', md: '2.4rem' }, lineHeight: 1 }}>
            How much will you actually build?
          </Typography>

          <Box sx={{ mt: 4 }}>
            <Stack direction="row" justifyContent="space-between" alignItems="baseline">
              <Typography variant="body2" sx={{ color: '#cfc7b8', fontWeight: 700 }}>
                Build sessions / month
              </Typography>
              <Typography sx={{ fontFamily: tokens.font.mono, color: tokens.color.accent.lime, fontWeight: 800, fontSize: 18 }}>
                {sessions}
              </Typography>
            </Stack>
            <Slider
              value={sessions}
              min={5}
              max={300}
              step={5}
              onChange={(_, v) => setSessions(v as number)}
              sx={{
                mt: 1.2,
                color: tokens.color.accent.lime,
                '& .MuiSlider-rail':  { color: 'rgba(244,240,232,0.16)' },
                '& .MuiSlider-thumb': { boxShadow: '0 4px 12px rgba(229,255,0,0.32)' },
              }}
            />
            <Typography variant="caption" sx={{ color: '#9c968a' }}>
              A session ≈ one prompt that runs through the gates. Most builders use 20–80 / month.
            </Typography>
          </Box>

          <Box sx={{ mt: 4 }}>
            <Typography variant="body2" sx={{ color: '#cfc7b8', fontWeight: 700, mb: 1.2 }}>
              Effort dial
            </Typography>
            <ToggleButtonGroup
              value={effort}
              exclusive
              onChange={(_, v) => v && setEffort(v as Effort)}
              sx={{
                width: '100%',
                bgcolor: 'rgba(244,240,232,0.04)',
                borderRadius: '12px',
                p: 0.4,
                gap: 0.4,
                '& .MuiToggleButton-root': {
                  flex: 1,
                  border: 'none',
                  color: '#cfc7b8',
                  fontWeight: 800,
                  borderRadius: '8px',
                  py: 1,
                  '&.Mui-selected': {
                    bgcolor: tokens.color.accent.lime,
                    color: '#0d0e0f',
                    '&:hover': { bgcolor: tokens.color.accent.lime },
                  },
                  '&:hover': { bgcolor: 'rgba(244,240,232,0.06)' },
                },
              }}
            >
              <ToggleButton value="lite">Lite</ToggleButton>
              <ToggleButton value="economy">Economy</ToggleButton>
              <ToggleButton value="power">Power</ToggleButton>
            </ToggleButtonGroup>
            <Typography variant="caption" sx={{ mt: 1.2, color: '#9c968a', display: 'block' }}>
              The router blends Haiku, Sonnet, Opus by capability. Lite leans Haiku; Power pushes the harder turns to Opus.
            </Typography>
          </Box>

          <Box sx={{ mt: 4 }}>
            <Typography variant="body2" sx={{ color: '#cfc7b8', fontWeight: 700, mb: 1.2 }}>
              Billing
            </Typography>
            <ToggleButtonGroup
              value={billing}
              exclusive
              onChange={(_, v) => v && setBilling(v as 'monthly' | 'yearly')}
              sx={{
                bgcolor: 'rgba(244,240,232,0.04)',
                borderRadius: '12px',
                p: 0.4,
                gap: 0.4,
                '& .MuiToggleButton-root': {
                  border: 'none',
                  color: '#cfc7b8',
                  fontWeight: 800,
                  borderRadius: '8px',
                  px: 2.4, py: 0.9,
                  '&.Mui-selected': {
                    bgcolor: tokens.color.accent.lime,
                    color: '#0d0e0f',
                    '&:hover': { bgcolor: tokens.color.accent.lime },
                  },
                },
              }}
            >
              <ToggleButton value="monthly">Monthly</ToggleButton>
              <ToggleButton value="yearly">Yearly · save 20%</ToggleButton>
            </ToggleButtonGroup>
          </Box>
        </Box>

        {/* Output */}
        <Box sx={{ p: { xs: 3, md: 4.5 }, bgcolor: '#15161a', position: 'relative' }}>
          <Box sx={{
            position: 'absolute',
            inset: '0 0 auto auto',
            width: 260, height: 260,
            background: `radial-gradient(circle, rgba(229,255,0,0.16), transparent 60%)`,
            pointerEvents: 'none',
          }} />
          <Typography variant="overline" sx={{ color: tokens.color.accent.lime, fontWeight: 900, letterSpacing: '0.14em', position: 'relative' }}>
            Estimated monthly cost
          </Typography>
          <Stack direction="row" alignItems="baseline" spacing={1.2} sx={{ mt: 1, position: 'relative' }}>
            <Typography sx={{ fontFamily: tokens.font.display, fontWeight: 400, fontSize: { xs: '3.8rem', md: '5.2rem' }, lineHeight: 1, color: tokens.color.bg.alabaster }}>
              ${monthlyProviderCost.toFixed(2)}
            </Typography>
            <Typography variant="caption" sx={{ color: '#9c968a', fontWeight: 700 }}>provider cost</Typography>
          </Stack>
          <Typography variant="body2" sx={{ mt: 2, color: '#cfc7b8', maxWidth: 420, position: 'relative' }}>
            Based on rough Anthropic + OpenAI rates and a {SESSION_INPUT_K}K in / {SESSION_OUTPUT_K}K out average session.
          </Typography>

          <Box sx={{
            mt: 4,
            borderRadius: 3,
            bgcolor: 'rgba(244,240,232,0.04)',
            border: `1px solid rgba(229,255,0,0.18)`,
            p: 3,
            position: 'relative',
          }}>
            <Stack direction="row" alignItems="center" spacing={1.2}>
              <Bolt sx={{ color: tokens.color.accent.lime, fontSize: 22 }} />
              <Typography variant="overline" sx={{ color: tokens.color.accent.lime, fontWeight: 900, letterSpacing: '0.14em' }}>
                Recommended plan
              </Typography>
            </Stack>
            <Stack direction="row" alignItems="baseline" spacing={1.2} sx={{ mt: 1 }}>
              <Typography sx={{ fontFamily: tokens.font.display, fontWeight: 400, fontSize: 36, lineHeight: 1, color: tokens.color.bg.alabaster }}>
                {recommended.name}
              </Typography>
              {subscriptionShown !== null && (
                <Typography variant="body2" sx={{ color: '#cfc7b8', fontWeight: 800 }}>
                  ${subscriptionShown} / mo
                </Typography>
              )}
            </Stack>
            {recommended.cap !== null && (
              <Stack direction="row" spacing={1} sx={{ mt: 2 }} flexWrap="wrap" useFlexGap>
                <Chip
                  size="small"
                  label={`Cost cap $${recommended.cap}`}
                  sx={{ bgcolor: 'rgba(229,255,0,0.16)', color: tokens.color.accent.lime, fontWeight: 800, borderRadius: '999px' }}
                />
                <Chip
                  size="small"
                  icon={<TrendingUp sx={{ fontSize: 14 }} />}
                  label={`Your margin: $${subscriptionShown !== null ? (subscriptionShown - monthlyProviderCost).toFixed(2) : '—'}/mo (company)`}
                  sx={{ bgcolor: 'rgba(244,240,232,0.08)', color: tokens.color.bg.alabaster, fontWeight: 800, borderRadius: '999px' }}
                />
              </Stack>
            )}
            <Typography variant="caption" sx={{ mt: 2, color: '#9c968a', display: 'block', lineHeight: 1.5 }}>
              When your projected provider cost approaches the cap, the router automatically downgrades to cheaper models — so you stay inside the cap without changing how you prompt.
            </Typography>
          </Box>

          <Box sx={{ mt: 3, position: 'relative' }}>
            <Typography variant="caption" sx={{ color: '#9c968a', textTransform: 'uppercase', letterSpacing: '0.14em' }}>
              Routing mix
            </Typography>
            <Stack spacing={1} sx={{ mt: 1.4 }}>
              {(Object.keys(effortBlend[effort]) as Effort[]).map((k) => {
                const pct = Math.round(effortBlend[effort][k] * 100);
                if (pct === 0) return null;
                return (
                  <Box key={k}>
                    <Stack direction="row" justifyContent="space-between" sx={{ mb: 0.5 }}>
                      <Typography variant="caption" sx={{ color: tokens.color.bg.alabaster, fontFamily: tokens.font.mono, fontSize: 12 }}>{MODEL_RATES[k].name}</Typography>
                      <Typography variant="caption" sx={{ color: '#cfc7b8', fontFamily: tokens.font.mono, fontSize: 12 }}>{pct}%</Typography>
                    </Stack>
                    <Box sx={{ height: 5, borderRadius: 999, bgcolor: 'rgba(244,240,232,0.08)', overflow: 'hidden' }}>
                      <Box sx={{ width: `${pct}%`, height: '100%', bgcolor: tokens.color.accent.lime }} />
                    </Box>
                  </Box>
                );
              })}
            </Stack>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
