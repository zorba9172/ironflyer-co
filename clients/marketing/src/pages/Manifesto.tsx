import { Button, Container, Typography } from '@mui/material';
import { Head } from 'vite-react-ssg';
import { Reveal } from '@ironflyer/ui-web/motion';
import { Eyebrow } from '../components/text';

export function Manifesto() {
  return (
    <>
      <Head>
        <title>Manifesto — Ironflyer</title>
        <meta name="description" content="Why we built a finisher instead of another generator. The demo is the easy part. Shipping is the job." />
      </Head>

      <Container maxWidth="md" sx={{ pt: { xs: 11, md: 14 }, pb: 5, maxWidth: 720 }}>
        <Eyebrow>Manifesto</Eyebrow>
        <Typography variant="h1" sx={{ fontSize: { xs: '2.5rem', md: '4rem' }, my: 3.5 }}>The demo is the easy part.</Typography>

        <Typography sx={{ fontSize: '1.3rem', color: 'text.primary', lineHeight: 1.6, mb: 2.25 }}>
          For two years the whole industry has been racing to generate apps faster. Type a sentence, get a screen. It's genuinely good — and it stops exactly where the work starts.
        </Typography>

        <Typography sx={{ color: 'text.secondary', fontSize: '1.1rem', lineHeight: 1.7, mb: 2.25 }}>
          Because the screen isn't the product. The product is the login that remembers you, the payment that clears, the database that survives a schema change, the secret that isn't sitting in the source, the deploy you can roll back at 2am. None of that demos well. All of it is the difference between a clip on social and a company.
        </Typography>

        <Reveal>
        <Typography variant="h2" sx={{ fontSize: { xs: '1.6rem', md: '2.2rem' }, mt: 5.5, mb: 2 }}>So we built the other half.</Typography>
        <Typography sx={{ color: 'text.secondary', fontSize: '1.1rem', lineHeight: 1.7, mb: 2.25 }}>
          Ironflyer doesn't compete with the tool that made your prototype. It picks up where that tool quits. It reads what you have, tells you the truth about what's missing, and closes those gaps with you — one finisher gate at a time — until you're looking at something you can put a price on.
        </Typography>
        <Typography sx={{ color: 'text.secondary', fontSize: '1.1rem', lineHeight: 1.7, mb: 2.25 }}>
          We made three promises to ourselves while building it. The product tells you the truth, even when the truth is "this isn't ready." The code is yours, exportable, every day. And nothing ships that we wouldn't ship ourselves.
        </Typography>
        </Reveal>

        <Reveal>
        <Typography variant="h2" sx={{ fontSize: { xs: '1.6rem', md: '2.2rem' }, mt: 5.5, mb: 2 }}>Finishing is a craft.</Typography>
        <Typography sx={{ color: 'text.secondary', fontSize: '1.1rem', lineHeight: 1.7, mb: 2.25 }}>
          Anyone can start. Starting feels like progress because the screen fills up. Finishing is quieter and harder — it's the unglamorous list nobody posts about. We think that list deserves a serious tool, built to an international standard, for people who intend to ship.
        </Typography>

        <Typography sx={(t) => ({ fontFamily: t.brand.font.display, fontSize: '1.4rem', color: 'text.primary', my: 4.5 })}>
          That's the whole idea. Now go finish something.
        </Typography>

        <Button variant="contained" size="large" href="https://app.ironflyer.com/start">Start building</Button>
        </Reveal>
      </Container>
    </>
  );
}
