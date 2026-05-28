import { Box, Stack, Tooltip, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { motion } from '@ironflyer/ui-web/fx';
import { AGENTS, agentStatus, type Gate } from '../studioData';
import { agentColor } from './statusColor';

const MotionBox = motion.create(Box);

// Compact roster of the specialist agents the orchestrator is running, with
// live status. Working agents pulse. Status derives from each agent's gate.
export function AgentsRail({ gates }: { gates: Gate[] }) {
  const theme = useTheme();
  const working = AGENTS.filter((a) => agentStatus(a, gates) === 'working').length;

  return (
    <Box sx={{ mb: 3 }}>
      <Stack direction="row" alignItems="baseline" spacing={1} sx={{ mb: 1.25 }}>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>Agents</Typography>
        <Typography sx={{ fontSize: '0.78rem', color: 'text.secondary' }}>{working} working</Typography>
      </Stack>
      <Stack direction="row" spacing={1.25} sx={{ overflowX: 'auto', pb: 1 }}>
        {AGENTS.map((a) => {
          const status = agentStatus(a, gates);
          const color = agentColor(theme, status);
          return (
            <Tooltip key={a.id} title={`${a.role} · ${status}`} arrow>
              <Stack
                direction="row"
                alignItems="center"
                spacing={1}
                sx={{ flexShrink: 0, px: 1.25, py: 0.75, borderRadius: 99, border: 1, borderColor: 'divider', bgcolor: 'background.paper' }}
              >
                <Box sx={{ position: 'relative', width: 8, height: 8 }}>
                  <Box sx={{ position: 'absolute', inset: 0, borderRadius: 99, bgcolor: color }} />
                  {status === 'working' && (
                    <MotionBox
                      sx={{ position: 'absolute', inset: 0, borderRadius: 99, bgcolor: color }}
                      animate={{ scale: [1, 2.4], opacity: [0.6, 0] }}
                      transition={{ duration: 1.6, repeat: Infinity, ease: 'easeOut' }}
                    />
                  )}
                </Box>
                <Typography sx={{ fontSize: '0.82rem', fontWeight: 500, whiteSpace: 'nowrap' }}>{a.name}</Typography>
              </Stack>
            </Tooltip>
          );
        })}
      </Stack>
    </Box>
  );
}
