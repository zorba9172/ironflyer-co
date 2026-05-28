import { Box, Button, Card, Container, Stack, Typography } from '@mui/material';
import { Head } from 'vite-react-ssg';
import { Reveal } from '@ironflyer/ui-web/motion';
import { Eyebrow, GradientText } from '../components/text';

const entryCards = [
  { kicker: 'Bring your build', title: 'Start from what AI already made', body: 'Import a Lovable, Bolt, or v0 project — or your own repo. Ironflyer reads it and maps what works against what still has to ship.' },
  { kicker: 'Close the gaps', title: 'Walk the finisher gates', body: 'Auth, payments, data, security, deploy. Each gate is a checklist with the work attached, not a vague suggestion. Closed means closed.' },
  { kicker: 'Ship for real', title: 'Deploy on a domain you own', body: 'Real environments, real secrets, real billing wired in. The thing you press deploy on is the thing your users get.' },
];

const stats = [
  { n: '6', label: 'finisher gates from prototype to production' },
  { n: '80→100', label: 'percent — the last mile AI tools skip' },
  { n: '1', label: 'codebase: web, mobile, and backoffice' },
  { n: '0', label: 'config files you write by hand' },
];

const gates = [
  { name: 'Identity', desc: 'Sessions, roles, and ownership wired in — not a login form that talks to nothing.' },
  { name: 'Money', desc: 'Stripe and Paddle, real prices, real webhooks, reconciliation that balances.' },
  { name: 'Data', desc: 'Migrations, backups, and a schema that survives the second release.' },
  { name: 'Security', desc: 'Secrets out of source, scoped access, and a policy that says no by default.' },
  { name: 'Deploy', desc: 'Your domain, your environment, rollbacks that work at 2am.' },
  { name: 'Signal', desc: 'Errors, spend, and usage on one board so you know before your users do.' },
];

export function Home() {
  return (
    <>
      <Head>
        <title>Ironflyer — finish what your AI started</title>
        <meta name="description" content="AI gets a product to 80%. Ironflyer closes the other 20% — auth, payments, security, deploy — the part that actually ships." />
      </Head>

      {/* Hero */}
      <Box sx={{ position: 'relative', overflow: 'hidden', pt: { xs: 12, md: 15 }, pb: 8 }}>
        <Box sx={(t) => ({ position: 'absolute', inset: '-40% 0 auto 0', height: 600, background: `radial-gradient(60% 60% at 50% 0%, ${t.brand.accent.primary}2e, transparent 70%)`, pointerEvents: 'none' })} />
        <Container maxWidth="lg" sx={{ position: 'relative' }}>
          <Eyebrow>AI Product Finisher</Eyebrow>
          <Typography variant="h1" sx={{ fontSize: { xs: '2.75rem', md: '4.5rem' }, mt: 2.5 }}>
            AI gets you to eighty.<br />
            <GradientText>Ironflyer ships the rest.</GradientText>
          </Typography>
          <Typography sx={{ mt: 3.5, maxWidth: '56ch', fontSize: '1.18rem', color: 'text.secondary', lineHeight: 1.6 }}>
            Prompt-to-app tools hand you a demo that breaks the moment a real user touches it. Ironflyer takes that demo and closes the part nobody likes doing — auth, payments, security, deploy — until it's a product you can charge for.
          </Typography>
          <Stack direction="row" spacing={1.75} sx={{ mt: 4.5, flexWrap: 'wrap' }}>
            <Button variant="contained" size="large" href="https://app.ironflyer.com/start">Start building</Button>
            <Button variant="outlined" size="large" color="inherit" href="/product">See how it works →</Button>
          </Stack>
          <Typography sx={(t) => ({ mt: 2.25, fontFamily: t.brand.font.mono, fontSize: '0.8rem', color: 'text.disabled' })}>
            No credit card. Bring an existing build or start clean.
          </Typography>
        </Container>
      </Box>

      {/* Entry cards */}
      <Container maxWidth="lg" sx={{ py: 9 }}>
        <Reveal>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 2.5 }}>
          {entryCards.map((c) => (
            <Card key={c.title} sx={{ p: 3.5, transition: 'transform .2s ease, border-color .2s ease', '&:hover': { transform: 'translateY(-3px)', borderColor: 'divider' } }}>
              <Typography sx={{ fontSize: '0.72rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'secondary.main' }}>{c.kicker}</Typography>
              <Typography variant="h3" sx={{ fontSize: '1.3rem', mt: 1.75, mb: 1.25 }}>{c.title}</Typography>
              <Typography sx={{ color: 'text.secondary', lineHeight: 1.6 }}>{c.body}</Typography>
            </Card>
          ))}
        </Box>
        </Reveal>
      </Container>

      {/* Stats */}
      <Box sx={{ borderBlock: 1, borderColor: 'divider', py: 9 }}>
        <Container maxWidth="lg">
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 4 }}>
            {stats.map((s) => (
              <Box key={s.label}>
                <GradientText sx={(t) => ({ fontFamily: t.brand.font.display, fontSize: '2.75rem', fontWeight: 700, display: 'block' })}>{s.n}</GradientText>
                <Typography sx={{ mt: 1, color: 'text.disabled', fontSize: '0.9rem', maxWidth: '24ch' }}>{s.label}</Typography>
              </Box>
            ))}
          </Box>
        </Container>
      </Box>

      {/* Finisher gates */}
      <Container maxWidth="lg" sx={{ py: 9 }}>
        <Box sx={{ maxWidth: '60ch', mb: 6 }}>
          <Eyebrow>The finisher gates</Eyebrow>
          <Typography variant="h2" sx={{ fontSize: { xs: '2rem', md: '2.75rem' }, my: 2 }}>Done is a checklist, not a feeling.</Typography>
          <Typography sx={{ color: 'text.secondary', fontSize: '1.1rem', lineHeight: 1.6 }}>
            Every project moves through the same six gates. Each one names exactly what's unclosed and hands you the work to close it. You always know how far from shippable you are — to the gate.
          </Typography>
        </Box>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: 'repeat(3, 1fr)' }, gap: '1px', bgcolor: 'divider', border: 1, borderColor: 'divider', borderRadius: 4, overflow: 'hidden' }}>
          {gates.map((g, i) => (
            <Box key={g.name} sx={{ bgcolor: 'background.paper', p: 3.5 }}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, color: 'primary.main', fontSize: '0.8rem' })}>0{i + 1}</Typography>
              <Typography variant="h3" sx={{ fontSize: '1.25rem', mt: 1.5, mb: 1.25 }}>{g.name}</Typography>
              <Typography sx={{ color: 'text.secondary', lineHeight: 1.6, fontSize: '0.95rem' }}>{g.desc}</Typography>
            </Box>
          ))}
        </Box>
      </Container>

      {/* Quote */}
      <Container maxWidth="lg" sx={{ py: 9, textAlign: 'center' }}>
        <Typography variant="h2" sx={{ fontSize: { xs: '1.5rem', md: '2.4rem' }, fontWeight: 600, maxWidth: '22ch', mx: 'auto', lineHeight: 1.25 }}>
          “The demo took an afternoon. The last ten percent took three weeks — and that's the part our customers actually paid for.”
        </Typography>
        <Typography sx={(t) => ({ mt: 2.5, fontFamily: t.brand.font.mono, color: 'text.disabled' })}>— every founder who shipped without a finisher</Typography>
      </Container>

      {/* Final CTA */}
      <Container maxWidth="lg" sx={{ py: 9 }}>
        <Reveal>
        <Card sx={{ textAlign: 'center', py: 9, px: 4 }}>
          <Typography variant="h2" sx={{ fontSize: { xs: '2rem', md: '3rem' } }}>Stop demoing. Start shipping.</Typography>
          <Typography sx={{ mt: 2, mb: 4, color: 'text.secondary', fontSize: '1.1rem' }}>Point Ironflyer at your build and watch the gates light up.</Typography>
          <Button variant="contained" size="large" href="https://app.ironflyer.com/start">Start building</Button>
        </Card>
        </Reveal>
      </Container>
    </>
  );
}
