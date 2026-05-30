import { useMemo, useState } from 'react';
import { Box, Chip, Stack, Typography } from '@mui/material';
import { LuSparkles } from 'react-icons/lu';
import { toast } from '@ironflyer/ui-web/fx';
import { formatUSD } from '@ironflyer/core';
import { useWallet } from '../hooks/useEconomics';
import { AmbientBackdrop } from './home/AmbientBackdrop';
import { BillingToggle, type BillingCadence } from './plans/BillingToggle';
import { PlanCard, type PlanTier } from './plans/PlanCard';
import { WalletPanel } from './plans/WalletPanel';

// In-app plans. Our model is prepaid wallet credits (not seats): a plan tops
// the wallet monthly; ProfitGuard meters every run against it. `priceUSD` is
// the monthly list price; the annual cadence applies the shared discount and
// the cards display the resulting effective monthly figure.
const tiers = [
  { name: 'Starter', priceUSD: 0, credits: '100 credits / mo', popular: false, features: ['1 project', 'All finisher gates', 'Preview deploys', 'Community support'] },
  { name: 'Builder', priceUSD: 19, credits: '500 credits / mo', popular: true, features: ['Unlimited projects', 'Production deploys', 'Mobile target', 'Spend & error board'] },
  { name: 'Pro', priceUSD: 49, credits: '1,500 credits / mo', popular: false, features: ['Everything in Builder', 'Priority agent throughput', 'Custom domain', 'Remove branding'] },
  { name: 'Elite', priceUSD: 149, credits: '5,000 credits / mo', popular: false, features: ['Everything in Pro', 'Shared workspaces & roles', 'SSO & audit log', 'Per-project spend controls'] },
];

const topUps = [25, 75, 150];

// Annual cadence trades a discount for commitment — the highest-ROI lever on a
// pricing page. The effective monthly price is shown so the cards stay scannable.
const ANNUAL_SAVINGS_PCT = 17;

export function PlansPage() {
  const { wallet, isLive } = useWallet();
  const [cadence, setCadence] = useState<BillingCadence>('annual');

  const cards: PlanTier[] = useMemo(() => {
    const annual = cadence === 'annual';
    return tiers.map((t) => {
      const effective = annual ? Math.round(t.priceUSD * (1 - ANNUAL_SAVINGS_PCT / 100)) : t.priceUSD;
      return {
        name: t.name,
        price: formatUSD(effective, { cents: false }),
        cadenceLabel: t.priceUSD === 0 ? 'forever' : annual ? '/mo billed annually' : '/mo',
        credits: t.credits,
        popular: t.popular,
        features: t.features,
      };
    });
  }, [cadence]);

  return (
    <Box sx={{ position: 'relative', overflow: 'hidden' }}>
      <AmbientBackdrop />

      <Box sx={{ position: 'relative', zIndex: 1, px: { xs: 3, md: 6 }, py: { xs: 5, md: 8 }, maxWidth: 1240, mx: 'auto' }}>
        {/* Headline — final phrase gradient-filled, per the locked formula. */}
        <Stack alignItems="center" textAlign="center" spacing={2.5}>
          <Chip
            icon={<LuSparkles size={15} />}
            label="Prepaid wallet credits — no seats, no surprises"
            sx={(theme) => ({
              height: 34,
              px: 1.25,
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

          <Typography variant="h2" sx={{ fontSize: { xs: '2.2rem', md: '3rem' }, maxWidth: 760, lineHeight: 1.08 }}>
            Fund your build wallet{' '}
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
              and ship without surprises.
            </Box>
          </Typography>

          <Typography color="text.secondary" sx={{ maxWidth: 600, fontSize: { xs: '1rem', md: '1.075rem' }, lineHeight: 1.5 }}>
            Plans top up prepaid credits each month. Paid runs reserve first, then debit exactly what they use — so you never pay for a half-finished build.
          </Typography>

          <Box sx={{ mt: 1 }}>
            <BillingToggle value={cadence} onChange={setCadence} savingsPct={ANNUAL_SAVINGS_PCT} />
          </Box>
        </Stack>

        {/* Pricing grid — recommended tier highlighted with the gradient ring. */}
        <Box
          sx={{
            mt: { xs: 4, md: 6 },
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(4, 1fr)' },
            gap: 2.5,
            alignItems: 'stretch',
          }}
        >
          {cards.map((tier) => (
            <PlanCard
              key={tier.name}
              tier={tier}
              onSelect={(name) => toast(`${name} selected — opening secure checkout…`, 'success')}
            />
          ))}
        </Box>

        {/* Live wallet top-up — viz-first balance mirror + quick amounts. */}
        <Box sx={{ mt: { xs: 4, md: 6 } }}>
          <WalletPanel
            wallet={wallet}
            isLive={isLive}
            topUps={topUps}
            recommended={75}
            formatUSD={formatUSD}
            onTopUp={(amount) => toast(`${formatUSD(amount, { cents: false })} wallet top-up selected - opening secure checkout...`, 'success')}
          />
        </Box>
      </Box>
    </Box>
  );
}
