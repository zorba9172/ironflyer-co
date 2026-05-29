import { useMemo, useState } from 'react';
import {
  Box, Button, Card, Chip, Dialog, DialogActions, DialogContent, DialogTitle, Divider,
  FormControl, FormControlLabel, IconButton, InputLabel, MenuItem, Select, Stack, Switch,
  TextField, Tooltip, Typography,
} from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import {
  AGENTS, agentStatus, newAgent, scheduleLabel, SCHEDULE_TRIGGERS, WEEKDAY_OPTIONS,
  type Agent, type AgentSchedule, type AgentScheduleMode, type AgentStatus, type StudioProject,
} from '../studioData';
import { agentColor } from '../components/statusColor';
import { useStudio } from '../store';

const statusText: Record<AgentStatus, string> = { working: 'Working', done: 'Done', blocked: 'Blocked', idle: 'Idle' };

function ClockGlyph() {
  return <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg>;
}

function AgentAvatar({ name }: { name: string }) {
  return (
    <Box sx={(t) => ({ width: 38, height: 38, borderRadius: 2, display: 'grid', placeItems: 'center', color: t.palette.primary.contrastText, backgroundImage: t.brand.gradient.signature, fontWeight: 700, flexShrink: 0 })}>
      {(name.trim()[0] ?? 'A').toUpperCase()}
    </Box>
  );
}

function SkillChips({ skills }: { skills?: string[] }) {
  if (!skills || skills.length === 0) return null;
  return (
    <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.5, mt: 1.25 }}>
      {skills.map((s) => (
        <Chip key={s} size="small" label={s} sx={{ height: 20, fontSize: '0.66rem', bgcolor: 'action.hover' }} />
      ))}
    </Stack>
  );
}

export function AgentsManagerPane({ project }: { project: StudioProject }) {
  const theme = useTheme();
  const customAgents = useStudio((s) => s.customAgents);
  const removeAgent = useStudio((s) => s.removeAgent);
  const [editing, setEditing] = useState<Agent | null>(null);

  const removeCustom = async (a: Agent) => {
    const ok = await confirmAction({ title: `Delete ${a.name}?`, text: 'This removes the agent and its schedule. The build keeps running.', confirmText: 'Delete', danger: true });
    if (ok) { removeAgent(a.id); toast(`${a.name} deleted.`, 'success'); }
  };

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="flex-start" justifyContent="space-between" sx={{ mb: 2.5, gap: 2 }}>
          <Box>
            <Typography variant="h4" sx={{ fontSize: '1.6rem', mb: 0.5 }}>Agents</Typography>
            <Typography sx={{ color: 'text.secondary' }}>
              Define what each agent does, the skills it may use, and when it runs. The orchestrator routes work to them and reports cost as they go.
            </Typography>
          </Box>
          <Button variant="contained" sx={{ flexShrink: 0 }} onClick={() => setEditing(newAgent())}>New agent</Button>
        </Stack>

        {/* Operator-created agents */}
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.25 })}>
          Your agents ({customAgents.length})
        </Typography>
        {customAgents.length === 0 ? (
          <Card sx={{ p: 4, textAlign: 'center', border: '1.5px dashed', borderColor: 'divider', mb: 4 }}>
            <Typography sx={{ color: 'text.secondary', mb: 1.5 }}>No custom agents yet. Create one — e.g. an "Einstein" research agent that runs daily and grounds the build.</Typography>
            <Button variant="outlined" color="inherit" onClick={() => setEditing(newAgent())}>Create your first agent</Button>
          </Card>
        ) : (
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 2, mb: 4 }}>
            {customAgents.map((a) => {
              const status = agentStatus(a, project.gates);
              const color = agentColor(theme, status);
              const gate = a.gateId ? project.gates.find((g) => g.id === a.gateId) : undefined;
              return (
                <Card key={a.id} sx={{ p: 2.5, display: 'flex', flexDirection: 'column' }}>
                  <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.25 }}>
                    <Stack direction="row" alignItems="center" spacing={1.5} sx={{ minWidth: 0 }}>
                      <AgentAvatar name={a.name} />
                      <Typography variant="h6" sx={{ fontSize: '1.02rem' }} noWrap>{a.name}</Typography>
                    </Stack>
                    <Chip size="small" label={statusText[status]} sx={{ height: 20, fontSize: '0.66rem', bgcolor: `${color}22`, color }} />
                  </Stack>
                  <Typography sx={{ color: 'text.secondary', fontSize: '0.88rem' }}>{a.role || 'No objective set'}</Typography>
                  {a.instructions && (
                    <Typography sx={{ fontSize: '0.8rem', color: 'text.disabled', mt: 0.75, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>{a.instructions}</Typography>
                  )}
                  <SkillChips skills={a.skills} />
                  <Box sx={{ flex: 1 }} />
                  <Stack direction="row" alignItems="center" spacing={1} sx={{ mt: 1.5 }}>
                    <Chip
                      size="small"
                      icon={<Box sx={{ display: 'flex', color: 'inherit', ml: 0.5 }}><ClockGlyph /></Box>}
                      label={scheduleLabel(a.schedule)}
                      sx={(t) => ({ height: 22, fontSize: '0.66rem', fontFamily: t.brand.font.mono, bgcolor: 'action.hover', color: a.schedule?.enabled === false ? 'text.disabled' : 'text.secondary' })}
                    />
                    {gate && <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.66rem', color: 'text.disabled' })}>owns {gate.name}</Typography>}
                  </Stack>
                  <Divider sx={{ my: 1.5 }} />
                  <Stack direction="row" spacing={1}>
                    <Button size="small" color="inherit" onClick={() => setEditing(a)}>Edit</Button>
                    <Button size="small" color="inherit" onClick={() => toast(`${a.name} dispatched — watch the board.`, 'success')}>Run now</Button>
                    <Box sx={{ flex: 1 }} />
                    <Button size="small" color="error" onClick={() => void removeCustom(a)}>Delete</Button>
                  </Stack>
                </Card>
              );
            })}
          </Box>
        )}

        {/* Built-in finisher roster */}
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.25 })}>
          Finisher roster
        </Typography>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 2 }}>
          {AGENTS.map((a) => {
            const status = agentStatus(a, project.gates);
            const color = agentColor(theme, status);
            return (
              <Tooltip key={a.id} title="Built-in agent — managed by the orchestrator" arrow>
                <Card sx={{ p: 2.5 }}>
                  <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.25 }}>
                    <Stack direction="row" alignItems="center" spacing={1.5} sx={{ minWidth: 0 }}>
                      <AgentAvatar name={a.name} />
                      <Typography variant="h6" sx={{ fontSize: '1.02rem' }} noWrap>{a.name}</Typography>
                    </Stack>
                    <Chip size="small" label={statusText[status]} sx={{ height: 20, fontSize: '0.66rem', bgcolor: `${color}22`, color }} />
                  </Stack>
                  <Typography sx={{ color: 'text.secondary', fontSize: '0.88rem' }}>{a.role}</Typography>
                  <SkillChips skills={a.skills} />
                  {a.gateId && (
                    <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', color: 'text.disabled', mt: 1.5 })}>owns the {a.gateId} gate</Typography>
                  )}
                </Card>
              </Tooltip>
            );
          })}
        </Box>
      </Box>

      {editing && (
        <AgentDialog
          agent={editing}
          gates={project.gates}
          onClose={() => setEditing(null)}
        />
      )}
    </Box>
  );
}

const MODE_OPTIONS: { value: AgentScheduleMode; label: string }[] = [
  { value: 'manual', label: 'Manual — only when dispatched' },
  { value: 'interval', label: 'On an interval' },
  { value: 'daily', label: 'Daily' },
  { value: 'weekly', label: 'Weekly' },
  { value: 'on_event', label: 'On a project event' },
];

function AgentDialog({ agent, gates, onClose }: { agent: Agent; gates: StudioProject['gates']; onClose: () => void }) {
  const addAgent = useStudio((s) => s.addAgent);
  const updateAgent = useStudio((s) => s.updateAgent);
  const exists = useStudio((s) => s.customAgents.some((a) => a.id === agent.id));

  const [name, setName] = useState(agent.name);
  const [role, setRole] = useState(agent.role);
  const [instructions, setInstructions] = useState(agent.instructions ?? '');
  const [skills, setSkills] = useState<string[]>(agent.skills ?? []);
  const [skillDraft, setSkillDraft] = useState('');
  const [gateId, setGateId] = useState(agent.gateId ?? '');
  const [schedule, setSchedule] = useState<AgentSchedule>(agent.schedule ?? { mode: 'manual', enabled: true });

  const valid = name.trim().length > 0 && role.trim().length > 0;

  const setMode = (mode: AgentScheduleMode) => setSchedule((s) => ({ ...s, mode }));
  const patchSchedule = (patch: Partial<AgentSchedule>) => setSchedule((s) => ({ ...s, ...patch }));

  const addSkill = () => {
    const v = skillDraft.trim();
    if (!v) return;
    if (!skills.includes(v)) setSkills([...skills, v]);
    setSkillDraft('');
  };

  const save = () => {
    const next: Agent = {
      ...agent,
      name: name.trim(),
      role: role.trim(),
      instructions: instructions.trim() || undefined,
      skills,
      gateId: gateId || undefined,
      schedule,
      custom: true,
    };
    if (exists) updateAgent(next); else addAgent(next);
    toast(`${next.name} ${exists ? 'updated' : 'created'}.`, 'success');
    onClose();
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth slotProps={{ paper: { sx: { border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}>
      <DialogTitle sx={{ fontWeight: 700, fontSize: '1.05rem' }}>{exists ? 'Edit agent' : 'New agent'}</DialogTitle>
      <DialogContent dividers>
        <Stack spacing={2.5}>
          <TextField label="Name" value={name} onChange={(e) => setName(e.target.value)} fullWidth size="small" placeholder="Einstein" autoFocus />
          <TextField label="Objective" value={role} onChange={(e) => setRole(e.target.value)} fullWidth size="small" placeholder="What this agent is for, in one line" />
          <TextField
            label="Instructions"
            value={instructions}
            onChange={(e) => setInstructions(e.target.value)}
            fullWidth multiline minRows={3}
            placeholder={'Exactly what the agent should do, step by step.\ne.g. Research the domain, summarize findings into a doc, and flag risks for the Security gate.'}
          />

          <Box>
            <Typography sx={{ fontSize: '0.8rem', color: 'text.secondary', mb: 0.75 }}>Skills</Typography>
            <TextField
              value={skillDraft}
              onChange={(e) => setSkillDraft(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addSkill(); } }}
              fullWidth size="small"
              placeholder="Type a skill and press Enter (e.g. research, sql, code-review)"
            />
            {skills.length > 0 && (
              <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.75, mt: 1 }}>
                {skills.map((s) => (
                  <Chip key={s} label={s} size="small" onDelete={() => setSkills(skills.filter((x) => x !== s))} sx={{ bgcolor: 'action.hover' }} />
                ))}
              </Stack>
            )}
          </Box>

          <FormControl fullWidth size="small">
            <InputLabel id="agent-gate-label">Assigned gate (optional)</InputLabel>
            <Select labelId="agent-gate-label" label="Assigned gate (optional)" value={gateId} onChange={(e) => setGateId(e.target.value)}>
              <MenuItem value=""><em>Unassigned</em></MenuItem>
              {gates.map((g) => <MenuItem key={g.id} value={g.id}>{g.no} · {g.name}</MenuItem>)}
            </Select>
          </FormControl>

          <Divider />

          <Box>
            <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.5 }}>
              <Typography sx={{ fontWeight: 600, fontSize: '0.95rem' }}>Schedule</Typography>
              <FormControlLabel
                control={<Switch size="small" checked={schedule.enabled} onChange={(e) => patchSchedule({ enabled: e.target.checked })} />}
                label={<Typography sx={{ fontSize: '0.82rem', color: 'text.secondary' }}>{schedule.enabled ? 'Enabled' : 'Paused'}</Typography>}
                sx={{ mr: 0 }}
              />
            </Stack>
            <FormControl fullWidth size="small">
              <InputLabel id="agent-mode-label">Runs</InputLabel>
              <Select labelId="agent-mode-label" label="Runs" value={schedule.mode} onChange={(e) => setMode(e.target.value as AgentScheduleMode)}>
                {MODE_OPTIONS.map((m) => <MenuItem key={m.value} value={m.value}>{m.label}</MenuItem>)}
              </Select>
            </FormControl>

            <Stack direction="row" spacing={1.5} sx={{ mt: 1.5 }}>
              {schedule.mode === 'interval' && (
                <TextField label="Every" value={schedule.every ?? ''} onChange={(e) => patchSchedule({ every: e.target.value })} size="small" placeholder="6h" sx={{ flex: 1 }} />
              )}
              {(schedule.mode === 'daily' || schedule.mode === 'weekly') && (
                <TextField label="At" type="time" value={schedule.at ?? '09:00'} onChange={(e) => patchSchedule({ at: e.target.value })} size="small" sx={{ flex: 1 }} slotProps={{ inputLabel: { shrink: true } }} />
              )}
              {schedule.mode === 'weekly' && (
                <FormControl size="small" sx={{ flex: 1 }}>
                  <InputLabel id="agent-weekday-label">Day</InputLabel>
                  <Select labelId="agent-weekday-label" label="Day" value={schedule.weekday ?? 1} onChange={(e) => patchSchedule({ weekday: Number(e.target.value) })}>
                    {WEEKDAY_OPTIONS.map((d) => <MenuItem key={d.value} value={d.value}>{d.label}</MenuItem>)}
                  </Select>
                </FormControl>
              )}
              {schedule.mode === 'on_event' && (
                <FormControl size="small" sx={{ flex: 1 }}>
                  <InputLabel id="agent-trigger-label">Trigger</InputLabel>
                  <Select labelId="agent-trigger-label" label="Trigger" value={schedule.trigger ?? 'gate_blocked'} onChange={(e) => patchSchedule({ trigger: e.target.value as AgentSchedule['trigger'] })}>
                    {SCHEDULE_TRIGGERS.map((t) => <MenuItem key={t.value} value={t.value}>{t.label}</MenuItem>)}
                  </Select>
                </FormControl>
              )}
            </Stack>
          </Box>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={!valid} onClick={save}>{exists ? 'Save changes' : 'Create agent'}</Button>
      </DialogActions>
    </Dialog>
  );
}
