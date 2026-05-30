import { useMemo, useState } from 'react';
import { Box, Button, Chip, IconButton, Stack, Switch, TextField, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { useQueryClient } from '@tanstack/react-query';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { PaneHeader } from '../components/operate/PaneHeader';
import { StudioDataGrid, type DataGridCellParams, type DataGridColumn, type StudioTableTab } from '../components/tables';
import { StudioChart, donutOption, horizontalBarOption, type EChartsOption } from '../components/charts';
import { GlassPanel, SectionHeader } from '../components/studio';
import { text } from '@ironflyer/design-tokens/brand';

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

const methodOrder = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'] as const;

const fmtRelative = (iso: string | null): string => {
  if (!iso) return 'Never';
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60e3);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
};

// Inline copy icon — no external dep
function CopyIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <rect x="9" y="9" width="13" height="13" rx="2" />
      <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
    </svg>
  );
}

function TrashIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
      <path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" />
    </svg>
  );
}

export function ApiPane() {
  const t = useTheme();
  const request = useRequest();
  const qc = useQueryClient();
  const liveProjectId = useOperateProjectId();
  const [busy, setBusy] = useState(false);
  const [keyName, setKeyName] = useState('');
  const [freshSecret, setFreshSecret] = useState<string | null>(null);
  const [hookUrl, setHookUrl] = useState('');
  const [hookEvents, setHookEvents] = useState('');
  const [keyView, setKeyView] = useState('active');
  const [keySearch, setKeySearch] = useState('');

  const { data: keys, isLive } = useGraphQLQuery<ApiKey[], { appApiKeys: ApiKey[] }>({
    key: ['app-api-keys', liveProjectId ?? 'none'], operationName: 'AppApiKeys', query: operations.APP_API_KEYS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE_KEYS, enabled: !!liveProjectId, map: (r) => r.appApiKeys ?? [],
  });
  const { data: endpoints } = useGraphQLQuery<Endpoint[], { appEndpoints: Endpoint[] }>({
    key: ['app-endpoints', liveProjectId ?? 'none'], operationName: 'AppEndpoints', query: operations.APP_ENDPOINTS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE_EPS, enabled: !!liveProjectId, map: (r) => r.appEndpoints ?? [],
  });
  const { data: hooks } = useGraphQLQuery<Webhook[], { appWebhooks: Webhook[] }>({
    key: ['app-webhooks', liveProjectId ?? 'none'], operationName: 'AppWebhooks', query: operations.APP_WEBHOOKS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE_HOOKS, enabled: !!liveProjectId, map: (r) => r.appWebhooks ?? [],
  });

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['app-api-keys', liveProjectId ?? 'none'] });
    void qc.invalidateQueries({ queryKey: ['app-webhooks', liveProjectId ?? 'none'] });
  };

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

  const refreshHooks = () => { void qc.invalidateQueries({ queryKey: ['app-webhooks', liveProjectId ?? 'none'] }); };

  const createHook = async () => {
    if (!hookUrl.trim() || !hookEvents.trim() || !liveProjectId) return;
    if (!request) { toast('Connect the orchestrator to add webhooks.', 'error'); return; }
    const events = hookEvents.split(',').map((e) => e.trim()).filter(Boolean);
    if (events.length === 0) { toast('Add at least one event (comma-separated).', 'error'); return; }
    setBusy(true);
    try {
      await request('CreateAppWebhook', operations.CREATE_APP_WEBHOOK, { input: { projectID: liveProjectId, url: hookUrl.trim(), events } });
      setHookUrl(''); setHookEvents(''); refreshHooks(); toast('Webhook added.', 'success');
    } catch (e) { toast(e instanceof Error ? e.message : 'Could not add webhook.', 'error'); }
    finally { setBusy(false); }
  };

  const toggleHook = async (w: Webhook) => {
    if (!request) return;
    setBusy(true);
    try { await request('SetAppWebhookEnabled', operations.SET_APP_WEBHOOK_ENABLED, { id: w.id, enabled: !w.enabled }); refreshHooks(); }
    catch (e) { toast(e instanceof Error ? e.message : 'Update failed.', 'error'); }
    finally { setBusy(false); }
  };

  const deleteHook = async (w: Webhook) => {
    if (!request) return;
    const ok = await confirmAction({ title: 'Delete webhook?', text: w.url, confirmText: 'Delete', danger: true });
    if (!ok) return;
    setBusy(true);
    try { await request('DeleteAppWebhook', operations.DELETE_APP_WEBHOOK, { id: w.id }); refreshHooks(); toast('Webhook deleted.', 'success'); }
    catch (e) { toast(e instanceof Error ? e.message : 'Delete failed.', 'error'); }
    finally { setBusy(false); }
  };

  const keyColumns = useMemo<DataGridColumn<ApiKey>[]>(() => [
    {
      field: 'name', headerName: 'Name', flex: 1, minWidth: 150,
      cellRenderer: ({ value }: DataGridCellParams<ApiKey, string>) => (
        <Typography sx={{ fontSize: text.s84, fontWeight: 600 }}>{value}</Typography>
      ),
    },
    {
      field: 'prefix', headerName: 'Key', width: 200,
      cellRenderer: ({ value }: DataGridCellParams<ApiKey, string>) => (
        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s76, color: 'text.secondary' })}>{value}••••••</Typography>
      ),
    },
    {
      field: 'scopes', headerName: 'Scopes', width: 160,
      cellRenderer: ({ data }: DataGridCellParams<ApiKey>) => data ? (
        <Stack direction="row" spacing={0.5}>
          {data.scopes.map((s) => (
            <Chip key={s} size="small" label={s} sx={(th) => ({ height: 18, fontSize: text.s58, bgcolor: `${th.studio.neon.blue}1a`, color: 'text.secondary' })} />
          ))}
        </Stack>
      ) : null,
    },
    {
      field: 'lastUsedAt', headerName: 'Last used', width: 120,
      cellRenderer: ({ value }: DataGridCellParams<ApiKey, string>) => (
        <Typography sx={{ fontSize: text.s76, color: 'text.secondary' }}>{fmtRelative(value ?? null)}</Typography>
      ),
    },
    {
      field: 'revoked', headerName: 'Status', width: 110,
      cellRenderer: ({ data }: DataGridCellParams<ApiKey>) => data ? (
        <Chip
          size="small"
          label={data.revoked ? 'revoked' : 'active'}
          sx={(th) => ({
            height: 20, fontSize: text.s62, textTransform: 'uppercase',
            bgcolor: data.revoked ? `${th.palette.error.main}22` : `${th.palette.success.main}22`,
            color: data.revoked ? 'error.main' : 'success.main',
          })}
        />
      ) : null,
    },
    {
      colId: 'actions', headerName: '', width: 100, sortable: false, filter: false,
      cellRenderer: ({ data }: DataGridCellParams<ApiKey>) => data && !data.revoked ? (
        <Button size="small" variant="text" color="inherit" disabled={busy} onClick={(e) => { e.stopPropagation(); void revoke(data); }}>
          Revoke
        </Button>
      ) : null,
    },
  ], [busy]); // eslint-disable-line react-hooks/exhaustive-deps

  const keyTabs = useMemo<StudioTableTab[]>(() => {
    const active = keys.filter((key) => !key.revoked).length;
    const revoked = keys.length - active;
    return [
      { value: 'active', label: 'Active', count: active, tone: 'success' },
      { value: 'revoked', label: 'Revoked', count: revoked, tone: revoked > 0 ? 'error' : 'default' },
      { value: 'all', label: 'All', count: keys.length },
    ];
  }, [keys]);

  const visibleKeys = useMemo(() => {
    const q = keySearch.trim().toLowerCase();
    return keys.filter((key) => {
      if (keyView === 'active' && key.revoked) return false;
      if (keyView === 'revoked' && !key.revoked) return false;
      if (!q) return true;
      return [key.name, key.prefix, key.scopes.join(' ')].some((value) => value.toLowerCase().includes(q));
    });
  }, [keys, keyView, keySearch]);

  // ── Usage charts ──────────────────────────────────────────────────────────
  const keyHealthChart = useMemo<EChartsOption>(() => {
    const active = keys.filter((k) => !k.revoked).length;
    const revoked = keys.length - active;
    return donutOption(t, {
      data: [
        { name: 'Active', value: active, color: t.palette.success.main },
        { name: 'Revoked', value: revoked, color: t.palette.error.main },
      ],
      centerLabel: `${active}\nactive`,
      centerColor: active > 0 ? t.palette.success.main : t.palette.text.secondary,
    });
  }, [keys, t]);

  const endpointMethodsChart = useMemo<EChartsOption>(() => {
    const counts = methodOrder
      .map((method) => ({ method, count: endpoints.filter((ep) => ep.method === method).length }))
      .filter((item) => item.count > 0);
    const rows = counts.length ? counts : [{ method: 'GET', count: 0 }];
    return horizontalBarOption(t, {
      labels: rows.map((item) => item.method),
      values: rows.map((item) => item.count),
      colors: rows.map((item) => methodColor(t, item.method)),
    });
  }, [endpoints, t]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <PaneHeader title="API" isLive={isLive} subtitle="the deployed app's API surface" />

        {/* ── Visual lead: key health + endpoint method breakdown ── */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '280px 1fr' }, gap: 2, mb: 3, alignItems: 'stretch' }}>
          <GlassPanel accent={t.studio.neon.success} pad={2}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>
              Key health
            </Typography>
            <StudioChart option={keyHealthChart} height={200} />
          </GlassPanel>
          <GlassPanel accent={t.studio.neon.blue} pad={2}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>
              Endpoints by method
            </Typography>
            <StudioChart option={endpointMethodsChart} height={200} />
          </GlassPanel>
        </Box>

        {/* ── Fresh key banner ── */}
        {freshSecret && (
          <GlassPanel accent={t.palette.success.main} pad={2} sx={{ mb: 2.5 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'success.main', mb: 0.75 })}>
              New key — copy it now, it won't be shown again
            </Typography>
            <Stack direction="row" alignItems="center" spacing={1}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s82, flex: 1, wordBreak: 'break-all' })}>{freshSecret}</Typography>
              <IconButton size="small" onClick={() => copy(freshSecret)} sx={{ color: 'text.secondary' }}><CopyIcon /></IconButton>
              <Button size="small" variant="outlined" color="inherit" onClick={() => setFreshSecret(null)}>Dismiss</Button>
            </Stack>
          </GlassPanel>
        )}

        {/* ── Create key form ── */}
        <GlassPanel pad={2.5} sx={{ mb: 2.5 }}>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.25} alignItems={{ sm: 'center' }}>
            <TextField
              size="small" fullWidth label="New API key name"
              value={keyName} onChange={(e) => setKeyName(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') void createKey(); }}
            />
            <Button variant="contained" disabled={!keyName.trim() || !liveProjectId || busy} onClick={() => void createKey()}>
              Create key
            </Button>
          </Stack>
        </GlassPanel>

        <StudioDataGrid
          title="API keys"
          subtitle="Keys are grouped by lifecycle. Copy new secrets immediately."
          tabs={keyTabs}
          activeTab={keyView}
          onTabChange={setKeyView}
          searchValue={keySearch}
          onSearchChange={setKeySearch}
          searchPlaceholder="Search keys"
          footer="Revoked keys stay visible for audit history but cannot be used for requests."
          rows={visibleKeys} columns={keyColumns} getRowId={(row) => row.id}
          density="compact" emptyLabel="No keys yet." height={240} minHeight={160}
        />

        {/* ── Endpoints + Webhooks ── */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.3fr 1fr' }, gap: 2, mt: 3 }}>
          {/* Endpoints */}
          <GlassPanel accent={t.studio.neon.violet} pad={2.5}>
            <SectionHeader eyebrow="Routes" title="Endpoints" />
            <Stack spacing={0.75}>
              {endpoints.map((ep) => (
                <Stack
                  key={ep.method + ep.path}
                  direction="row" alignItems="center" spacing={1}
                  sx={(th) => ({
                    p: 1, borderRadius: `${th.studio.radius.sm}px`,
                    border: `1px solid ${th.palette.cardBorder}`,
                    bgcolor: 'cardBg',
                    transition: `background-color ${th.studio.motion.fast}`,
                    '&:hover': { bgcolor: 'surfaceHover' },
                  })}
                >
                  <Chip
                    size="small"
                    label={ep.method}
                    sx={{ height: 22, minWidth: 54, fontSize: text.s62, fontWeight: 700, bgcolor: `${methodColor(t, ep.method)}22`, color: methodColor(t, ep.method) }}
                  />
                  <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s78, flex: 1 })} noWrap>{ep.path}</Typography>
                  {ep.auth !== 'none' && (
                    <Chip size="small" label={ep.auth} sx={(th) => ({ height: 18, fontSize: text.s56, bgcolor: `${th.studio.neon.violet}1a`, color: 'text.secondary' })} />
                  )}
                </Stack>
              ))}
              {endpoints.length === 0 && (
                <Typography sx={{ fontSize: text.s80, color: 'text.disabled' }}>No endpoints detected yet.</Typography>
              )}
            </Stack>
          </GlassPanel>

          {/* Webhooks */}
          <GlassPanel accent={t.studio.neon.pink} pad={2.5}>
            <SectionHeader eyebrow="Push events" title="Webhooks" />
            <Stack spacing={1} sx={{ mb: 2 }}>
              <TextField size="small" label="Endpoint URL" placeholder="https://crm.acme.com/hooks/orders" value={hookUrl} onChange={(e) => setHookUrl(e.target.value)} />
              <Stack direction="row" spacing={1}>
                <TextField
                  size="small" fullWidth label="Events (comma-separated)"
                  placeholder="order.created, user.created" value={hookEvents}
                  onChange={(e) => setHookEvents(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') void createHook(); }}
                />
                <Button
                  variant="contained"
                  disabled={!hookUrl.trim() || !hookEvents.trim() || !liveProjectId || busy}
                  onClick={() => void createHook()}
                >
                  Add
                </Button>
              </Stack>
            </Stack>
            <Stack spacing={1}>
              {hooks.length === 0 && (
                <Typography sx={{ fontSize: text.s80, color: 'text.disabled' }}>No webhooks configured.</Typography>
              )}
              {hooks.map((w) => (
                <Box
                  key={w.id}
                  sx={(th) => ({
                    p: 1.25,
                    borderRadius: `${th.studio.radius.sm}px`,
                    border: `1px solid ${th.palette.cardBorder}`,
                    bgcolor: 'cardBg',
                  })}
                >
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s76, flex: 1, minWidth: 0 })} noWrap>{w.url}</Typography>
                    <Switch size="small" checked={w.enabled} disabled={busy} onChange={() => void toggleHook(w)} />
                    <IconButton size="small" aria-label="Delete webhook" disabled={busy} onClick={() => void deleteHook(w)} sx={{ color: 'text.disabled' }}>
                      <TrashIcon />
                    </IconButton>
                  </Stack>
                  <Stack direction="row" spacing={0.5} sx={{ mt: 0.75 }}>
                    {w.events.map((ev) => (
                      <Chip key={ev} size="small" label={ev} sx={(th) => ({ height: 18, fontSize: text.s56, bgcolor: `${th.studio.neon.pink}18`, color: 'text.secondary' })} />
                    ))}
                  </Stack>
                </Box>
              ))}
            </Stack>
          </GlassPanel>
        </Box>
      </Box>
    </Box>
  );
}
