// /docs index — a directory page for the documentation site. Acts as a
// hub: a brief intro, then card links into the major sections.

import type { Metadata } from 'next';
import Link from 'next/link';
import { Box, Stack, Typography } from '@mui/material';
import { tokens } from '../../../../packages/design-tokens';

export const metadata: Metadata = {
  title: 'Docs',
  description:
    'The Ironflyer documentation site — start here for the finisher loop, the API surface, the SDK, and our editor integrations.',
  openGraph: {
    title: 'Ironflyer Docs',
    description: 'Concepts, references, and walkthroughs for the AI Product Finisher.',
    images: ['/opengraph-image'],
  },
};

interface SectionCard {
  href: string;
  label: string;
  he: string;
  description: string;
}

const SECTIONS: SectionCard[] = [
  {
    href: '/docs/getting-started',
    label: 'Getting Started',
    he: 'מדריך התחלה',
    description: 'Sign up, run your first finisher loop, and ship a live preview in under five minutes.',
  },
  {
    href: '/docs/concepts/finisher-gates',
    label: 'Concepts',
    he: 'מושגי הליבה',
    description: 'How gates, patches, the runtime sandbox, and the transparent budget fit together.',
  },
  {
    href: '/docs/api/auth',
    label: 'API Reference',
    he: 'תיעוד API',
    description: 'Every HTTP route the orchestrator and runtime expose, with auth, payloads, and curl examples.',
  },
  {
    href: '/docs/sdk',
    label: 'SDK',
    he: 'ערכת פיתוח',
    description: 'Zero-dependency TypeScript client for the orchestrator and runtime. Browsers, Node, Bun, Deno.',
  },
  {
    href: '/docs/vscode-extension',
    label: 'VSCode Extension',
    he: 'תוסף VSCode',
    description: 'Use Ironflyer without leaving your editor — chat, gates, patches, live preview, status bar.',
  },
  {
    href: '/docs/cli',
    label: 'CLI',
    he: 'שורת פקודה',
    description: 'The Ironflyer command-line — shipping next. Drives finisher runs from your terminal.',
  },
];

export default function DocsIndex() {
  return (
    <Box>
      <Typography
        variant="overline"
        sx={{ color: '#5c5750', letterSpacing: '0.16em', fontWeight: 800, fontSize: 12 }}
      >
        Ironflyer Platform · גרסה 1.0
      </Typography>
      <Typography
        component="h1"
        sx={{
          fontFamily: tokens.font.display,
          fontSize: { xs: 38, md: 52 },
          lineHeight: 1.05,
          color: '#0d0e0f',
          mt: 1,
          mb: 1.5,
        }}
      >
        Build the way the AI finishes.
      </Typography>
      <Typography sx={{ color: '#3a3530', fontSize: 18, lineHeight: 1.6, maxWidth: 680, mb: 5 }}>
        Ironflyer is an AI Product Finisher — it ships your idea through Spec, UX, Architecture, Code,
        Lint, Tests, Security, and Deploy gates that <em>block</em> until they pass. These docs are how
        you wire your code, your team, and your editor into that loop. בעברית, באנגלית, ובכל שפה שבה
        השחקנים שלכם מקלידים.
      </Typography>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)' },
          gap: 2.5,
        }}
      >
        {SECTIONS.map((s) => (
          <Link key={s.href} href={s.href} style={{ color: 'inherit', textDecoration: 'none' }}>
            <Box
              sx={{
                p: 3,
                borderRadius: 2.5,
                bgcolor: '#fbf8f1',
                border: '1px solid rgba(17,17,17,0.10)',
                transition: `border-color ${tokens.motion.base} ${tokens.motion.curve}, transform ${tokens.motion.base} ${tokens.motion.curve}, box-shadow ${tokens.motion.base} ${tokens.motion.curve}`,
                '&:hover': {
                  borderColor: tokens.color.border.accent,
                  transform: 'translateY(-1px)',
                  boxShadow: '0 14px 32px rgba(13,14,15,0.08)',
                },
              }}
            >
              <Stack direction="row" alignItems="baseline" justifyContent="space-between" sx={{ mb: 1 }}>
                <Typography sx={{ fontFamily: tokens.font.display, fontSize: 22, color: '#0d0e0f' }}>
                  {s.label}
                </Typography>
                <Typography sx={{ color: '#77736b', fontSize: 12.5, fontWeight: 600 }}>{s.he}</Typography>
              </Stack>
              <Typography sx={{ color: '#3a3530', fontSize: 14.5, lineHeight: 1.55 }}>
                {s.description}
              </Typography>
            </Box>
          </Link>
        ))}
      </Box>
    </Box>
  );
}
