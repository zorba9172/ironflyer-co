// BlogPost — shell for an individual /blog/[slug] page. Hero gradient,
// title block, body slot, CTA card linking to pricing. Server component;
// no client state required.

import { ReactNode } from 'react';
import Link from 'next/link';
import { Box, Container, Stack, Typography } from '@mui/material';
import { tokens } from '../../../../packages/design-tokens';
import { MarketingShellClient } from '../../app/marketing-shell';

export interface BlogPostProps {
  title: string;
  subtitle: string;
  tag: string;
  date: string;
  gradient: string;
  children: ReactNode;
}

export function BlogPost({ title, subtitle, tag, date, gradient, children }: BlogPostProps) {
  return (
    <MarketingShellClient>
      <Box sx={{ bgcolor: tokens.color.bg.alabaster }}>
        <Box
          sx={{
            background: gradient,
            position: 'relative',
            color: '#0d0e0f',
            '&::after': {
              content: '""',
              position: 'absolute',
              inset: 0,
              background: 'radial-gradient(circle at 20% 20%, rgba(255,255,255,0.32), transparent 55%)',
              pointerEvents: 'none',
            },
          }}
        >
          <Container maxWidth="md" sx={{ py: { xs: 7, md: 12 }, position: 'relative', zIndex: 1 }}>
            <Stack direction="row" alignItems="center" spacing={1.4} sx={{ mb: 2.5 }}>
              <Box
                sx={{
                  px: 1.2,
                  py: 0.4,
                  borderRadius: 1.5,
                  bgcolor: 'rgba(13,14,15,0.85)',
                  color: '#fff',
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  fontWeight: 800,
                  letterSpacing: '0.08em',
                  textTransform: 'uppercase',
                }}
              >
                {tag}
              </Box>
              <Typography sx={{ color: '#262320', fontSize: 13, fontWeight: 600 }}>{date}</Typography>
              <Typography sx={{ color: '#262320', fontSize: 13 }}>· Ironflyer Team</Typography>
            </Stack>
            <Typography
              component="h1"
              sx={{
                fontFamily: tokens.font.display,
                fontSize: { xs: 38, md: 56 },
                lineHeight: 1.05,
                color: '#0d0e0f',
                mb: 1.8,
              }}
            >
              {title}
            </Typography>
            <Typography sx={{ color: '#0d0e0f', fontSize: { xs: 18, md: 21 }, lineHeight: 1.45, maxWidth: 640 }}>
              {subtitle}
            </Typography>
          </Container>
        </Box>

        <Container maxWidth="md" sx={{ py: { xs: 6, md: 10 } }}>
          <Box
            component="article"
            sx={{
              color: '#1a1a1a',
              '& h2': {
                fontFamily: tokens.font.display,
                fontSize: { xs: 26, md: 30 },
                lineHeight: 1.2,
                color: '#0d0e0f',
                mt: 5,
                mb: 1.5,
              },
              '& h3': {
                fontFamily: tokens.font.family,
                fontSize: 19,
                fontWeight: 800,
                color: '#0d0e0f',
                mt: 4,
                mb: 1.2,
              },
              '& p': {
                fontSize: 17,
                lineHeight: 1.8,
                color: '#262320',
                mb: 2.2,
              },
              '& ul, & ol': {
                pl: 3,
                mb: 2.2,
                '& li': { fontSize: 17, lineHeight: 1.75, color: '#262320', mb: 0.8 },
              },
              '& a': {
                color: '#5c6300',
                textDecoration: 'underline',
                textUnderlineOffset: 3,
                fontWeight: 600,
                '&:hover': { color: '#3a4000' },
              },
              '& code': {
                fontFamily: tokens.font.mono,
                fontSize: 14.5,
                bgcolor: 'rgba(229,255,0,0.18)',
                px: 0.7,
                py: 0.2,
                borderRadius: 0.6,
              },
              '& blockquote': {
                borderLeft: `3px solid ${tokens.color.accent.lime}`,
                pl: 2.4,
                ml: 0,
                my: 3,
                color: '#3a3530',
                fontStyle: 'italic',
                fontSize: 18,
              },
            }}
          >
            {children}
          </Box>

          <Box
            sx={{
              mt: 8,
              p: { xs: 3, md: 4.5 },
              borderRadius: 3,
              bgcolor: '#0d0e0f',
              color: tokens.color.bg.alabaster,
              position: 'relative',
              overflow: 'hidden',
              '&::after': {
                content: '""',
                position: 'absolute',
                top: -60,
                right: -80,
                width: 240,
                height: 240,
                borderRadius: '50%',
                background: `radial-gradient(circle, ${tokens.color.accent.lime}, transparent 65%)`,
                opacity: 0.4,
              },
            }}
          >
            <Box sx={{ position: 'relative', zIndex: 1 }}>
              <Typography
                variant="overline"
                sx={{ color: tokens.color.accent.lime, letterSpacing: '0.16em', fontWeight: 800, fontSize: 12 }}
              >
                Try the finisher loop
              </Typography>
              <Typography
                sx={{
                  fontFamily: tokens.font.display,
                  fontSize: { xs: 26, md: 32 },
                  color: tokens.color.bg.alabaster,
                  mt: 1,
                  mb: 1.5,
                }}
              >
                Ship the next idea through nine gates.
              </Typography>
              <Typography sx={{ color: '#b9b3a8', fontSize: 15.5, lineHeight: 1.6, mb: 3, maxWidth: 480 }}>
                Free tier ships with four projects, ~50 runs / month, a real Linux sandbox, and the same
                budget transparency you see on this page.
              </Typography>
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5}>
                <Link href="/pricing" style={{ textDecoration: 'none' }}>
                  <Box
                    sx={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: 0.8,
                      bgcolor: tokens.color.accent.lime,
                      color: '#050505',
                      px: 2.5,
                      py: 1.2,
                      borderRadius: '999px',
                      fontWeight: 800,
                      fontSize: 14,
                      '&:hover': { bgcolor: '#f0ff36' },
                    }}
                  >
                    See pricing
                  </Box>
                </Link>
                <Link href="/docs/getting-started" style={{ textDecoration: 'none' }}>
                  <Box
                    sx={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: 0.8,
                      color: tokens.color.bg.alabaster,
                      px: 2.5,
                      py: 1.2,
                      borderRadius: '999px',
                      border: '1px solid rgba(244,240,232,0.24)',
                      fontWeight: 700,
                      fontSize: 14,
                      '&:hover': { borderColor: tokens.color.accent.lime, color: tokens.color.accent.lime },
                    }}
                  >
                    Read the quickstart
                  </Box>
                </Link>
              </Stack>
            </Box>
          </Box>
        </Container>
      </Box>
    </MarketingShellClient>
  );
}

export default BlogPost;
