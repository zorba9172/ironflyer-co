import { useMemo, useState } from 'react';
import { Box, Button, Chip, InputBase, Stack, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { Icon } from '../icons';
import { toast } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useStudio } from '../store';
import { useIntegrations } from '../integrationsStore';
import type { GateVerdict } from '../lib/liveGates';
import { StudioChart, donutOption } from '../components/charts';
import { IntegrationCard, type Connector } from './integrations/IntegrationCard';

// Connectors the finisher wires into a project. `gate` is the real finisher
// gate name when one exists (so the chip reflects live verdict); null means
// there's no dedicated gate yet and the connection is recorded intent only.
const connectors: Connector[] = [
  { name: 'Stripe', desc: 'Payments, subscriptions, and webhooks.', label: 'Money', gate: null, glyph: 'S' },
  { name: 'Paddle', desc: 'Merchant-of-record billing and tax.', label: 'Money', gate: null, glyph: 'P' },
  { name: 'Postgres', desc: 'Managed database, migrations, backups.', label: 'Data', gate: null, glyph: 'D' },
  { name: 'GitHub', desc: 'Source, PRs, and the release workflow.', label: 'Deploy', gate: 'deploy', glyph: 'G' },
  { name: 'Slack', desc: 'Alerts for deploys, errors, and spend.', label: 'Signal', gate: null, glyph: '#' },
  { name: 'Sentry', desc: 'Error tracking wired into the board.', label: 'Signal', gate: null, glyph: '◎' },
  { name: 'Vercel', desc: 'Deploys to a domain you own.', label: 'Deploy', gate: 'deploy', glyph: '▲' },
  { name: 'Auth0', desc: 'Identity, SSO, and roles.', label: 'Identity', gate: null, glyph: 'A' },
];

const ALL = 'All';

export function IntegrationsPage() {
  const theme = useTheme();
  const [q, setQ] = useState('');
  const [category, setCategory] = useState<string>(ALL);
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

  // Category facets derived from the connector labels (Linear-style filter rail).
  const categories = useMemo(
    () => [ALL, ...Array.from(new Set(connectors.map((c) => c.label)))],
    [],
  );

  const list = useMemo(
    () =>
      connectors.filter((c) => {
        const matchesQ =
          c.name.toLowerCase().includes(q.toLowerCase()) ||
          c.desc.toLowerCase().includes(q.toLowerCase());
        const matchesCat = category === ALL || c.label === category;
        return matchesQ && matchesCat;
      }),
    [q, category],
  );

  const connectedCount = connectors.filter((c) => connected.includes(c.name)).length;
  const availableCount = connectors.length - connectedCount;

  const overviewOption = useMemo(
    () =>
      donutOption(theme, {
        data: [
          { name: 'Connected', value: connectedCount, color: theme.studio.neon.success },
          { name: 'Available', value: availableCount, color: theme.palette.action.hover },
        ],
        centerLabel: `${connectedCount}/${connectors.length}`,
        centerColor: theme.studio.neon.success,
        emptyLabel: 'None connected',
      }),
    [theme, connectedCount, availableCount],
  );

  return (
    <Box sx={{ p: { xs: 3, md: 5 }, maxWidth: 1180, mx: 'auto' }}>
      {/* Header — title + intent, with a donut that mirrors connect coverage. */}
      <Box
        sx={(theme) => ({
          display: 'grid',
          gap: 3,
          gridTemplateColumns: { xs: '1fr', md: '1fr auto' },
          alignItems: 'center',
          mb: 4,
          p: { xs: 3, md: 3.5 },
          backgroundColor: theme.palette.cardBg,
          border: `1px solid ${theme.palette.cardBorder}`,
          borderRadius: `${theme.studio.radius.lg}px`,
          backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
          WebkitBackdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
        })}
      >
        <Box sx={{ minWidth: 0 }}>
          <Chip
            icon={<Icon name="plug" size={15} />}
            label="Connectors"
            sx={(theme) => ({
              height: 28,
              mb: 1.5,
              borderRadius: theme.studio.radius.pill,
              border: `1px solid ${theme.palette.divider}`,
              backgroundColor: theme.palette.surfaceRaised,
              color: theme.palette.text.secondary,
              fontWeight: 600,
              '& .MuiChip-icon': { color: theme.studio.neon.blue, ml: 0.5 },
              '& .MuiChip-label': { px: 1 },
            })}
          />
          <Typography variant="h4" sx={{ fontWeight: 800, lineHeight: 1.1, mb: 1 }}>
            Integrations
          </Typography>
          <Typography color="text.secondary" sx={{ maxWidth: 560, lineHeight: 1.5 }}>
            Connect a service and the finisher wires it into your project. Where a
            finisher gate exists, the chip shows its live verdict.
          </Typography>
        </Box>

        <Box
          sx={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifySelf: { xs: 'center', md: 'end' },
          }}
        >
          <Box sx={{ width: 180 }}>
            <StudioChart option={overviewOption} height={150} />
          </Box>
          <Typography variant="caption" color="text.secondary" sx={{ mt: -0.5 }}>
            services connected
          </Typography>
        </Box>
      </Box>

      {/* Controls — search field + category filter rail. */}
      <Stack
        direction={{ xs: 'column', md: 'row' }}
        spacing={2}
        alignItems={{ xs: 'stretch', md: 'center' }}
        justifyContent="space-between"
        sx={{ mb: 3 }}
      >
        <Box
          sx={(theme) => ({
            display: 'flex',
            alignItems: 'center',
            gap: 1,
            px: 2,
            py: 1,
            width: { xs: '100%', md: 360 },
            borderRadius: `${theme.studio.radius.cta}px`,
            border: `1px solid ${theme.palette.divider}`,
            backgroundColor: theme.palette.surfaceRaised,
            transition: `border-color ${theme.studio.motion.fast}, box-shadow ${theme.studio.motion.fast}`,
            '&:focus-within': {
              borderColor: theme.studio.neon.blue,
              boxShadow: `0 0 0 3px ${theme.studio.neon.blue}22`,
            },
          })}
        >
          <Box component="span" sx={{ display: 'flex', color: 'text.disabled' }}>
            <Icon name="search" size={17} />
          </Box>
          <InputBase
            fullWidth
            placeholder="Search integrations"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            inputProps={{ 'aria-label': 'Search integrations' }}
          />
          {q && (
            <Box
              component="button"
              type="button"
              aria-label="Clear search"
              onClick={() => setQ('')}
              sx={(theme) => ({
                display: 'flex',
                alignItems: 'center',
                cursor: 'pointer',
                border: 0,
                p: 0.25,
                background: 'transparent',
                color: theme.palette.text.disabled,
                borderRadius: theme.studio.radius.pill,
                transition: `color ${theme.studio.motion.fast}`,
                '&:hover': { color: theme.palette.text.primary },
              })}
            >
              <Icon name="close" size={15} />
            </Box>
          )}
        </Box>

        <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', gap: 1 }}>
          {categories.map((cat) => {
            const active = cat === category;
            return (
              <Chip
                key={cat}
                label={cat}
                onClick={() => setCategory(cat)}
                variant="outlined"
                sx={(theme) => ({
                  borderRadius: theme.studio.radius.pill,
                  fontWeight: 600,
                  transition: `all ${theme.studio.motion.fast}`,
                  color: active ? theme.palette.text.primary : theme.palette.text.secondary,
                  borderColor: active ? theme.studio.neon.violet : theme.palette.divider,
                  backgroundColor: active ? theme.palette.surfaceHover : 'transparent',
                  '&:hover': {
                    backgroundColor: theme.palette.surfaceHover,
                    borderColor: theme.studio.neon.violet,
                  },
                })}
              />
            );
          })}
        </Stack>
      </Stack>

      {/* Catalog grid. */}
      {list.length > 0 ? (
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' },
            gap: 2.5,
          }}
        >
          {list.map((c) => {
            const isOn = connected.includes(c.name);
            return (
              <IntegrationCard
                key={c.name}
                connector={c}
                isOn={isOn}
                status={gateStatus(c.gate)}
                onToggle={() => {
                  toggle(c.name);
                  toast(isOn ? `${c.name} disconnected.` : `${c.name} connected.`, 'success');
                }}
              />
            );
          })}
        </Box>
      ) : (
        <Box
          sx={(theme) => ({
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            textAlign: 'center',
            gap: 1.5,
            py: 8,
            px: 3,
            borderRadius: `${theme.studio.radius.lg}px`,
            border: `1px dashed ${theme.palette.divider}`,
            backgroundColor: theme.palette.cardBg,
          })}
        >
          <Box
            aria-hidden
            sx={(theme) => ({
              display: 'grid',
              placeItems: 'center',
              width: 52,
              height: 52,
              borderRadius: theme.studio.radius.pill,
              color: theme.palette.text.disabled,
              background: theme.studio.gradient.soft,
              border: `1px solid ${theme.palette.borderSubtle}`,
            })}
          >
            <Icon name="search" size={22} />
          </Box>
          <Typography variant="subtitle1" sx={(theme) => ({ fontWeight: theme.typography.fontWeightBold })}>
            No integrations found
          </Typography>
          <Typography color="text.secondary" sx={{ maxWidth: 360 }}>
            Nothing matches the current filters. Try a different search or category.
          </Typography>
          <Button
            variant="outlined"
            color="inherit"
            onClick={() => {
              setQ('');
              setCategory(ALL);
            }}
            sx={{ mt: 1, fontWeight: 600 }}
          >
            Clear filters
          </Button>
        </Box>
      )}
    </Box>
  );
}
