import { useState } from 'react';
import { Box, Button, Card, InputBase, Stack, Typography } from '@mui/material';
import { toast } from '@ironflyer/ui-web/fx';

// Connectors the finisher wires into a project. Each maps to a gate so the
// integration isn't just an OAuth token — it gets verified end-to-end.
const connectors = [
  { name: 'Stripe', desc: 'Payments, subscriptions, and webhooks.', gate: 'Money', glyph: 'S' },
  { name: 'Paddle', desc: 'Merchant-of-record billing and tax.', gate: 'Money', glyph: 'P' },
  { name: 'Postgres', desc: 'Managed database, migrations, backups.', gate: 'Data', glyph: 'D' },
  { name: 'GitHub', desc: 'Source, PRs, and the release workflow.', gate: 'Deploy', glyph: 'G' },
  { name: 'Slack', desc: 'Alerts for deploys, errors, and spend.', gate: 'Signal', glyph: '#' },
  { name: 'Sentry', desc: 'Error tracking wired into the board.', gate: 'Signal', glyph: '◎' },
  { name: 'Vercel', desc: 'Deploys to a domain you own.', gate: 'Deploy', glyph: '▲' },
  { name: 'Auth0', desc: 'Identity, SSO, and roles.', gate: 'Identity', glyph: 'A' },
];

export function IntegrationsPage() {
  const [q, setQ] = useState('');
  const list = connectors.filter((c) => c.name.toLowerCase().includes(q.toLowerCase()));

  return (
    <Box sx={{ p: { xs: 3, md: 5 }, maxWidth: 1100, mx: 'auto' }}>
      <Typography variant="h3" sx={{ fontSize: '2.5rem', mb: 1 }}>Integrations</Typography>
      <Typography sx={{ color: 'text.secondary', mb: 3 }}>Connect a service and the finisher verifies it end-to-end — not just an OAuth token, but a closed gate.</Typography>

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, border: 1, borderColor: 'divider', borderRadius: 2, px: 2, py: 1, mb: 3, maxWidth: 420, bgcolor: 'background.paper' }}>
        <Box component="span" sx={{ color: 'text.disabled' }}>⌕</Box>
        <InputBase fullWidth placeholder="Search integrations" value={q} onChange={(e) => setQ(e.target.value)} sx={{ fontSize: '0.9rem' }} />
      </Box>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 2 }}>
        {list.map((c) => (
          <Card key={c.name} sx={{ p: 2.5 }}>
            <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 1.5 }}>
              <Box sx={(t) => ({ width: 40, height: 40, borderRadius: 2, display: 'grid', placeItems: 'center', fontFamily: t.brand.font.mono, fontWeight: 700, color: '#fff', backgroundImage: t.brand.gradient.signature })}>{c.glyph}</Box>
              <Box>
                <Typography variant="h6" sx={{ fontSize: '1.05rem', lineHeight: 1.1 }}>{c.name}</Typography>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', color: 'text.disabled' })}>{c.gate} gate</Typography>
              </Box>
            </Stack>
            <Typography sx={{ color: 'text.secondary', fontSize: '0.86rem', mb: 2, minHeight: '2.5em' }}>{c.desc}</Typography>
            <Button fullWidth variant="outlined" color="inherit" onClick={() => toast(`${c.name} connected — ${c.gate} gate will verify it.`, 'success')}>Connect</Button>
          </Card>
        ))}
      </Box>
    </Box>
  );
}
