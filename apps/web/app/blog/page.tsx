// /blog — grid of post cards. Each post has its own page.

import type { Metadata } from 'next';
import Link from 'next/link';
import { Box, Container, Stack, Typography } from '@mui/material';
import { MarketingShellClient } from '../marketing-shell';
import { tokens } from '../../../../packages/design-tokens';

export const metadata: Metadata = {
  title: 'Blog — Ironflyer',
  description: 'Deep dives on the finisher loop, the transparent budget model, and the story behind shipping Ironflyer.',
  openGraph: {
    title: 'Blog · Ironflyer',
    description: 'Essays from the team building the AI Product Finisher.',
    images: ['/opengraph-image'],
  },
};

interface PostCard {
  slug: string;
  title: string;
  subtitle: string;
  date: string;
  gradient: string;
  tag: string;
}

const POSTS: PostCard[] = [
  {
    slug: 'multi-provider-routing',
    title: 'How we route across Anthropic, OpenAI, and Gemini.',
    subtitle: 'A capability-tagged router, a billing guard, and a speculative-decoding race — the three pieces that make multi-provider routing real.',
    date: '2026-05-24',
    gradient: 'linear-gradient(135deg, #671dfc 0%, #ff6c3a 100%)',
    tag: 'Engineering',
  },
  {
    slug: 'completion-infrastructure',
    title: 'Why we built the completion-infrastructure layer.',
    subtitle: 'Memory, audit, and DAG orchestration — what real reliability looks like.',
    date: '2026-05-23',
    gradient: 'linear-gradient(135deg, #0d0e0f 0%, #3a3530 55%, #e5ff00 100%)',
    tag: 'Engineering',
  },
  {
    slug: 'why-finished-products',
    title: 'Why we built a finisher, not another generator.',
    subtitle: 'Demos are easy. Finishing the last 20% is the product.',
    date: '2026-05-08',
    gradient: 'linear-gradient(135deg, #e5ff00 0%, #79e07a 100%)',
    tag: 'Product',
  },
  {
    slug: 'the-eight-gates',
    title: 'The nine gates, explained.',
    subtitle: 'Spec, UX, Architecture, Code, Lint, Tests, Security, Budget, Deploy — and what each one really blocks on.',
    date: '2026-04-24',
    gradient: 'linear-gradient(135deg, #671dfc 0%, #8b5cff 60%, #e5ff00 100%)',
    tag: 'Engineering',
  },
  {
    slug: 'budget-transparency',
    title: 'Why we publish our margin.',
    subtitle: 'Subscription − provider cost = margin. We thought the simplest way to be trusted was to do the arithmetic in public.',
    date: '2026-04-02',
    gradient: 'linear-gradient(135deg, #ffc400 0%, #ff6c3a 100%)',
    tag: 'Pricing',
  },
  {
    slug: 'launching-vscode-extension',
    title: 'Shipping the VSCode extension we wanted to use.',
    subtitle: 'Behind the 0.3 release: live preview in a webview, a patches tree with a real diff editor, and a quick action for every diagnostic.',
    date: '2026-03-05',
    gradient: 'linear-gradient(135deg, #78dbff 0%, #671dfc 100%)',
    tag: 'Tooling',
  },
];

export default function BlogIndex() {
  return (
    <MarketingShellClient>
      <Box sx={{ bgcolor: tokens.color.bg.alabaster, minHeight: '100vh' }}>
        <Container maxWidth="lg" sx={{ py: { xs: 6, md: 10 } }}>
          <Typography variant="overline" sx={{ color: '#5c5750', letterSpacing: '0.16em', fontWeight: 800, fontSize: 12 }}>
            Blog
          </Typography>
          <Typography
            component="h1"
            sx={{
              fontFamily: tokens.font.display,
              fontSize: { xs: 42, md: 58 },
              lineHeight: 1.04,
              color: '#0d0e0f',
              mt: 1,
              mb: 1.5,
            }}
          >
            Essays from the team.
          </Typography>
          <Typography sx={{ color: '#3a3530', fontSize: 18, lineHeight: 1.6, maxWidth: 640, mb: 6 }}>
            Long-form notes on how we are building Ironflyer: the gates we enforce, the tradeoffs we accept,
            the competitors we study, and the production standards we refuse to dilute.
          </Typography>

          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)' },
              gap: 3,
            }}
          >
            {POSTS.map((post) => (
              <Link key={post.slug} href={`/blog/${post.slug}`} style={{ color: 'inherit', textDecoration: 'none' }}>
                <Box
                  component="article"
                  sx={{
                    borderRadius: 3,
                    overflow: 'hidden',
                    bgcolor: '#fbf8f1',
                    border: '1px solid rgba(17,17,17,0.10)',
                    transition: `border-color ${tokens.motion.base} ${tokens.motion.curve}, transform ${tokens.motion.base} ${tokens.motion.curve}, box-shadow ${tokens.motion.base} ${tokens.motion.curve}`,
                    '&:hover': {
                      borderColor: tokens.color.border.accent,
                      transform: 'translateY(-2px)',
                      boxShadow: '0 22px 48px rgba(13,14,15,0.10)',
                    },
                  }}
                >
                  <Box
                    sx={{
                      height: 168,
                      background: post.gradient,
                      position: 'relative',
                      '&::after': {
                        content: '""',
                        position: 'absolute',
                        inset: 0,
                        background:
                          'radial-gradient(circle at 20% 20%, rgba(255,255,255,0.32), transparent 55%)',
                      },
                    }}
                  />
                  <Box sx={{ p: 3 }}>
                    <Stack direction="row" alignItems="center" spacing={1.4} sx={{ mb: 1.6 }}>
                      <Typography
                        sx={{
                          fontFamily: tokens.font.mono,
                          fontSize: 11,
                          letterSpacing: '0.1em',
                          textTransform: 'uppercase',
                          color: '#5c5750',
                          fontWeight: 800,
                        }}
                      >
                        {post.tag}
                      </Typography>
                      <Box sx={{ width: 3, height: 3, borderRadius: '999px', bgcolor: '#bdb7ab' }} />
                      <Typography sx={{ color: '#77736b', fontSize: 12.5 }}>{post.date}</Typography>
                    </Stack>
                    <Typography
                      sx={{
                        fontFamily: tokens.font.display,
                        fontSize: 24,
                        lineHeight: 1.15,
                        color: '#0d0e0f',
                        mb: 1.2,
                      }}
                    >
                      {post.title}
                    </Typography>
                    <Typography sx={{ color: '#3a3530', fontSize: 15, lineHeight: 1.55 }}>
                      {post.subtitle}
                    </Typography>
                  </Box>
                </Box>
              </Link>
            ))}
          </Box>
        </Container>
      </Box>
    </MarketingShellClient>
  );
}
