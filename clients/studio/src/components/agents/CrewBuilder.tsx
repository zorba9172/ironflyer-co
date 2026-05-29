import { useState } from 'react';
import {
  Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle, Divider,
  FormControl, InputLabel, MenuItem, Select, Stack, TextField, Typography,
} from '@mui/material';
import { toast } from '@ironflyer/ui-web/fx';
import { CREW_PROCESSES, type Agent, type AgentSchedule, type Crew } from '../../studioData';
import { ScheduleEditor } from './ScheduleEditor';
import { ProcessDiagram } from './ProcessDiagram';

interface CrewBuilderProps {
  crew: Crew;
  agents: Agent[];
  exists: boolean;
  onClose: () => void;
  onSave: (c: Crew) => void;
}

// Compose several agents into a crew that runs together. The process decides how
// they collaborate; a live topology diagram mirrors the choice. Hierarchical
// crews additionally name a manager that plans and delegates.
export function CrewBuilder({ crew, agents, exists, onClose, onSave }: CrewBuilderProps) {
  const [draft, setDraft] = useState<Crew>(crew);
  const set = <K extends keyof Crew>(key: K, value: Crew[K]) => setDraft((d) => ({ ...d, [key]: value }));

  const nameOf = (id: string) => agents.find((a) => a.id === id)?.name ?? id;
  const valid = draft.name.trim().length > 0 && draft.goal.trim().length > 0 && draft.memberIds.length >= 1;

  const toggleMember = (id: string) => setDraft((d) => {
    const has = d.memberIds.includes(id);
    const memberIds = has ? d.memberIds.filter((x) => x !== id) : [...d.memberIds, id];
    return { ...d, memberIds, managerId: d.managerId && memberIds.includes(d.managerId) ? d.managerId : undefined };
  });

  const save = () => {
    const next: Crew = { ...draft, name: draft.name.trim(), goal: draft.goal.trim() };
    onSave(next);
    toast(`${next.name} ${exists ? 'updated' : 'created'}.`, 'success');
    onClose();
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth slotProps={{ paper: { sx: { border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}>
      <DialogTitle sx={{ fontWeight: 700, fontSize: '1.05rem' }}>{exists ? 'Edit crew' : 'New crew'}</DialogTitle>
      <DialogContent dividers>
        <Stack spacing={2.5}>
          <TextField label="Name" value={draft.name} onChange={(e) => set('name', e.target.value)} fullWidth size="small" placeholder="Launch squad" autoFocus />
          <TextField label="Goal" value={draft.goal} onChange={(e) => set('goal', e.target.value)} fullWidth size="small" placeholder="The outcome this crew owns, in one line" />

          <Box>
            <Typography sx={{ fontSize: '0.8rem', color: 'text.secondary', mb: 0.75 }}>Process</Typography>
            <FormControl fullWidth size="small">
              <Select value={draft.process} onChange={(e) => set('process', e.target.value as Crew['process'])}>
                {CREW_PROCESSES.map((p) => (
                  <MenuItem key={p.value} value={p.value}>
                    <Box>
                      <Typography sx={{ fontSize: '0.86rem' }}>{p.label}</Typography>
                      <Typography sx={{ fontSize: '0.72rem', color: 'text.disabled' }}>{p.desc}</Typography>
                    </Box>
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Box>

          <Box>
            <Typography sx={{ fontSize: '0.8rem', color: 'text.secondary', mb: 0.75 }}>Members ({draft.memberIds.length})</Typography>
            <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.75 }}>
              {agents.map((a) => {
                const on = draft.memberIds.includes(a.id);
                return (
                  <Chip
                    key={a.id}
                    label={a.name}
                    size="small"
                    onClick={() => toggleMember(a.id)}
                    variant={on ? 'filled' : 'outlined'}
                    sx={(t) => ({
                      cursor: 'pointer',
                      borderColor: on ? 'transparent' : 'divider',
                      bgcolor: on ? `${t.palette.primary.main}26` : 'transparent',
                      color: on ? 'primary.main' : 'text.secondary',
                      fontWeight: on ? 600 : 500,
                      '&:hover': { bgcolor: on ? `${t.palette.primary.main}33` : 'action.hover' },
                    })}
                  />
                );
              })}
            </Stack>
          </Box>

          {draft.process === 'hierarchical' && draft.memberIds.length > 0 && (
            <FormControl fullWidth size="small">
              <InputLabel id="manager-label">Manager</InputLabel>
              <Select labelId="manager-label" label="Manager" value={draft.managerId ?? ''} onChange={(e) => set('managerId', e.target.value || undefined)}>
                <MenuItem value=""><em>Pick a manager</em></MenuItem>
                {draft.memberIds.map((id) => <MenuItem key={id} value={id}>{nameOf(id)}</MenuItem>)}
              </Select>
            </FormControl>
          )}

          {draft.memberIds.length > 0 && (
            <Box sx={{ p: 2, borderRadius: 2, border: 1, borderColor: 'divider', bgcolor: 'background.default', overflowX: 'auto' }}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.62rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.25 })}>Topology</Typography>
              <ProcessDiagram process={draft.process} members={draft.memberIds.map(nameOf)} manager={draft.managerId ? nameOf(draft.managerId) : undefined} />
            </Box>
          )}

          <Divider />
          <Box>
            <Typography sx={{ fontWeight: 600, fontSize: '0.95rem', mb: 1.5 }}>Schedule</Typography>
            <ScheduleEditor schedule={draft.schedule ?? { mode: 'manual', enabled: true }} onChange={(s: AgentSchedule) => set('schedule', s)} />
          </Box>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={!valid} onClick={save}>{exists ? 'Save changes' : 'Create crew'}</Button>
      </DialogActions>
    </Dialog>
  );
}
