import { Box, Button, Card, Container, Stack, Typography } from '@mui/material';
import { Head } from 'vite-react-ssg';
import { Eyebrow, GradientText } from '../components/text';

const panels = [
  { name: 'The map', body: 'A live diagram of your product — routes, models, integrations — with every unclosed gate marked in place. You read state before you read code.' },
  { name: 'The agent', body: 'Describe the gap. It drafts the patch against your real workspace, shows the diff, and waits for you to apply. No silent edits.' },
  { name: 'The terminal', body: 'A real shell into the sandbox when you want it. The studio is visual first, but the floor is never locked to pros.' },
  { name: 'The board', body: 'Spend, errors, and traffic in one view, per project. The number you watch is the one that decides if you keep shipping.' },
];

export function Studio() {
  return (
    <>
      <Head>
        <title>Studio — Ironflyer</title>
        <meta name="description" content="The Ironflyer studio: a visual cockpit for AI-built products. See what's unfinished, close it with the agent, and ship — with the code editor one click away." />
      </Head>

      <Container maxWidth="lg" sx={{ pt: { xs: 11, md: 14 }, pb: 3 }}>
        <Eyebrow>Studio</Eyebrow>
        <Typography variant="h1" sx={{ fontSize: { xs: '2.5rem', md: '4rem' }, my: 2, maxWidth: '16ch' }}>
          A cockpit that shows you the <GradientText>state</GradientText>, not just the code.
        </Typography>
        <Typography sx={{ color: 'text.secondary', fontSize: '1.12rem', lineHeight: 1.6, maxWidth: '62ch' }}>
          Most tools drop you into a file tree and wish you luck. The Ironflyer studio opens on a picture of your whole product and names what's between you and shipping. Code is one click away when you want it — never the only way in.
        </Typography>
        <Stack direction="row" spacing={1.75} sx={{ mt: 3.75, flexWrap: 'wrap' }}>
          <Button variant="contained" size="large" href="https://app.ironflyer.com/start">Open the studio</Button>
          <Button variant="outlined" size="large" color="inherit" href="/product">How finishing works →</Button>
        </Stack>
      </Container>

      <Container maxWidth="lg" sx={{ py: 8 }}>
        <Card sx={{ overflow: 'hidden', boxShadow: (t) => t.brand.shadow.lg }}>
          <Stack direction="row" spacing={1} sx={{ px: 2.25, py: 1.75, borderBottom: 1, borderColor: 'divider' }}>
            {[0, 1, 2].map((i) => <Box key={i} sx={{ width: 11, height: 11, borderRadius: 99, bgcolor: 'divider' }} />)}
          </Stack>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '2.4fr 1fr' }, minHeight: 360 }}>
            <Box sx={(t) => ({ p: 3.5, fontFamily: t.brand.font.mono, color: 'text.disabled', backgroundImage: t.brand.gradient.signatureSoft })}>map · 6 gates · 2 open</Box>
            <Box sx={(t) => ({ p: 3.5, borderLeft: { sm: 1 }, borderColor: { sm: 'divider' }, fontFamily: t.brand.font.mono, color: 'text.disabled' })}>agent</Box>
          </Box>
        </Card>
      </Container>

      <Container maxWidth="lg" sx={{ py: 8 }}>
        <Box sx={{ maxWidth: '62ch', mb: 6 }}>
          <Eyebrow>Inside</Eyebrow>
          <Typography variant="h2" sx={{ fontSize: { xs: '2rem', md: '2.75rem' }, mt: 1.75 }}>Four surfaces, one job: finish the product.</Typography>
        </Box>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 2.5 }}>
          {panels.map((p) => (
            <Card key={p.name} sx={{ p: 3.5 }}>
              <Typography variant="h3" sx={{ fontSize: '1.25rem', mb: 1.25 }}>{p.name}</Typography>
              <Typography sx={{ color: 'text.secondary', lineHeight: 1.6 }}>{p.body}</Typography>
            </Card>
          ))}
        </Box>
      </Container>
    </>
  );
}
