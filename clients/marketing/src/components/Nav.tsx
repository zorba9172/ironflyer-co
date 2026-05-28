import { Box, Button, Container, Stack, Typography } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { Link, useLocation } from 'react-router-dom';
import { LogoMark } from './Logo';
import { ThemeToggle } from './ThemeToggle';

const links = [
  { to: '/product', label: 'Product' },
  { to: '/studio', label: 'Studio' },
  { to: '/pricing', label: 'Pricing' },
  { to: '/manifesto', label: 'Manifesto' },
];

export function Nav() {
  const { pathname } = useLocation();
  return (
    <Box
      component="header"
      sx={(t) => ({
        position: 'sticky',
        top: 0,
        zIndex: 50,
        backdropFilter: 'blur(12px)',
        backgroundColor: alpha(t.palette.background.default, 0.72),
        borderBottom: 1,
        borderColor: 'divider',
      })}
    >
      <Container maxWidth="lg" sx={{ height: 68, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 3 }}>
        <Stack component={Link} to="/" direction="row" alignItems="center" spacing={1.25} sx={{ color: 'text.primary' }}>
          <LogoMark />
          <Typography variant="h6" sx={{ fontSize: '1.1rem' }}>Ironflyer</Typography>
        </Stack>

        <Stack direction="row" spacing={3.5} sx={{ display: { xs: 'none', md: 'flex' } }}>
          {links.map((l) => (
            <Typography
              key={l.to}
              component={Link}
              to={l.to}
              sx={{ fontSize: '0.9rem', color: pathname === l.to ? 'text.primary' : 'text.secondary', '&:hover': { color: 'text.primary' } }}
            >
              {l.label}
            </Typography>
          ))}
        </Stack>

        <Stack direction="row" alignItems="center" spacing={1.5}>
          <ThemeToggle />
          <Button href="https://app.ironflyer.com" sx={{ display: { xs: 'none', sm: 'inline-flex' }, color: 'text.secondary' }}>Sign in</Button>
          <Button variant="contained" href="https://app.ironflyer.com/start">Start building</Button>
        </Stack>
      </Container>
    </Box>
  );
}
