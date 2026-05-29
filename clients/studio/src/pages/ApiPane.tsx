import { useMemo, useState } from 'react';
import { Box, Button, Card, Chip, IconButton, Stack, TextField, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { useQueryClient } from '@tanstack/react-query';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { toast } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { PaneHeader } from '../components/operate/PaneHeader';

interface ApiKey { id: string; name: string; prefix: string; scopes: string[]; lastUsedAt: string | null; createdAt: string; revoked: boolean }
interface Endpoint { method: string; path: string; description: string; auth: string }
interface Webhook { id: string; url: string; events: string[]; enabled: boolean; createdAt: string }

const SAMPLE_KEYS: ApiKey[] = [
  { id: 'k1', name: 'Production', prefix: 'ifk_live_a3f', scopes: ['read', 'write'], lastUsedAt: new Date(Date.now() - 36e5).toISOString(), createdAt: '', revoked: false },
  { id: 'k2', name: 'CI pipeline', prefix: 'ifk_live_9b1', scopes: ['read'], lastUsedAt: null, createdAt: '', revoked: false },
];
const SAMPLE_EPS: Endpoint[] = [
  { method: 'GET', path: '/api/health', description: 'Liveness probe', auth: 'none' },
  { method: 'POST', path: '/api/auth/login', description: 'Issue a session', auth: 'none' },
  { method: 'GET', path: '/api/products', description: 'List products', auth: 'api_key' },
];
const SAMPLE_HOOKS: Webhook[] = [{ id: 'w1', url: 'https://crm.acme.com/hooks/orders', events: ['order.created'], enabled: true, createdAt: '' }];

function methodColor(t: Theme, m: string): string {
  switch (m) {
    case 'GET': return t.palette.success.main;
    case 'POST': return t.brand.accent.primary;
    case 'DELETE': return t.palette.error.main;
    default: return t.palette.warning.main;
  }
}

export function ApiPane() {
  const t = useTheme();
  const request = useRequest();
  const qc = useQueryClient();
  const liveProjectId = useOperateProjectId();
  const [busy, setBusy] = useState(false);
  const [keyName, setKeyName] = useState('');
  const [freshSecret, setFreshSecret] = useState<string | null>(null);

  const { data: keys, isLive } = useGraphQLQuery<ApiKey[], { appApiKeys: ApiKey[] }>({
    key: ['app-api-keys', liveProjectId ?? 'none'], operationName: 'AppApiKeys', query: operations.APP_API_KEYS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE_KEYS, enabled: !!liveProjectId, map: (r) => r.appApiKeys ?? SAMPLE_KEYS,
  });
  const { data: endpoints } = useGraphQLQuery<Endpoint[], { appEndpoints: Endpoint[] }>({
    key: ['app-endpoints', liveProjectId ?? 'none'], operationName: 'AppEndpoints', query: operations.APP_ENDPOINTS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE_EPS, enabled: !!liveProjectId, map: (r) => (r.appEndpoints?.length ? r.appEndpoints : SAMPLE_EPS),
  });
  const { data: hooks } = useGraphQLQuery<Webhook[], { appWebhooks: Webhook[] }>({
    key: ['app-webhooks', liveProjectId ?? 'none'], operationName: 'AppWebhooks', query: operations.APP_WEBHOOKS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE_HOOKS, enabled: !!liveProjectId, map: (r) => r.appWebhooks ?? SAMPLE_HOOKS,
  });

  const refresh = () => { void qc.invalidateQueries({ queryKey: ['app-api-keys', liveProjectId ?? 'none'] }); void qc.invalidateQueries({ queryKey: ['app-webhooks', liveProjectId ?? 'none'] }); };
  const createKey = async () => {
    if (!keyName.trim() || !liveProjectId) return;
    if (!request) { toast('Connect the orchestrator to issue keys.', 'error'); return; }
    setBusy(true);
    try {
      const res = await request<{ createAppApiKey: { secret: string } }>('CreateAppApiKey', operations.CREATE_APP_API_KEY, { input: { projectID: liveProjectId, name: keyName.trim(), scopes: ['read', 'write'] } });
      setFreshSecret(res.createAppApiKey.secret);
      setKeyName(''); refresh();
    } catch (e) { toast(e instanceof Error ? e.message : 'Key creation failed.', 'error'); }
    finally { setBusy(false); }
  };
  const revoke = async (k: ApiKey) => {
    if (!request) return;
    setBusy(true);
    try { await request('RevokeAppApiKey', operations.REVOKE_APP_API_KEY, { id: k.id }); refresh(); toast('Key revoked.', 'success'); }
    catch (e) { toast(e instanceof Error ? e.message : 'Revoke failed.', 'error'); }
    finally { setBusy(false); }
  };
  const copy = (s: string) => { void navigator.clipboard?.writeText(s); toast('Copied.', 'success'); };

  const keyColumns = useMemo<DataGridColumn<ApiKey>[]>(() => [
    { field: 'name', headerName: 'Name', flex: 1, minWidth: 150, cellRenderer: ({ value }: DataGridCellParams<ApiKey, string>) => <Typography sx={{ fontSize: '0.84rem' }}>{value}</Typography> },
    { field: 'prefix', headerName: 'Key', width: 180, cellRenderer: ({ value }: DataGridCellParams<ApiKey, string>) => <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.76rem', color: 'text.secondary' })}>{value}••••••</Typography> },
    { field: 'scopes', headerName: 'Scopes', width: 150, cellRenderer: ({ data }: DataGridCellParams<ApiKey>) => data ? <Stack direction="row" spacing={0.5}>{data.scopes.map((s) => <Chip key={s} size="small" label={s} sx={{ height: 18, fontSize: '0.58rem' }} />)}</Stack> : null },
    { field: 'revoked', headerName: 'Status', width: 110, cellRenderer: ({ data }: DataGridCellParams<ApiKey>) => data ? <Chip size="small" label={data.revoked ? 'revoked' : 'active'} sx={(th) => ({ height: 20, fontSize: '0.62rem', textTransform: 'uppercase', bgcolor: data.revoked ? `${th.palette.error.main}22` : `${th.palette.success.main}22`, color: data.revoked ? 'error.main' : 'success.main' })} /> : null },
    { colId: 'actions', headerName: '', width: 100, sortable: false, filter: false, cellRenderer: ({ data }: DataGridCellParams<ApiKey>) => data && !data.revoked ? <Button size="small" variant="text" color="inherit" disabled={busy} onClick={(e) => { e.stopPropagation(); void revoke(data); }}>Revoke</Button> : null },
  ], [busy]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <PaneHeader title="API" isLive={isLive} subtitle="the deployed app's API surface" />

        {freshSecret && (
          <Card sx={{ p: 2, mb: 2, borderColor: 'success.main', borderWidth: 1, borderStyle: 'solid' }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'success.main', mb: 0.75 })}>New key — copy it now, it won't be shown again</Typography>
            <Stack direction="row" alignItems="center" spacing={1}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.82rem', flex: 1, wordBreak: 'break-all' })}>{freshSecret}</Typography>
              <IconButton size="small" onClick={() => copy(freshSecret)} sx={{ color: 'text.secondary' }}><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="9" y="9" width="13" height="13" rx="2" /><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" /></svg></IconButton>
              <Button size="small" variant="outlined" color="inherit" onClick={() => setFreshSecret(null)}>Dismiss</Button>
            </Stack>
          </Card>
        )}

        <Card sx={{ p: 2.5, mb: 2 }}>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.25} alignItems={{ sm: 'center' }}>
            <TextField size="small" fullWidth label="New API key name" value={keyName} onChange={(e) => setKeyName(e.target.value)} onKeyDown={(e) => { if (e.key === 'Enter') void createKey(); }} />
            <Button variant="contained" disabled={!keyName.trim() || !liveProjectId || busy} onClick={() => void createKey()}>Create key</Button>
          </Stack>
        </Card>

        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>API keys</Typography>
        <DataGrid rows={keys} columns={keyColumns} getRowId={(row) => row.id} density="compact" emptyLabel="No keys yet." height={240} minHeight={160} />

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.3fr 1fr' }, gap: 1.5, mt: 3 }}>
          <Card sx={{ p: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Endpoints</Typography>
            <Stack spacing={0.75}>
              {endpoints.map((e) => (
                <Stack key={e.method + e.path} direction="row" alignItems="center" spacing={1}>
                  <Chip size="small" label={e.method} sx={{ height: 20, minWidth: 52, fontSize: '0.6rem', fontWeight: 700, bgcolor: `${methodColor(t, e.method)}22`, color: methodColor(t, e.method) }} />
                  <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.78rem', flex: 1 })} noWrap>{e.path}</Typography>
                  <Chip size="small" label={e.auth} sx={{ height: 18, fontSize: '0.56rem' }} />
                </Stack>
              ))}
            </Stack>
          </Card>
          <Card sx={{ p: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Webhooks</Typography>
            <Stack spacing={1}>
              {hooks.length === 0 && <Typography sx={{ fontSize: '0.8rem', color: 'text.disabled' }}>No webhooks configured.</Typography>}
              {hooks.map((w) => (
                <Box key={w.id}>
                  <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.76rem' })} noWrap>{w.url}</Typography>
                  <Stack direction="row" spacing={0.5} sx={{ mt: 0.5 }}>
                    {w.events.map((ev) => <Chip key={ev} size="small" label={ev} sx={{ height: 18, fontSize: '0.56rem' }} />)}
                    <Chip size="small" label={w.enabled ? 'on' : 'off'} sx={(th) => ({ height: 18, fontSize: '0.56rem', color: w.enabled ? 'success.main' : 'text.disabled' })} />
                  </Stack>
                </Box>
              ))}
            </Stack>
          </Card>
        </Box>
      </Box>
    </Box>
  );
}
