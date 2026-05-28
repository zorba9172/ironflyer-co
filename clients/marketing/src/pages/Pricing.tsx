import { Box, Button, Card, Container, Stack, Typography } from '@mui/material';
import { Head } from 'vite-react-ssg';
import { Reveal } from '@ironflyer/ui-web/motion';
import { Eyebrow } from '../components/text';

const tiers = [
  { name: 'Builder', price: '$0', cadence: 'to start', line: 'For the first build you want to take seriously.', features: ['One project', 'All six finisher gates', 'Preview deploys', 'Usage-based agent runs', 'Community support'], cta: 'Start free', featured: false },
  { name: 'Pro', price: '$39', cadence: 'per month', line: 'For shipping real products to real users.', features: ['Unlimited projects', 'Production deploys on your domain', 'Mobile (iOS + Android) target', 'Spend & error board', 'Priority agent throughput'], cta: 'Start 14-day trial', featured: true },
  { name: 'Studio', price: "Let's talk", cadence: 'for teams', line: 'For teams running several products at once.', features: ['Everything in Pro', 'Shared workspaces & roles', 'Backoffice access', 'SSO & audit log', 'Spend controls per project'], cta: 'Contact us', featured: false },
];

const faqs = [
  { q: 'Do I have to start from scratch?', a: 'No. Import an existing build from Lovable, Bolt, v0, Cursor, or any Git repo. Ironflyer is built to finish what you already have.' },
  { q: 'How does usage billing work?', a: 'Agent runs and deploys are metered against a wallet you top up. You see provider cost next to what you spend, so nothing is a surprise.' },
  { q: 'Who owns the code?', a: 'You do. Export the repo any time. Ironflyer is a finisher, not a lock-in.' },
  { q: 'Is the deploy really mine?', a: 'Yes — your domain, your environment variables, your database. We do not hold your production hostage.' },
];

export function Pricing() {
  return (
    <>
      <Head>
        <title>Pricing — Ironflyer</title>
        <meta name="description" content="Start free, ship on Pro, scale on Studio. Usage-based agent runs, deploys on a domain you own, and spend you can actually see." />
      </Head>

      <Container maxWidth="lg" sx={{ pt: { xs: 11, md: 14 }, pb: 3 }}>
        <Eyebrow>Pricing</Eyebrow>
        <Typography variant="h1" sx={{ fontSize: { xs: '2.5rem', md: '4rem' }, my: 2 }}>Pay to ship, not to demo.</Typography>
        <Typography sx={{ color: 'text.secondary', fontSize: '1.12rem', lineHeight: 1.6, maxWidth: '62ch' }}>
          Start free. When a build becomes a product, the price tracks what you actually use — and you can see the provider cost behind every run.
        </Typography>
      </Container>

      <Container maxWidth="lg" sx={{ py: 8 }}>
        <Reveal>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 2.5, alignItems: 'start' }}>
          {tiers.map((t) => (
            <Card key={t.name} sx={(th) => ({ position: 'relative', p: 4, ...(t.featured ? { boxShadow: `0 0 0 1.5px ${th.palette.primary.main}`, border: 'none' } : {}) })}>
              {t.featured && (
                <Box sx={(th) => ({ position: 'absolute', top: -11, left: 28, fontFamily: th.brand.font.mono, fontSize: '0.68rem', letterSpacing: '0.08em', textTransform: 'uppercase', px: 1.25, py: 0.6, borderRadius: 99, color: 'primary.contrastText', backgroundImage: th.brand.gradient.signature })}>Most teams pick this</Box>
              )}
              <Typography variant="h3" sx={{ fontSize: '1.4rem' }}>{t.name}</Typography>
              <Stack direction="row" alignItems="baseline" spacing={1} sx={{ mt: 1.75, mb: 0.75 }}>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.display, fontSize: '2.6rem', fontWeight: 700 })}>{t.price}</Typography>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, color: 'text.disabled', fontSize: '0.85rem' })}>{t.cadence}</Typography>
              </Stack>
              <Typography sx={{ color: 'text.secondary', minHeight: '3em' }}>{t.line}</Typography>
              <Stack spacing={1.5} sx={{ my: 2.75 }}>
                {t.features.map((f) => (
                  <Stack key={f} direction="row" spacing={1.25} alignItems="flex-start">
                    <Box sx={{ mt: '7px', width: 14, height: 8, borderLeft: 2, borderBottom: 2, borderColor: 'secondary.main', transform: 'rotate(-45deg)', flexShrink: 0 }} />
                    <Typography sx={{ color: 'text.secondary', fontSize: '0.95rem' }}>{f}</Typography>
                  </Stack>
                ))}
              </Stack>
              <Button fullWidth variant={t.featured ? 'contained' : 'outlined'} color={t.featured ? 'primary' : 'inherit'} href="https://app.ironflyer.com/start">{t.cta}</Button>
            </Card>
          ))}
        </Box>
        </Reveal>
      </Container>

      <Container maxWidth="lg" sx={{ py: 8 }}>
        <Reveal>
        <Box sx={{ mb: 5 }}>
          <Eyebrow>Questions</Eyebrow>
          <Typography variant="h2" sx={{ fontSize: { xs: '2rem', md: '2.75rem' }, mt: 1.75 }}>The honest answers.</Typography>
        </Box>
        <Stack spacing={1.5} sx={{ maxWidth: 760 }}>
          {faqs.map((f) => (
            <Card key={f.q} component="details" sx={{ p: 2.5, '& summary': { cursor: 'pointer', fontWeight: 600, listStyle: 'none' }, '& summary::-webkit-details-marker': { display: 'none' } }}>
              <Box component="summary">{f.q}</Box>
              <Typography sx={{ mt: 1.5, color: 'text.secondary', lineHeight: 1.6 }}>{f.a}</Typography>
            </Card>
          ))}
        </Stack>
        </Reveal>
      </Container>
    </>
  );
}
