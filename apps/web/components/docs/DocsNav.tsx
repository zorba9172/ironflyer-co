'use client';

// DocsNav — left sidebar for /docs. Server pages compose this client
// component so the active highlight + collapsible groups stay reactive
// to client-side route transitions.

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useState } from 'react';
import { Box, Stack, Typography, Collapse } from '@mui/material';
import ExpandLessIcon from '@mui/icons-material/ExpandLess';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import { tokens } from '../../../../packages/design-tokens';

export interface NavLink {
  label: string;
  he?: string;
  href: string;
}

export interface NavGroup {
  label: string;
  he?: string;
  links: NavLink[];
}

export const DOCS_NAV: NavGroup[] = [
  {
    label: 'Getting Started',
    he: 'התחלה מהירה',
    links: [
      { label: 'Quickstart', he: 'מדריך התחלה', href: '/docs/getting-started' },
    ],
  },
  {
    label: 'Concepts',
    he: 'מושגים',
    links: [
      { label: 'Finisher Gates', he: 'שערי הגימור', href: '/docs/concepts/finisher-gates' },
      { label: 'Patches', he: 'טלאים', href: '/docs/concepts/patches' },
      { label: 'Budget', he: 'תקציב', href: '/docs/concepts/budget' },
      { label: 'Runtime Sandbox', he: 'ארגז חול', href: '/docs/concepts/runtime-sandbox' },
    ],
  },
  {
    label: 'API Reference',
    he: 'תיעוד API',
    links: [
      { label: 'Auth', href: '/docs/api/auth' },
      { label: 'Projects', href: '/docs/api/projects' },
      { label: 'Patches', href: '/docs/api/patches' },
      { label: 'Budget', href: '/docs/api/budget' },
      { label: 'Webhooks', href: '/docs/api/webhooks' },
      { label: 'Deploy', href: '/docs/api/deploy' },
      { label: 'Runtime', href: '/docs/api/runtime' },
    ],
  },
  {
    label: 'SDK',
    he: 'SDK',
    links: [
      { label: '@ironflyer/sdk', href: '/docs/sdk' },
    ],
  },
  {
    label: 'Clients',
    he: 'לקוחות',
    links: [
      { label: 'VSCode Extension', he: 'תוסף VSCode', href: '/docs/vscode-extension' },
      { label: 'CLI', he: 'שורת פקודה', href: '/docs/cli' },
    ],
  },
];

function NavGroupBlock({ group, pathname }: { group: NavGroup; pathname: string }) {
  const hasActive = group.links.some((l) => pathname === l.href || pathname.startsWith(l.href + '/'));
  // Groups start open by default; the hasActive check exists to keep the
  // signature honest even when we later make collapsed-by-default a setting.
  const [open, setOpen] = useState<boolean>(hasActive || true);

  return (
    <Box sx={{ mb: 2.2 }}>
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        onClick={() => setOpen((v) => !v)}
        sx={{
          cursor: 'pointer',
          px: 1,
          py: 0.5,
          borderRadius: 1.5,
          '&:hover': { bgcolor: 'rgba(17,17,17,0.04)' },
        }}
      >
        <Typography
          variant="overline"
          sx={{
            color: '#5c5750',
            fontSize: 11,
            letterSpacing: '0.14em',
            fontWeight: 800,
          }}
        >
          {group.label}
          {group.he ? (
            <Box component="span" sx={{ color: '#999', ml: 1, fontWeight: 500 }}>
              {group.he}
            </Box>
          ) : null}
        </Typography>
        {open ? (
          <ExpandLessIcon sx={{ fontSize: 16, color: '#999' }} />
        ) : (
          <ExpandMoreIcon sx={{ fontSize: 16, color: '#999' }} />
        )}
      </Stack>
      <Collapse in={open} unmountOnExit>
        <Stack spacing={0.25} sx={{ mt: 0.6 }}>
          {group.links.map((link) => {
            const active = pathname === link.href || pathname.startsWith(link.href + '/');
            return (
              <Link key={link.href} href={link.href} style={{ color: 'inherit', textDecoration: 'none' }}>
                <Box
                  sx={{
                    px: 1.4,
                    py: 0.7,
                    borderRadius: 1.5,
                    borderLeft: active ? `3px solid ${tokens.color.accent.lime}` : '3px solid transparent',
                    bgcolor: active ? 'rgba(229,255,0,0.16)' : 'transparent',
                    color: active ? '#111' : '#3a3530',
                    fontWeight: active ? 700 : 500,
                    fontSize: 14,
                    transition: `background-color ${tokens.motion.fast} ${tokens.motion.curve}, color ${tokens.motion.fast} ${tokens.motion.curve}`,
                    '&:hover': {
                      bgcolor: active ? 'rgba(229,255,0,0.22)' : 'rgba(17,17,17,0.04)',
                      color: '#111',
                    },
                  }}
                >
                  {link.label}
                </Box>
              </Link>
            );
          })}
        </Stack>
      </Collapse>
    </Box>
  );
}

export function DocsNav() {
  const pathname = usePathname() || '/docs';

  return (
    <Box
      component="nav"
      aria-label="Docs navigation"
      sx={{
        position: 'sticky',
        top: 88,
        alignSelf: 'flex-start',
        maxHeight: 'calc(100vh - 100px)',
        overflowY: 'auto',
        pr: 1.5,
      }}
    >
      <Stack spacing={0.5} sx={{ pb: 2, mb: 2.5, borderBottom: '1px solid rgba(17,17,17,0.08)' }}>
        <Link href="/docs" style={{ color: 'inherit', textDecoration: 'none' }}>
          <Typography sx={{ fontFamily: tokens.font.display, fontSize: 18, color: '#111' }}>
            DOCS
          </Typography>
        </Link>
        <Typography sx={{ color: '#77736b', fontSize: 12.5 }}>
          תיעוד מלא · Ironflyer Platform
        </Typography>
      </Stack>
      {DOCS_NAV.map((g) => (
        <NavGroupBlock key={g.label} group={g} pathname={pathname} />
      ))}
    </Box>
  );
}

export default DocsNav;
