import { Box, Chip, Stack, Typography } from '@mui/material';
import { Icon, type IconName } from '../../icons';
import { GlassPanel } from '../../components/studio';
import { mockProject, statusLabel, type Gate, type GateStatus } from '../../studioData';

// ─────────────────────────────────────────────────────────────────────────
// Live Build column. The ambient, supporting context on the right of Home — a
// real-time mirror of the orchestrator's gate timeline plus the deployment
// target. Each step names what is open end-to-end (the "what's not closed"
// law): a blocked gate shows its blocking reason, a running gate shows its
// share of completion. Every tone comes from the semantic palette.
// ─────────────────────────────────────────────────────────────────────────

type Tone = 'success' | 'primary' | 'warning' | 'danger' | 'muted';

const STATUS_TONE: Record<GateStatus, Tone> = {
  closed: 'success',
  running: 'primary',
  open: 'warning',
  blocked: 'danger',
  unstarted: 'muted',
};

const STATUS_ICON: Record<GateStatus, IconName> = {
  closed: 'check',
  running: 'activity',
  open: 'clock',
  blocked: 'alert',
  unstarted: 'clock',
};

function toneColor(theme: import('@mui/material').Theme, tone: Tone): string {
  switch (tone) {
    case 'success': return theme.palette.success.main;
    case 'primary': return theme.palette.primary.main;
    case 'warning': return theme.palette.warning.main;
    case 'danger': return theme.palette.error.main;
    default: return theme.palette.text.disabled;
  }
}

function Step({ gate, last }: { gate: Gate; last: boolean }) {
  const tone = STATUS_TONE[gate.status];
  const running = gate.status === 'running';
  return (
    <Stack direction="row" spacing={1.5} sx={{ minWidth: 0 }}>
      {/* timeline rail */}
      <Stack alignItems="center" sx={{ flexShrink: 0 }}>
        <Box
          sx={(theme) => {
            const c = toneColor(theme, tone);
            return {
              width: 28,
              height: 28,
              borderRadius: 99,
              display: 'grid',
              placeItems: 'center',
              color: c,
              bgcolor: `${c}1A`,
              border: `1px solid ${c}40`,
            };
          }}
        >
          <Icon name={STATUS_ICON[gate.status]} size={14} />
        </Box>
        {!last && (
          <Box sx={(theme) => ({ flex: 1, width: '2px', my: 0.5, bgcolor: theme.palette.borderSubtle, minHeight: 14 })} />
        )}
      </Stack>

      <Box sx={{ minWidth: 0, pb: last ? 0 : 1.5 }}>
        <Stack direction="row" alignItems="center" spacing={1} sx={{ minWidth: 0 }}>
          <Typography variant="body2" sx={{ fontWeight: 700 }} noWrap>{gate.name}</Typography>
          <Typography
            variant="caption"
            sx={(theme) => ({ fontWeight: 700, color: toneColor(theme, tone) })}
          >
            {statusLabel[gate.status]}
          </Typography>
        </Stack>
        {gate.blocking && gate.status !== 'closed' && (
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.25 }}>
            {gate.blocking}
          </Typography>
        )}
        {running && (
          <Box
            sx={(theme) => ({
              mt: 0.75,
              height: 4,
              borderRadius: 99,
              bgcolor: theme.palette.surfaceHover,
              overflow: 'hidden',
            })}
          >
            <Box
              sx={(theme) => ({
                width: `${Math.round(gate.level * 100)}%`,
                height: '100%',
                borderRadius: 99,
                bgcolor: theme.palette.primary.main,
              })}
            />
          </Box>
        )}
      </Box>
    </Stack>
  );
}

export function LiveBuild() {
  const { gates, deploy, name } = mockProject;
  const isLive = deploy.status === 'production';

  return (
    <Stack spacing={2} sx={{ width: '100%' }}>
      <GlassPanel pad={2}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.5 }}>
          <Stack direction="row" alignItems="center" spacing={1}>
            <Box sx={(theme) => ({ color: theme.palette.primary.main, display: 'inline-flex' })}>
              <Icon name="activity" size={17} />
            </Box>
            <Typography variant="h6" sx={{ fontWeight: 700 }}>Live build</Typography>
          </Stack>
          <Chip
            size="small"
            icon={<Box sx={(theme) => ({ width: 7, height: 7, borderRadius: 99, bgcolor: theme.palette.success.main })} />}
            label="Live"
            sx={(theme) => ({
              height: 22,
              fontWeight: 700,
              color: theme.palette.success.main,
              bgcolor: `${theme.palette.success.main}14`,
              border: `1px solid ${theme.palette.success.main}33`,
              '& .MuiChip-icon': { ml: 1 },
              '& .MuiChip-label': { px: 1 },
            })}
          />
        </Stack>

        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1.75 }} noWrap>
          {name}
        </Typography>

        <Box>
          {gates.map((g, i) => (
            <Step key={g.id} gate={g} last={i === gates.length - 1} />
          ))}
        </Box>
      </GlassPanel>

      <GlassPanel pad={2}>
        <Stack direction="row" alignItems="center" justifyContent="space-between">
          <Stack direction="row" alignItems="center" spacing={1.25} sx={{ minWidth: 0 }}>
            <Box
              sx={(theme) => ({
                width: 34,
                height: 34,
                borderRadius: `${theme.studio.radius.sm}px`,
                display: 'grid',
                placeItems: 'center',
                color: theme.palette.primary.main,
                bgcolor: `${theme.palette.primary.main}14`,
                flexShrink: 0,
              })}
            >
              <Icon name="deployments" size={17} />
            </Box>
            <Box sx={{ minWidth: 0 }}>
              <Typography variant="body2" sx={{ fontWeight: 700 }}>Deployment</Typography>
              <Typography variant="caption" color="text.secondary" noWrap sx={{ display: 'block' }}>
                {deploy.url ?? 'Not deployed yet'}
              </Typography>
            </Box>
          </Stack>
          <Chip
            size="small"
            label={isLive ? 'Production' : 'Preview'}
            sx={(theme) => {
              const c = isLive ? theme.palette.success.main : theme.palette.warning.main;
              return {
                flexShrink: 0,
                height: 22,
                fontWeight: 700,
                color: c,
                bgcolor: `${c}14`,
                border: `1px solid ${c}33`,
                '& .MuiChip-label': { px: 1 },
              };
            }}
          />
        </Stack>
      </GlassPanel>
    </Stack>
  );
}
