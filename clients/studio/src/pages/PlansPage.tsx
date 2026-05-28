import { Box, Button, Card, Chip, Stack, Typography } from '@mui/material';
import { toast } from '@ironflyer/ui-web/fx';

// In-app plans. Our model is prepaid wallet credits (not seats): a plan tops
// the wallet monthly; ProfitGuard meters every run against it.
const tiers = [
  { name: 'Starter', price: '$0', credits: '100 credits / mo', popular: false, features: ['1 project', 'All finisher gates', 'Preview deploys', 'Community support'] },
  { name: 'Builder', price: '$19', credits: '500 credits / mo', popular: true, features: ['Unlimited projects', 'Production deploys', 'Mobile target', 'Spend & error board'] },
  { name: 'Pro', price: '$49', credits: '1,500 credits / mo', popular: false, features: ['Everything in Builder', 'Priority agent throughput', 'Custom domain', 'Remove branding'] },
  { name: 'Elite', price: '$149', credits: '5,000 credits / mo', popular: false, features: ['Everything in Pro', 'Shared workspaces & roles', 'SSO & audit log', 'Per-project spend controls'] },
];

export function PlansPage() {
  return (
    <Box sx={{ p: { xs: 3, md: 6 }, maxWidth: 1180, mx: 'auto' }}>
      <Typography variant="h2" sx={{ fontSize: { xs: '2.25rem', md: '3rem' }, textAlign: 'center' }}>Choose the plan that's right for you</Typography>
      <Typography sx={{ color: 'text.secondary', textAlign: 'center', mt: 1.5, mb: 5 }}>
        Plans top your wallet in credits. You only ever spend on runs that pass ProfitGuard.
      </Typography>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(4, 1fr)' }, gap: 2, alignItems: 'start' }}>
        {tiers.map((t) => (
          <Card key={t.name} sx={(th) => ({ position: 'relative', p: 3, ...(t.popular ? { boxShadow: `0 0 0 1.5px ${th.palette.primary.main}`, border: 'none' } : {}) })}>
            {t.popular && (
              <Box sx={(th) => ({ position: 'absolute', top: -11, left: '50%', transform: 'translateX(-50%)', fontFamily: th.brand.font.mono, fontSize: '0.64rem', letterSpacing: '0.08em', textTransform: 'uppercase', px: 1.5, py: 0.5, borderRadius: 99, color: '#fff', backgroundImage: th.brand.gradient.signature })}>Most popular</Box>
            )}
            <Typography variant="h6" sx={{ fontSize: '1.2rem' }}>{t.name}</Typography>
            <Stack direction="row" alignItems="baseline" spacing={0.75} sx={{ mt: 1.5 }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.display, fontSize: '2.4rem', fontWeight: 700 })}>{t.price}</Typography>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, color: 'text.disabled', fontSize: '0.8rem' })}>/mo</Typography>
            </Stack>
            <Chip size="small" label={t.credits} sx={{ mt: 1, mb: 2, bgcolor: 'action.hover', fontFamily: 'var(--if-font-mono)', fontSize: '0.7rem' }} />
            <Button fullWidth variant={t.popular ? 'contained' : 'outlined'} color={t.popular ? 'primary' : 'inherit'} onClick={() => toast(`${t.name} selected — opening secure checkout…`, 'success')}>Get {t.name}</Button>
            <Stack spacing={1.25} sx={{ mt: 2.5 }}>
              {t.features.map((f) => (
                <Stack key={f} direction="row" spacing={1.25} alignItems="flex-start">
                  <Box sx={{ mt: '6px', width: 13, height: 7, borderLeft: 2, borderBottom: 2, borderColor: 'secondary.main', transform: 'rotate(-45deg)', flexShrink: 0 }} />
                  <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem' }}>{f}</Typography>
                </Stack>
              ))}
            </Stack>
          </Card>
        ))}
      </Box>
    </Box>
  );
}
