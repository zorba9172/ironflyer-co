import { useMemo } from 'react';
import { Box, Button, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { Chart, type EChartsOption } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { useOperateMutation } from '../hooks/useOperateMutation';
import { PaneHeader } from '../components/operate/PaneHeader';
import { text } from '@ironflyer/design-tokens/brand';

interface EndUser { id: string; email: string; name: string; role: string; status: string; provider: string; lastSeenAt: string | null; createdAt: string }
interface RoleCount { role: string; count: number }
interface UserStats { total: number; active7d: number; newThisWeek: number; suspended: number; byRole: RoleCount[] }

const SAMPLE_USERS: EndUser[] = [
  { id: 'eu_1', email: 'maya.cohen@example.com', name: 'Maya Cohen', role: 'admin', status: 'active', provider: 'google', lastSeenAt: new Date(Date.now() - 36e5).toISOString(), createdAt: new Date(Date.now() - 90 * 864e5).toISOString() },
  { id: 'eu_2', email: 'liam.park@example.com', name: 'Liam Park', role: 'member', status: 'active', provider: 'email', lastSeenAt: new Date(Date.now() - 8 * 36e5).toISOString(), createdAt: new Date(Date.now() - 40 * 864e5).toISOString() },
  { id: 'eu_3', email: 'noa.levi@example.com', name: 'Noa Levi', role: 'viewer', status: 'suspended', provider: 'github', lastSeenAt: null, createdAt: new Date(Date.now() - 12 * 864e5).toISOString() },
];
const SAMPLE_STATS: UserStats = { total: 1284, active7d: 412, newThisWeek: 38, suspended: 6, byRole: [{ role: 'member', count: 980 }, { role: 'viewer', count: 240 }, { role: 'admin', count: 64 }] };
const ROLES = ['admin', 'member', 'viewer'];

function statusColor(t: Theme, s: string): string {
  if (s === 'active') return t.palette.success.main;
  if (s === 'suspended') return t.palette.error.main;
  return t.palette.warning.main;
}
const fmtWhen = (iso: string | null) => (iso ? new Date(iso).toLocaleDateString() : '—');

export function UsersPane() {
  const t = useTheme();
  const liveProjectId = useOperateProjectId();
  const { busy, run } = useOperateMutation();

  const { data: users, isLive } = useGraphQLQuery<EndUser[], { appEndUsers: EndUser[] }>({
    key: ['app-users', liveProjectId ?? 'none'],
    operationName: 'AppEndUsers', query: operations.APP_END_USERS,
    variables: { projectID: liveProjectId, limit: 200, offset: 0 }, fallbackData: SAMPLE_USERS, enabled: !!liveProjectId,
    map: (r) => r.appEndUsers ?? [],
  });
  const { data: stats } = useGraphQLQuery<UserStats, { appUserStats: UserStats }>({
    key: ['app-user-stats', liveProjectId ?? 'none'],
    operationName: 'AppUserStats', query: operations.APP_USER_STATS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE_STATS, enabled: !!liveProjectId,
    map: (r) => r.appUserStats ?? SAMPLE_STATS,
  });

  const invalidate = [['app-users', liveProjectId ?? 'none'], ['app-user-stats', liveProjectId ?? 'none']];
  const cycleRole = (u: EndUser) => {
    const next = ROLES[(ROLES.indexOf(u.role) + 1) % ROLES.length];
    void run('Role updated', (req) => req('SetAppUserRole', operations.SET_APP_USER_ROLE, { projectID: liveProjectId, userID: u.id, role: next }), invalidate);
  };
  const toggleSuspend = (u: EndUser) => void run(u.status === 'suspended' ? 'Restored' : 'Suspended', (req) => req('SetAppUserSuspended', operations.SET_APP_USER_SUSPENDED, { projectID: liveProjectId, userID: u.id, suspended: u.status !== 'suspended' }), invalidate);

  const roleDonut = useMemo<EChartsOption>(() => ({
    tooltip: { trigger: 'item' },
    legend: { bottom: 0, textStyle: { color: t.palette.text.secondary, fontSize: 11 } },
    series: [{
      type: 'pie', radius: ['58%', '80%'],
      itemStyle: { borderColor: t.palette.background.paper, borderWidth: 2 },
      label: { show: true, position: 'center', formatter: `${stats.total.toLocaleString()}\nusers`, color: t.palette.text.primary, fontSize: 20, lineHeight: 20 },
      data: stats.byRole.map((r, i) => ({ value: r.count, name: r.role, itemStyle: { color: [t.brand.accent.primary, t.brand.accent.secondary, t.palette.warning.main, t.palette.info.main][i % 4] } })),
    }],
  }), [stats, t]);

  const metrics = [
    { label: 'Total users', value: stats.total.toLocaleString(), sub: 'all time' },
    { label: 'Active (7d)', value: stats.active7d.toLocaleString(), sub: 'seen this week' },
    { label: 'New this week', value: stats.newThisWeek.toLocaleString(), sub: 'sign-ups' },
    { label: 'Suspended', value: String(stats.suspended), sub: 'blocked accounts' },
  ];

  const columns = useMemo<DataGridColumn<EndUser>[]>(() => [
    { field: 'name', headerName: 'User', flex: 1, minWidth: 200, cellRenderer: ({ data }: DataGridCellParams<EndUser>) => data ? (
      <Stack spacing={0} sx={{ lineHeight: 1.2, py: 0.5 }}>
        <Typography sx={{ fontSize: text.s84 }} noWrap>{data.name}</Typography>
        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s68, color: 'text.secondary' })} noWrap>{data.email}</Typography>
      </Stack>
    ) : null },
    { field: 'role', headerName: 'Role', width: 110, cellRenderer: ({ data }: DataGridCellParams<EndUser>) => data ? <Chip size="small" label={data.role} sx={(th) => ({ height: 20, fontSize: text.s62, textTransform: 'uppercase', bgcolor: `${th.brand.accent.primary}1f`, color: th.brand.accent.primary })} /> : null },
    { field: 'status', headerName: 'Status', width: 116, cellRenderer: ({ data }: DataGridCellParams<EndUser>) => data ? <Chip size="small" label={data.status} sx={{ height: 20, fontSize: text.s62, textTransform: 'uppercase', bgcolor: `${statusColor(t, data.status)}22`, color: statusColor(t, data.status) }} /> : null },
    { field: 'provider', headerName: 'Provider', width: 110, cellRenderer: ({ value }: DataGridCellParams<EndUser, string>) => <Typography sx={{ fontSize: text.s80, color: 'text.secondary' }}>{value}</Typography> },
    { field: 'lastSeenAt', headerName: 'Last seen', width: 120, cellRenderer: ({ value }: DataGridCellParams<EndUser, string>) => <Typography sx={{ fontSize: text.s80, color: 'text.secondary' }}>{fmtWhen(value ?? null)}</Typography> },
    { colId: 'actions', headerName: '', width: 180, sortable: false, filter: false, cellRenderer: ({ data }: DataGridCellParams<EndUser>) => data ? (
      <Stack direction="row" spacing={0.75}>
        <Button size="small" variant="text" color="inherit" disabled={busy} onClick={(e) => { e.stopPropagation(); cycleRole(data); }}>Role</Button>
        <Button size="small" variant="outlined" color="inherit" disabled={busy} onClick={(e) => { e.stopPropagation(); toggleSuspend(data); }}>{data.status === 'suspended' ? 'Restore' : 'Suspend'}</Button>
      </Stack>
    ) : null },
  ], [t, busy, liveProjectId]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <PaneHeader title="Users" isLive={isLive} subtitle="end-users of the deployed app" />

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '300px 1fr' }, gap: 1.5, mb: 3, alignItems: 'stretch' }}>
          <Card sx={{ p: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>By role</Typography>
            <Chart option={roleDonut} height={200} />
          </Card>
          <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 1.5 }}>
            {metrics.map((m) => (
              <Card key={m.label} sx={{ p: 2.5, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled' })}>{m.label}</Typography>
                <Typography variant="h4" sx={{ fontSize: text.s180, mt: 0.5 }}>{m.value}</Typography>
                <Typography sx={{ fontSize: text.s76, color: 'text.secondary' }}>{m.sub}</Typography>
              </Card>
            ))}
          </Box>
        </Box>

        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s70, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Roster</Typography>
        <DataGrid rows={users} columns={columns} getRowId={(row) => row.id} density="compact" emptyLabel="No users yet." height={460} minHeight={260} />
      </Box>
    </Box>
  );
}
