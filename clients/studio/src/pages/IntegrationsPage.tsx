import { useState } from 'react';
import { Box, Button, Card, Chip, InputBase, Stack, Typography } from '@mui/material';
import { toast } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useStudio } from '../store';
import { useIntegrations } from '../integrationsStore';
import { TechIcon } from '../lib/techIcons';
import type { GateVerdict } from '../lib/liveGates';
import { text } from '@ironflyer/design-tokens/brand';

// Connectors the finisher wires into a project. `gate` is the real finisher
// gate name when one exists (so the chip reflects live verdict); null means
// there's no dedicated gate yet and the connection is recorded intent only.
const connectors = [
  { name: 'Stripe', desc: 'Payments, subscriptions, and webhooks.', label: 'Money', gate: null, glyph: 'S' },
  { name: 'Paddle', desc: 'Merchant-of-record billing and tax.', label: 'Money', gate: null, glyph: 'P' },
  { name: 'Postgres', desc: 'Managed database, migrations, backups.', label: 'Data', gate: null, glyph: 'D' },
  { name: 'GitHub', desc: 'Source, PRs, and the release workflow.', label: 'Deploy', gate: 'deploy', glyph: 'G' },
  { name: 'Slack', desc: 'Alerts for deploys, errors, and spend.', label: 'Signal', gate: null, glyph: '#' },
  { name: 'Sentry', desc: 'Error tracking wired into the board.', label: 'Signal', gate: null, glyph: '◎' },
  { name: 'Vercel', desc: 'Deploys to a domain you own.', label: 'Deploy', gate: 'deploy', glyph: '▲' },
  { name: 'Auth0', desc: 'Identity, SSO, and roles.', label: 'Identity', gate: null, glyph: 'A' },
];

export function IntegrationsPage() {
  const [q, setQ] = useState('');
  const { connected, toggle } = useIntegrations();
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const projectId = storeProjectId ?? firstProjectId;

  const { data: gates } = useGraphQLQuery<GateVerdict[], { gates: GateVerdict[] }>({
    key: ['gates', projectId ?? 'none'],
    operationName: 'Gates', query: operations.GATES,
    variables: { projectId }, fallbackData: [], enabled: !!projectId,
    map: (r) => r.gates ?? [],
  });
  const gateStatus = (name: string | null) => (name ? gates.find((g) => g.gate === name)?.status : undefined);

  const list = connectors.filter((c) => c.name.toLowerCase().includes(q.toLowerCase()));

  return (
    <Box sx={{ p: { xs: 3, md: 5 }, maxWidth: 1100, mx: 'auto' }}>
      <Typography variant="h3" sx={{ fontSize: text.s250, mb: 1 }}>Integrations</Typography>
      <Typography sx={{ color: 'text.secondary', mb: 3 }}>Connect a service and the finisher wires it into your project. Where a finisher gate exists, the chip shows its live verdict.</Typography>

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, border: 1, borderColor: 'divider', borderRadius: 2, px: 2, py: 1, mb: 3, maxWidth: 420, bgcolor: 'background.paper' }}>
        <Box component="span" sx={{ color: 'text.disabled' }}>⌕</Box>
        <InputBase fullWidth placeholder="Search integrations" value={q} onChange={(e) => setQ(e.target.value)} sx={{ fontSize: text.s90 }} />
      </Box>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 2 }}>
        {list.map((c) => {
          const isOn = connected.includes(c.name);
          const status = gateStatus(c.gate);
          const passed = status === 'passed';
          return (
            <Card key={c.name} sx={{ p: 2.5 }}>
              <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 1.5 }}>
                <Box sx={{ width: 40, height: 40, borderRadius: 2, display: 'grid', placeItems: 'center', bgcolor: 'action.hover', color: 'text.primary' }}><TechIcon name={c.name} size={22} title={c.name} /></Box>
                <Box sx={{ minWidth: 0, flex: 1 }}>
                  <Typography variant="h6" sx={{ fontSize: text.s105, lineHeight: 1.1 }}>{c.name}</Typography>
                  <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, color: 'text.disabled' })}>{c.label} · {c.gate ?? 'no gate yet'}</Typography>
                </Box>
                {status && (
                  <Chip size="small" label={status} color={passed ? 'success' : 'warning'} variant="outlined" sx={{ height: 20, fontSize: text.s62 }} />
                )}
              </Stack>
              <Typography sx={{ color: 'text.secondary', fontSize: text.s86, mb: 2, minHeight: '2.5em' }}>{c.desc}</Typography>
              <Button
                fullWidth
                variant={isOn ? 'outlined' : 'contained'}
                color={isOn ? 'inherit' : 'primary'}
                onClick={() => { toggle(c.name); toast(isOn ? `${c.name} disconnected.` : `${c.name} connected.`, 'success'); }}
              >
                {isOn ? 'Connected ✓' : 'Connect'}
              </Button>
            </Card>
          );
        })}
      </Box>
    </Box>
  );
}
