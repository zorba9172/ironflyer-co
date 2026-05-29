import { Box, Button, Card, Chip, Divider, Stack, Tooltip, Typography } from '@mui/material';
import { crewProcessLabel, scheduleLabel, type Agent, type Crew } from '../../studioData';
import { ProcessDiagram } from './ProcessDiagram';

function MemberAvatar({ name, manager }: { name: string; manager?: boolean }) {
  return (
    <Tooltip title={manager ? `${name} · manager` : name} arrow>
      <Box sx={(t) => ({ width: 28, height: 28, borderRadius: '50%', display: 'grid', placeItems: 'center', fontSize: '0.7rem', fontWeight: 700, color: t.palette.primary.contrastText, backgroundImage: t.brand.gradient.signature, border: manager ? `2px solid ${t.palette.warning.main}` : 'none', marginLeft: '-6px', boxShadow: `0 0 0 2px ${t.palette.background.paper}` })}>
        {(name.trim()[0] ?? 'A').toUpperCase()}
      </Box>
    </Tooltip>
  );
}

export function CrewCard({ crew, agents, onEdit, onRun, onDelete }: {
  crew: Crew;
  agents: Agent[];
  onEdit?: (c: Crew) => void;
  onRun?: (c: Crew) => void;
  onDelete?: (c: Crew) => void;
}) {
  const nameOf = (id: string) => agents.find((a) => a.id === id)?.name ?? id;
  const memberNames = crew.memberIds.map(nameOf);

  return (
    <Card sx={{ p: 2.5, display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
        <Typography variant="h6" sx={{ fontSize: '1.02rem' }} noWrap>{crew.name || 'Untitled crew'}</Typography>
        <Chip size="small" label={crewProcessLabel(crew.process)} sx={(t) => ({ height: 20, fontSize: '0.66rem', fontFamily: t.brand.font.mono, bgcolor: `${t.palette.primary.main}1f`, color: 'primary.main' })} />
      </Stack>
      <Typography sx={{ color: 'text.secondary', fontSize: '0.86rem', mb: 1.25 }}>{crew.goal || 'No goal set'}</Typography>

      <Stack direction="row" alignItems="center" sx={{ pl: '6px', mb: 1.5 }}>
        {memberNames.slice(0, 6).map((n, i) => (
          <MemberAvatar key={`${crew.memberIds[i]}`} name={n} manager={crew.process === 'hierarchical' && crew.memberIds[i] === crew.managerId} />
        ))}
        {memberNames.length > 6 && <Typography sx={{ fontSize: '0.72rem', color: 'text.disabled', ml: 1 }}>+{memberNames.length - 6}</Typography>}
        {memberNames.length === 0 && <Typography sx={{ fontSize: '0.78rem', color: 'text.disabled' }}>No members</Typography>}
      </Stack>

      {memberNames.length > 0 && (
        <Box sx={{ p: 1.25, borderRadius: 2, border: 1, borderColor: 'divider', bgcolor: 'background.default', overflowX: 'auto', mb: 1 }}>
          <ProcessDiagram process={crew.process} members={memberNames} manager={crew.managerId ? nameOf(crew.managerId) : undefined} dense />
        </Box>
      )}

      <Box sx={{ flex: 1 }} />
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.66rem', color: 'text.disabled', mt: 0.5 })}>
        {crew.memberIds.length} {crew.memberIds.length === 1 ? 'agent' : 'agents'} · {scheduleLabel(crew.schedule)}
      </Typography>

      <Divider sx={{ my: 1.5 }} />
      <Stack direction="row" spacing={1}>
        <Button size="small" color="inherit" onClick={() => onEdit?.(crew)}>Edit</Button>
        <Button size="small" color="inherit" onClick={() => onRun?.(crew)}>Run crew</Button>
        <Box sx={{ flex: 1 }} />
        <Button size="small" color="error" onClick={() => onDelete?.(crew)}>Delete</Button>
      </Stack>
    </Card>
  );
}
