import { useMemo, useState } from 'react';
import { Box, Button, Card, MenuItem, Stack, Switch, TextField, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { confirmAction } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { useOperateMutation } from '../hooks/useOperateMutation';
import { PaneHeader } from '../components/operate/PaneHeader';
import { text } from '@ironflyer/design-tokens/brand';

interface Automation { id: string; name: string; triggerKind: string; triggerConfig: string; action: string; enabled: boolean; lastRunAt: string | null; lastStatus: string; runs: number; createdAt: string; updatedAt: string }

const SAMPLE: Automation[] = [
  { id: 'a1', name: 'Welcome email', triggerKind: 'event', triggerConfig: 'user.created', action: 'send_email:welcome', enabled: true, lastRunAt: new Date(Date.now() - 36e5).toISOString(), lastStatus: 'ok', runs: 128, createdAt: '', updatedAt: '' },
  { id: 'a2', name: 'Weekly digest', triggerKind: 'cron', triggerConfig: '0 9 * * 1', action: 'send_email:digest', enabled: true, lastRunAt: new Date(Date.now() - 4 * 864e5).toISOString(), lastStatus: 'ok', runs: 12, createdAt: '', updatedAt: '' },
  { id: 'a3', name: 'Churn webhook', triggerKind: 'webhook', triggerConfig: '/hooks/churn', action: 'post:crm.sync', enabled: false, lastRunAt: null, lastStatus: 'never', runs: 0, createdAt: '', updatedAt: '' },
];
const TRIGGERS = [{ v: 'cron', l: 'Schedule (cron)' }, { v: 'event', l: 'App event' }, { v: 'webhook', l: 'Inbound webhook' }];

function statusColor(t: Theme, s: string): string {
  if (s === 'ok') return t.palette.success.main;
  if (s === 'failed' || s === 'error') return t.palette.error.main;
  return t.palette.text.disabled;
}
const triggerIcon = (k: string) => (k === 'cron' ? '⏱' : k === 'webhook' ? '🔗' : '⚡');

export function AutomationsPane() {
  const t = useTheme();
  const liveProjectId = useOperateProjectId();
  const { busy, run: mutate } = useOperateMutation();
  const [name, setName] = useState('');
  const [triggerKind, setTriggerKind] = useState('cron');
  const [triggerConfig, setTriggerConfig] = useState('');
  const [action, setAction] = useState('');

  const { data: rows, isLive } = useGraphQLQuery<Automation[], { automations: Automation[] }>({
    key: ['automations', liveProjectId ?? 'none'],
    operationName: 'Automations', query: operations.AUTOMATIONS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE, enabled: !!liveProjectId,
    map: (r) => r.automations ?? [],
  });

  const invalidate = [['automations', liveProjectId ?? 'none']];
  const create = () => {
    if (!name.trim() || !triggerConfig.trim() || !action.trim() || !liveProjectId) return;
    void mutate('Created', async (req) => {
      await req('CreateAutomation', operations.CREATE_AUTOMATION, { input: { projectID: liveProjectId, name: name.trim(), triggerKind, triggerConfig: triggerConfig.trim(), action: action.trim() } });
      setName(''); setTriggerConfig(''); setAction('');
    }, invalidate);
  };
  const toggle = (a: Automation) => void mutate('Updated', (req) => req('SetAutomationEnabled', operations.SET_AUTOMATION_ENABLED, { id: a.id, enabled: !a.enabled }), invalidate);
  const run = (a: Automation) => void mutate('Run queued', (req) => req('RunAutomation', operations.RUN_AUTOMATION, { id: a.id }), invalidate);
  const remove = async (a: Automation) => {
    const ok = await confirmAction({ title: `Delete “${a.name}”?`, text: 'This removes the automation and its schedule. This cannot be undone.', confirmText: 'Delete', danger: true });
    if (ok) void mutate('Deleted', (req) => req('DeleteAutomation', operations.DELETE_AUTOMATION, { id: a.id }), invalidate);
  };

  const active = rows.filter((a) => a.enabled).length;

  const columns = useMemo<DataGridColumn<Automation>[]>(() => [
    { field: 'name', headerName: 'Automation', flex: 1, minWidth: 200, cellRenderer: ({ data }: DataGridCellParams<Automation>) => data ? (
      <Stack direction="row" alignItems="center" spacing={1}>
        <Box sx={{ fontSize: text.s100 }}>{triggerIcon(data.triggerKind)}</Box>
        <Stack spacing={0} sx={{ lineHeight: 1.2, py: 0.5 }}>
          <Typography sx={{ fontSize: text.s84 }} noWrap>{data.name}</Typography>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, color: 'text.secondary' })} noWrap>{data.triggerKind} · {data.triggerConfig}</Typography>
        </Stack>
      </Stack>
    ) : null },
    { field: 'action', headerName: 'Action', flex: 1, minWidth: 160, cellRenderer: ({ value }: DataGridCellParams<Automation, string>) => <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s74, color: 'text.secondary' })} noWrap>{value}</Typography> },
    { field: 'lastStatus', headerName: 'Last run', width: 140, cellRenderer: ({ data }: DataGridCellParams<Automation>) => data ? (
      <Stack direction="row" alignItems="center" spacing={0.75}>
        <Box sx={{ width: 7, height: 7, borderRadius: 9, bgcolor: statusColor(t, data.lastStatus) }} />
        <Typography sx={{ fontSize: text.s78, color: 'text.secondary' }}>{data.runs} runs</Typography>
      </Stack>
    ) : null },
    { field: 'enabled', headerName: 'On', width: 70, sortable: false, filter: false, cellRenderer: ({ data }: DataGridCellParams<Automation>) => data ? <Switch size="small" checked={data.enabled} disabled={busy} onChange={() => toggle(data)} /> : null },
    { colId: 'actions', headerName: '', width: 150, sortable: false, filter: false, cellRenderer: ({ data }: DataGridCellParams<Automation>) => data ? (
      <Stack direction="row" spacing={0.5}>
        <Button size="small" variant="text" color="inherit" disabled={busy} onClick={(e) => { e.stopPropagation(); run(data); }}>Run</Button>
        <Button size="small" variant="text" color="inherit" disabled={busy} onClick={(e) => { e.stopPropagation(); void remove(data); }}>Delete</Button>
      </Stack>
    ) : null },
  ], [t, busy]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <PaneHeader title="Automations" isLive={isLive} subtitle={`${active} of ${rows.length} active · ProfitGuard meters any paid work they trigger`} />

        <Card sx={{ p: 2.5, mb: 3 }}>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>New automation</Typography>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.2fr 1fr 1.2fr 1.2fr auto' }, gap: 1.25, alignItems: 'center' }}>
            <TextField size="small" label="Name" value={name} onChange={(e) => setName(e.target.value)} />
            <TextField size="small" select label="Trigger" value={triggerKind} onChange={(e) => setTriggerKind(e.target.value)}>
              {TRIGGERS.map((tr) => <MenuItem key={tr.v} value={tr.v}>{tr.l}</MenuItem>)}
            </TextField>
            <TextField size="small" label={triggerKind === 'cron' ? 'Cron (0 9 * * 1)' : triggerKind === 'webhook' ? 'Path (/hooks/x)' : 'Event (user.created)'} value={triggerConfig} onChange={(e) => setTriggerConfig(e.target.value)} />
            <TextField size="small" label="Action (send_email:welcome)" value={action} onChange={(e) => setAction(e.target.value)} />
            <Button variant="contained" disabled={!name.trim() || !triggerConfig.trim() || !action.trim() || !liveProjectId || busy} onClick={create}>Add</Button>
          </Box>
        </Card>

        <DataGrid rows={rows} columns={columns} getRowId={(row) => row.id} density="compact" emptyLabel="No automations yet — add one above." height={420} minHeight={240} />
      </Box>
    </Box>
  );
}
