import { Box, Stack, Tooltip, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { motion } from '@ironflyer/ui-web/fx';
import { Icon } from '../icons';
import { AGENTS, agentStatus, type Gate } from '../studioData';
import { agentColor } from './statusColor';
import { GlassPanel } from './studio';
import { text } from '@ironflyer/design-tokens/brand';

const MotionBox = motion.create(Box);

// Individual agent tile — mirrors the reference "Active Agents" row: a small
// status-tinted icon tile, the agent name, and a one-line role. Working agents
// carry a live pulse ring around the icon so the row reads at a glance.
function AgentTile({ agent, status, color }: { agent: (typeof AGENTS)[number]; status: string; color: string }) {
  const isWorking = status === 'working';
  const live = isWorking || status === 'blocked';

  return (
    <Tooltip title={`${agent.role} · ${status}`} arrow>
      <GlassPanel
        accent={live ? color : undefined}
        pad={1.25}
        interactive={false}
        sx={{ minWidth: 0, flex: 1 }}
      >
        <Stack direction="row" alignItems="center" spacing={1.25}>
          {/* Status-tinted icon tile with a live pulse for working agents */}
          <Box sx={{ position: 'relative', width: 30, height: 30, flexShrink: 0 }}>
            <Box
              sx={(t) => ({
                position: 'absolute',
                inset: 0,
                borderRadius: `${t.studio.radius.sm}px`,
                display: 'grid',
                placeItems: 'center',
                color,
                backgroundColor: `${color}1f`,
                border: `1px solid ${color}33`,
              })}
            >
              <Icon name="bot" size={15} />
            </Box>
            {isWorking && (
              <MotionBox
                sx={(t) => ({ position: 'absolute', inset: 0, borderRadius: `${t.studio.radius.sm}px`, border: `1.5px solid ${color}` })}
                animate={{ scale: [1, 1.35], opacity: [0.55, 0] }}
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
              sx={{
                fontSize: text.s66,
                color: 'text.secondary',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
            >
              {agent.role}
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
