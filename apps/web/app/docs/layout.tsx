// Docs surface layout — sticky left nav + body. The right "On this page"
// TOC is rendered per-page by <DocPage /> because its entries change with
// content. Wraps everything in the marketing chrome so users get the same
// top nav + footer they see elsewhere.

import type { Metadata } from 'next';
import { Box, Container } from '@mui/material';
import { MarketingShellClient } from '../marketing-shell';
import { DocsNav } from '../../components/docs/DocsNav';
import { tokens } from '../../../../packages/design-tokens';

export const metadata: Metadata = {
  title: {
    default: 'Docs — Ironflyer',
    template: '%s · Ironflyer Docs',
  },
  description:
    'Reference and concepts for the Ironflyer AI Product Finisher: finisher gates, patches, budget, runtime sandbox, SDK, VSCode extension.',
  openGraph: {
    title: 'Ironflyer Docs',
    description: 'How the finisher loop works, the API surface, and the SDK.',
    images: ['/opengraph-image'],
  },
};

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  return (
    <MarketingShellClient>
      <Box sx={{ bgcolor: tokens.color.bg.alabaster, minHeight: '100vh' }}>
        <Container maxWidth="xl" sx={{ py: { xs: 4, md: 6 } }}>
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', md: '240px minmax(0, 1fr)' },
              gap: { xs: 3, md: 5 },
              alignItems: 'flex-start',
            }}
          >
            <Box sx={{ display: { xs: 'none', md: 'block' } }}>
              <DocsNav />
            </Box>
            <Box sx={{ minWidth: 0 }}>{children}</Box>
          </Box>
        </Container>
      </Box>
    </MarketingShellClient>
  );
}
