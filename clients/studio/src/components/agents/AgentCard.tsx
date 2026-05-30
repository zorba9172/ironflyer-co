import { Box, Button, Card, Chip, Divider, Stack, Tooltip, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { Icon } from '../../icons';
import { agentStatus, scheduleLabel, type Agent, type AgentStatus, type Gate } from '../../studioData';
import { autonomyLabel, skillLabel } from '../../agentLibrary';
import { agentColor } from '../statusColor';
import { text } from '@ironflyer/design-tokens/brand';

const statusText: Record<AgentStatus, string> = { working: 'Working', done: 'Done', blocked: 'Blocked', idle: 'Idle' };

function Avatar({ name }: { name: string }) {
  return (
    <Box sx={(t) => ({ width: 38, height: 38, borderRadius: 2, display: 'grid', placeItems: 'center', color: t.palette.primary.contrastText, backgroundImage: t.studio.gradient.signature, fontWeight: t.typography.fontWeightBold, flexShrink: 0 })}>
      {(name.trim()[0] ?? 'A').toUpperCase()}
    </Box>
  );
}

function MetaChips({ skills }: { skills?: string[] }) {
  if (!skills || skills.length === 0) return null;
  const shown = skills.slice(0, 4);
  const extra = skills.length - shown.length;
  return (
    <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.5, mt: 1.25 }}>
      {shown.map((s) => (
        <Chip key={s} size="small" label={skillLabel(s)} sx={{ height: 20, fontSize: text.s66, bgcolor: 'action.hover' }} />
      ))}
      {extra > 0 && <Chip size="small" label={`+${extra}`} sx={{ height: 20, fontSize: text.s66, bgcolor: 'action.hover', color: 'text.secondary' }} />}
    </Stack>
  );
}

interface AgentCardProps {
  agent: Agent;
  gates: Gate[];
  /** built-in roster agents are read-only — managed by the orchestrator */
  builtIn?: boolean;
  onEdit?: (a: Agent) => void;
  onRun?: (a: Agent) => void;
  onDelete?: (a: Agent) => void;
}

// One agent as a glanceable card: status, objective, skills, schedule, the gate
// it owns, and (for custom agents) edit / run / delete. Built-in roster cards
// are read-only with a tooltip. Shared by the catalog and the editor pane.
export function AgentCard({ agent, gates, builtIn, onEdit, onRun, onDelete }: AgentCardProps) {
  const theme = useTheme();
  const status = agentStatus(agent, gates);
  const color = agentColor(theme, status);
  const gate = agent.gateId ? gates.find((g) => g.id === agent.gateId) : undefined;

  const card = (
    <Card sx={{ p: 2.5, display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.25 }}>
        <Stack direction="row" alignItems="center" spacing={1.5} sx={{ minWidth: 0 }}>
          <Avatar name={agent.name || 'A'} />
          <Box sx={{ minWidth: 0 }}>
            <Typography variant="h6" sx={{ fontSize: text.s102 }} noWrap>{agent.name || 'Untitled agent'}</Typography>
            {agent.autonomy && (
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s62, color: 'text.disabled', textTransform: 'uppercase', letterSpacing: '0.08em' })}>
                {autonomyLabel(agent.autonomy)}
              </Typography>
            )}
          </Box>
        </Stack>
        <Chip size="small" label={statusText[status]} sx={{ height: 20, fontSize: text.s66, bgcolor: `${color}22`, color }} />
      </Stack>

      <Typography sx={{ color: 'text.secondary', fontSize: text.s88 }}>{agent.role || 'No objective set'}</Typography>
      {agent.description && (
        <Typography sx={{ fontSize: text.s80, color: 'text.disabled', mt: 0.75, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>{agent.description}</Typography>
      )}

      <MetaChips skills={agent.skills} />

      <Box sx={{ flex: 1 }} />

      <Stack direction="row" alignItems="center" spacing={1} sx={{ mt: 1.5, flexWrap: 'wrap', gap: 0.75 }}>
        {agent.custom && (
          <Chip
            size="small"
            icon={<Box sx={{ display: 'flex', color: 'inherit', ml: 0.5 }}><Icon name="clock" size={12} /></Box>}
            label={scheduleLabel(agent.schedule)}
            sx={(t) => ({ height: 22, fontSize: text.s66, fontFamily: t.brand.font.mono, bgcolor: 'action.hover', color: agent.schedule?.enabled === false ? 'text.disabled' : 'text.secondary' })}
          />
        )}
        {(agent.tools?.length ?? 0) > 0 && (
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s66, color: 'text.disabled' })}>{agent.tools!.length} tools</Typography>
        )}
        {gate && <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s66, color: 'text.disabled' })}>owns {gate.name}</Typography>}
      </Stack>

      {!builtIn && (
        <>
          <Divider sx={{ my: 1.5 }} />
          <Stack direction="row" spacing={1}>
            <Button size="small" color="inherit" onClick={() => onEdit?.(agent)}>Edit</Button>
            <Button size="small" color="inherit" onClick={() => onRun?.(agent)}>Run now</Button>
            <Box sx={{ flex: 1 }} />
            <Button size="small" color="error" onClick={() => onDelete?.(agent)}>Delete</Button>
          </Stack>
        </>
      )}
    </Card>
  );

  if (builtIn) {
    return <Tooltip title="Built-in agent — managed by the orchestrator" arrow>{card}</Tooltip>;
  }
  return card;
}
