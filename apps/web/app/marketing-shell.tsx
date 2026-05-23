'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import {
  ArrowForward, GitHub, LinkedIn, Twitter, YouTube,
} from '@mui/icons-material';
import {
  Box, Button, Container, Divider, IconButton, Stack, Typography,
} from '@mui/material';
import { tokens } from '../../../packages/design-tokens';
import { IronflyerLogo } from '../components/brand/IronflyerLogo';
import { StatusBadge } from '../components/StatusBadge';

// Shared marketing chrome — sticky blur nav, generous footer with real links,
// announcement bar. Lives in a client component so we can drive the scroll-
// based blur effect; child marketing surfaces stay server components.

export const navItems = [
  { label: 'Product',    href: '/product' },
  { label: 'Templates',  href: '/templates' },
  { label: 'Pricing',    href: '/pricing' },
  { label: 'Solutions',  href: '/solutions' },
  { label: 'Security',   href: '/security' },
  { label: 'Enterprise', href: '/enterprise' },
];

const footerGroups = [
  {
    title: 'Product',
    links: [
      { label: 'Platform',   href: '/product' },
      { label: 'Templates',  href: '/templates' },
      { label: 'Pricing',    href: '/pricing' },
      { label: 'Changelog',  href: '/product#changelog' },
      { label: 'Status',     href: '/status' },
    ],
  },
  {
    title: 'Solutions',
    links: [
      { label: 'Startup MVP',     href: '/solutions#mvp' },
      { label: 'Internal Tools',  href: '/solutions#internal' },
      { label: 'Client Work',     href: '/solutions#agency' },
      { label: 'Internal AI',     href: '/solutions#ai' },
    ],
  },
  {
    title: 'Company',
    links: [
      { label: 'Security',   href: '/security' },
      { label: 'Enterprise', href: '/enterprise' },
      { label: 'Contact',    href: 'mailto:hello@ironflyer.dev' },
      { label: 'Careers',    href: '/enterprise#careers' },
    ],
  },
  {
    title: 'Legal',
    links: [
      { label: 'Privacy', href: '/legal/privacy' },
      { label: 'Terms',   href: '/legal/terms' },
      { label: 'DPA',     href: '/legal/dpa' },
    ],
  },
];

export function SiteNav() {
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    function onScroll() {
      setScrolled(window.scrollY > 12);
    }
    onScroll();
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  const navLinkSx = {
    color: '#111',
    fontWeight: 700,
    fontSize: 14,
    letterSpacing: 0,
    transition: `color ${tokens.motion.base} ${tokens.motion.curve}`,
    '&:hover': { color: '#5c6300' },
  };

  return (
    <Box
      component="header"
      sx={{
        position: 'sticky',
        top: 0,
        zIndex: 30,
        backgroundColor: scrolled ? 'rgba(244,240,232,0.78)' : 'rgba(244,240,232,1)',
        backdropFilter: scrolled ? 'saturate(140%) blur(14px)' : 'none',
        WebkitBackdropFilter: scrolled ? 'saturate(140%) blur(14px)' : 'none',
        borderBottom: scrolled ? '1px solid rgba(17,17,17,0.08)' : '1px solid transparent',
        transition: `background-color ${tokens.motion.base} ${tokens.motion.curve}, border-color ${tokens.motion.base} ${tokens.motion.curve}`,
      }}
    >
      <Box sx={{ bgcolor: '#0d0e0f', color: tokens.color.bg.alabaster }}>
        <Container maxWidth="xl" sx={{
          minHeight: 42,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 1.2,
        }}>
          <Box sx={{
            width: 7, height: 7, borderRadius: '999px',
            bgcolor: tokens.color.accent.lime,
            boxShadow: '0 0 10px rgba(229,255,0,0.65)',
          }} />
          <Typography variant="body2" sx={{ fontWeight: 800, fontSize: 12.5, letterSpacing: 0.2 }}>
            Ironflyer — the #1 AI Completion Engine for proved software
          </Typography>
          <Link href="/product" style={{ color: tokens.color.accent.lime, textDecoration: 'none' }}>
            <Stack direction="row" spacing={0.4} alignItems="center">
              <Typography variant="body2" sx={{ fontWeight: 800, fontSize: 12.5 }}>
                What changed
              </Typography>
              <ArrowForward sx={{ fontSize: 14 }} />
            </Stack>
          </Link>
        </Container>
      </Box>

      <Container maxWidth="xl" sx={{
        minHeight: { xs: 60, md: 64 },
        display: 'grid',
        gridTemplateColumns: { xs: '1fr auto', md: '1fr auto 1fr' },
        alignItems: 'center',
        gap: { xs: 1.5, md: 3 },
      }}>
        <Link href="/" style={{ color: 'inherit', textDecoration: 'none' }}>
          <IronflyerLogo size={30} tone="light" />
        </Link>

        <Stack direction="row" spacing={{ md: 3.2, lg: 4 }} justifyContent="center" sx={{ display: { xs: 'none', md: 'flex' } }}>
          {navItems.map((item) => (
            <Link key={item.href} href={item.href} style={{ color: 'inherit', textDecoration: 'none' }}>
              <Typography variant="body2" sx={navLinkSx}>{item.label}</Typography>
            </Link>
          ))}
        </Stack>

        <Stack direction="row" spacing={1} alignItems="center" justifyContent="flex-end">
          <Button
            component={Link}
            href="/login"
            variant="text"
            sx={{
              color: '#111',
              fontWeight: 700,
              display: { xs: 'none', sm: 'inline-flex' },
              '&:hover': { bgcolor: 'rgba(17,17,17,0.05)' },
            }}
          >
            Log in
          </Button>
          <Button
            component={Link}
            href="/app"
            variant="contained"
            endIcon={<ArrowForward sx={{ fontSize: 16 }} />}
            sx={{
              minWidth: { xs: 92, sm: 132 },
              bgcolor: tokens.color.accent.lime,
              color: '#050505',
              borderRadius: '999px',
              py: 0.9,
              px: 2.2,
              fontWeight: 800,
              boxShadow: scrolled ? '0 6px 18px rgba(229,255,0,0.28)' : 'none',
              '&:hover': { bgcolor: '#f0ff36' },
            }}
          >
            <Box component="span" sx={{ display: { xs: 'none', sm: 'inline' } }}>Start building</Box>
            <Box component="span" sx={{ display: { xs: 'inline', sm: 'none' } }}>Build</Box>
          </Button>
        </Stack>
      </Container>
    </Box>
  );
}

export function SiteFooter() {
  return (
    <Box component="footer" sx={{ bgcolor: '#0a0b0c', color: tokens.color.bg.alabaster, mt: 0 }}>
      <Container maxWidth="xl" sx={{ py: { xs: 7, md: 10 } }}>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.2fr 0.9fr 0.9fr 0.9fr 0.9fr' }, gap: { xs: 5, md: 4 } }}>
          <Box>
            <IronflyerLogo size={36} tone="dark" tagline />
            <Typography variant="body2" sx={{ mt: 2.2, maxWidth: 320, color: '#9c968a', lineHeight: 1.55 }}>
              The #1 AI Completion Engine. Spec to deploy, gated end-to-end — built so AI-generated software survives real review.
            </Typography>
            <Stack direction="row" spacing={0.5} sx={{ mt: 3 }}>
              <IconButton component="a" href="https://github.com/ironflyer" aria-label="GitHub" sx={socialIconSx}><GitHub fontSize="small" /></IconButton>
              <IconButton component="a" href="https://twitter.com/ironflyer" aria-label="Twitter" sx={socialIconSx}><Twitter fontSize="small" /></IconButton>
              <IconButton component="a" href="https://linkedin.com/company/ironflyer" aria-label="LinkedIn" sx={socialIconSx}><LinkedIn fontSize="small" /></IconButton>
              <IconButton component="a" href="https://youtube.com/@ironflyer" aria-label="YouTube" sx={socialIconSx}><YouTube fontSize="small" /></IconButton>
            </Stack>
          </Box>
          {footerGroups.map((group) => (
            <Box key={group.title}>
              <Typography variant="overline" sx={{ color: '#7d7770', fontSize: 11, letterSpacing: '0.14em' }}>{group.title}</Typography>
              <Stack spacing={1.25} sx={{ mt: 1.6 }}>
                {group.links.map((link) => (
                  <Link key={link.label} href={link.href} style={{ color: 'inherit', textDecoration: 'none' }}>
                    <Typography variant="body2" sx={{
                      color: tokens.color.bg.alabaster,
                      fontWeight: 600,
                      fontSize: 14,
                      transition: `color ${tokens.motion.base} ${tokens.motion.curve}`,
                      '&:hover': { color: tokens.color.accent.lime },
                    }}>
                      {link.label}
                    </Typography>
                  </Link>
                ))}
              </Stack>
            </Box>
          ))}
        </Box>
        <Divider sx={{ my: 5, borderColor: 'rgba(244,240,232,0.08)' }} />
        <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" alignItems={{ xs: 'flex-start', sm: 'center' }} spacing={1}>
          <Typography variant="caption" sx={{ color: '#7d7770', fontSize: 12 }}>
            © {new Date().getFullYear()} Ironflyer Labs. All rights reserved.
          </Typography>
          <Stack direction="row" spacing={3} sx={{ color: '#7d7770' }}>
            <StatusBadge />
            <Link href="/legal/privacy" style={{ color: 'inherit', textDecoration: 'none' }}>
              <Typography variant="caption" sx={{ color: '#9c968a', fontSize: 12 }}>Privacy</Typography>
            </Link>
            <Link href="/legal/terms" style={{ color: 'inherit', textDecoration: 'none' }}>
              <Typography variant="caption" sx={{ color: '#9c968a', fontSize: 12 }}>Terms</Typography>
            </Link>
          </Stack>
        </Stack>
      </Container>
    </Box>
  );
}

const socialIconSx = {
  color: '#9c968a',
  border: '1px solid rgba(244,240,232,0.08)',
  borderRadius: '8px',
  width: 36, height: 36,
  '&:hover': {
    color: tokens.color.accent.lime,
    borderColor: 'rgba(229,255,0,0.36)',
    bgcolor: 'rgba(229,255,0,0.06)',
  },
};

export function MarketingShellClient({ children }: { children: React.ReactNode }) {
  return (
    <Box sx={{
      minHeight: '100vh',
      bgcolor: tokens.color.bg.alabaster,
      color: tokens.color.text.inverse,
    }}>
      <SiteNav />
      {children}
      <SiteFooter />
    </Box>
  );
}
