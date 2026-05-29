import { Box, Button, Card, Chip, Stack, Typography } from '@mui/material';
import { toast } from '@ironflyer/ui-web/fx';
import { formatUSD } from '@ironflyer/core';
import { text } from '@ironflyer/design-tokens/brand';
import { useWallet } from '../hooks/useEconomics';

// In-app plans. Our model is prepaid wallet credits (not seats): a plan tops
// the wallet monthly; ProfitGuard meters every run against it.
const tiers = [
  { name: 'Starter', price: '$0', credits: '100 credits / mo', popular: false, features: ['1 project', 'All finisher gates', 'Preview deploys', 'Community support'] },
  { name: 'Builder', price: '$19', credits: '500 credits / mo', popular: true, features: ['Unlimited projects', 'Production deploys', 'Mobile target', 'Spend & error board'] },
  { name: 'Pro', price: '$49', credits: '1,500 credits / mo', popular: false, features: ['Everything in Builder', 'Priority agent throughput', 'Custom domain', 'Remove branding'] },
  { name: 'Elite', price: '$149', credits: '5,000 credits / mo', popular: false, features: ['Everything in Pro', 'Shared workspaces & roles', 'SSO & audit log', 'Per-project spend controls'] },
];

const topUps = [25, 75, 150];

export function PlansPage() {
  const { wallet, isLive } = useWallet();

  return (
    <Box sx={{ p: { xs: 3, md: 6 }, maxWidth: 1180, mx: 'auto' }}>
      <Typography variant="h2" sx={{ fontSize: { xs: text.s225, md: text.s260 }, textAlign: 'center' }}>Fund your build wallet</Typography>
      <Typography sx={{ color: 'text.secondary', textAlign: 'center', mt: 1.5, mb: 5 }}>
        Plans top up prepaid credits. Paid runs reserve first, then debit exactly what they use.
      </Typography>

      <Card sx={{ p: { xs: 2, md: 2.5 }, mb: 3 }}>
        <Stack direction={{ xs: 'column', md: 'row' }} alignItems={{ xs: 'stretch', md: 'center' }} justifyContent="space-between" spacing={2}>
          <Stack spacing={0.5}>
            <Stack direction="row" spacing={1} alignItems="center">
              <Typography variant="h6" sx={{ fontSize: text.s110 }}>Wallet top-up</Typography>
              <Chip size="small" label={isLive ? 'live wallet' : 'offline preview'} sx={(th) => ({ height: 20, fontSize: text.s64, fontFamily: th.brand.font.mono, bgcolor: isLive ? `${th.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })} />
            </Stack>
            <Typography sx={{ color: 'text.secondary', fontSize: text.s84 }}>
              {isLive ? `${formatUSD(wallet.availableUSD)} available now. ` : 'Connect to see live balance. '}
              A 402 clears when the next reservation can be covered.
            </Typography>
          </Stack>
          <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', gap: 1 }}>
            {topUps.map((amount) => (
              <Button
                key={amount}
                variant={amount === 75 ? 'contained' : 'outlined'}
                color={amount === 75 ? 'primary' : 'inherit'}
                onClick={() => toast(`${formatUSD(amount, { cents: false })} wallet top-up selected - opening secure checkout...`, 'success')}
              >
                Add {formatUSD(amount, { cents: false })}
              </Button>
            ))}
          </Stack>
        </Stack>
      </Card>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(4, 1fr)' }, gap: 2, alignItems: 'start' }}>
        {tiers.map((t) => (
          <Card key={t.name} sx={(th) => ({ position: 'relative', p: 3, ...(t.popular ? { boxShadow: `0 0 0 1.5px ${th.palette.primary.main}`, border: 'none' } : {}) })}>
            {t.popular && (
              <Box sx={(th) => ({ position: 'absolute', top: -11, left: '50%', transform: 'translateX(-50%)', fontFamily: th.brand.font.mono, fontSize: text.s64, letterSpacing: '0.08em', textTransform: 'uppercase', px: 1.5, py: 0.5, borderRadius: 99, color: th.palette.primary.contrastText, backgroundImage: th.brand.gradient.signature })}>Most popular</Box>
            )}
            <Typography variant="h6" sx={{ fontSize: text.s120 }}>{t.name}</Typography>
            <Stack direction="row" alignItems="baseline" spacing={0.75} sx={{ mt: 1.5 }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.display, fontSize: text.s240, fontWeight: 700 })}>{t.price}</Typography>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, color: 'text.disabled', fontSize: text.s80 })}>/mo</Typography>
            </Stack>
            <Chip size="small" label={t.credits} sx={(th) => ({ mt: 1, mb: 2, bgcolor: 'action.hover', fontFamily: th.brand.font.mono, fontSize: text.s70 })} />
            <Button fullWidth variant={t.popular ? 'contained' : 'outlined'} color={t.popular ? 'primary' : 'inherit'} onClick={() => toast(`${t.name} selected — opening secure checkout…`, 'success')}>Get {t.name}</Button>
            <Stack spacing={1.25} sx={{ mt: 2.5 }}>
              {t.features.map((f) => (
                <Stack key={f} direction="row" spacing={1.25} alignItems="flex-start">
                  <Box sx={{ mt: 0.75, width: 13, height: 7, borderLeft: 2, borderBottom: 2, borderColor: 'secondary.main', transform: 'rotate(-45deg)', flexShrink: 0 }} />
                  <Typography sx={{ color: 'text.secondary', fontSize: text.s90 }}>{f}</Typography>
                </Stack>
              ))}
            </Stack>
          </Card>
        ))}
      </Box>
    </Box>
  );
}
