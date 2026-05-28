import { Box, Button, Card, Container, Stack, Typography } from '@mui/material';
import { Head } from 'vite-react-ssg';
import { Eyebrow } from '../components/text';

const flow = [
  { step: '01', title: 'Connect a build', body: 'Paste a repo URL or import from Lovable, Bolt, v0, or Cursor. Ironflyer clones it into a sandbox and runs it — no local setup.' },
  { step: '02', title: 'Read the gaps', body: 'A first pass maps every route, model, and integration against the six gates, and tells you what is mocked, missing, or unsafe.' },
  { step: '03', title: 'Close gates with the agent', body: 'Pick a gate. The agent proposes patches you can read, edit, and apply — running against your real workspace, not a guess.' },
  { step: '04', title: 'Deploy and watch', body: 'Ship to a domain you own. Spend, errors, and usage land on one board the moment traffic starts.' },
];

const blocks = [
  { tag: 'Deploy', h: 'Environments that match production', p: 'Staging and production with real secrets, managed migrations, and one-click rollback. The preview link is the real stack, scaled down — not a different codebase that drifts.' },
  { tag: 'Mobile', h: 'One product, every screen', p: 'The same build targets web and a native iOS + Android app. Shared design tokens and data layer mean a fix lands everywhere at once, not three times.' },
  { tag: 'Budget', h: 'It pays for itself, on purpose', p: 'Every run is metered against your wallet. You see provider cost versus what you charge, per project, so a feature is never quietly losing money.' },
];

export function Product() {
  return (
    <>
      <Head>
        <title>Product — Ironflyer</title>
        <meta name="description" content="How Ironflyer turns an AI prototype into a product you can charge for: six finisher gates, real deploys, and a single board for spend and signal." />
      </Head>

      <Container maxWidth="lg" sx={{ pt: { xs: 11, md: 14 }, pb: 4 }}>
        <Eyebrow>Product</Eyebrow>
        <Typography variant="h1" sx={{ fontSize: { xs: '2.5rem', md: '4rem' }, my: 2, maxWidth: '18ch' }}>From a prototype that demos to a product that bills.</Typography>
        <Typography sx={{ color: 'text.secondary', fontSize: '1.12rem', lineHeight: 1.6, maxWidth: '62ch' }}>
          Ironflyer is the layer between "the AI made something" and "people pay for it." It reads your build, names what's unfinished, and closes it with you — gate by gate.
        </Typography>
      </Container>

      <Container maxWidth="lg" sx={{ py: 8 }}>
        <Box sx={{ display: 'grid', gap: '1px', bgcolor: 'divider', border: 1, borderColor: 'divider', borderRadius: 4, overflow: 'hidden' }}>
          {flow.map((f) => (
            <Box key={f.step} sx={{ bgcolor: 'background.paper', display: 'grid', gridTemplateColumns: { xs: '48px 1fr', md: '64px 1fr' }, gap: 3, p: { xs: 3, md: 4 } }}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, color: 'primary.main', fontSize: '1.1rem' })}>{f.step}</Typography>
              <Box>
                <Typography variant="h3" sx={{ fontSize: '1.3rem', mb: 1 }}>{f.title}</Typography>
                <Typography sx={{ color: 'text.secondary', lineHeight: 1.6, maxWidth: '70ch' }}>{f.body}</Typography>
              </Box>
            </Box>
          ))}
        </Box>
      </Container>

      {blocks.map((b, i) => (
        <Container key={b.tag} maxWidth="lg" sx={{ py: 8 }}>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 7, alignItems: 'center' }}>
            <Box sx={{ order: { md: i % 2 === 1 ? 2 : 0 } }}>
              <Eyebrow>{b.tag}</Eyebrow>
              <Typography variant="h2" sx={{ fontSize: { xs: '1.8rem', md: '2.6rem' }, my: 2 }}>{b.h}</Typography>
              <Typography sx={{ color: 'text.secondary', fontSize: '1.12rem', lineHeight: 1.6 }}>{b.p}</Typography>
            </Box>
            <Card sx={(t) => ({ aspectRatio: '4 / 3', borderRadius: 5, backgroundImage: t.brand.gradient.signatureSoft, display: 'grid', placeItems: 'center' })}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, color: 'text.disabled', letterSpacing: '0.2em', textTransform: 'uppercase', fontSize: '0.8rem' })}>{b.tag}</Typography>
            </Card>
          </Box>
        </Container>
      ))}

      <Container maxWidth="lg" sx={{ py: 9, textAlign: 'center' }}>
        <Typography variant="h2" sx={{ fontSize: { xs: '1.8rem', md: '2.6rem' }, mb: 3.5 }}>See it on your own build.</Typography>
        <Stack direction="row" justifyContent="center">
          <Button variant="contained" size="large" href="https://app.ironflyer.com/start">Start building</Button>
        </Stack>
      </Container>
    </>
  );
}
