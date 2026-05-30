import { useMemo, useState } from 'react';
import { Box, Button, MenuItem, Stack, Switch, TextField, Tooltip, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { confirmAction } from '@ironflyer/ui-web/fx';
import { LuClock3, LuRadioTower, LuWebhook } from 'react-icons/lu';
import type { IconType } from 'react-icons';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { useOperateMutation } from '../hooks/useOperateMutation';
import { PaneHeader } from '../components/operate/PaneHeader';
import { StudioDataGrid, type DataGridCellParams, type DataGridColumn, type StudioTableTab } from '../components/tables';
import { StudioChart, donutOption, horizontalBarOption, type EChartsOption } from '../components/charts';
import { GlassPanel, StatCard, SectionHeader } from '../components/studio';
import { text } from '@ironflyer/design-tokens/brand';

interface Automation {
  id: string; name: string; triggerKind: string; triggerConfig: string;
  action: string; enabled: boolean; lastRunAt: string | null; lastStatus: string;
  runs: number; createdAt: string; updatedAt: string;
}

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

const TRIGGER_ICONS: Record<string, IconType> = { cron: LuClock3, webhook: LuWebhook, event: LuRadioTower };

function TriggerIcon({ kind }: { kind: string }) {
  const Icon = TRIGGER_ICONS[kind] ?? LuRadioTower;
  return <Icon size={17} strokeWidth={1.7} />;
}

function RunIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polygon points="5 3 19 12 5 21 5 3" />
    </svg>
  );
}

function TrashIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6" />
    </svg>
  );
}

export function AutomationsPane() {
  const t = useTheme();
  const liveProjectId = useOperateProjectId();
  const { busy, run: mutate } = useOperateMutation();
  const [name, setName] = useState('');
  const [triggerKind, setTriggerKind] = useState('cron');
  const [triggerConfig, setTriggerConfig] = useState('');
  const [action, setAction] = useState('');
  const [tableView, setTableView] = useState('all');
  const [tableSearch, setTableSearch] = useState('');

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
    const ok = await confirmAction({ title: `Delete "${a.name}"?`, text: 'This removes the automation and its schedule. This cannot be undone.', confirmText: 'Delete', danger: true });
    if (ok) void mutate('Deleted', (req) => req('DeleteAutomation', operations.DELETE_AUTOMATION, { id: a.id }), invalidate);
  };

  const active = rows.filter((a) => a.enabled).length;
  const inactive = rows.length - active;
  const totalRuns = rows.reduce((sum, a) => sum + a.runs, 0);
  const failed = rows.filter((a) => ['failed', 'error'].includes(a.lastStatus)).length;
  const tableTabs = useMemo<StudioTableTab[]>(() => [
    { value: 'all', label: 'All', count: rows.length },
    { value: 'active', label: 'Active', count: active, tone: 'success' },
    { value: 'paused', label: 'Paused', count: inactive, tone: inactive > 0 ? 'warning' : 'default' },
    { value: 'failed', label: 'Failed', count: failed, tone: 'error' },
  ], [rows.length, active, inactive, failed]);

  const activityDonut = useMemo<EChartsOption>(() => donutOption(t, {
    data: [
      { name: 'Active', value: active, color: t.palette.success.main },
      { name: 'Paused', value: inactive, color: t.palette.text.disabled },
    ],
    centerLabel: `${active}\nactive`,
    centerColor: active > 0 ? t.palette.success.main : t.palette.text.secondary,
  }), [active, inactive, t]);

  const triggerBar = useMemo<EChartsOption>(() => {
    const labels = TRIGGERS.map((tr) => tr.v);
    return horizontalBarOption(t, {
      labels,
      values: labels.map((label) => rows.filter((row) => row.triggerKind === label).length),
    });
  }, [rows, t]);

  const columns = useMemo<DataGridColumn<Automation>[]>(() => [
    {
      field: 'name', headerName: 'Automation', flex: 1, minWidth: 200,
      cellRenderer: ({ data: row }: DataGridCellParams<Automation>) => row ? (
        <Stack direction="row" alignItems="center" spacing={1}>
          <Box sx={(th) => ({ color: th.studio.neon.violet, display: 'inline-flex' })}>
            <TriggerIcon kind={row.triggerKind} />
          </Box>
          <Stack spacing={0} sx={{ lineHeight: 1.2, py: 0.5 }}>
            <Typography sx={{ fontSize: text.s84 }} noWrap>{row.name}</Typography>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, color: 'text.secondary' })} noWrap>
              {row.triggerKind} · {row.triggerConfig}
            </Typography>
          </Stack>
        </Stack>
      ) : null,
    },
    {
      field: 'action', headerName: 'Action', flex: 1, minWidth: 160,
      cellRenderer: ({ value }: DataGridCellParams<Automation, string>) => (
        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s74, color: 'text.secondary' })} noWrap>{value}</Typography>
      ),
    },
    {
      field: 'lastStatus', headerName: 'Last run', width: 150,
      cellRenderer: ({ data: row }: DataGridCellParams<Automation>) => row ? (
        <Stack direction="row" alignItems="center" spacing={0.75}>
          <Box sx={{ width: 7, height: 7, borderRadius: 9, bgcolor: statusColor(t, row.lastStatus) }} />
          <Typography sx={{ fontSize: text.s78, color: 'text.secondary' }}>{row.runs} runs</Typography>
          {row.lastRunAt && (
            <Typography sx={{ fontSize: text.s68, color: 'text.disabled' }}>
              · {new Date(row.lastRunAt).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
            </Typography>
          )}
        </Stack>
      ) : null,
    },
    {
      field: 'enabled', headerName: 'On', width: 70, sortable: false, filter: false,
      cellRenderer: ({ data: row }: DataGridCellParams<Automation>) => row ? (
        <Switch size="small" checked={row.enabled} disabled={busy} onChange={() => toggle(row)} />
      ) : null,
    },
    {
      colId: 'actions', headerName: '', width: 150, sortable: false, filter: false,
      cellRenderer: ({ data: row }: DataGridCellParams<Automation>) => row ? (
        <Stack direction="row" spacing={0.5} alignItems="center">
          <Tooltip title="Run now">
            <Button
              size="small" variant="text" color="inherit"
              aria-label="Run automation"
              disabled={busy}
              onClick={(e) => { e.stopPropagation(); run(row); }}
              sx={{ minWidth: 0, p: 0.75 }}
            >
              <RunIcon />
            </Button>
          </Tooltip>
          <Tooltip title="Delete">
            <Button
              size="small" variant="text" color="error"
              aria-label="Delete automation"
              disabled={busy}
              onClick={(e) => { e.stopPropagation(); void remove(row); }}
              sx={{ minWidth: 0, p: 0.75 }}
            >
              <TrashIcon />
            </Button>
          </Tooltip>
        </Stack>
      ) : null,
    },
  ], [t, busy]);

  const tableRows = useMemo(() => {
    const q = tableSearch.trim().toLowerCase();
    return rows.filter((row) => {
      if (tableView === 'active' && !row.enabled) return false;
      if (tableView === 'paused' && row.enabled) return false;
      if (tableView === 'failed' && !['failed', 'error'].includes(row.lastStatus)) return false;
      if (!q) return true;
      return [row.name, row.triggerKind, row.triggerConfig, row.action, row.lastStatus].some((value) => value.toLowerCase().includes(q));
    });
  }, [rows, tableView, tableSearch]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <PaneHeader
          title="Automations"
          isLive={isLive}
          subtitle={`${active} of ${rows.length} active · ProfitGuard meters any paid work they trigger`}
        />

        {/* KPI strip */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 1.5, mb: 2.5 }}>
          <StatCard label="Total automations" value={String(rows.length)} hint="Across all trigger types" accent={t.studio.neon.violet} />
          <StatCard label="Active" value={String(active)} hint={active > 0 ? 'Running on schedule' : 'None active'} accent={t.studio.neon.success} />
          <StatCard label="Total runs" value={String(totalRuns)} hint="Lifetime executions" accent={t.studio.neon.blue} />
          <StatCard label="Last status" value={rows.find((a) => a.lastStatus === 'failed' || a.lastStatus === 'error') ? 'Errors' : 'OK'} hint="Most recent execution" accent={rows.find((a) => a.lastStatus === 'failed') ? t.palette.error.main : t.studio.neon.success} />
        </Box>

        {/* Donut + trigger distribution */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '260px 1fr' }, gap: 2, mb: 2.5, alignItems: 'stretch' }}>
          <GlassPanel pad={2.5} accent={active > 0 ? t.studio.neon.success : t.palette.text.disabled}>
            <Typography
              sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', letterSpacing: '0.12em', color: 'text.disabled', mb: 0.5 })}
            >
              Automation state
            </Typography>
            <StudioChart option={activityDonut} height={190} />
          </GlassPanel>

          <GlassPanel pad={2.5}>
            <SectionHeader eyebrow="Triggers" title="Triggers by type" subtitle="How your automations are fired — schedule, events, and webhooks." />
            <StudioChart option={triggerBar} height={170} />
          </GlassPanel>
        </Box>

        {/* New automation form */}
        <GlassPanel pad={2.5} sx={{ mb: 2.5 }}>
          <SectionHeader eyebrow="Create" title="New automation" subtitle="Configure a trigger, schedule, or event then attach a typed action." />
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.2fr 1fr 1.2fr 1.2fr auto' }, gap: 1.25, alignItems: 'flex-end' }}>
            <TextField size="small" label="Name" value={name} onChange={(e) => setName(e.target.value)} />
            <TextField size="small" select label="Trigger" value={triggerKind} onChange={(e) => setTriggerKind(e.target.value)}>
              {TRIGGERS.map((tr) => <MenuItem key={tr.v} value={tr.v}>{tr.l}</MenuItem>)}
            </TextField>
            <TextField
              size="small"
              label={triggerKind === 'cron' ? 'Cron expression (0 9 * * 1)' : triggerKind === 'webhook' ? 'Path (/hooks/x)' : 'Event (user.created)'}
              value={triggerConfig}
              onChange={(e) => setTriggerConfig(e.target.value)}
            />
            <TextField size="small" label="Action (send_email:welcome)" value={action} onChange={(e) => setAction(e.target.value)} />
            <Button
              variant="contained"
              disabled={!name.trim() || !triggerConfig.trim() || !action.trim() || !liveProjectId || busy}
              onClick={create}
            >
              Add
            </Button>
          </Box>
        </GlassPanel>

        <StudioDataGrid
          title="Automations"
          subtitle="Organized by execution state. Review triggers before enabling long-running work."
          tabs={tableTabs}
          activeTab={tableView}
          onTabChange={setTableView}
          searchValue={tableSearch}
          onSearchChange={setTableSearch}
          searchPlaceholder="Search automations"
          footer="ProfitGuard gates any execution that would consume wallet balance — automations that trigger AI work require sufficient headroom."
          rows={tableRows} columns={columns} getRowId={(row) => row.id}
          density="compact" emptyLabel="No automations yet — add one above." height={420} minHeight={240}
        />
      </Box>
    </Box>
  );
}
