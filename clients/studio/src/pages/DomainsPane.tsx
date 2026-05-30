import { useMemo, useState } from 'react';
import { Box, Button, Chip, Stack, TextField, Tooltip, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { useOperateMutation } from '../hooks/useOperateMutation';
import { PaneHeader } from '../components/operate/PaneHeader';
import { StudioChart, donutOption, type EChartsOption } from '../components/charts';
import { StudioDataGrid, type DataGridCellParams, type DataGridColumn, type StudioTableTab } from '../components/tables';
import { GlassPanel, StatCard, SectionHeader } from '../components/studio';
import { text } from '@ironflyer/design-tokens/brand';

interface DNSRecord { type: string; name: string; value: string; ttl: number | null }
interface DeployDomain {
  id: string; hostname: string; kind: string; status: string; provider: string;
  registrar: string | null; primary: boolean; verificationStatus: string;
  certificateStatus: string; instructions: string; createdAt: string;
  verifiedAt: string | null; liveAt: string | null; dnsRecords: DNSRecord[];
}

const SAMPLE: DeployDomain[] = [
  { id: 'd_managed', hostname: 'peak-illustrious.ironflyer.app', kind: 'managed', status: 'live', provider: 'ironflyer', registrar: null, primary: true, verificationStatus: 'verified', certificateStatus: 'issued', instructions: '', createdAt: new Date(Date.now() - 864e5).toISOString(), verifiedAt: new Date(Date.now() - 864e5).toISOString(), liveAt: new Date(Date.now() - 864e5).toISOString(), dnsRecords: [] },
  { id: 'd_custom', hostname: 'app.northwind.com', kind: 'custom', status: 'pending_verification', provider: 'ironflyer', registrar: null, primary: false, verificationStatus: 'pending', certificateStatus: 'pending', instructions: 'Add the CNAME below at your DNS provider, then verify.', createdAt: new Date(Date.now() - 36e5).toISOString(), verifiedAt: null, liveAt: null, dnsRecords: [{ type: 'CNAME', name: 'app', value: 'cname.ironflyer.app', ttl: 3600 }] },
];

const isLiveStatus = (s: string) => ['live', 'active'].includes(s.toLowerCase());
const isFail = (s: string) => ['failed', 'error', 'rejected'].includes(s.toLowerCase());

function statusColor(t: Theme, s: string): string {
  if (isLiveStatus(s) || ['verified', 'issued'].includes(s.toLowerCase())) return t.palette.success.main;
  if (isFail(s)) return t.palette.error.main;
  return t.palette.warning.main;
}

const pretty = (s: string) => s.replace(/_/g, ' ');

function GlobeIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="9" />
      <path d="M12 3c-2.5 3-2.5 6 0 9s2.5 6 0 9" />
      <path d="M3 12h18" />
    </svg>
  );
}

function ShieldIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 3l7 3v5c0 5-3.4 8.4-7 10-3.6-1.6-7-5-7-10V6z" />
    </svg>
  );
}

function CopyIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
      <rect x="9" y="9" width="13" height="13" rx="2" />
      <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
    </svg>
  );
}

export function DomainsPane() {
  const t = useTheme();
  const liveProjectId = useOperateProjectId();
  const { busy, run } = useOperateMutation();
  const [hostname, setHostname] = useState('');
  const [tableView, setTableView] = useState('all');
  const [tableSearch, setTableSearch] = useState('');

  const { data: rows, isLive } = useGraphQLQuery<DeployDomain[], { deployDomains: DeployDomain[] }>({
    key: ['deploy-domains', liveProjectId ?? 'none'],
    operationName: 'DeployDomains', query: operations.DEPLOY_DOMAINS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE, enabled: !!liveProjectId,
    refetchInterval: 15000,
    map: (r) => r.deployDomains ?? [],
  });

  const invalidate = [['deploy-domains', liveProjectId ?? 'none']];
  const connect = () => {
    const h = hostname.trim();
    if (!h || !liveProjectId) return;
    void run('Connect', async (req) => {
      await req('ConnectDeployDomain', operations.CONNECT_DEPLOY_DOMAIN, { input: { projectID: liveProjectId, hostname: h, primary: false } });
      setHostname('');
    }, invalidate);
  };
  const check = (id: string) => void run('Verify', (req) => req('CheckDeployDomain', operations.CHECK_DEPLOY_DOMAIN, { id }), invalidate);
  const setPrimary = (id: string) => void run('Set primary', (req) => req('SetPrimaryDeployDomain', operations.SET_PRIMARY_DEPLOY_DOMAIN, { id }), invalidate);

  const live = rows.filter((d) => isLiveStatus(d.status)).length;
  const pending = rows.filter((d) => !isLiveStatus(d.status) && !isFail(d.status)).length;
  const failed = rows.filter((d) => isFail(d.status)).length;
  const primary = rows.find((d) => d.primary);
  const tableTabs = useMemo<StudioTableTab[]>(() => [
    { value: 'all', label: 'All', count: rows.length },
    { value: 'live', label: 'Live', count: live, tone: 'success' },
    { value: 'pending', label: 'Pending', count: pending, tone: pending > 0 ? 'warning' : 'default' },
    { value: 'failed', label: 'Failed', count: failed, tone: 'error' },
  ], [rows.length, live, pending, failed]);

  const donut = useMemo<EChartsOption>(() => {
    const donutData = [
      { value: live, name: 'Live', color: t.palette.success.main },
      { value: pending, name: 'Pending', color: t.palette.warning.main },
      { value: failed, name: 'Failed', color: t.palette.error.main },
    ].filter((d) => d.value > 0);
    const open = pending + failed;
    return donutOption(t, {
      data: donutData,
      centerLabel: open > 0 ? `${open}\nopen` : 'all\nlive',
      centerColor: open > 0 ? t.palette.warning.main : t.palette.success.main,
    });
  }, [live, pending, failed, t]);

  const columns = useMemo<DataGridColumn<DeployDomain>[]>(() => [
    {
      field: 'hostname', headerName: 'Hostname', flex: 1, minWidth: 220,
      cellRenderer: ({ data: row }: DataGridCellParams<DeployDomain>) => row ? (
        <Stack direction="row" alignItems="center" spacing={0.85}>
          <Box sx={{ color: isLiveStatus(row.status) ? 'success.main' : 'text.disabled', display: 'inline-flex', opacity: 0.7 }}><GlobeIcon /></Box>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s82 })} noWrap>{row.hostname}</Typography>
          {row.primary && (
            <Chip size="small" label="primary" sx={(th) => ({ height: 18, fontSize: text.s58, textTransform: 'uppercase', bgcolor: `${th.palette.primary.main}22`, color: 'primary.main' })} />
          )}
        </Stack>
      ) : null,
    },
    {
      field: 'kind', headerName: 'Kind', width: 100,
      cellRenderer: ({ value }: DataGridCellParams<DeployDomain, string>) => (
        <Typography sx={{ fontSize: text.s80, color: 'text.secondary' }}>{pretty(value ?? '')}</Typography>
      ),
    },
    {
      field: 'status', headerName: 'Status', width: 168,
      cellRenderer: ({ data: row }: DataGridCellParams<DeployDomain>) => row ? (
        <Chip
          size="small"
          label={pretty(row.status)}
          sx={{ height: 20, fontSize: text.s62, textTransform: 'uppercase', bgcolor: `${statusColor(t, row.status)}22`, color: statusColor(t, row.status) }}
        />
      ) : null,
    },
    {
      field: 'certificateStatus', headerName: 'TLS', width: 110,
      cellRenderer: ({ data: row }: DataGridCellParams<DeployDomain>) => row ? (
        <Stack direction="row" alignItems="center" spacing={0.5}>
          <Box sx={{ color: statusColor(t, row.certificateStatus), display: 'inline-flex', opacity: 0.8 }}><ShieldIcon /></Box>
          <Chip
            size="small"
            label={pretty(row.certificateStatus)}
            sx={{ height: 20, fontSize: text.s60, textTransform: 'uppercase', bgcolor: `${statusColor(t, row.certificateStatus)}1a`, color: statusColor(t, row.certificateStatus) }}
          />
        </Stack>
      ) : null,
    },
    {
      colId: 'actions', headerName: '', width: 184, sortable: false, filter: false,
      cellRenderer: ({ data: row }: DataGridCellParams<DeployDomain>) => row ? (
        <Stack direction="row" spacing={0.75}>
          {!isLiveStatus(row.status) && (
            <Button size="small" variant="outlined" color="inherit" disabled={busy} onClick={(e) => { e.stopPropagation(); check(row.id); }}>Verify</Button>
          )}
          {!row.primary && (
            <Button size="small" variant="text" color="inherit" disabled={busy} onClick={(e) => { e.stopPropagation(); setPrimary(row.id); }}>Primary</Button>
          )}
        </Stack>
      ) : null,
    },
  ], [t, busy]);

  const tableRows = useMemo(() => {
    const q = tableSearch.trim().toLowerCase();
    return rows.filter((row) => {
      if (tableView === 'live' && !isLiveStatus(row.status)) return false;
      if (tableView === 'pending' && (isLiveStatus(row.status) || isFail(row.status))) return false;
      if (tableView === 'failed' && !isFail(row.status)) return false;
      return !q || [row.hostname, row.status, row.provider, row.registrar ?? '', row.verificationStatus, row.certificateStatus, row.kind].some((value) => value.toLowerCase().includes(q));
    });
  }, [rows, tableSearch, tableView]);

  const pendingDns = rows.find((d) => !isLiveStatus(d.status) && d.dnsRecords.length > 0);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <PaneHeader
          title="Domains"
          isLive={isLive}
          subtitle={`${primary ? primary.hostname : 'no primary domain'}${pending > 0 ? ` · ${pending} pending verification` : ''}`}
        />

        {/* KPI strip */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(3, 1fr)' }, gap: 1.5, mb: 2.5 }}>
          <StatCard label="Live domains" value={String(live)} hint={live === rows.length ? 'All domains live' : `${rows.length - live} pending`} accent={t.palette.success.main} />
          <StatCard label="Pending verification" value={String(pending)} hint={pending > 0 ? 'DNS action required' : 'Nothing pending'} accent={pending > 0 ? t.palette.warning.main : t.palette.success.main} />
          <StatCard label="TLS certificates" value={`${rows.filter((d) => d.certificateStatus === 'issued').length}/${rows.length}`} hint="Auto-provisioned on verify" accent={t.palette.primary.main} />
        </Box>

        {/* Domain health donut + connect form */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '280px 1fr' }, gap: 2, mb: 2.5, alignItems: 'stretch' }}>
          <GlassPanel pad={2.5} accent={live === rows.length ? t.palette.success.main : t.palette.warning.main}>
            <Typography
              sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', letterSpacing: '0.12em', color: 'text.disabled', mb: 0.5 })}
            >
              Domain health
            </Typography>
            <StudioChart option={donut} height={190} />
          </GlassPanel>

          <GlassPanel pad={2.5} sx={{ display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <SectionHeader eyebrow="Custom domain" title="Connect a domain" subtitle="We issue and renew TLS certificates automatically once DNS resolves." />
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.25} alignItems={{ sm: 'center' }}>
              <TextField
                size="small" fullWidth placeholder="app.yourdomain.com" value={hostname}
                onChange={(e) => setHostname(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') connect(); }}
              />
              <Button variant="contained" disabled={!hostname.trim() || !liveProjectId || busy} onClick={connect} sx={{ whiteSpace: 'nowrap' }}>
                Connect
              </Button>
            </Stack>
          </GlassPanel>
        </Box>

        {/* Pending DNS instructions */}
        {pendingDns && (
          <GlassPanel
            pad={2.5}
            accent={t.palette.warning.main}
            sx={{ mb: 2.5, borderColor: `${t.palette.warning.main}44` }}
          >
            <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.5 }}>
              <Typography
                sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s70, textTransform: 'uppercase', letterSpacing: '0.12em', color: 'warning.main' })}
              >
                DNS records required · {pendingDns.hostname}
              </Typography>
              <Chip size="small" label="Action needed" sx={(th) => ({ height: 20, fontSize: text.s62, color: 'warning.main', bgcolor: `${th.palette.warning.main}22` })} />
            </Stack>
            <Typography sx={{ fontSize: text.s80, color: 'text.secondary', mb: 1.5 }}>{pendingDns.instructions}</Typography>
            <Stack spacing={0.75}>
              {pendingDns.dnsRecords.map((rec, i) => (
                <Stack
                  key={i}
                  direction="row"
                  spacing={2}
                  alignItems="center"
                  sx={(th) => ({
                    fontFamily: th.brand.font.mono,
                    fontSize: text.s78,
                    p: 1,
                    borderRadius: `${th.studio.radius.sm}px`,
                    bgcolor: `${th.palette.warning.main}0a`,
                    border: `1px solid ${th.palette.warning.main}22`,
                  })}
                >
                  <Box sx={{ width: 64, color: 'warning.main', fontWeight: 700 }}>{rec.type}</Box>
                  <Box sx={{ width: 120 }}>{rec.name}</Box>
                  <Box sx={{ flex: 1, color: 'text.secondary' }}>{rec.value}</Box>
                  {rec.ttl != null && <Box sx={{ color: 'text.disabled' }}>TTL {rec.ttl}</Box>}
                  <Tooltip title="Copy value">
                    <Box
                      component="button"
                      aria-label="Copy DNS value"
                      onClick={() => void navigator.clipboard.writeText(rec.value)}
                      sx={{ display: 'inline-flex', alignItems: 'center', border: 'none', bgcolor: 'transparent', cursor: 'pointer', color: 'text.disabled', p: 0.5, '&:hover': { color: 'text.primary' } }}
                    >
                      <CopyIcon />
                    </Box>
                  </Tooltip>
                </Stack>
              ))}
            </Stack>
          </GlassPanel>
        )}

        <StudioDataGrid
          title="Domains"
          subtitle="Grouped by DNS and certificate state."
          tabs={tableTabs}
          activeTab={tableView}
          onTabChange={setTableView}
          searchValue={tableSearch}
          onSearchChange={setTableSearch}
          searchPlaceholder="Search domains"
          footer="Managed subdomains go live instantly. Custom domains move pending → verified → live once DNS resolves and the certificate is issued."
          rows={tableRows} columns={columns} getRowId={(row) => row.id}
          density="compact" emptyLabel="No domains yet — connect one above." height={360} minHeight={220}
        />
      </Box>
    </Box>
  );
}
