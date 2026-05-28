import { Box, Button, Card, Container, Stack, Typography } from '@mui/material';
import { Head } from 'vite-react-ssg';
import { Reveal } from '@ironflyer/ui-web/motion';
import { Eyebrow, GradientText } from '../components/text';

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

const scanners = [
  { name: 'Secrets', desc: 'Every commit scanned for keys, tokens, and credentials left in source — caught before they reach a remote.' },
  { name: 'Dependencies', desc: 'A signed SBOM per build, with known-vulnerable packages flagged against the advisories that matter to you.' },
  { name: 'SAST', desc: 'Static analysis on your code paths — injection, auth bypass, unsafe deserialization — not a generic checklist.' },
  { name: 'Containers', desc: 'Image layers and base OS scanned, so what you ship to production carries no surprise CVEs.' },
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
        <Reveal>
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
        </Reveal>
      </Container>

      {blocks.map((b, i) => (
        <Container key={b.tag} maxWidth="lg" sx={{ py: 8 }}>
          <Reveal>
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
          </Reveal>
        </Container>
      ))}

      {/* Security — the AppSec moat */}
      <Box sx={{ borderBlock: 1, borderColor: 'divider', py: { xs: 8, md: 10 } }}>
        <Container maxWidth="lg">
          <Reveal>
          <Box sx={{ maxWidth: '64ch', mb: 6 }}>
            <Eyebrow>Security</Eyebrow>
            <Typography variant="h2" sx={{ fontSize: { xs: '2rem', md: '2.75rem' }, my: 2 }}>
              Real AppSec, not a <GradientText>security</GradientText> sticker.
            </Typography>
            <Typography sx={{ color: 'text.secondary', fontSize: '1.12rem', lineHeight: 1.6 }}>
              Generators hand you a prototype and call it done. Ironflyer runs the scanners a security team would run, behind a policy plane that refuses anything it wasn't told to allow. The findings are yours to export — they don't just live in our dashboard.
            </Typography>
          </Box>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: '1px', bgcolor: 'divider', border: 1, borderColor: 'divider', borderRadius: 4, overflow: 'hidden' }}>
            {scanners.map((s) => (
              <Box key={s.name} sx={{ bgcolor: 'background.paper', p: 3.5 }}>
                <Typography variant="h3" sx={{ fontSize: '1.2rem', mb: 1.25 }}>{s.name}</Typography>
                <Typography sx={{ color: 'text.secondary', lineHeight: 1.6, fontSize: '0.95rem' }}>{s.desc}</Typography>
              </Box>
            ))}
          </Box>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 2.5, mt: 2.5 }}>
            <Card sx={{ p: 3.5 }}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, color: 'primary.main', fontSize: '0.8rem', textTransform: 'uppercase', letterSpacing: '0.1em' })}>Deny by default</Typography>
              <Typography variant="h3" sx={{ fontSize: '1.2rem', mt: 1.5, mb: 1.25 }}>The policy plane says no until you say yes</Typography>
              <Typography sx={{ color: 'text.secondary', lineHeight: 1.6, fontSize: '0.95rem' }}>
                Outbound calls, new dependencies, and privileged operations are blocked unless a policy permits them. The AI works inside that boundary — it cannot widen its own access.
              </Typography>
            </Card>
            <Card sx={{ p: 3.5 }}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, color: 'primary.main', fontSize: '0.8rem', textTransform: 'uppercase', letterSpacing: '0.1em' })}>Portable evidence</Typography>
              <Typography variant="h3" sx={{ fontSize: '1.2rem', mt: 1.5, mb: 1.25 }}>SARIF and SBOM you can take anywhere</Typography>
              <Typography sx={{ color: 'text.secondary', lineHeight: 1.6, fontSize: '0.95rem' }}>
                Scan results export as standard SARIF; your bill of materials exports as SPDX/CycloneDX. Drop them into your own CI, your auditor's review, or your customer's security questionnaire.
              </Typography>
            </Card>
          </Box>
          </Reveal>
        </Container>
      </Box>

      <Container maxWidth="lg" sx={{ py: 9, textAlign: 'center' }}>
        <Reveal>
        <Typography variant="h2" sx={{ fontSize: { xs: '1.8rem', md: '2.6rem' }, mb: 3.5 }}>See it on your own build.</Typography>
        <Stack direction="row" justifyContent="center">
          <Button variant="contained" size="large" href="https://app.ironflyer.com/start">Start building</Button>
        </Stack>
        </Reveal>
      </Container>
    </>
  );
}
