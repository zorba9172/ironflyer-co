import { Box, Button, Card, Container, Stack, Typography } from '@mui/material';
import { Head } from 'vite-react-ssg';
import { Reveal } from '@ironflyer/ui-web/motion';
import { Eyebrow, GradientText } from '../components/text';

const entryCards = [
  { kicker: 'Economic enforcement', title: 'No surprise bills, ever', body: 'A prepaid wallet, live cost on every run, and a per-execution margin ledger. ProfitGuard gates each expensive call by expected ROI — so a feature never quietly loses money.' },
  { kicker: 'Finisher gates that block', title: 'Broken work cannot ship', body: 'Spec, build, test, security, and deploy gates refuse to pass insecure or unfinished code. Every gate verdict names what is unclosed and attaches the patch. Closed means closed.' },
  { kicker: 'Ship for real', title: 'Deploy on a domain you own', body: 'A real Docker workspace, real secrets, real billing wired in, an owner check on every resource. The thing you press deploy on is the thing your users get.' },
];

const stats = [
  { n: '6', label: 'finisher gates that block fake-shipping' },
  { n: '$0', label: 'surprise bills — every call gated by ProfitGuard ROI' },
  { n: '1', label: 'append-only ledger: provider cost vs. what you charge' },
  { n: '0', label: 'exposed DBs or reversed auth shipped by default' },
];

const gates = [
  { name: 'Identity', desc: 'Sessions, roles, and an owner check on every resource — not a login form that talks to nothing.' },
  { name: 'Money', desc: 'Prepaid wallet, real prices, real webhooks, and a ledger that reconciles to the cent.' },
  { name: 'Data', desc: 'Migrations, backups, and a schema that survives the second release.' },
  { name: 'Security', desc: 'Secrets out of source, a signed SBOM, scoped access, and a policy that says no by default.' },
  { name: 'Deploy', desc: 'Your domain, your environment, rollbacks that work at 2am.' },
  { name: 'Signal', desc: 'Errors, live spend, and margin on one board so you know before your users do.' },
];

export function Home() {
  return (
    <>
      <Head>
        <title>Ironflyer — the AI execution engine that enforces cost and blocks broken shipping</title>
        <meta name="description" content="Every other AI builder sells fast code and leaves you the bill. Ironflyer adds the two things they skip: economic enforcement (prepaid wallet, live cost, ProfitGuard) and finisher gates that refuse to ship insecure or unfinished work." />
      </Head>

      {/* Hero */}
      <Box sx={{ position: 'relative', overflow: 'hidden', pt: { xs: 12, md: 15 }, pb: 8 }}>
        <Box sx={(t) => ({ position: 'absolute', inset: '-40% 0 auto 0', height: 600, background: `radial-gradient(60% 60% at 50% 0%, ${t.brand.accent.primary}2e, transparent 70%)`, pointerEvents: 'none' })} />
        <Container maxWidth="lg" sx={{ position: 'relative' }}>
          <Eyebrow>AI Product Finisher</Eyebrow>
          <Typography variant="h1" sx={{ fontSize: { xs: '2.75rem', md: '4.5rem' }, mt: 2.5 }}>
            They generate fast.<br />
            <GradientText>Ironflyer ships profitably.</GradientText>
          </Typography>
          <Typography sx={{ mt: 3.5, maxWidth: '56ch', fontSize: '1.18rem', color: 'text.secondary', lineHeight: 1.6 }}>
            Lovable, Bolt, v0, Cursor, Devin — they sell fast code and leave you the surprise bill and a demo that's quietly insecure. Ironflyer adds the two things they skip: a prepaid wallet with ProfitGuard on every expensive call, and finisher gates that refuse to ship broken or exposed work.
          </Typography>
          <Stack direction="row" spacing={1.75} sx={{ mt: 4.5, flexWrap: 'wrap' }}>
            <Button variant="contained" size="large" href="https://app.ironflyer.com/start">Start building</Button>
            <Button variant="outlined" size="large" color="inherit" href="/product">See how it works →</Button>
          </Stack>
          <Typography sx={(t) => ({ mt: 2.25, fontFamily: t.brand.font.mono, fontSize: '0.8rem', color: 'text.disabled' })}>
            Prepaid wallet, no surprise bills. Bring an existing build or start clean.
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
          <Typography variant="h2" sx={{ fontSize: { xs: '2rem', md: '2.75rem' }, my: 2 }}>Gates that block, not vibes that pass.</Typography>
          <Typography sx={{ color: 'text.secondary', fontSize: '1.1rem', lineHeight: 1.6 }}>
            Every project moves through the same six gates. A failed gate verdict blocks the ship and names exactly what's unclosed — then hands you the patch. Other tools call a prototype done; Ironflyer refuses until it actually is.
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
          “The demo took an afternoon. Then came the $900 token bill and the database anyone could read — the part the generator never warned me about.”
        </Typography>
        <Typography sx={(t) => ({ mt: 2.5, fontFamily: t.brand.font.mono, color: 'text.disabled' })}>— every founder who shipped without enforcement</Typography>
      </Container>

      {/* Final CTA */}
      <Container maxWidth="lg" sx={{ py: 9 }}>
        <Reveal>
        <Card sx={{ textAlign: 'center', py: 9, px: 4 }}>
          <Typography variant="h2" sx={{ fontSize: { xs: '2rem', md: '3rem' } }}>Stop guessing the bill. Start shipping.</Typography>
          <Typography sx={{ mt: 2, mb: 4, color: 'text.secondary', fontSize: '1.1rem' }}>Point Ironflyer at your build and watch the gate verdicts and the cost ledger light up.</Typography>
          <Button variant="contained" size="large" href="https://app.ironflyer.com/start">Start building</Button>
        </Card>
        </Reveal>
      </Container>
    </>
  );
}
