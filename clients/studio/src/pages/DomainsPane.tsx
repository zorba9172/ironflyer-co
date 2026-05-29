import { useMemo, useState } from 'react';
import { Box, Button, Card, Chip, Stack, TextField, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { Chart, type EChartsOption } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { useOperateMutation } from '../hooks/useOperateMutation';
import { PaneHeader } from '../components/operate/PaneHeader';

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

export function DomainsPane() {
  const t = useTheme();
  const liveProjectId = useOperateProjectId();
  const { busy, run } = useOperateMutation();
  const [hostname, setHostname] = useState('');

  const { data: rows, isLive } = useGraphQLQuery<DeployDomain[], { deployDomains: DeployDomain[] }>({
    key: ['deploy-domains', liveProjectId ?? 'none'],
    operationName: 'DeployDomains', query: operations.DEPLOY_DOMAINS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE, enabled: !!liveProjectId,
    refetchInterval: 15000,
    map: (r) => (r.deployDomains?.length ? r.deployDomains : SAMPLE),
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

  const donut = useMemo<EChartsOption>(() => {
    const data = [
      { value: live, name: 'Live', itemStyle: { color: t.palette.success.main } },
      { value: pending, name: 'Pending', itemStyle: { color: t.palette.warning.main } },
      { value: failed, name: 'Failed', itemStyle: { color: t.palette.error.main } },
    ].filter((d) => d.value > 0);
    const open = pending + failed;
    return {
      tooltip: { trigger: 'item' },
      legend: { bottom: 0, textStyle: { color: t.palette.text.secondary, fontSize: 11 } },
      series: [{
        type: 'pie', radius: ['58%', '80%'], avoidLabelOverlap: true,
        itemStyle: { borderColor: t.palette.background.paper, borderWidth: 2 },
        label: { show: true, position: 'center', formatter: open > 0 ? `${open}\nopen` : 'all\nlive', color: open > 0 ? t.palette.warning.main : t.palette.success.main, fontSize: 22, lineHeight: 22 },
        data,
      }],
    };
  }, [live, pending, failed, t]);

  const columns = useMemo<DataGridColumn<DeployDomain>[]>(() => [
    { field: 'hostname', headerName: 'Hostname', flex: 1, minWidth: 220, cellRenderer: ({ data }: DataGridCellParams<DeployDomain>) => data ? (
      <Stack direction="row" alignItems="center" spacing={0.85}>
        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.82rem' })} noWrap>{data.hostname}</Typography>
        {data.primary && <Chip size="small" label="primary" sx={(th) => ({ height: 18, fontSize: '0.58rem', textTransform: 'uppercase', bgcolor: `${th.palette.primary.main}22`, color: 'primary.main' })} />}
      </Stack>
    ) : null },
    { field: 'kind', headerName: 'Kind', width: 100, cellRenderer: ({ value }: DataGridCellParams<DeployDomain, string>) => <Typography sx={{ fontSize: '0.8rem', color: 'text.secondary' }}>{pretty(value ?? '')}</Typography> },
    { field: 'status', headerName: 'Status', width: 168, cellRenderer: ({ data }: DataGridCellParams<DeployDomain>) => data ? <Chip size="small" label={pretty(data.status)} sx={{ height: 20, fontSize: '0.62rem', textTransform: 'uppercase', bgcolor: `${statusColor(t, data.status)}22`, color: statusColor(t, data.status) }} /> : null },
    { field: 'certificateStatus', headerName: 'TLS', width: 110, cellRenderer: ({ data }: DataGridCellParams<DeployDomain>) => data ? <Chip size="small" label={pretty(data.certificateStatus)} sx={{ height: 20, fontSize: '0.6rem', textTransform: 'uppercase', bgcolor: `${statusColor(t, data.certificateStatus)}1a`, color: statusColor(t, data.certificateStatus) }} /> : null },
    { colId: 'actions', headerName: '', width: 184, sortable: false, filter: false, cellRenderer: ({ data }: DataGridCellParams<DeployDomain>) => data ? (
      <Stack direction="row" spacing={0.75}>
        {!isLiveStatus(data.status) && <Button size="small" variant="outlined" color="inherit" disabled={busy} onClick={(e) => { e.stopPropagation(); check(data.id); }}>Verify</Button>}
        {!data.primary && <Button size="small" variant="text" color="inherit" disabled={busy} onClick={(e) => { e.stopPropagation(); setPrimary(data.id); }}>Primary</Button>}
      </Stack>
    ) : null },
  ], [t, busy]);

  const pendingDns = rows.find((d) => !isLiveStatus(d.status) && d.dnsRecords.length > 0);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <PaneHeader title="Domains" isLive={isLive} subtitle={`${primary ? primary.hostname : 'no primary domain'}${pending > 0 ? ` · ${pending} pending verification` : ''}`} />

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '300px 1fr' }, gap: 1.5, mb: 3, alignItems: 'stretch' }}>
          <Card sx={{ p: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Domain health</Typography>
            <Chart option={donut} height={200} />
          </Card>
          <Card sx={{ p: 2.5, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Connect a custom domain</Typography>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.25} alignItems={{ sm: 'center' }}>
              <TextField
                size="small" fullWidth placeholder="app.yourdomain.com" value={hostname}
                onChange={(e) => setHostname(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') connect(); }}
              />
              <Button variant="contained" disabled={!hostname.trim() || !liveProjectId || busy} onClick={connect}>Connect</Button>
            </Stack>
            <Typography sx={{ fontSize: '0.76rem', color: 'text.disabled', mt: 1 }}>We issue the TLS certificate automatically once the DNS records below resolve.</Typography>
          </Card>
        </Box>

        {pendingDns && (
          <Card sx={{ p: 2, mb: 3, borderColor: 'warning.main', borderWidth: 1, borderStyle: 'solid' }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'warning.main', mb: 1 })}>DNS records · {pendingDns.hostname}</Typography>
            <Stack spacing={0.75}>
              {pendingDns.dnsRecords.map((d, i) => (
                <Stack key={i} direction="row" spacing={2} sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.78rem' })}>
                  <Box sx={{ width: 64, color: 'text.secondary' }}>{d.type}</Box>
                  <Box sx={{ width: 120 }}>{d.name}</Box>
                  <Box sx={{ flex: 1, color: 'text.secondary' }} component="span">{d.value}</Box>
                  {d.ttl != null && <Box sx={{ color: 'text.disabled' }}>TTL {d.ttl}</Box>}
                </Stack>
              ))}
            </Stack>
          </Card>
        )}

        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>All domains</Typography>
        <DataGrid
          rows={rows} columns={columns} getRowId={(row) => row.id}
          density="compact" emptyLabel="No domains yet — connect one above." height={360} minHeight={220}
        />
        <Typography sx={{ fontSize: '0.76rem', color: 'text.disabled', mt: 1.5 }}>Managed subdomains go live instantly; custom domains move pending → verified → live once DNS resolves and the certificate is issued.</Typography>
      </Box>
    </Box>
  );
}
