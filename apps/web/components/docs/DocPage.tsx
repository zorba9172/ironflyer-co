// DocPage — shell for an individual docs page. Renders the title, a short
// description, the page body, and a right-side "On this page" table of
// contents. Server component; client interactivity (active anchor highlight)
// lives in DocsTOC below.

import { ReactNode } from 'react';
import { Box, Stack, Typography } from '@mui/material';
import { tokens } from '../../../../packages/design-tokens';
import DocsTOC from './DocsTOC';

export interface DocTOCEntry {
  id: string;
  label: string;
  depth?: 2 | 3;
}

export interface DocPageProps {
  title: string;
  eyebrow?: string;
  description?: string;
  toc?: DocTOCEntry[];
  children: ReactNode;
}

// Slugify — turn a heading label into an anchor id. Used by callers that
// want consistent ids without retyping kebab-case strings.
export function slugify(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9֐-׿]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export function DocPage({ title, eyebrow, description, toc, children }: DocPageProps) {
  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', lg: 'minmax(0, 1fr) 220px' },
        gap: { xs: 0, lg: 5 },
        alignItems: 'flex-start',
      }}
    >
      <Box component="article" sx={{ maxWidth: 760, color: '#1a1a1a' }}>
        {eyebrow ? (
          <Typography
            variant="overline"
            sx={{
              color: '#5c5750',
              letterSpacing: '0.16em',
              fontWeight: 800,
              fontSize: 12,
            }}
          >
            {eyebrow}
          </Typography>
        ) : null}
        <Typography
          component="h1"
          sx={{
            fontFamily: tokens.font.display,
            fontSize: { xs: 36, md: 46 },
            lineHeight: 1.05,
            color: '#0d0e0f',
            mt: eyebrow ? 1 : 0,
            mb: 1.6,
          }}
        >
          {title}
        </Typography>
        {description ? (
          <Typography
            sx={{
              color: '#3a3530',
              fontSize: 18,
              lineHeight: 1.55,
              mb: 4,
              maxWidth: 640,
            }}
          >
            {description}
          </Typography>
        ) : null}
        <Box
          sx={{
            // Body typography — these selectors style the markdown-ish
            // children rendered by individual pages.
            '& h2': {
              fontFamily: tokens.font.display,
              fontSize: 26,
              lineHeight: 1.2,
              color: '#0d0e0f',
              mt: 5,
              mb: 1.5,
              scrollMarginTop: 88,
            },
            '& h3': {
              fontFamily: tokens.font.family,
              fontSize: 18,
              fontWeight: 800,
              color: '#1a1a1a',
              mt: 3.5,
              mb: 1.2,
              scrollMarginTop: 88,
            },
            '& p': {
              fontSize: 16,
              lineHeight: 1.75,
              color: '#262320',
              mb: 2,
            },
            '& ul, & ol': {
              pl: 3,
              mb: 2,
              '& li': { fontSize: 16, lineHeight: 1.7, color: '#262320', mb: 0.6 },
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
              fontSize: 13.5,
              bgcolor: 'rgba(229,255,0,0.18)',
              px: 0.7,
              py: 0.2,
              borderRadius: 0.6,
              color: '#1a1a1a',
            },
            '& blockquote': {
              borderLeft: `3px solid ${tokens.color.accent.lime}`,
              pl: 2.2,
              ml: 0,
              my: 2.5,
              color: '#3a3530',
              fontStyle: 'italic',
            },
            '& table': {
              width: '100%',
              borderCollapse: 'collapse',
              my: 3,
              fontSize: 14,
              '& th, & td': {
                textAlign: 'left',
                px: 1.4,
                py: 1.1,
                borderBottom: '1px solid rgba(17,17,17,0.08)',
              },
              '& th': { color: '#5c5750', fontWeight: 700, fontSize: 12, letterSpacing: '0.06em', textTransform: 'uppercase' },
            },
            '& hr': {
              border: 'none',
              borderTop: '1px solid rgba(17,17,17,0.10)',
              my: 5,
            },
          }}
        >
          {children}
        </Box>
        <Box sx={{ mt: 8, pt: 4, borderTop: '1px solid rgba(17,17,17,0.10)' }}>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} alignItems="center" justifyContent="space-between">
            <Typography sx={{ color: '#77736b', fontSize: 13 }}>
              Questions? Write to us at{' '}
              <a href="mailto:docs@ironflyer.dev" style={{ color: '#5c6300', textDecoration: 'underline', fontWeight: 600 }}>
                docs@ironflyer.dev
              </a>
            </Typography>
            <Typography sx={{ color: '#77736b', fontSize: 13 }}>
              Last updated · {new Date().toISOString().slice(0, 10)}
            </Typography>
          </Stack>
        </Box>
      </Box>
      {toc && toc.length > 0 ? <DocsTOC entries={toc} /> : null}
    </Box>
  );
}

export default DocPage;
