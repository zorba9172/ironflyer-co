import { useEffect, useState } from 'react';
import { Box, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { motion } from '@ironflyer/ui-web/fx';
import { useRunProjectFeed } from '@ironflyer/data';
import { formatRelativeTime } from '@ironflyer/core';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import type { ActivityEvent } from '../studioData';
import { text } from '@ironflyer/design-tokens/brand';

const MotionBox = motion.create(Box);

const SIM_LINES: Pick<ActivityEvent, 'kind' | 'text'>[] = [
  { kind: 'ledger', text: 'Ledger: debited $0.12 for premium model call' },
  { kind: 'profitguard', text: 'ProfitGuard: allow — expected margin 61%' },
  { kind: 'patch', text: 'Coder applied patch: add owner check to /orders' },
  { kind: 'gate', text: 'Data gate → running: rebuilding indexes' },
];

function kindColor(t: Theme, kind: ActivityEvent['kind']): string {
  switch (kind) {
    case 'gate': return t.studio?.neon?.blue ?? t.brand.accent.secondary;
    case 'patch': return t.palette.primary.main;
    case 'profitguard': return t.palette.warning.main;
    case 'deploy': return t.palette.success.main;
    default: return t.palette.text.disabled;
  }
}

function kindLabel(kind: ActivityEvent['kind']): string {
  switch (kind) {
    case 'gate': return 'Gate';
    case 'patch': return 'Patch';
    case 'profitguard': return 'ProfitGuard';
    case 'deploy': return 'Deploy';
    case 'ledger': return 'Ledger';
    default: return kind;
  }
}

// Live orchestration feed — streams the real run (gate/patch/profitguard/deploy
// events) over graphql-ws on the live project. Offline (no live project) it
// shows the seed and gently simulates so the operator always sees motion.
// `projectId` is kept for back-compat with callers but the live stream binds to
// the resolved backend project id, never the mock fixture id.
export function ActivityFeed({ seed }: { projectId?: string; seed: ActivityEvent[] }) {
  const theme = useTheme();
  const liveProjectId = useLiveProjectId();
  const { events, isLive } = useRunProjectFeed(liveProjectId);
  const [extra, setExtra] = useState<ActivityEvent[]>([]);

  useEffect(() => {
    if (isLive) return;
    let i = 0;
    const id = setInterval(() => {
      const line = SIM_LINES[i % SIM_LINES.length]!;
      setExtra((prev) => [{ id: `sim-${Date.now()}`, ts: Date.now(), ...line }, ...prev].slice(0, 8));
      i += 1;
    }, 6000);
    return () => clearInterval(id);
  }, [isLive]);

  // Live events are RunLogEvent (structurally an ActivityEvent: id/ts/kind/text).
  const all: ActivityEvent[] = isLive ? events.slice(0, 14) : [...extra, ...seed].slice(0, 14);

  return (
    <Box>
      {/* Section header */}
      <Stack direction="row" alignItems="center" spacing={1.25} sx={{ mb: 1.75 }}>
        <Typography
          sx={(t) => ({
            fontFamily: t.brand.font.mono,
            fontSize: text.s68,
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
            color: 'text.disabled',
          })}
        >
          Activity
        </Typography>
        <Chip
          size="small"
          label={isLive ? 'live' : 'simulated'}
          sx={(t) => ({
            height: 18,
            fontSize: text.s62,
            fontFamily: t.brand.font.mono,
            bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover',
            color: isLive ? 'success.main' : 'text.disabled',
          })}
        />
        {isLive && (
          <MotionBox
            sx={(t) => ({
              width: 6,
              height: 6,
              borderRadius: 99,
              bgcolor: t.palette.success.main,
              flexShrink: 0,
            })}
            animate={{ opacity: [1, 0.3, 1] }}
            transition={{ duration: 1.8, repeat: Infinity, ease: 'easeInOut' }}
          />
        )}
      </Stack>

      {/* Event rows */}
      <Stack spacing={0.5}>
        {all.map((e) => {
          const color = kindColor(theme, e.kind);
          return (
            <Stack
              key={e.id}
              direction="row"
              alignItems="flex-start"
              spacing={1.25}
              sx={(t) => ({
                py: 0.875,
                borderBottom: `1px solid ${t.palette.divider}`,
                '&:last-child': { borderBottom: 'none' },
              })}
            >
              {/* Kind dot */}
              <Box
                sx={{
                  width: 7,
                  height: 7,
                  borderRadius: 99,
                  flexShrink: 0,
                  bgcolor: color,
                  mt: 0.6,
                }}
              />

              <Box sx={{ flex: 1, minWidth: 0 }}>
                {/* Kind label */}
                <Stack direction="row" alignItems="center" spacing={0.75} sx={{ mb: 0.25 }}>
                  <Typography
                    sx={(t) => ({
                      fontFamily: t.brand.font.mono,
                      fontSize: text.s66,
                      letterSpacing: '0.08em',
                      textTransform: 'uppercase',
                      color,
                    })}
                  >
                    {kindLabel(e.kind)}
                  </Typography>
                  <Typography
                    sx={(t) => ({
                      fontFamily: t.brand.font.mono,
                      fontSize: text.s66,
                      color: 'text.disabled',
                    })}
                  >
                    {formatRelativeTime(e.ts)}
                  </Typography>
                </Stack>
                {/* Event text */}
                <Typography
                  sx={{ fontSize: text.s78, color: 'text.secondary', lineHeight: 1.4 }}
                  noWrap
                >
                  {e.text}
                </Typography>
              </Box>
            </Stack>
          );
        })}
      </Stack>
    </Box>
  );
}
