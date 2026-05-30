import { Box, Stack, Tooltip, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { motion } from '@ironflyer/ui-web/fx';
import { AGENTS, agentStatus, type Gate } from '../studioData';
import { agentColor } from './statusColor';
import { GlassPanel } from './studio';
import { text } from '@ironflyer/design-tokens/brand';

const MotionBox = motion.create(Box);

// Individual agent tile — glanceable status card with live pulse.
function AgentTile({ agent, status, color }: { agent: (typeof AGENTS)[number]; status: string; color: string }) {
  const isWorking = status === 'working';
  const isDone = status === 'done';
  const isBlocked = status === 'blocked';

  return (
    <Tooltip title={`${agent.role} · ${status}`} arrow>
      <GlassPanel
        accent={isWorking || isBlocked ? color : undefined}
        pad={1.5}
        interactive={false}
        sx={{ minWidth: 0, flex: 1 }}
      >
        <Stack direction="row" alignItems="center" spacing={1}>
          {/* Status dot with pulse for working agents */}
          <Box sx={{ position: 'relative', width: 8, height: 8, flexShrink: 0 }}>
            <Box sx={{ position: 'absolute', inset: 0, borderRadius: 99, bgcolor: color }} />
            {isWorking && (
              <MotionBox
                sx={{ position: 'absolute', inset: 0, borderRadius: 99, bgcolor: color }}
                animate={{ scale: [1, 2.4], opacity: [0.6, 0] }}
                transition={{ duration: 1.6, repeat: Infinity, ease: 'easeOut' }}
              />
            )}
          </Box>

          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography
              sx={{
                fontSize: text.s78,
                fontWeight: 600,
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
            >
              {agent.name}
            </Typography>
            <Typography
              sx={(t) => ({
                fontFamily: t.brand.font.mono,
                fontSize: text.s66,
                color: isDone
                  ? 'success.main'
                  : isBlocked
                  ? 'error.main'
                  : isWorking
                  ? 'text.secondary'
                  : 'text.disabled',
                textTransform: 'uppercase',
                letterSpacing: '0.06em',
              })}
            >
              {status}
            </Typography>
          </Box>
        </Stack>
      </GlassPanel>
    </Tooltip>
  );
}

// Compact roster of the specialist agents the orchestrator is running, with
// live status. Working agents pulse. Status derives from each agent's gate.
export function AgentsRail({ gates }: { gates: Gate[] }) {
  const theme = useTheme();
  const working = AGENTS.filter((a) => agentStatus(a, gates) === 'working').length;
  const blocked = AGENTS.filter((a) => agentStatus(a, gates) === 'blocked').length;

  return (
    <Box sx={{ mb: 2.5 }}>
      {/* Section label */}
      <Stack direction="row" alignItems="baseline" spacing={1.5} sx={{ mb: 1.5 }}>
        <Typography
          sx={(t) => ({
            fontFamily: t.brand.font.mono,
            fontSize: text.s68,
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
            color: 'text.disabled',
          })}
        >
          Agents
        </Typography>
        <Stack direction="row" spacing={1} alignItems="center">
          {working > 0 && (
            <Typography sx={{ fontSize: text.s74, color: 'text.secondary' }}>
              {working} working
            </Typography>
          )}
          {blocked > 0 && (
            <Typography sx={{ fontSize: text.s74, color: 'error.main' }}>
              · {blocked} blocked
            </Typography>
          )}
          {working === 0 && blocked === 0 && (
            <Typography sx={{ fontSize: text.s74, color: 'text.disabled' }}>
              all idle
            </Typography>
          )}
        </Stack>
      </Stack>

      {/* Agent tiles — 2-column grid for a tighter, more readable layout */}
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: 1,
        }}
      >
        {AGENTS.map((a) => {
          const status = agentStatus(a, gates);
          const color = agentColor(theme, status);
          return (
            <AgentTile key={a.id} agent={a} status={status} color={color} />
          );
        })}
      </Box>
    </Box>
  );
}
