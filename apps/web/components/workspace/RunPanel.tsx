'use client';

// RunPanel — the right rail. Shows a big "Run Finisher" CTA, the live event
// timeline coming off the orchestrator's SSE stream, and surfaces gate
// failures with an inline "Repair" action that triggers a repair run.
//
// Keep strings short and explicit so this panel is easy to localize later.

import { useMemo, useState } from 'react';
import {
  Box, Button, Chip, CircularProgress, Collapse, IconButton, LinearProgress, Stack,
  Tooltip, Typography,
} from '@mui/material';
import {
  AutoAwesome, CheckCircle, Cancel, RestartAlt, RocketLaunch, Bolt, Build,
  HourglassBottom, Schedule, ExpandLess, ExpandMore,
} from '@mui/icons-material';
import { tokens } from '../../lib/theme';
import { RunEvent, eventSeverity } from '../../lib/api/orchestrator-stream';
import { VirtualList } from '../performance/VirtualList';

interface Props {
  events: RunEvent[];
  running: boolean;
  streamHealthy: boolean;
  onRun: () => void;
  onRepair?: (gateKey?: string) => void;
  emptyHint?: string;
}

export function RunPanel({ events, running, streamHealthy, onRun, onRepair, emptyHint }: Props) {
  const lastFailure = useMemo(
    () => events.slice().reverse().find((e) => eventSeverity(e) === 'danger'),
    [events],
  );
  const progress = useMemo(() => computeProgress(events), [events]);
  const orderedEvents = useMemo(() => events.slice().reverse(), [events]);

  return (
    <Stack spacing={0.8} sx={{ overflowY: 'auto', height: '100%', minHeight: 0 }}>
      <Box sx={panelSx}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ px: 1.2, pt: 1.1 }}>
          <Stack direction="row" alignItems="center" spacing={1}>
            <RocketLaunch fontSize="small" sx={{ color: tokens.color.accent.lime }} />
            <Typography variant="overline" color="text.secondary">Finisher</Typography>
          </Stack>
          <Stream pulse={running} healthy={streamHealthy} />
        </Stack>

        <Box sx={{ px: 1.2, pt: 0.7 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 800, lineHeight: 1.2 }}>
            {running ? 'Running the gates...' : 'Ready for the next run'}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.35 }}>
            {running
              ? 'The agent plans, writes, tests, and repairs until every gate passes.'
              : 'We run spec, UX, code, tests, security, and deploy gates. If something fails, the agent repairs it and loops.'}
          </Typography>
        </Box>

        <Box sx={{ px: 1.2, pt: 1.1, pb: 1.2 }}>
          <Button
            fullWidth
            variant="contained"
            size="large"
            disabled={running}
            startIcon={running ? <CircularProgress size={16} sx={{ color: 'currentColor' }} /> : <Bolt />}
            onClick={onRun}
            sx={{
              minHeight: 44,
              borderRadius: `${tokens.radius.sm}px`,
              fontWeight: 900,
              fontSize: 15,
            }}
          >
            {running ? 'Run in progress' : 'Run Finisher'}
          </Button>
          <LinearProgress
            variant={running ? 'indeterminate' : 'determinate'}
            value={progress.percent}
            sx={{
              mt: 1.2,
              height: 6,
              borderRadius: '999px',
              bgcolor: tokens.color.bg.inset,
              '& .MuiLinearProgress-bar': {
                bgcolor: progress.failed ? tokens.color.accent.danger : tokens.color.accent.lime,
              },
            }}
          />
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.6 }}>
            {progress.label}
          </Typography>
        </Box>
      </Box>

      {lastFailure && (
        <FailureCard event={lastFailure} onRepair={() => onRepair?.(lastFailure.gate as string | undefined)} />
      )}

      <Box sx={{ ...panelSx, flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ px: 1.2, pt: 1.1, pb: 0.5 }}>
          <Typography variant="overline" color="text.secondary">Live activity</Typography>
          <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono }}>
            {events.length} events
          </Typography>
        </Stack>

        <Box sx={{ flex: 1, minHeight: 0, overflow: 'hidden', px: 0.8, pb: 1 }}>
          {events.length === 0 ? (
            <EmptyState hint={emptyHint} />
          ) : (
            <VirtualList
              items={orderedEvents}
              itemHeight={58}
              getItemHeight={timelineRowHeight}
              height="100%"
              keyExtractor={(event, index) => event.id || `${event.kind}-${event.createdAt}-${index}`}
              ariaLabel="Live run activity"
              sx={{ pt: 0.6 }}
              renderItem={(event) => <TimelineRow event={event} />}
            />
          )}
        </Box>
      </Box>
    </Stack>
  );
}

function FailureCard({ event, onRepair }: { event: RunEvent; onRepair: () => void }) {
  const [open, setOpen] = useState(true);
  return (
    <Box sx={{
      ...panelSx,
      borderColor: 'rgba(255, 24, 24, 0.42)',
      bgcolor: 'rgba(255, 24, 24, 0.08)',
    }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ px: 1.1, py: 1 }}>
        <Cancel fontSize="small" sx={{ color: tokens.color.accent.danger }} />
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="subtitle2" noWrap>
            {event.gate ? `${event.gate} gate failed` : 'Run failed'}
          </Typography>
          <Typography variant="caption" color="text.secondary" noWrap title={event.message}>
            {event.message}
          </Typography>
        </Box>
        <IconButton size="small" onClick={() => setOpen((v) => !v)} sx={{ color: tokens.color.text.secondary }}>
          {open ? <ExpandLess fontSize="small" /> : <ExpandMore fontSize="small" />}
        </IconButton>
      </Stack>
      <Collapse in={open}>
        <Box sx={{ px: 1.1, pb: 1.1 }}>
          {event.detail && (
            <Box sx={{
              px: 1, py: 0.9,
              borderRadius: `${tokens.radius.sm}px`,
              bgcolor: tokens.color.bg.inset,
              border: `1px solid ${tokens.color.border.subtle}`,
              fontFamily: tokens.font.mono, fontSize: 12,
              whiteSpace: 'pre-wrap', color: tokens.color.text.primary,
              maxHeight: 140, overflow: 'auto',
            }}>
              {event.detail}
            </Box>
          )}
          <Button
            startIcon={<Build fontSize="small" />}
            variant="contained"
            size="small"
            onClick={onRepair}
            sx={{ mt: 1, borderRadius: `${tokens.radius.sm}px` }}
          >
            Repair this gate
          </Button>
        </Box>
      </Collapse>
    </Box>
  );
}

function TimelineRow({ event }: { event: RunEvent }) {
  const sev = eventSeverity(event);
  const colour = SEV_COLOUR[sev];
  const Icon = SEV_ICON[sev];
  return (
    <Stack
      direction="row"
      spacing={1}
      sx={{
        px: 0.65, py: 0.55,
        borderRadius: `${tokens.radius.sm}px`,
        '&:hover': { bgcolor: tokens.color.bg.surfaceHover },
      }}
    >
      <Box sx={{
        mt: 0.3, width: 22, height: 22,
        borderRadius: `${tokens.radius.sm}px`,
        flexShrink: 0,
        display: 'grid', placeItems: 'center',
        bgcolor: `${colour}1f`,
        color: colour,
      }}>
        <Icon sx={{ fontSize: 14 }} />
      </Box>
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Stack direction="row" spacing={0.8} alignItems="baseline" sx={{ minWidth: 0 }}>
          <Typography variant="caption" sx={{ fontWeight: 800, color: tokens.color.text.primary }}>
            {event.gate ?? event.step ?? event.kind.replace(/_/g, ' ')}
          </Typography>
          <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono, fontSize: 10 }}>
            {formatTime(event.createdAt)}
          </Typography>
        </Stack>
        <Typography
          variant="caption"
          sx={{
            display: '-webkit-box',
            color: tokens.color.text.secondary,
            mt: 0.1,
            overflow: 'hidden',
            WebkitBoxOrient: 'vertical',
            WebkitLineClamp: 3,
          }}
          title={event.message}
        >
          {event.message}
        </Typography>
      </Box>
    </Stack>
  );
}

function timelineRowHeight(event: RunEvent): number {
  const length = event.message?.length ?? 0;
  if (length > 160) return 98;
  if (length > 80) return 76;
  return 58;
}

function EmptyState({ hint }: { hint?: string }) {
  return (
    <Stack alignItems="center" spacing={1} sx={{ py: 2.4, textAlign: 'center', px: 1.2 }}>
      <Box sx={{
        width: 42, height: 42,
        borderRadius: `${tokens.radius.sm}px`,
        display: 'grid', placeItems: 'center',
        bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tokens.color.border.subtle}`,
        color: tokens.color.text.muted,
      }}>
        <AutoAwesome fontSize="small" />
      </Box>
      <Typography variant="body2" sx={{ fontWeight: 700 }}>
        No activity yet
      </Typography>
      <Typography variant="caption" color="text.secondary" sx={{ maxWidth: 220 }}>
        {hint ?? 'Run the Finisher to watch the agent work in real time.'}
      </Typography>
    </Stack>
  );
}

function Stream({ pulse, healthy }: { pulse: boolean; healthy: boolean }) {
  const colour = healthy ? tokens.color.accent.lime : tokens.color.accent.warning;
  return (
    <Tooltip title={healthy ? 'Live stream connected' : 'Reconnecting…'}>
      <Stack direction="row" alignItems="center" spacing={0.6}>
        <Box sx={{
          width: 8, height: 8, borderRadius: '50%', bgcolor: colour,
          boxShadow: pulse ? `0 0 0 0 ${colour}` : 'none',
          animation: pulse ? 'rp-pulse 1.4s ease-out infinite' : 'none',
          '@keyframes rp-pulse': {
            '0%':   { boxShadow: `0 0 0 0 ${colour}66` },
            '70%':  { boxShadow: `0 0 0 8px ${colour}00` },
            '100%': { boxShadow: `0 0 0 0 ${colour}00` },
          },
        }} />
        <Typography variant="caption" sx={{ fontFamily: tokens.font.mono, color: tokens.color.text.muted, fontSize: 10 }}>
          live
        </Typography>
      </Stack>
    </Tooltip>
  );
}

const SEV_COLOUR: Record<'info' | 'success' | 'danger' | 'progress', string> = {
  info: tokens.color.text.muted,
  success: tokens.color.accent.success,
  danger: tokens.color.accent.danger,
  progress: tokens.color.accent.lime,
};

const SEV_ICON: Record<'info' | 'success' | 'danger' | 'progress', typeof CheckCircle> = {
  info: Schedule,
  success: CheckCircle,
  danger: Cancel,
  progress: HourglassBottom,
};

function computeProgress(events: RunEvent[]): { percent: number; failed: boolean; label: string } {
  const passed = events.filter((e) => e.kind === 'gate_passed').length;
  const failed = events.some((e) => e.kind === 'gate_failed' || e.kind === 'run_failed');
  const complete = events.some((e) => e.kind === 'run_complete');
  // 9 gates total — match GATE_ORDER in the workspace shell.
  const total = 9;
  const percent = complete ? 100 : Math.min(100, Math.round((passed / total) * 100));
  const label = complete
    ? 'Run complete. Every gate passed.'
    : failed
      ? `${passed}/${total} passed. Repair required.`
      : passed === 0
        ? 'The agent is waiting for input.'
        : `${passed}/${total} passed.`;
  return { percent, failed, label };
}

function formatTime(iso: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  } catch {
    return '';
  }
}

const panelSx = {
  borderRadius: `${tokens.radius.sm}px`,
  border: `1px solid ${tokens.color.border.subtle}`,
  bgcolor: tokens.color.bg.surfaceRaised,
  boxShadow: tokens.shadow.sm,
  overflow: 'hidden',
};
