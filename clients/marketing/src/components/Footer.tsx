import { Box, Container, Stack, Typography } from '@mui/material';
import { Link } from 'react-router-dom';
import { LogoMark } from './Logo';

const cols = [
  { title: 'Product', links: [['/product', 'Overview'], ['/studio', 'Studio'], ['/pricing', 'Pricing']] },
  { title: 'Build', links: [['https://app.ironflyer.com', 'Open app'], ['/product', 'Deploy'], ['/manifesto', 'Manifesto']] },
  { title: 'Company', links: [['/manifesto', 'About'], ['mailto:hello@ironflyer.com', 'Contact'], ['/pricing', 'FAQ']] },
];

function FootLink({ to, children }: { to: string; children: React.ReactNode }) {
  const external = to.startsWith('http') || to.startsWith('mailto');
  const props = external ? { component: 'a' as const, href: to } : { component: Link, to };
  return (
    <Typography {...props} sx={{ fontSize: '0.9rem', color: 'text.secondary', '&:hover': { color: 'text.primary' } }}>
      {children}
    </Typography>
  );
}

export function Footer() {
  const year = new Date().getFullYear();
  return (
    <Box component="footer" sx={{ borderTop: 1, borderColor: 'divider', mt: 12, pt: 8, pb: 5 }}>
      <Container maxWidth="lg">
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: '1.6fr repeat(3, 1fr)' }, gap: 5 }}>
          <Box>
            <Stack direction="row" alignItems="center" spacing={1.25}>
              <LogoMark size={24} />
              <Typography variant="h6" sx={{ fontSize: '1.1rem' }}>Ironflyer</Typography>
            </Stack>
            <Typography sx={{ mt: 2, color: 'text.disabled', maxWidth: '30ch', fontSize: '0.9rem' }}>
              The finisher for AI-built products. You ship the part that's actually done.
            </Typography>
          </Box>
          {cols.map((c) => (
            <Stack key={c.title} spacing={1.5}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.72rem', letterSpacing: '0.12em', textTransform: 'uppercase', color: 'text.disabled' })}>{c.title}</Typography>
              {c.links.map(([to, label]) => <FootLink key={label} to={to as string}>{label}</FootLink>)}
            </Stack>
          ))}
        </Box>
        <Stack direction="row" justifyContent="space-between" sx={{ mt: 7, pt: 3, borderTop: 1, borderColor: 'divider', color: 'text.disabled', fontSize: '0.85rem' }}>
          <span>© {year} Ironflyer. Built to finish.</span>
        </Stack>
      </Container>
    </Box>
  );
}
