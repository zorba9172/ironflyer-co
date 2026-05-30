import { Avatar, Box, Stack, Tooltip, Typography } from '@mui/material';
import { Icon, type IconName } from '../../icons';
import { AGENTS, agentStatus, mockProject, type Agent, type AgentStatus } from '../../studioData';

// ─────────────────────────────────────────────────────────────────────────
// Active Agents row. A horizontal roster of the specialist agents the
// orchestrator runs, each as a uniform pill: an avatar with a live status dot,
// the agent name, and its role. Status mirrors real gate state (working /
// blocked / done / idle) from the current project. No raw color literals —
// tones come from the semantic palette; glyphs from the Icon barrel.
// ─────────────────────────────────────────────────────────────────────────

const STATUS_TONE: Record<AgentStatus, 'success' | 'warning' | 'danger' | 'muted'> = {
  working: 'success',
  done: 'success',
  blocked: 'danger',
  idle: 'muted',
};

const STATUS_LABEL: Record<AgentStatus, string> = {
  working: 'Working',
  done: 'Done',
  blocked: 'Blocked',
  idle: 'Idle',
};

// Map each agent to a glyph from the barrel (no inline svg / emoji).
const AGENT_ICON: Record<string, IconName> = {
  orchestrator: 'network',
  coder: 'code',
  identity: 'users',
  payments: 'wallet',
  data: 'data',
  security: 'shield',
  deployer: 'deployments',
  mobile: 'smartphone',
};

function toneColor(theme: import('@mui/material').Theme, tone: 'success' | 'warning' | 'danger' | 'muted'): string {
  switch (tone) {
    case 'success': return theme.palette.success.main;
    case 'warning': return theme.palette.warning.main;
    case 'danger': return theme.palette.error.main;
    default: return theme.palette.text.disabled;
  }
}

function AgentChip({ agent, status }: { agent: Agent; status: AgentStatus }) {
  const tone = STATUS_TONE[status];
  const icon = AGENT_ICON[agent.id] ?? 'bot';
  return (
    <Tooltip title={`${agent.role} · ${STATUS_LABEL[status]}`} arrow>
      <Stack
        direction="row"
        alignItems="center"
        spacing={1.25}
        sx={(theme) => ({
          p: 1,
          pr: 1.75,
          minWidth: 0,
          borderRadius: `${theme.studio.radius.pill}px`,
          border: `1px solid ${theme.palette.cardBorder}`,
          bgcolor: theme.palette.background.paper,
          transition: `transform ${theme.studio.motion.base}, border-color ${theme.studio.motion.base}`,
          '&:hover': { transform: 'translateY(-2px)', borderColor: theme.palette.primary.main },
        })}
      >
        <Box sx={{ position: 'relative', flexShrink: 0 }}>
          <Avatar
            sx={(theme) => ({
              width: 30,
              height: 30,
              color: theme.palette.primary.main,
              bgcolor: `${theme.palette.primary.main}14`,
            })}
          >
            <Icon name={icon} size={15} />
          </Avatar>
          <Box
            aria-hidden
            sx={(theme) => ({
              position: 'absolute',
              right: -1,
              bottom: -1,
              width: 11,
              height: 11,
              borderRadius: 99,
              bgcolor: toneColor(theme, tone),
              border: `2px solid ${theme.palette.background.paper}`,
            })}
          />
        </Box>
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="body2" sx={{ fontWeight: 700, lineHeight: 1.2 }} noWrap>
            {agent.name}
          </Typography>
          <Typography variant="caption" color="text.secondary" noWrap sx={{ display: 'block' }}>
            {agent.role}
          </Typography>
        </Box>
      </Stack>
    </Tooltip>
  );
}

export function FeatureGrid() {
  const gates = mockProject.gates;
  const roster = AGENTS.slice(0, 6);
  return (
    <Box>
      <Typography variant="h5" sx={{ fontWeight: 700, mb: 2 }}>Active agents</Typography>
      <Box
        sx={{
          display: 'grid',
          gap: 1.5,
          gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', lg: 'repeat(3, 1fr)' },
        }}
      >
        {roster.map((agent) => (
          <AgentChip key={agent.id} agent={agent} status={agentStatus(agent, gates)} />
        ))}
      </Box>
    </Box>
  );
}
