import { useMemo, useState } from 'react';
import { Box, Button, Chip, Stack, Typography, useMediaQuery } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { useOperateMutation } from '../hooks/useOperateMutation';
import { PaneHeader } from '../components/operate/PaneHeader';
import { Icon } from '../icons';
import { StudioChart, donutOption, type EChartsOption } from '../components/charts';
import { StudioDataGrid, type DataGridCellParams, type DataGridColumn, type StudioTableTab } from '../components/tables';
import { GlassPanel, StatCard } from '../components/studio';
import { text } from '@ironflyer/design-tokens/brand';

interface EndUser { id: string; email: string; name: string; role: string; status: string; provider: string; lastSeenAt: string | null; createdAt: string }
interface RoleCount { role: string; count: number }
interface UserStats { total: number; active7d: number; newThisWeek: number; suspended: number; byRole: RoleCount[] }

const SAMPLE_USERS: EndUser[] = [
  { id: 'eu_1', email: 'maya.cohen@example.com', name: 'Maya Cohen', role: 'admin', status: 'active', provider: 'google', lastSeenAt: new Date(Date.now() - 36e5).toISOString(), createdAt: new Date(Date.now() - 90 * 864e5).toISOString() },
  { id: 'eu_2', email: 'liam.park@example.com', name: 'Liam Park', role: 'member', status: 'active', provider: 'email', lastSeenAt: new Date(Date.now() - 8 * 36e5).toISOString(), createdAt: new Date(Date.now() - 40 * 864e5).toISOString() },
  { id: 'eu_3', email: 'noa.levi@example.com', name: 'Noa Levi', role: 'viewer', status: 'suspended', provider: 'github', lastSeenAt: null, createdAt: new Date(Date.now() - 12 * 864e5).toISOString() },
];
const SAMPLE_STATS: UserStats = {
  total: 1284, active7d: 412, newThisWeek: 38, suspended: 6,
  byRole: [{ role: 'member', count: 980 }, { role: 'viewer', count: 240 }, { role: 'admin', count: 64 }],
};
const ROLES = ['admin', 'member', 'viewer'];

function statusColor(t: Theme, s: string): string {
  if (s === 'active') return t.palette.success.main;
  if (s === 'suspended') return t.palette.error.main;
  return t.palette.warning.main;
}

const fmtWhen = (iso: string | null) => (iso ? new Date(iso).toLocaleDateString() : '—');

export function UsersPane() {
  const t = useTheme();
  const compactSummary = useMediaQuery(t.breakpoints.down('sm'));
  const liveProjectId = useOperateProjectId();
  const { busy, run } = useOperateMutation();
  const [tableView, setTableView] = useState('all');
  const [tableSearch, setTableSearch] = useState('');

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

  const toggleSuspend = (u: EndUser) =>
    void run(
      u.status === 'suspended' ? 'Restored' : 'Suspended',
      (req) => req('SetAppUserSuspended', operations.SET_APP_USER_SUSPENDED, { projectID: liveProjectId, userID: u.id, suspended: u.status !== 'suspended' }),
      invalidate,
    );

  // ── Role breakdown donut (mirrors stats.byRole, data-bound) ──────────────
  const roleDonut = useMemo<EChartsOption>(() => donutOption(t, {
    centerLabel: `${stats.total.toLocaleString()}\nusers`,
    centerColor: t.palette.text.primary,
    data: stats.byRole.map((r) => ({ value: r.count, name: r.role })),
  }), [stats, t]);

  // ── Roster grid columns ───────────────────────────────────────────────────
  const columns = useMemo<DataGridColumn<EndUser>[]>(() => [
    {
      field: 'name', headerName: 'User', flex: 1, minWidth: 200,
      cellRenderer: ({ data: row }: DataGridCellParams<EndUser>) => row ? (
        <Stack spacing={0} sx={{ lineHeight: 1.2, py: 0.5 }}>
          <Typography sx={{ fontSize: text.s84, fontWeight: 600 }} noWrap>{row.name}</Typography>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s68, color: 'text.secondary' })} noWrap>{row.email}</Typography>
        </Stack>
      ) : null,
    },
    {
      field: 'role', headerName: 'Role', width: 110,
      cellRenderer: ({ data: row }: DataGridCellParams<EndUser>) => row ? (
        <Chip
          size="small"
          label={row.role}
          sx={(th) => ({ height: 20, fontSize: text.s62, textTransform: 'uppercase', bgcolor: `${th.brand.accent.primary}1f`, color: th.brand.accent.primary })}
        />
      ) : null,
    },
    {
      field: 'status', headerName: 'Status', width: 116,
      cellRenderer: ({ data: row }: DataGridCellParams<EndUser>) => row ? (
        <Chip
          size="small"
          label={row.status}
          sx={{ height: 20, fontSize: text.s62, textTransform: 'uppercase', bgcolor: `${statusColor(t, row.status)}22`, color: statusColor(t, row.status) }}
        />
      ) : null,
    },
    {
      field: 'provider', headerName: 'Provider', width: 110,
      cellRenderer: ({ value }: DataGridCellParams<EndUser, string>) => (
        <Typography sx={{ fontSize: text.s80, color: 'text.secondary' }}>{value}</Typography>
      ),
    },
    {
      field: 'lastSeenAt', headerName: 'Last seen', width: 120,
      cellRenderer: ({ value }: DataGridCellParams<EndUser, string>) => (
        <Typography sx={{ fontSize: text.s80, color: 'text.secondary' }}>{fmtWhen(value ?? null)}</Typography>
      ),
    },
    {
      colId: 'actions', headerName: '', width: 180, sortable: false, filter: false,
      cellRenderer: ({ data: row }: DataGridCellParams<EndUser>) => row ? (
        <Stack direction="row" spacing={0.75}>
          <Button size="small" variant="text" color="inherit" disabled={busy} onClick={(e) => { e.stopPropagation(); cycleRole(row); }}>
            Role
          </Button>
          <Button size="small" variant="outlined" color="inherit" disabled={busy} onClick={(e) => { e.stopPropagation(); toggleSuspend(row); }}>
            {row.status === 'suspended' ? 'Restore' : 'Suspend'}
          </Button>
        </Stack>
      ) : null,
    },
  ], [t, busy, liveProjectId]); // eslint-disable-line react-hooks/exhaustive-deps

  const tableTabs = useMemo<StudioTableTab[]>(() => [
    { value: 'all', label: 'All', count: users.length },
    { value: 'active', label: 'Active', count: users.filter((u) => u.status === 'active').length, tone: 'success' },
    { value: 'suspended', label: 'Suspended', count: users.filter((u) => u.status === 'suspended').length, tone: 'error' },
    { value: 'admins', label: 'Admins', count: users.filter((u) => u.role === 'admin').length, tone: 'info' },
  ], [users]);

  const tableRows = useMemo(() => {
    const q = tableSearch.trim().toLowerCase();
    return users.filter((user) => {
      if (tableView === 'active' && user.status !== 'active') return false;
      if (tableView === 'suspended' && user.status !== 'suspended') return false;
      if (tableView === 'admins' && user.role !== 'admin') return false;
      if (!q) return true;
      return [user.name, user.email, user.role, user.status, user.provider].some((value) => value.toLowerCase().includes(q));
    });
  }, [users, tableView, tableSearch]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: { xs: 2, md: 3 } }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <PaneHeader title="Users" isLive={isLive} subtitle="end-users of the deployed app" />

        {/* ── Visual lead: role donut + four StatCards ── */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '280px 1fr' }, gap: { xs: 1.5, md: 2 }, mb: { xs: 2, md: 3 }, alignItems: 'stretch' }}>
          {/* Role breakdown donut — mirrors stats.byRole */}
          <GlassPanel accent={t.palette.primary.main} pad={2} sx={{ minHeight: { xs: 184, md: 0 } }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>
              By role
            </Typography>
            <StudioChart option={roleDonut} height={compactSummary ? 146 : 200} />
          </GlassPanel>

          {/* Four KPI stat cards */}
          <Box
            sx={(theme) => ({
              display: 'grid',
              gridTemplateColumns: { xs: 'repeat(4, minmax(136px, 1fr))', sm: '1fr 1fr' },
              gap: { xs: 1, md: 2 },
              overflowX: { xs: 'auto', sm: 'visible' },
              pb: { xs: 0.25, sm: 0 },
              scrollSnapType: { xs: 'x proximity', sm: 'none' },
              scrollbarWidth: 'thin',
              '&::-webkit-scrollbar': { height: 8 },
              '&::-webkit-scrollbar-thumb': {
                bgcolor: theme.palette.divider,
                borderRadius: 999,
              },
            })}
          >
            <StatCard
              label="Total users"
              value={stats.total.toLocaleString()}
              hint="all time"
              accent={t.palette.primary.main}
              icon={<Icon name="users" size={16} />}
              sx={{ scrollSnapAlign: 'start', p: { xs: 1.75, sm: 2.5 } }}
            />
            <StatCard
              label="Active (7d)"
              value={stats.active7d.toLocaleString()}
              hint="seen this week"
              accent={t.studio.neon.success}
              icon={<Icon name="activity" size={16} />}
              sx={{ scrollSnapAlign: 'start', p: { xs: 1.75, sm: 2.5 } }}
            />
            <StatCard
              label="New this week"
              value={stats.newThisWeek.toLocaleString()}
              hint="sign-ups"
              accent={t.palette.primary.main}
              icon={<Icon name="add" size={16} />}
              sx={{ scrollSnapAlign: 'start', p: { xs: 1.75, sm: 2.5 } }}
            />
            <StatCard
              label="Suspended"
              value={String(stats.suspended)}
              hint="blocked accounts"
              accent={t.palette.error.main}
              icon={<Icon name="close" size={16} />}
              sx={{ scrollSnapAlign: 'start', p: { xs: 1.75, sm: 2.5 } }}
            />
          </Box>
        </Box>

        <StudioDataGrid
          title="Members"
          subtitle={`${users.length.toLocaleString()} loaded · use Role / Suspend to manage access`}
          tabs={tableTabs}
          activeTab={tableView}
          onTabChange={setTableView}
          searchValue={tableSearch}
          onSearchChange={setTableSearch}
          searchPlaceholder="Search members"
          footer="Role and suspension changes are routed through the app-user mutation layer."
          rows={tableRows}
          columns={columns}
          getRowId={(row) => row.id}
          density="compact"
          emptyLabel="No users yet."
          height={460}
          minHeight={260}
        />
      </Box>
    </Box>
  );
}
