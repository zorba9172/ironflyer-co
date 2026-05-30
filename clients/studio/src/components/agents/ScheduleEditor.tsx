import { FormControl, FormControlLabel, InputLabel, MenuItem, Select, Stack, Switch, TextField, Typography } from '@mui/material';
import { scheduleLabel, SCHEDULE_TRIGGERS, WEEKDAY_OPTIONS, type AgentSchedule, type AgentScheduleMode } from '../../studioData';
import { text } from '@ironflyer/design-tokens/brand';

const MODE_OPTIONS: { value: AgentScheduleMode; label: string }[] = [
  { value: 'manual', label: 'Manual — only when dispatched' },
  { value: 'interval', label: 'On an interval' },
  { value: 'daily', label: 'Daily' },
  { value: 'weekly', label: 'Weekly' },
  { value: 'on_event', label: 'On a project event' },
];

// When and how an agent or crew runs. Shared by the agent and crew builders.
export function ScheduleEditor({ schedule, onChange }: { schedule: AgentSchedule; onChange: (s: AgentSchedule) => void }) {
  const patch = (p: Partial<AgentSchedule>) => onChange({ ...schedule, ...p });
  return (
    <Stack spacing={1.5}>
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography sx={{ fontSize: text.s86, color: 'text.secondary' }}>{scheduleLabel(schedule)}</Typography>
        <FormControlLabel
          control={<Switch size="small" checked={schedule.enabled} onChange={(e) => patch({ enabled: e.target.checked })} />}
          label={<Typography sx={{ fontSize: text.s82, color: 'text.secondary' }}>{schedule.enabled ? 'Enabled' : 'Paused'}</Typography>}
          sx={{ mr: 0 }}
        />
      </Stack>
      <FormControl fullWidth size="small">
        <InputLabel id="sched-mode-label">Runs</InputLabel>
        <Select labelId="sched-mode-label" label="Runs" value={schedule.mode} onChange={(e) => patch({ mode: e.target.value as AgentScheduleMode })}>
          {MODE_OPTIONS.map((m) => <MenuItem key={m.value} value={m.value}>{m.label}</MenuItem>)}
        </Select>
      </FormControl>
      <Stack direction="row" spacing={1.5}>
        {schedule.mode === 'interval' && (
          <TextField label="Every" value={schedule.every ?? ''} onChange={(e) => patch({ every: e.target.value })} size="small" placeholder="6h" sx={{ flex: 1 }} />
        )}
        {(schedule.mode === 'daily' || schedule.mode === 'weekly') && (
          <TextField label="At" type="time" value={schedule.at ?? '09:00'} onChange={(e) => patch({ at: e.target.value })} size="small" sx={{ flex: 1 }} slotProps={{ inputLabel: { shrink: true } }} />
        )}
        {schedule.mode === 'weekly' && (
          <FormControl size="small" sx={{ flex: 1 }}>
            <InputLabel id="sched-weekday-label">Day</InputLabel>
            <Select labelId="sched-weekday-label" label="Day" value={schedule.weekday ?? 1} onChange={(e) => patch({ weekday: Number(e.target.value) })}>
              {WEEKDAY_OPTIONS.map((d) => <MenuItem key={d.value} value={d.value}>{d.label}</MenuItem>)}
            </Select>
          </FormControl>
        )}
        {schedule.mode === 'on_event' && (
          <FormControl size="small" sx={{ flex: 1 }}>
            <InputLabel id="sched-trigger-label">Trigger</InputLabel>
            <Select labelId="sched-trigger-label" label="Trigger" value={schedule.trigger ?? 'gate_blocked'} onChange={(e) => patch({ trigger: e.target.value as AgentSchedule['trigger'] })}>
              {SCHEDULE_TRIGGERS.map((t) => <MenuItem key={t.value} value={t.value}>{t.label}</MenuItem>)}
            </Select>
          </FormControl>
        )}
      </Stack>
    </Stack>
  );
}
